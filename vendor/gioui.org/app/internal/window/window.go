// SPDX-License-Identifier: Unlicense OR MIT

// Package window implements platform specific windows
// and GPU contexts.
package window

import (
	"errors"

	"gioui.org/gpu/backend"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type Options struct {
	Width, Height       unit.Value
	MinWidth, MinHeight unit.Value
	MaxWidth, MaxHeight unit.Value
	Title               string
}

type FrameEvent struct {
	system.FrameEvent

	Sync bool
}

type Callbacks interface {
	SetDriver(d Driver)
	Event(e event.Event)
}

type Context interface {
	Backend() (backend.Device, error)
	Present() error
	MakeCurrent() error
	Release()
	Lock()
	Unlock()
}

// ErrDeviceLost is returned from Context.Present when
// the underlying GPU device is gone and should be
// recreated.
var ErrDeviceLost = errors.New("GPU device lost")

// Driver is the interface for the platform implementation
// of a window.
type Driver interface {
	// SetAnimating sets the animation flag. When the window is animating,
	// FrameEvents are delivered as fast as the display can handle them.
	SetAnimating(anim bool)
	// ShowTextInput updates the virtual keyboard state.
	ShowTextInput(show bool)
	NewContext() (Context, error)

	// ReadClipboard requests the clipboard content.
	ReadClipboard()
	// WriteClipboard requests a clipboard write.
	WriteClipboard(s string)

	// Close the window.
	Close()
}

type windowRendezvous struct {
	in   chan windowAndOptions
	out  chan windowAndOptions
	errs chan error
}

type windowAndOptions struct {
	window Callbacks
	opts   *Options
}

func newWindowRendezvous() *windowRendezvous {
	wr := &windowRendezvous{
		in:   make(chan windowAndOptions),
		out:  make(chan windowAndOptions),
		errs: make(chan error),
	}
	go func() {
		var main windowAndOptions
		var out chan windowAndOptions
		for {
			select {
			case w := <-wr.in:
				var err error
				if main.window != nil {
					err = errors.New("multiple windows are not supported")
				}
				wr.errs <- err
				main = w
				out = wr.out
			case out <- main:
			}
		}
	}()
	return wr
}
