// +build !darwin,!nucular_gio nucular_shiny

package nucular

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/aarzilli/nucular/clipboard"
	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/font"
	"github.com/aarzilli/nucular/label"
	"github.com/aarzilli/nucular/rect"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"

	"github.com/golang/freetype/raster"

	ifont "golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/hashicorp/golang-lru"
)

//go:generate go-bindata -o internal/assets/assets.go -pkg assets DroidSansMono.ttf

var clipboardStarted bool = false
var clipboardMu sync.Mutex

type masterWindow struct {
	masterWindowCommon

	Title  string
	screen screen.Screen
	wnd    screen.Window
	wndb   screen.Buffer
	bounds image.Rectangle

	initialSize image.Point

	// window is focused
	Focus bool

	textbuffer bytes.Buffer

	closing     bool
	focusedOnce bool
}

// Creates new master window
func NewMasterWindowSize(flags WindowFlags, title string, sz image.Point, updatefn UpdateFn) MasterWindow {
	ctx := &context{}
	wnd := &masterWindow{}

	wnd.masterWindowCommonInit(ctx, flags, updatefn, wnd)

	wnd.Title = title
	wnd.initialSize = sz

	clipboardMu.Lock()
	if !clipboardStarted {
		clipboardStarted = true
		clipboard.Start()
	}
	clipboardMu.Unlock()

	return wnd
}

// Shows window, runs event loop
func (mw *masterWindow) Main() {
	driver.Main(mw.main)
}

func (mw *masterWindow) Lock() {
	mw.uilock.Lock()
}

func (mw *masterWindow) Unlock() {
	mw.uilock.Unlock()
}

func (mw *masterWindow) main(s screen.Screen) {
	var err error
	mw.screen = s
	width, height := mw.ctx.scale(mw.initialSize.X), mw.ctx.scale(mw.initialSize.Y)
	mw.wnd, err = s.NewWindow(&screen.NewWindowOptions{width, height, mw.Title})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create window: %v", err)
		return
	}
	mw.setupBuffer(image.Point{width, height})
	mw.Changed()

	go mw.updater()

	for {
		ei := mw.wnd.NextEvent()
		mw.uilock.Lock()
		r := mw.handleEventLocked(ei)
		mw.uilock.Unlock()
		if !r {
			break
		}
	}
}

