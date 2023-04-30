// SPDX-License-Identifier: Unlicense OR MIT

package wm

import (
	"errors"
	"syscall/js"

	"gioui.org/gpu"
	"gioui.org/internal/gl"
	"gioui.org/internal/srgb"
)

type context struct {
	ctx     js.Value
	cnv     js.Value
	srgbFBO *srgb.FBO
}

func newContext(w *window) (*context, error) {
	args := map[string]interface{}{
		// Enable low latency rendering.
		// See https://developers.google.com/web/updates/2019/05/desynchronized.
		"desynchronized":        true,
		"preserveDrawingBuffer": true,
	}
	ctx := w.cnv.Call("getContext", "webgl2", args)
	if ctx.IsNull() {
		ctx = w.cnv.Call("getContext", "webgl", args)
	}
	if ctx.IsNull() {
		return nil, errors.New("app: webgl is not supported")
	}
	c := &context{
		ctx: ctx,
		cnv: w.cnv,
	}
	return c, nil
}

func (c *context) API() gpu.API {
	return gpu.OpenGL{Context: gl.Context(c.ctx)}
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
		c.srgbFBO, err = srgb.New(gl.Context(c.ctx))
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
