package nucular

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"io"
	"time"

	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/rect"
	nstyle "github.com/aarzilli/nucular/style"

	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
)

const perfUpdate = false
const dumpFrame = false

var UnknownCommandErr = errors.New("unknown command")

type context struct {
	mw             MasterWindow
	Input          Input
	Style          nstyle.Style
	Windows        []*Window
	DockedWindows  dockedTree
	changed        int32
	activateEditor *TextEditor
	cmds           []command.Command
	trashFrame     bool
	autopos        image.Point

	finalCmds command.Buffer

	dockedWindowFocus int
	floatWindowFocus  int
	scrollwheelFocus  int
	dockedCnt         int

	cmdstim []time.Duration // contains timing for all commands
}

func contextAllCommands(ctx *context) {
	ctx.cmds = ctx.cmds[:0]
	for i, w := range ctx.Windows {
		ctx.cmds = append(ctx.cmds, w.cmds.Commands...)
		if i == 0 {
			ctx.DockedWindows.Walk(func(w *Window) *Window {
				ctx.cmds = append(ctx.cmds, w.cmds.Commands...)
				return w
			})
		}
	}
	ctx.cmds = append(ctx.cmds, ctx.finalCmds.Commands...)
}

func (ctx *context) setupMasterWindow(layout *panel, updatefn UpdateFn) {
	ctx.Windows = append(ctx.Windows, createWindow(ctx, ""))
	ctx.Windows[0].idx = 0
	ctx.Windows[0].layout = layout
	ctx.Windows[0].flags = layout.Flags | WindowNonmodal
	ctx.Windows[0].updateFn = updatefn
}

func (ctx *context) Update() {
	for count := 0; count < 2; count++ {
		contextBegin(ctx, ctx.Windows[0].layout)
		for i := 0; i < len(ctx.Windows); i++ {
			ctx.Windows[i].began = false
		}
		ctx.Restack()
		ctx.FindFocus()
		for i := 0; i < len(ctx.Windows); i++ { // this must not use range or tooltips won't work
			ctx.updateWindow(ctx.Windows[i])
			if i == 0 {
				t := ctx.DockedWindows.Update(ctx.Windows[0].Bounds, ctx.Style.Scaling)
				if t != nil {
					ctx.DockedWindows = *t
				}
			}
		}
		contextEnd(ctx)
		if !ctx.trashFrame {
			break
		} else {
			ctx.Reset()
		}
	}
}

func (ctx *context) updateWindow(win *Window) {
	if win.updateFn != nil {
		win.specialPanelBegin()
		win.updateFn(win)
	}

	if !win.began {
		win.close = true
		return
	}

	if win.title == tooltipWindowTitle {
		win.close = true
	}

	if win.flags&windowPopup != 0 {
		panelEnd(ctx, win)
	}
}

func (ctx *context) processKeyEvent(e key.Event, textbuffer *bytes.Buffer) {
	if e.Direction == key.DirRelease {
		return
	}

	evinNotext := func() {
		for _, k := range ctx.Input.Keyboard.Keys {
			if k.Code == e.Code {
				k.Modifiers |= e.Modifiers
				return
			}
		}
		ctx.Input.Keyboard.Keys = append(ctx.Input.Keyboard.Keys, e)
	}
	evinText := func() {
		if e.Modifiers == 0 || e.Modifiers == key.ModShift {
			io.WriteString(textbuffer, string(e.Rune))
		}

		evinNotext()
	}

	switch {
	case e.Code == key.CodeUnknown:
		if e.Rune > 0 {
			evinText()
		}
	case (e.Code >= key.CodeA && e.Code <= key.Code0) || e.Code == key.CodeSpacebar || e.Code == key.CodeHyphenMinus || e.Code == key.CodeEqualSign || e.Code == key.CodeLeftSquareBracket || e.Code == key.CodeRightSquareBracket || e.Code == key.CodeBackslash || e.Code == key.CodeSemicolon || e.Code == key.CodeApostrophe || e.Code == key.CodeGraveAccent || e.Code == key.CodeComma || e.Code == key.CodeFullStop || e.Code == key.CodeSlash || (e.Code >= key.CodeKeypadSlash && e.Code <= key.CodeKeypadPlusSign) || (e.Code >= key.CodeKeypad1 && e.Code <= key.CodeKeypadEqualSign):
		evinText()
	case e.Code == key.CodeTab:
		e.Rune = '\t'
		evinText()
	case e.Code == key.CodeReturnEnter || e.Code == key.CodeKeypadEnter:
		e.Rune = '\n'
		evinText()
	default:
		evinNotext()
	}
}

