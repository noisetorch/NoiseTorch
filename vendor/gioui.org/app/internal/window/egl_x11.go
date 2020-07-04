// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nox11 freebsd openbsd

package window

import (
	"unsafe"

	"gioui.org/app/internal/egl"
)

type x11Context struct {
	win *x11Window
	*egl.Context
}

func (w *x11Window) NewContext() (Context, error) {
	disp := egl.NativeDisplayType(unsafe.Pointer(w.display()))
	ctx, err := egl.NewContext(disp)
	if err != nil {
		return nil, err
	}
	return &x11Context{win: w, Context: ctx}, nil
}

func (c *x11Context) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *x11Context) MakeCurrent() error {
	c.Context.ReleaseSurface()
	win, width, height := c.win.window()
	eglSurf := egl.NativeWindowType(uintptr(win))
	if err := c.Context.CreateSurface(eglSurf, width, height); err != nil {
		return err
	}
	if err := c.Context.MakeCurrent(); err != nil {
		return err
	}
	c.Context.EnableVSync(true)
	return nil
}

func (c *x11Context) Lock() {}

func (c *x11Context) Unlock() {}