func (w *masterWindow) handleEventLocked(ei interface{}) bool {
	switch e := ei.(type) {
	case paint.Event:
		// On darwin we must respond to a paint.Event by reuploading the buffer or
		// the appplication will freeze.
		// On windows when the window goes off screen part of the window contents
		// will be discarded and must be redrawn.
		w.prevCmds = w.prevCmds[:0]
		w.updateLocked()

	case lifecycle.Event:
		if e.Crosses(lifecycle.StageDead) == lifecycle.CrossOn || e.To == lifecycle.StageDead || w.closing {
			w.closing = true
			w.closeLocked()
			return false
		}
		c := false
		switch cross := e.Crosses(lifecycle.StageFocused); cross {
		case lifecycle.CrossOn:
			if !w.focusedOnce {
				// on linux uploads that happen before this event don't get displayed
				// for some reason, force a reupload
				w.focusedOnce = true
				w.prevCmds = w.prevCmds[:0]
			}
			w.Focus = true
			c = true
		case lifecycle.CrossOff:
			w.Focus = false
			c = true
		}
		if c {
			if changed := atomic.LoadInt32(&w.ctx.changed); changed < 2 {
				atomic.StoreInt32(&w.ctx.changed, 2)
			}
		}
	case size.Event:
		sz := e.Size()
		bb := w.wndb.Bounds()
		if sz.X <= bb.Dx() && sz.Y <= bb.Dy() {
			w.bounds = w.wndb.Bounds()
			w.bounds.Max.Y = w.bounds.Min.Y + sz.Y
			w.bounds.Max.X = w.bounds.Min.X + sz.X
		} else {
			if w.wndb != nil {
				w.wndb.Release()
			}
			w.setupBuffer(sz)
		}
		w.prevCmds = w.prevCmds[:0]
		if changed := atomic.LoadInt32(&w.ctx.changed); changed < 2 {
			atomic.StoreInt32(&w.ctx.changed, 2)
		}

	case mouse.Event:
		changed := atomic.LoadInt32(&w.ctx.changed)
		if changed < 2 {
			atomic.StoreInt32(&w.ctx.changed, 2)
		}
		switch e.Direction {
		case mouse.DirStep:
			switch e.Button {
			case mouse.ButtonWheelUp:
				w.ctx.Input.Mouse.ScrollDelta++
			case mouse.ButtonWheelDown:
				w.ctx.Input.Mouse.ScrollDelta--
			}
		case mouse.DirPress, mouse.DirRelease:
			down := e.Direction == mouse.DirPress

			if e.Button >= 0 && int(e.Button) < len(w.ctx.Input.Mouse.Buttons) {
				btn := &w.ctx.Input.Mouse.Buttons[e.Button]
				if btn.Down == down {
					break
				}

				if down {
					btn.ClickedPos.X = int(e.X)
					btn.ClickedPos.Y = int(e.Y)
				}
				btn.Clicked = true
				btn.Down = down
			}
		case mouse.DirNone:
			w.ctx.Input.Mouse.Pos.X = int(e.X)
			w.ctx.Input.Mouse.Pos.Y = int(e.Y)
			w.ctx.Input.Mouse.Delta = w.ctx.Input.Mouse.Pos.Sub(w.ctx.Input.Mouse.Prev)
		}

	case key.Event:
		changed := atomic.LoadInt32(&w.ctx.changed)
		if changed < 2 {
			atomic.StoreInt32(&w.ctx.changed, 2)
		}
		w.ctx.processKeyEvent(e, &w.textbuffer)
	}

	return true
}

func (w *masterWindow) updater() {
	var down bool
	for {
		if down {
			time.Sleep(10 * time.Millisecond)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
		func() {
			w.uilock.Lock()
			defer w.uilock.Unlock()
			if w.closing {
				return
			}
			changed := atomic.LoadInt32(&w.ctx.changed)
			if changed > 0 {
				atomic.AddInt32(&w.ctx.changed, -1)
				w.updateLocked()
			} else {
				down = false
				for _, btn := range w.ctx.Input.Mouse.Buttons {
					if btn.Down {
						down = true
					}
				}
				if down {
					w.updateLocked()
				}
			}
		}()
	}
}

func (w *masterWindow) updateLocked() {
	w.ctx.Windows[0].Bounds = rect.FromRectangle(w.bounds)
	in := &w.ctx.Input
	in.Mouse.clip = nk_null_rect
	in.Keyboard.Text = w.textbuffer.String()
	w.textbuffer.Reset()

	var t0, t1, te time.Time
	if perfUpdate || w.Perf {
		t0 = time.Now()
	}

	if dumpFrame && !perfUpdate {
		panic("dumpFrame")
	}

	w.ctx.Update()

	if perfUpdate || w.Perf {
		t1 = time.Now()
	}
	nprimitives := w.draw()
	if perfUpdate && nprimitives > 0 {
		te = time.Now()

		fps := 1.0 / te.Sub(t0).Seconds()

		fmt.Printf("Update %0.4f msec = %0.4f updatefn + %0.4f draw (%d primitives) [max fps %0.2f]\n", te.Sub(t0).Seconds()*1000, t1.Sub(t0).Seconds()*1000, te.Sub(t1).Seconds()*1000, nprimitives, fps)
	}
	if w.Perf && nprimitives > 0 {
		te = time.Now()
		img := w.wndb.RGBA()
		bounds := w.bounds
		fps := 1.0 / te.Sub(t0).Seconds()

		s := fmt.Sprintf("%0.4fms + %0.4fms (%0.2f)", t1.Sub(t0).Seconds()*1000, te.Sub(t1).Seconds()*1000, fps)
		d := ifont.Drawer{
			Dst:  img,
			Src:  image.White,
			Face: fontFace2fontFace(&w.ctx.Style.Font).face}

		width := d.MeasureString(s).Ceil()

		bounds.Min.X = bounds.Max.X - width
		bounds.Min.Y = bounds.Max.Y - (w.ctx.Style.Font.Metrics().Ascent + w.ctx.Style.Font.Metrics().Descent).Ceil()
		draw.Draw(img, bounds, image.Black, bounds.Min, draw.Src)
		d.Dot = fixed.P(bounds.Min.X, bounds.Min.Y+w.ctx.Style.Font.Metrics().Ascent.Ceil())
		d.DrawString(s)
	}
	if dumpFrame && frameCnt < 1000 && nprimitives > 0 {
		w.dumpFrame(w.wndb.RGBA(), t0, t1, te, nprimitives)
	}
	if nprimitives > 0 {
		w.wnd.Upload(w.bounds.Min, w.wndb, w.bounds)
		w.wnd.Publish()
	}
}

func (w *masterWindow) closeLocked() {
	w.closing = true
	if w.wndb != nil {
		w.wndb.Release()
	}
	w.wnd.Release()
}

// Programmatically closes window.
func (mw *masterWindow) Close() {
	mw.wnd.Send(lifecycle.Event{From: lifecycle.StageAlive, To: lifecycle.StageDead})
}

// Returns true if the window is closed.
func (mw *masterWindow) Closed() bool {
	mw.uilock.Lock()
	defer mw.uilock.Unlock()
	return mw.closing
}

func (w *masterWindow) setupBuffer(sz image.Point) {
	var err error
	oldb := w.wndb
	w.wndb, err = w.screen.NewBuffer(sz)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not setup buffer: %v", err)
		w.wndb = oldb
	}
	w.bounds = w.wndb.Bounds()
}