func contextBegin(ctx *context, layout *panel) {
	for _, w := range ctx.Windows {
		w.usingSub = false
		w.curNode = w.rootNode
		w.close = false
		w.widgets.reset()
		w.cmds.Reset()
	}
	ctx.finalCmds.Reset()
	ctx.DockedWindows.Walk(func(w *Window) *Window {
		w.usingSub = false
		w.curNode = w.rootNode
		w.close = false
		w.widgets.reset()
		w.cmds.Reset()
		return w
	})

	ctx.trashFrame = false
	ctx.Windows[0].layout = layout
	panelBegin(ctx, ctx.Windows[0], "")
	layout.Offset = &ctx.Windows[0].Scrollbar
}

func contextEnd(ctx *context) {
	panelEnd(ctx, ctx.Windows[0])
}

func (ctx *context) Reset() {
	prevNumWindows := len(ctx.Windows)
	for i := 0; i < len(ctx.Windows); i++ {
		if ctx.Windows[i].close {
			if i != len(ctx.Windows)-1 {
				copy(ctx.Windows[i:], ctx.Windows[i+1:])
				i--
			}
			ctx.Windows = ctx.Windows[:len(ctx.Windows)-1]
		}
	}
	for i := range ctx.Windows {
		ctx.Windows[i].idx = i
	}
	if prevNumWindows == 2 && len(ctx.Windows) == 1 && ctx.Input.Mouse.valid {
		ctx.DockedWindows.Walk(func(w *Window) *Window {
			if w.flags&windowDocked == 0 {
				return w
			}
			for _, b := range []mouse.Button{mouse.ButtonLeft, mouse.ButtonRight, mouse.ButtonMiddle} {
				btn := ctx.Input.Mouse.Buttons[b]
				if btn.Clicked && w.Bounds.Contains(btn.ClickedPos) {
					ctx.dockedWindowFocus = w.idx
					return w
				}
			}
			return w
		})
	}
	ctx.activateEditor = nil
	in := &ctx.Input
	in.Mouse.Buttons[mouse.ButtonLeft].Clicked = false
	in.Mouse.Buttons[mouse.ButtonMiddle].Clicked = false
	in.Mouse.Buttons[mouse.ButtonRight].Clicked = false
	in.Mouse.ScrollDelta = 0
	in.Mouse.Prev.X = in.Mouse.Pos.X
	in.Mouse.Prev.Y = in.Mouse.Pos.Y
	in.Mouse.Delta = image.Point{}
	in.Keyboard.Keys = in.Keyboard.Keys[0:0]
}

func (ctx *context) Restack() {
	clicked := false
	for _, b := range []mouse.Button{mouse.ButtonLeft, mouse.ButtonRight, mouse.ButtonMiddle} {
		if ctx.Input.Mouse.Buttons[b].Clicked && ctx.Input.Mouse.Buttons[b].Down {
			clicked = true
			break
		}
	}
	if !clicked {
		return
	}
	ctx.dockedWindowFocus = 0
	nonmodalToplevel := false
	var toplevelIdx int
	for i := len(ctx.Windows) - 1; i >= 0; i-- {
		if ctx.Windows[i].flags&windowTooltip == 0 {
			toplevelIdx = i
			nonmodalToplevel = ctx.Windows[i].flags&WindowNonmodal != 0
			break
		}
	}
	if !nonmodalToplevel {
		return
	}
	// toplevel window is non-modal, proceed to change the stacking order if
	// the user clicked outside of it
	restacked := false
	found := false
	for i := len(ctx.Windows) - 1; i > 0; i-- {
		if ctx.Windows[i].flags&windowTooltip != 0 {
			continue
		}
		if ctx.restackClick(ctx.Windows[i]) {
			found = true
			if toplevelIdx != i {
				newToplevel := ctx.Windows[i]
				copy(ctx.Windows[i:toplevelIdx], ctx.Windows[i+1:toplevelIdx+1])
				ctx.Windows[toplevelIdx] = newToplevel
				restacked = true
			}
			break
		}
	}
	if restacked {
		for i := range ctx.Windows {
			ctx.Windows[i].idx = i
		}
	}
	if found {
		return
	}
	ctx.DockedWindows.Walk(func(w *Window) *Window {
		if ctx.restackClick(w) && (w.flags&windowDocked != 0) {
			ctx.dockedWindowFocus = w.idx
		}
		return w
	})
}

