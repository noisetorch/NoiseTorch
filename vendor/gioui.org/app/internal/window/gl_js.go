// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"errors"
	"syscall/js"

	"gioui.org/app/internal/glimpl"
	"gioui.org/app/internal/srgb"
	"gioui.org/gpu/backend"
	"gioui.org/gpu/gl"
)

type context struct {
	ctx     js.Value
	cnv     js.Value
	f       *glimpl.Functions
	srgbFBO *srgb.FBO
}

func newContext(w *window) (*context, error) {
	args := map[string]interface{}{
		// Enable low latency rendering.
		// See https://developers.google.com/web/updates/2019/05/desynchronized.
		"desynchronized":        true,
		"preserveDrawingBuffer": true,
	}
	version := 2
	ctx := w.cnv.Call("getContext", "webgl2", args)
	if ctx.IsNull() {
		version = 1
		ctx = w.cnv.Call("getContext", "webgl", args)
	}
	if ctx.IsNull() {
		return nil, errors.New("app: webgl is not supported")
	}
	f := &glimpl.Functions{Ctx: ctx}
	if err := f.Init(version); err != nil {
		return nil, err
	}
	c := &context{
		ctx: ctx,
		cnv: w.cnv,
		f:   f,
	}
	return c, nil
}

func (c *context) Backend() (backend.Device, error) {
	return gl.NewBackend(c.f)
}

func (c *context) Release() {
	if c.srgbFBO != nil {
		c.srgbFBO.Release()
		c.srgbFBO = nil
	}
}

func (c *context) Present() error {
	if c.srgbFBO != nil {
		c.srgbFBO.Blit()
	}
	if c.srgbFBO != nil {
		c.srgbFBO.AfterPresent()
	}
	if c.ctx.Call("isContextLost").Bool() {
		return errors.New("context lost")
	}
	return nil
}

func (c *context) Lock() {}

func (c *context) Unlock() {}

func (c *context) MakeCurrent() error {
	if c.srgbFBO == nil {
		var err error
		c.srgbFBO, err = srgb.New(c.f)
		if err != nil {
			c.Release()
			c.srgbFBO = nil
			return err
		}
	}
	w, h := c.cnv.Get("width").Int(), c.cnv.Get("height").Int()
	if err := c.srgbFBO.Refresh(w, h); err != nil {
		c.Release()
		return err
	}
	return nil
}

func (w *window) NewContext() (Context, error) {
	return newContext(w)
}