func (w *masterWindow) draw() int {
	if !w.drawChanged() {
		return 0
	}

	w.prevCmds = append(w.prevCmds[:0], w.ctx.cmds...)

	return w.ctx.Draw(w.wndb.RGBA())
}

var cnt = 0
var ln, frect, frectover, brrect, frrect, ftri, circ, fcirc, txt int

func (ctx *context) Draw(wimg *image.RGBA) int {
	var txttim, tritim, brecttim, frecttim, frectovertim, frrecttim time.Duration
	var t0 time.Time

	img := wimg

	var painter *myRGBAPainter
	var rasterizer *raster.Rasterizer

	roundAngle := func(cx, cy int, radius uint16, startAngle, angle float64, c color.Color) {
		rasterizer.Clear()
		rasterizer.Start(fixed.P(cx, cy))
		traceArc(rasterizer, float64(cx), float64(cy), float64(radius), float64(radius), startAngle, angle, false)
		rasterizer.Add1(fixed.P(cx, cy))
		painter.SetColor(c)
		rasterizer.Rasterize(painter)

	}

	setupRasterizer := func() {
		rasterizer = raster.NewRasterizer(img.Bounds().Dx(), img.Bounds().Dy())
		painter = &myRGBAPainter{Image: img}
	}

	if ctx.cmdstim != nil {
		ctx.cmdstim = ctx.cmdstim[:0]
	}

	transparentBorderOptimization := false

	for i := range ctx.cmds {
		if perfUpdate {
			t0 = time.Now()
		}
		icmd := &ctx.cmds[i]
		switch icmd.Kind {
		case command.ScissorCmd:
			img = wimg.SubImage(icmd.Rectangle()).(*image.RGBA)
			painter = nil
			rasterizer = nil

		case command.LineCmd:
			cmd := icmd.Line
			colimg := image.NewUniform(cmd.Color)
			op := draw.Over
			if cmd.Color.A == 0xff {
				op = draw.Src
			}

			h1 := int(cmd.LineThickness / 2)
			h2 := int(cmd.LineThickness) - h1

			if cmd.Begin.X == cmd.End.X {
				// draw vertical line
				r := image.Rect(cmd.Begin.X-h1, cmd.Begin.Y, cmd.Begin.X+h2, cmd.End.Y)
				drawFill(img, r, colimg, r.Min, op)
			} else if cmd.Begin.Y == cmd.End.Y {
				// draw horizontal line
				r := image.Rect(cmd.Begin.X, cmd.Begin.Y-h1, cmd.End.X, cmd.Begin.Y+h2)
				drawFill(img, r, colimg, r.Min, op)
			} else {
				if rasterizer == nil {
					setupRasterizer()
				}

				unzw := rasterizer.UseNonZeroWinding
				rasterizer.UseNonZeroWinding = true

				var p raster.Path
				p.Start(fixed.P(cmd.Begin.X-img.Bounds().Min.X, cmd.Begin.Y-img.Bounds().Min.Y))
				p.Add1(fixed.P(cmd.End.X-img.Bounds().Min.X, cmd.End.Y-img.Bounds().Min.Y))

				rasterizer.Clear()
				rasterizer.AddStroke(p, fixed.I(int(cmd.LineThickness)), nil, nil)
				painter.SetColor(cmd.Color)
				rasterizer.Rasterize(painter)

				rasterizer.UseNonZeroWinding = unzw
			}
			ln++

		case command.RectFilledCmd:
			cmd := icmd.RectFilled
			if i == 0 {
				// first command draws the background, insure that it's always fully opaque
				cmd.Color.A = 0xff
			}
			if transparentBorderOptimization {
				transparentBorderOptimization = false
				prevcmd := ctx.cmds[i-1].RectFilled
				const m = 1<<16 - 1
				sr, sg, sb, sa := cmd.Color.RGBA()
				a := (m - sa) * 0x101
				cmd.Color.R = uint8((uint32(prevcmd.Color.R)*a/m + sr) >> 8)
				cmd.Color.G = uint8((uint32(prevcmd.Color.G)*a/m + sg) >> 8)
				cmd.Color.B = uint8((uint32(prevcmd.Color.B)*a/m + sb) >> 8)
				cmd.Color.A = uint8((uint32(prevcmd.Color.A)*a/m + sa) >> 8)
			}
			colimg := image.NewUniform(cmd.Color)
			op := draw.Over
			if cmd.Color.A == 0xff {
				op = draw.Src
			}

			body := icmd.Rectangle()

			var lwing, rwing image.Rectangle

			// rounding is true if rounding has been requested AND we can draw it
			rounding := cmd.Rounding > 0 && int(cmd.Rounding*2) < icmd.W && int(cmd.Rounding*2) < icmd.H

			if rounding {
				body.Min.X += int(cmd.Rounding)
				body.Max.X -= int(cmd.Rounding)

				lwing = image.Rect(icmd.X, icmd.Y+int(cmd.Rounding), icmd.X+int(cmd.Rounding), icmd.Y+icmd.H-int(cmd.Rounding))
				rwing = image.Rect(icmd.X+icmd.W-int(cmd.Rounding), lwing.Min.Y, icmd.X+icmd.W, lwing.Max.Y)
			}

			bordopt := false

			if ok, border := borderOptimize(icmd, ctx.cmds, i+1); ok {
				// only draw parts of body if this command can be optimized to a border with the next command

				bordopt = true

				if ctx.cmds[i+1].RectFilled.Color.A != 0xff {
					transparentBorderOptimization = true
				}

				border += int(ctx.cmds[i+1].RectFilled.Rounding)

				top := image.Rect(body.Min.X, body.Min.Y, body.Max.X, body.Min.Y+border)
				bot := image.Rect(body.Min.X, body.Max.Y-border, body.Max.X, body.Max.Y)

				drawFill(img, top, colimg, top.Min, op)
				drawFill(img, bot, colimg, bot.Min, op)

				if border < int(cmd.Rounding) {
					// wings need shrinking
					d := int(cmd.Rounding) - border
					lwing.Max.Y -= d
					rwing.Min.Y += d
				} else {
					// display extra wings
					d := border - int(cmd.Rounding)

					xlwing := image.Rect(top.Min.X, top.Max.Y, top.Min.X+d, bot.Min.Y)
					xrwing := image.Rect(top.Max.X-d, top.Max.Y, top.Max.X, bot.Min.Y)

					drawFill(img, xlwing, colimg, xlwing.Min, op)
					drawFill(img, xrwing, colimg, xrwing.Min, op)
				}

				brrect++
			} else {
				drawFill(img, body, colimg, body.Min, op)
				if cmd.Rounding == 0 {
					if op == draw.Src {
						frect++
					} else {
						frectover++
					}
				} else {
					frrect++
				}
			}

			if rounding {
				drawFill(img, lwing, colimg, lwing.Min, op)
				drawFill(img, rwing, colimg, rwing.Min, op)

				rangle := math.Pi / 2

				if rasterizer == nil {
					setupRasterizer()
				}

				minx := img.Bounds().Min.X
				miny := img.Bounds().Min.Y

				roundAngle(icmd.X+icmd.W-int(cmd.Rounding)-minx, icmd.Y+int(cmd.Rounding)-miny, cmd.Rounding, -math.Pi/2, rangle, cmd.Color)
				roundAngle(icmd.X+icmd.W-int(cmd.Rounding)-minx, icmd.Y+icmd.H-int(cmd.Rounding)-miny, cmd.Rounding, 0, rangle, cmd.Color)
				roundAngle(icmd.X+int(cmd.Rounding)-minx, icmd.Y+icmd.H-int(cmd.Rounding)-miny, cmd.Rounding, math.Pi/2, rangle, cmd.Color)
				roundAngle(icmd.X+int(cmd.Rounding)-minx, icmd.Y+int(cmd.Rounding)-miny, cmd.Rounding, math.Pi, rangle, cmd.Color)
			}

			if perfUpdate {
				if bordopt {
					brecttim += time.Since(t0)
				} else {
					if cmd.Rounding > 0 {
						frrecttim += time.Since(t0)
					} else {
						d := time.Since(t0)
						if op == draw.Src {
							frecttim += d
						} else {
							if d > 8*time.Millisecond {
								fmt.Printf("outstanding rect")
							}
							frectovertim += d
						}
					}
				}
			}

		case command.TriangleFilledCmd:
			cmd := icmd.TriangleFilled
			if rasterizer == nil {
				setupRasterizer()
			}
			minx := img.Bounds().Min.X
			miny := img.Bounds().Min.Y
			rasterizer.Clear()
			rasterizer.Start(fixed.P(cmd.A.X-minx, cmd.A.Y-miny))
			rasterizer.Add1(fixed.P(cmd.B.X-minx, cmd.B.Y-miny))
			rasterizer.Add1(fixed.P(cmd.C.X-minx, cmd.C.Y-miny))
			rasterizer.Add1(fixed.P(cmd.A.X-minx, cmd.A.Y-miny))
			painter.SetColor(cmd.Color)
			rasterizer.Rasterize(painter)
			ftri++

			if perfUpdate {
				tritim += time.Since(t0)
			}

		case command.CircleFilledCmd:
			if rasterizer == nil {
				setupRasterizer()
			}
			rasterizer.Clear()
			startp := traceArc(rasterizer, float64(icmd.X-img.Bounds().Min.X)+float64(icmd.W/2), float64(icmd.Y-img.Bounds().Min.Y)+float64(icmd.H/2), float64(icmd.W/2), float64(icmd.H/2), 0, -math.Pi*2, true)
			rasterizer.Add1(startp) // closes path
			painter.SetColor(icmd.CircleFilled.Color)
			rasterizer.Rasterize(painter)
			fcirc++

		case command.ImageCmd:
			draw.Draw(img, icmd.Rectangle(), icmd.Image.Img, image.Point{}, draw.Src)

		case command.TextCmd:
			dstimg := wimg.SubImage(img.Bounds().Intersect(icmd.Rectangle())).(*image.RGBA)
			d := ifont.Drawer{
				Dst:  dstimg,
				Src:  image.NewUniform(icmd.Text.Foreground),
				Face: fontFace2fontFace(&icmd.Text.Face).face,
				Dot:  fixed.P(icmd.X, icmd.Y+icmd.Text.Face.Metrics().Ascent.Ceil())}

			start := 0
			for i := range icmd.Text.String {
				if icmd.Text.String[i] == '\n' {
					d.DrawString(icmd.Text.String[start:i])
					d.Dot.X = fixed.I(icmd.X)
					d.Dot.Y += fixed.I(FontHeight(icmd.Text.Face))
					start = i + 1
				}
			}
			if start < len(icmd.Text.String) {
				d.DrawString(icmd.Text.String[start:])
			}
			txt++
			if perfUpdate {
				txttim += time.Since(t0)
			}
		default:
			panic(UnknownCommandErr)
		}

		if dumpFrame {
			ctx.cmdstim = append(ctx.cmdstim, time.Since(t0))
		}
	}

	if perfUpdate {
		fmt.Printf("triangle: %0.4fms text: %0.4fms brect: %0.4fms frect: %0.4fms frectover: %0.4fms frrect %0.4f\n", tritim.Seconds()*1000, txttim.Seconds()*1000, brecttim.Seconds()*1000, frecttim.Seconds()*1000, frectovertim.Seconds()*1000, frrecttim.Seconds()*1000)
	}

	cnt++
	if perfUpdate /*&& (cnt%100) == 0*/ {
		fmt.Printf("ln %d, frect %d, frectover %d, frrect %d, brrect %d, ftri %d, circ %d, fcirc %d, txt %d\n", ln, frect, frectover, frrect, brrect, ftri, circ, fcirc, txt)
		ln, frect, frectover, frrect, brrect, ftri, circ, fcirc, txt = 0, 0, 0, 0, 0, 0, 0, 0, 0
	}

	return len(ctx.cmds)
}