func (ctx *context) FindFocus() {
	ctx.floatWindowFocus = 0
	for i := len(ctx.Windows) - 1; i >= 0; i-- {
		if ctx.Windows[i].flags&windowTooltip == 0 {
			ctx.floatWindowFocus = i
			break
		}
	}
	ctx.scrollwheelFocus = 0
	for i := len(ctx.Windows) - 1; i > 0; i-- {
		if ctx.Windows[i].Bounds.Contains(ctx.Input.Mouse.Pos) {
			ctx.scrollwheelFocus = i
			break
		}
	}
	if ctx.scrollwheelFocus == 0 {
		ctx.DockedWindows.Walk(func(w *Window) *Window {
			if w.Bounds.Contains(ctx.Input.Mouse.Pos) {
				ctx.scrollwheelFocus = w.idx
			}
			return w
		})
	}
}

func (ctx *context) Walk(fn WindowWalkFn) {
	fn(ctx.Windows[0].title, ctx.Windows[0].Data, false, 0, ctx.Windows[0].Bounds)
	ctx.DockedWindows.walkExt(func(t *dockedTree) {
		switch t.Type {
		case dockedNodeHoriz:
			fn("", nil, true, t.Split.Size, rect.Rect{})
		case dockedNodeVert:
			fn("", nil, true, -t.Split.Size, rect.Rect{})
		case dockedNodeLeaf:
			if t.W == nil {
				fn("", nil, true, 0, rect.Rect{})
			} else {
				fn(t.W.title, t.W.Data, true, 0, t.W.Bounds)
			}
		}
	})
	for _, win := range ctx.Windows[1:] {
		if win.flags&WindowNonmodal != 0 {
			fn(win.title, win.Data, false, 0, win.Bounds)
		}
	}
}

func (ctx *context) restackClick(w *Window) bool {
	if !ctx.Input.Mouse.valid {
		return false
	}
	for _, b := range []mouse.Button{mouse.ButtonLeft, mouse.ButtonRight, mouse.ButtonMiddle} {
		btn := ctx.Input.Mouse.Buttons[b]
		if btn.Clicked && btn.Down && w.Bounds.Contains(btn.ClickedPos) {
			return true
		}
	}
	return false
}

type dockedNodeType uint8

const (
	dockedNodeLeaf dockedNodeType = iota
	dockedNodeVert
	dockedNodeHoriz
)

type dockedTree struct {
	Type  dockedNodeType
	Split ScalableSplit
	Child [2]*dockedTree
	W     *Window
}

func (t *dockedTree) Update(bounds rect.Rect, scaling float64) *dockedTree {
	if t == nil {
		return nil
	}
	switch t.Type {
	case dockedNodeVert:
		b0, b1, _ := t.Split.verticalnw(bounds, scaling)
		t.Child[0] = t.Child[0].Update(b0, scaling)
		t.Child[1] = t.Child[1].Update(b1, scaling)
	case dockedNodeHoriz:
		b0, b1, _ := t.Split.horizontalnw(bounds, scaling)
		t.Child[0] = t.Child[0].Update(b0, scaling)
		t.Child[1] = t.Child[1].Update(b1, scaling)
	case dockedNodeLeaf:
		if t.W != nil {
			t.W.Bounds = bounds
			t.W.ctx.updateWindow(t.W)
			if t.W == nil {
				return nil
			}
			if t.W.close {
				t.W = nil
				return nil
			}
			return t
		}
		return nil
	}
	if t.Child[0] == nil {
		return t.Child[1]
	}
	if t.Child[1] == nil {
		return t.Child[0]
	}
	return t
}

func (t *dockedTree) walkExt(fn func(t *dockedTree)) {
	if t == nil {
		return
	}
	switch t.Type {
	case dockedNodeVert, dockedNodeHoriz:
		fn(t)
		t.Child[0].walkExt(fn)
		t.Child[1].walkExt(fn)
	case dockedNodeLeaf:
		fn(t)
	}
}

