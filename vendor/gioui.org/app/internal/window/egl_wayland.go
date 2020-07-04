// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nowayland freebsd

package window

import (
	"errors"
	"unsafe"

	"gioui.org/app/internal/egl"
)

/*
#cgo linux pkg-config: egl wayland-egl
#cgo freebsd openbsd LDFLAGS: -lwayland-egl
#cgo CFLAGS: -DMESA_EGL_NO_X11_HEADERS

#include <EGL/egl.h>
#include <wayland-client.h>
#include <wayland-egl.h>
*/
import "C"

type context struct {
	win *window
	*egl.Context
	eglWin *C.struct_wl_egl_window
}

func (w *window) NewContext() (Context, error) {
	disp := egl.NativeDisplayType(unsafe.Pointer(w.display()))
	ctx, err := egl.NewContext(disp)
	if err != nil {
		return nil, err
	}
	return &context{Context: ctx, win: w}, nil
}

func (c *context) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
	if c.eglWin != nil {
		C.wl_egl_window_destroy(c.eglWin)
		c.eglWin = nil
	}
}

func (c *context) MakeCurrent() error {
	c.Context.ReleaseSurface()
	if c.eglWin != nil {
		C.wl_egl_window_destroy(c.eglWin)
		c.eglWin = nil
	}
	surf, width, height := c.win.surface()
	if surf == nil {
		return errors.New("wayland: no surface")
	}
	eglWin := C.wl_egl_window_create(surf, C.int(width), C.int(height))
	if eglWin == nil {
		return errors.New("wayland: wl_egl_window_create failed")
	}
	c.eglWin = eglWin
	eglSurf := egl.NativeWindowType(uintptr(unsafe.Pointer(eglWin)))
	if err := c.Context.CreateSurface(eglSurf, width, height); err != nil {
		return err
	}
	return c.Context.MakeCurrent()
}

func (c *context) Lock() {}

func (c *context) Unlock() {}