// Returns true if cmds[idx] is a shrunk version of CommandFillRect and its
// color is not semitransparent and the border isn't greater than 128
func borderOptimize(cmd *command.Command, cmds []command.Command, idx int) (ok bool, border int) {
	if idx >= len(cmds) {
		return false, 0
	}

	if cmd.Kind != command.RectFilledCmd || cmds[idx].Kind != command.RectFilledCmd {
		return false, 0
	}

	cmd2 := cmds[idx]

	if cmd.RectFilled.Color.A != 0xff && cmd2.RectFilled.Color.A != 0xff {
		return false, 0
	}

	border = cmd2.X - cmd.X
	if border <= 0 || border > 128 {
		return false, 0
	}

	if shrinkRect(cmd.Rect, border) != cmd2.Rect {
		return false, 0
	}

	return true, border
}

func floatP(x, y float64) fixed.Point26_6 {
	return fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
}

// TraceArc trace an arc using a Liner
func traceArc(t *raster.Rasterizer, x, y, rx, ry, start, angle float64, first bool) fixed.Point26_6 {
	end := start + angle
	clockWise := true
	if angle < 0 {
		clockWise = false
	}
	if !clockWise {
		for start < end {
			start += math.Pi * 2
		}
		end = start + angle
	}
	ra := (math.Abs(rx) + math.Abs(ry)) / 2
	da := math.Acos(ra/(ra+0.125)) * 2
	//normalize
	if !clockWise {
		da = -da
	}
	angle = start
	var curX, curY float64
	var startX, startY float64
	for {
		if (angle < end-da/4) != clockWise {
			curX = x + math.Cos(end)*rx
			curY = y + math.Sin(end)*ry
			t.Add1(floatP(curX, curY))
			return floatP(startX, startY)
		}
		curX = x + math.Cos(angle)*rx
		curY = y + math.Sin(angle)*ry

		angle += da
		if first {
			first = false
			startX, startY = curX, curY
			t.Start(floatP(curX, curY))
		} else {
			t.Add1(floatP(curX, curY))
		}
	}
}

