package nucular

import (
	"bufio"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/rect"
	nstyle "github.com/aarzilli/nucular/style"
)

type MasterWindow interface {
	context() *context

	Main()
	Changed()
	Close()
	Closed() bool
	ActivateEditor(ed *TextEditor)

	Style() *nstyle.Style
	SetStyle(*nstyle.Style)

	GetPerf() bool
	SetPerf(bool)

	Input() *Input

	PopupOpen(title string, flags WindowFlags, rect rect.Rect, scale bool, updateFn UpdateFn)

	Walk(WindowWalkFn)
	ResetWindows() *DockSplit

	Lock()
	Unlock()
}

func NewMasterWindow(flags WindowFlags, title string, updatefn UpdateFn) MasterWindow {
	return NewMasterWindowSize(flags, title, image.Point{640, 480}, updatefn)
}

type WindowWalkFn func(title string, data interface{}, docked bool, splitSize int, rect rect.Rect)

type masterWindowCommon struct {
	ctx *context

	layout panel

	// show performance counters
	Perf bool

	uilock sync.Mutex

	prevCmds []command.Command
}

func (mw *masterWindowCommon) masterWindowCommonInit(ctx *context, flags WindowFlags, updatefn UpdateFn, wnd MasterWindow) {
	ctx.Input.Mouse.valid = true
	ctx.DockedWindows.Split.MinSize = 40

	mw.layout.Flags = flags

	ctx.setupMasterWindow(&mw.layout, updatefn)

	mw.ctx = ctx
	mw.ctx.mw = wnd

	mw.SetStyle(nstyle.FromTheme(nstyle.DefaultTheme, 1.0))
}

func (mw *masterWindowCommon) context() *context {
	return mw.ctx
}

func (mw *masterWindowCommon) Walk(fn WindowWalkFn) {
	mw.ctx.Walk(fn)
}

func (mw *masterWindowCommon) ResetWindows() *DockSplit {
	return mw.ctx.ResetWindows()
}

func (mw *masterWindowCommon) Input() *Input {
	return &mw.ctx.Input
}

func (mw *masterWindowCommon) ActivateEditor(ed *TextEditor) {
	mw.ctx.activateEditor = ed
}

func (mw *masterWindowCommon) Style() *nstyle.Style {
	return &mw.ctx.Style
}

func (mw *masterWindowCommon) SetStyle(style *nstyle.Style) {
	mw.ctx.Style = *style
	mw.ctx.Style.Defaults()
}

func (mw *masterWindowCommon) GetPerf() bool {
	return mw.Perf
}

func (mw *masterWindowCommon) SetPerf(perf bool) {
	mw.Perf = perf
}

// Forces an update of the window.
func (mw *masterWindowCommon) Changed() {
	atomic.AddInt32(&mw.ctx.changed, 1)
}

func (mw *masterWindowCommon) Lock() {
	mw.uilock.Lock()
}

func (mw *masterWindowCommon) Unlock() {
	mw.uilock.Unlock()
}

// Opens a popup window inside win. Will return true until the
// popup window is closed.
// The contents of the popup window will be updated by updateFn
func (mw *masterWindowCommon) PopupOpen(title string, flags WindowFlags, rect rect.Rect, scale bool, updateFn UpdateFn) {
	go func() {
		mw.ctx.mw.Lock()
		defer mw.ctx.mw.Unlock()
		mw.ctx.popupOpen(title, flags, rect, scale, updateFn)
		mw.ctx.mw.Changed()
	}()
}

var frameCnt int

func (w *masterWindowCommon) dumpFrame(wimg *image.RGBA, t0, t1, te time.Time, nprimitives int) {
	bounds := image.Rect(w.ctx.Input.Mouse.Pos.X, w.ctx.Input.Mouse.Pos.Y, w.ctx.Input.Mouse.Pos.X+10, w.ctx.Input.Mouse.Pos.Y+10)

	draw.Draw(wimg, bounds, image.White, bounds.Min, draw.Src)

	if fh, err := os.Create(fmt.Sprintf("framedump/frame%03d.png", frameCnt)); err == nil {
		png.Encode(fh, wimg)
		fh.Close()
	}

	if fh, err := os.Create(fmt.Sprintf("framedump/frame%03d.txt", frameCnt)); err == nil {
		wr := bufio.NewWriter(fh)
		fps := 1.0 / te.Sub(t0).Seconds()
		tot := time.Duration(0)
		fmt.Fprintf(wr, "# Update %0.4fms = %0.4f updatefn + %0.4f draw (%d primitives) [max fps %0.2f]\n", te.Sub(t0).Seconds()*1000, t1.Sub(t0).Seconds()*1000, te.Sub(t1).Seconds()*1000, nprimitives, fps)
		for i := range w.prevCmds {
			fmt.Fprintf(wr, "%0.2fms %#v\n", w.ctx.cmdstim[i].Seconds()*1000, w.prevCmds[i])
			tot += w.ctx.cmdstim[i]
		}
		fmt.Fprintf(wr, "sanity check %0.2fms\n", tot.Seconds()*1000)
		wr.Flush()
		fh.Close()
	}

	frameCnt++
}

// compares cmds to the last draw frame, returns true if there is a change
func (w *masterWindowCommon) drawChanged() bool {

	contextAllCommands(w.ctx)
	w.ctx.Reset()

	cmds := w.ctx.cmds

	if len(cmds) != len(w.prevCmds) {
		return true
	}

	for i := range cmds {
		if cmds[i].Kind != w.prevCmds[i].Kind {
			return true
		}

		cmd := &cmds[i]
		pcmd := &w.prevCmds[i]

		switch cmds[i].Kind {
		case command.ScissorCmd:
			if *pcmd != *cmd {
				return true
			}

		case command.LineCmd:
			if *pcmd != *cmd {
				return true
			}

		case command.RectFilledCmd:
			if i == 0 {
				cmd.RectFilled.Color.A = 0xff
			}
			if *pcmd != *cmd {
				return true
			}

		case command.TriangleFilledCmd:
			if *pcmd != *cmd {
				return true
			}

		case command.CircleFilledCmd:
			if *pcmd != *cmd {
				return true
			}

		case command.ImageCmd:
			if *pcmd != *cmd {
				return true
			}

		case command.TextCmd:
			if *pcmd != *cmd {
				return true
			}

		default:
			panic(UnknownCommandErr)
		}
	}

	return false
}
