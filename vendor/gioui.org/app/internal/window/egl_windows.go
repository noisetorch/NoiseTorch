// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"gioui.org/app/internal/egl"
)

type glContext struct {
	win *window
	*egl.Context
}

func init() {
	backends = append(backends, gpuAPI{
		priority: 2,
		initializer: func(w *window) (Context, error) {
			disp := egl.NativeDisplayType(w.HDC())
			ctx, err := egl.NewContext(disp)
			if err != nil {
				return nil, err
			}
			return &glContext{win: w, Context: ctx}, nil
		},
	})
}

func (c *glContext) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *glContext) MakeCurrent() error {
	c.Context.ReleaseSurface()
	win, width, height := c.win.HWND()
	eglSurf := egl.NativeWindowType(win)
	if err := c.Context.CreateSurface(eglSurf, width, height); err != nil {
		return err
	}
	if err := c.Context.MakeCurrent(); err != nil {
		return err
	}
	c.Context.EnableVSync(true)
	return nil
}

func (c *glContext) Lock() {}

func (c *glContext) Unlock() {}