type myRGBAPainter struct {
	Image *image.RGBA
	// cr, cg, cb and ca are the 16-bit color to paint the spans.
	cr, cg, cb, ca uint32
}

// SetColor sets the color to paint the spans.
func (r *myRGBAPainter) SetColor(c color.Color) {
	r.cr, r.cg, r.cb, r.ca = c.RGBA()
}

func (r *myRGBAPainter) Paint(ss []raster.Span, done bool) {
	b := r.Image.Bounds()
	cr8 := uint8(r.cr >> 8)
	cg8 := uint8(r.cg >> 8)
	cb8 := uint8(r.cb >> 8)
	for _, s := range ss {
		s.Y += b.Min.Y
		s.X0 += b.Min.X
		s.X1 += b.Min.X
		if s.Y < b.Min.Y {
			continue
		}
		if s.Y >= b.Max.Y {
			return
		}
		if s.X0 < b.Min.X {
			s.X0 = b.Min.X
		}
		if s.X1 > b.Max.X {
			s.X1 = b.Max.X
		}
		if s.X0 >= s.X1 {
			continue
		}
		// This code mimics drawGlyphOver in $GOROOT/src/image/draw/draw.go.
		ma := s.Alpha
		const m = 1<<16 - 1
		i0 := (s.Y-r.Image.Rect.Min.Y)*r.Image.Stride + (s.X0-r.Image.Rect.Min.X)*4
		i1 := i0 + (s.X1-s.X0)*4
		if ma != m || r.ca != m {
			for i := i0; i < i1; i += 4 {
				dr := uint32(r.Image.Pix[i+0])
				dg := uint32(r.Image.Pix[i+1])
				db := uint32(r.Image.Pix[i+2])
				da := uint32(r.Image.Pix[i+3])
				a := (m - (r.ca * ma / m)) * 0x101
				r.Image.Pix[i+0] = uint8((dr*a + r.cr*ma) / m >> 8)
				r.Image.Pix[i+1] = uint8((dg*a + r.cg*ma) / m >> 8)
				r.Image.Pix[i+2] = uint8((db*a + r.cb*ma) / m >> 8)
				r.Image.Pix[i+3] = uint8((da*a + r.ca*ma) / m >> 8)
			}
		} else {
			for i := i0; i < i1; i += 4 {
				r.Image.Pix[i+0] = cr8
				r.Image.Pix[i+1] = cg8
				r.Image.Pix[i+2] = cb8
				r.Image.Pix[i+3] = 0xff
			}
		}
	}
}