func (t *dockedTree) Walk(fn func(t *Window) *Window) {
	t.walkExt(func(t *dockedTree) {
		if t.Type == dockedNodeLeaf && t.W != nil {
			t.W = fn(t.W)
		}
	})
}

func newDockedLeaf(win *Window) *dockedTree {
	r := &dockedTree{Type: dockedNodeLeaf, W: win}
	r.Split.MinSize = 40
	return r
}

func (t *dockedTree) Dock(win *Window, pos image.Point, bounds rect.Rect, scaling float64) (bool, rect.Rect) {
	if t == nil {
		return false, rect.Rect{}
	}
	switch t.Type {
	case dockedNodeVert:
		b0, b1, _ := t.Split.verticalnw(bounds, scaling)
		canDock, r := t.Child[0].Dock(win, pos, b0, scaling)
		if canDock {
			return canDock, r
		}
		canDock, r = t.Child[1].Dock(win, pos, b1, scaling)
		if canDock {
			return canDock, r
		}
	case dockedNodeHoriz:
		b0, b1, _ := t.Split.horizontalnw(bounds, scaling)
		canDock, r := t.Child[0].Dock(win, pos, b0, scaling)
		if canDock {
			return canDock, r
		}
		canDock, r = t.Child[1].Dock(win, pos, b1, scaling)
		if canDock {
			return canDock, r
		}
	case dockedNodeLeaf:
		v := percentages(bounds, 0.03)
		for i := range v {
			if v[i].Contains(pos) {
				if t.W == nil {
					if win != nil {
						t.W = win
						win.ctx.dockWindow(win)
					}
					return true, bounds
				}
				w := percentages(bounds, 0.5)
				if win != nil {
					if i < 2 {
						// horizontal split
						t.Type = dockedNodeHoriz
						t.Split.Size = int(float64(w[0].H) / scaling)
						t.Child[i] = newDockedLeaf(win)
						t.Child[-i+1] = newDockedLeaf(t.W)
					} else {
						// vertical split
						t.Type = dockedNodeVert
						t.Split.Size = int(float64(w[2].W) / scaling)
						t.Child[i-2] = newDockedLeaf(win)
						t.Child[-(i-2)+1] = newDockedLeaf(t.W)
					}

					t.W = nil
					win.ctx.dockWindow(win)
				}
				return true, w[i]
			}
		}
	}
	return false, rect.Rect{}
}

func (ctx *context) dockWindow(win *Window) {
	win.undockedSz = image.Point{win.Bounds.W, win.Bounds.H}
	win.flags |= windowDocked
	win.layout.Flags |= windowDocked
	ctx.dockedCnt--
	win.idx = ctx.dockedCnt
	for i := range ctx.Windows {
		if ctx.Windows[i] == win {
			if i+1 < len(ctx.Windows) {
				copy(ctx.Windows[i:], ctx.Windows[i+1:])
			}
			ctx.Windows = ctx.Windows[:len(ctx.Windows)-1]
			return
		}
	}
}

func (t *dockedTree) Undock(win *Window) {
	t.Walk(func(w *Window) *Window {
		if w == win {
			return nil
		}
		return w
	})
	win.flags &= ^windowDocked
	win.layout.Flags &= ^windowDocked
	win.Bounds.H = win.undockedSz.Y
	win.Bounds.W = win.undockedSz.X
	win.idx = len(win.ctx.Windows)
	win.ctx.Windows = append(win.ctx.Windows, win)
}

func (t *dockedTree) Scale(win *Window, delta image.Point, scaling float64) image.Point {
	if t == nil || (delta.X == 0 && delta.Y == 0) {
		return image.Point{}
	}
	switch t.Type {
	case dockedNodeVert:
		d0 := t.Child[0].Scale(win, delta, scaling)
		if d0.X != 0 {
			t.Split.Size += int(float64(d0.X) / scaling)
			if t.Split.Size <= t.Split.MinSize {
				t.Split.Size = t.Split.MinSize
			}
			d0.X = 0
		}
		if d0 != image.ZP {
			return d0
		}
		return t.Child[1].Scale(win, delta, scaling)
	case dockedNodeHoriz:
		d0 := t.Child[0].Scale(win, delta, scaling)
		if d0.Y != 0 {
			t.Split.Size += int(float64(d0.Y) / scaling)
			if t.Split.Size <= t.Split.MinSize {
				t.Split.Size = t.Split.MinSize
			}
			d0.Y = 0
		}
		if d0 != image.ZP {
			return d0
		}
		return t.Child[1].Scale(win, delta, scaling)
	case dockedNodeLeaf:
		if t.W == win {
			return delta
		}
	}
	return image.Point{}
}

