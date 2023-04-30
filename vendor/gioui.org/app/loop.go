// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"image/color"
	"runtime"

	"gioui.org/app/internal/wm"
	"gioui.org/gpu"
	"gioui.org/op"
)

type renderLoop struct {
	summary string
	drawing bool
	err     error

	frames     chan frame
	results    chan frameResult
	refresh    chan struct{}
	refreshErr chan error
	ack        chan struct{}
	stop       chan struct{}
	stopped    chan struct{}
}

type frame struct {
	viewport image.Point
	ops      *op.Ops
}

type frameResult struct {
	profile string
	err     error
}

func newLoop(ctx wm.Context) (*renderLoop, error) {
	l := &renderLoop{
		frames:     make(chan frame),
		results:    make(chan frameResult),
		refresh:    make(chan struct{}),
		refreshErr: make(chan error),
		// Ack is buffered so GPU commands can be issued after
		// ack'ing the frame.
		ack:     make(chan struct{}, 1),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	if err := l.renderLoop(ctx); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *renderLoop) renderLoop(ctx wm.Context) error {
	// GL Operations must happen on a single OS thread, so
	// pass initialization result through a channel.
	initErr := make(chan error)
	go func() {
		defer close(l.stopped)
		runtime.LockOSThread()
		// Don't UnlockOSThread to avoid reuse by the Go runtime.

		if err := ctx.MakeCurrent(); err != nil {
			initErr <- err
			return
		}
		g, err := gpu.New(ctx.API())
		if err != nil {
			initErr <- err
			return
		}
		defer g.Release()
		initErr <- nil
	loop:
		for {
			select {
			case <-l.refresh:
				l.refreshErr <- ctx.MakeCurrent()
			case frame := <-l.frames:
				ctx.Lock()
				if runtime.GOOS == "js" {
					// Use transparent black when Gio is embedded, to allow mixing of Gio and
					// foreign content below.
					g.Clear(color.NRGBA{A: 0x00, R: 0x00, G: 0x00, B: 0x00})
				} else {
					g.Clear(color.NRGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff})
				}
				g.Collect(frame.viewport, frame.ops)
				// Signal that we're done with the frame ops.
				l.ack <- struct{}{}
				var res frameResult
				res.err = g.Frame()
				if res.err == nil {
					res.err = ctx.Present()
				}
				res.profile = g.Profile()
				ctx.Unlock()
				l.results <- res
			case <-l.stop:
				break loop
			}
		}
	}()
	return <-initErr
}

func (l *renderLoop) Release() {
	// Flush error.
	l.Flush()
	close(l.stop)
	<-l.stopped
	l.stop = nil
}

func (l *renderLoop) Flush() error {
	if l.drawing {
		st := <-l.results
		l.setErr(st.err)
		if st.profile != "" {
			l.summary = st.profile
		}
		l.drawing = false
	}
	return l.err
}

func (l *renderLoop) Summary() string {
	return l.summary
}

func (l *renderLoop) Refresh() {
	if l.err != nil {
		return
	}
	// Make sure any pending frame is complete.
	l.Flush()
	l.refresh <- struct{}{}
	l.setErr(<-l.refreshErr)
}

// Draw initiates a draw of a frame. It returns a channel
// than signals when the frame is no longer being accessed.
func (l *renderLoop) Draw(viewport image.Point, frameOps *op.Ops) <-chan struct{} {
	if l.err != nil {
		l.ack <- struct{}{}
		return l.ack
	}
	l.Flush()
	l.frames <- frame{viewport, frameOps}
	l.drawing = true
	return l.ack
}

func (l *renderLoop) setErr(err error) {
	if l.err == nil {
		l.err = err
	}
}