// tracks github.com/aarzilli/nucular/font.Face
type fontFace struct {
	face ifont.Face
}

func fontFace2fontFace(f *font.Face) *fontFace {
	return (*fontFace)(unsafe.Pointer(f))
}

func textClamp(f font.Face, text []rune, space int) []rune {
	text_width := 0
	fc := fontFace2fontFace(&f).face
	for i, ch := range text {
		_, _, _, xwfixed, _ := fc.Glyph(fixed.P(0, 0), ch)
		xw := xwfixed.Ceil()
		if text_width+xw >= space {
			return text[:i]
		}
		text_width += xw
	}
	return text
}

var fontWidthCache *lru.Cache
var fontWidthCacheSize int

func init() {
	fontWidthCacheSize = 256
	fontWidthCache, _ = lru.New(256)
}

func ChangeFontWidthCache(size int) {
	if size > fontWidthCacheSize {
		fontWidthCacheSize = size
		fontWidthCache, _ = lru.New(fontWidthCacheSize)
	}
}

type fontWidthCacheKey struct {
	f      font.Face
	string string
}

func FontWidth(f font.Face, str string) int {
	maxw := 0
	for {
		newline := strings.Index(str, "\n")
		line := str
		if newline >= 0 {
			line = str[:newline]
		}

		k := fontWidthCacheKey{f, line}

		var w int
		if val, ok := fontWidthCache.Get(k); ok {
			w = val.(int)
		} else {
			d := ifont.Drawer{Face: fontFace2fontFace(&f).face}
			w = d.MeasureString(line).Ceil()
			fontWidthCache.Add(k, w)
		}

		if w > maxw {
			maxw = w
		}

		if newline >= 0 {
			str = str[newline+1:]
		} else {
			break
		}
	}
	return maxw
}