func (ctx *context) ResetWindows() *DockSplit {
	ctx.DockedWindows = dockedTree{}
	ctx.Windows = ctx.Windows[:1]
	ctx.dockedCnt = 0
	return &DockSplit{ctx, &ctx.DockedWindows}
}

type DockSplit struct {
	ctx  *context
	node *dockedTree
}

func (ds *DockSplit) Split(horiz bool, size int) (left, right *DockSplit) {
	if horiz {
		ds.node.Type = dockedNodeHoriz
	} else {
		ds.node.Type = dockedNodeVert
	}
	ds.node.Split.Size = size
	ds.node.Child[0] = &dockedTree{Type: dockedNodeLeaf, Split: ScalableSplit{MinSize: 40}}
	ds.node.Child[1] = &dockedTree{Type: dockedNodeLeaf, Split: ScalableSplit{MinSize: 40}}
	return &DockSplit{ds.ctx, ds.node.Child[0]}, &DockSplit{ds.ctx, ds.node.Child[1]}
}

func (ds *DockSplit) Open(title string, flags WindowFlags, rect rect.Rect, scale bool, updateFn UpdateFn) {
	ds.ctx.popupOpen(title, flags, rect, scale, updateFn)
	ds.node.Type = dockedNodeLeaf
	ds.node.W = ds.ctx.Windows[len(ds.ctx.Windows)-1]
	ds.ctx.dockWindow(ds.node.W)
}

func percentages(bounds rect.Rect, f float64) (r [4]rect.Rect) {
	pw := int(float64(bounds.W) * f)
	ph := int(float64(bounds.H) * f)
	// horizontal split
	r[0] = bounds
	r[0].H = ph
	r[1] = bounds
	r[1].Y += r[1].H - ph
	r[1].H = ph

	// vertical split
	r[2] = bounds
	r[2].W = pw
	r[3] = bounds
	r[3].X += r[3].W - pw
	r[3].W = pw
	return
}

func clip(dst *image.RGBA, r *image.Rectangle, src image.Image, sp *image.Point) {
	orig := r.Min
	*r = r.Intersect(dst.Bounds())
	*r = r.Intersect(src.Bounds().Add(orig.Sub(*sp)))
	dx := r.Min.X - orig.X
	dy := r.Min.Y - orig.Y
	if dx == 0 && dy == 0 {
		return
	}
	sp.X += dx
	sp.Y += dy
}

func drawFill(dst *image.RGBA, r image.Rectangle, src *image.Uniform, sp image.Point, op draw.Op) {
	clip(dst, &r, src, &sp)
	if r.Empty() {
		return
	}
	sr, sg, sb, sa := src.RGBA()
	switch op {
	case draw.Over:
		drawFillOver(dst, r, sr, sg, sb, sa)
	case draw.Src:
		drawFillSrc(dst, r, sr, sg, sb, sa)
	default:
		draw.Draw(dst, r, src, sp, op)
	}
}

func drawFillSrc(dst *image.RGBA, r image.Rectangle, sr, sg, sb, sa uint32) {
	sr8 := uint8(sr >> 8)
	sg8 := uint8(sg >> 8)
	sb8 := uint8(sb >> 8)
	sa8 := uint8(sa >> 8)
	// The built-in copy function is faster than a straightforward for loop to fill the destination with
	// the color, but copy requires a slice source. We therefore use a for loop to fill the first row, and
	// then use the first row as the slice source for the remaining rows.
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	for i := i0; i < i1; i += 4 {
		dst.Pix[i+0] = sr8
		dst.Pix[i+1] = sg8
		dst.Pix[i+2] = sb8
		dst.Pix[i+3] = sa8
	}
	firstRow := dst.Pix[i0:i1]
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		i0 += dst.Stride
		i1 += dst.Stride
		copy(dst.Pix[i0:i1], firstRow)
	}
}