func glyphAdvance(f font.Face, ch rune) int {
	a, _ := fontFace2fontFace(&f).face.GlyphAdvance(ch)
	return a.Ceil()
}

func measureRunes(f font.Face, runes []rune) int {
	var advance fixed.Int26_6
	prevC := rune(-1)
	fc := fontFace2fontFace(&f).face
	for _, c := range runes {
		if prevC >= 0 {
			advance += fc.Kern(prevC, c)
		}
		a, ok := fc.GlyphAdvance(c)
		if !ok {
			// TODO: is falling back on the U+FFFD glyph the responsibility of
			// the Drawer or the Face?
			// TODO: set prevC = '\ufffd'?
			continue
		}
		advance += a
		prevC = c
	}
	return advance.Ceil()
}

///////////////////////////////////////////////////////////////////////////////////
// TEXT WIDGETS
///////////////////////////////////////////////////////////////////////////////////

const (
	tabSizeInSpaces = 8
)

type textWidget struct {
	Padding    image.Point
	Background color.RGBA
	Text       color.RGBA
}

func widgetText(o *command.Buffer, b rect.Rect, str string, t *textWidget, a label.Align, f font.Face) {
	b.H = max(b.H, 2*t.Padding.Y)
	lblrect := rect.Rect{X: 0, W: 0, Y: b.Y + t.Padding.Y, H: b.H - 2*t.Padding.Y}

	/* align in x-axis */
	switch a[0] {
	case 'L':
		lblrect.X = b.X + t.Padding.X
		lblrect.W = max(0, b.W-2*t.Padding.X)
	case 'C':
		text_width := FontWidth(f, str)
		text_width += (2.0 * t.Padding.X)
		lblrect.W = max(1, 2*t.Padding.X+text_width)
		lblrect.X = (b.X + t.Padding.X + ((b.W-2*t.Padding.X)-lblrect.W)/2)
		lblrect.X = max(b.X+t.Padding.X, lblrect.X)
		lblrect.W = min(b.X+b.W, lblrect.X+lblrect.W)
		if lblrect.W >= lblrect.X {
			lblrect.W -= lblrect.X
		}
	case 'R':
		text_width := FontWidth(f, str)
		text_width += (2.0 * t.Padding.X)
		lblrect.X = max(b.X+t.Padding.X, (b.X+b.W)-(2*t.Padding.X+text_width))
		lblrect.W = text_width + 2*t.Padding.X
	default:
		panic("unsupported alignment")
	}

	/* align in y-axis */
	if len(a) >= 2 {
		switch a[1] {
		case 'C':
			lblrect.Y = b.Y + b.H/2.0 - FontHeight(f)/2.0
		case 'B':
			lblrect.Y = b.Y + b.H - FontHeight(f)
		}
	}
	if lblrect.H < FontHeight(f)*2 {
		lblrect.H = FontHeight(f) * 2
	}

	o.DrawText(lblrect, str, f, t.Text)
}

func widgetTextWrap(o *command.Buffer, b rect.Rect, str []rune, t *textWidget, f font.Face) {
	var done int = 0
	var line rect.Rect
	var text textWidget

	text.Padding = image.Point{0, 0}
	text.Background = t.Background
	text.Text = t.Text

	b.W = max(b.W, 2*t.Padding.X)
	b.H = max(b.H, 2*t.Padding.Y)
	b.H = b.H - 2*t.Padding.Y

	line.X = b.X + t.Padding.X
	line.Y = b.Y + t.Padding.Y
	line.W = b.W - 2*t.Padding.X
	line.H = 2*t.Padding.Y + FontHeight(f)

	fitting := textClamp(f, str, line.W)
	for done < len(str) {
		if len(fitting) == 0 || line.Y+line.H >= (b.Y+b.H) {
			break
		}
		widgetText(o, line, string(fitting), &text, "LC", f)
		done += len(fitting)
		line.Y += FontHeight(f) + 2*t.Padding.Y
		fitting = textClamp(f, str[done:], line.W)
	}
}
