// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

package window

import (
	"gioui.org/app/internal/glimpl"
	"gioui.org/gpu/backend"
	"gioui.org/gpu/gl"
)

/*
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <AppKit/AppKit.h>
#include <OpenGL/gl3.h>

__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createGLView(void);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_contextForView(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_makeCurrentContext(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_flushContextBuffer(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_clearCurrentContext(void);
__attribute__ ((visibility ("hidden"))) void gio_lockContext(CFTypeRef ctxRef);
__attribute__ ((visibility ("hidden"))) void gio_unlockContext(CFTypeRef ctxRef);
*/
import "C"

type context struct {
	c    *glimpl.Functions
	ctx  C.CFTypeRef
	view C.CFTypeRef
}

func init() {
	viewFactory = func() C.CFTypeRef {
		return C.gio_createGLView()
	}
}

func newContext(w *window) (*context, error) {
	view := w.contextView()
	ctx := C.gio_contextForView(view)
	c := &context{
		ctx:  ctx,
		c:    new(glimpl.Functions),
		view: view,
	}
	return c, nil
}

func (c *context) Backend() (backend.Device, error) {
	return gl.NewBackend(c.c)
}

func (c *context) Release() {
	c.Lock()
	defer c.Unlock()
	C.gio_clearCurrentContext()
	// We could release the context with [view clearGLContext]
	// and rely on [view openGLContext] auto-creating a new context.
	// However that second context is not properly set up by
	// OpenGLContextView, so we'll stay on the safe side and keep
	// the first context around.
}

func (c *context) Present() error {
	// Assume the caller already locked the context.
	C.glFlush()
	return nil
}

func (c *context) Lock() {
	C.gio_lockContext(c.ctx)
}

func (c *context) Unlock() {
	C.gio_unlockContext(c.ctx)
}

func (c *context) MakeCurrent() error {
	c.Lock()
	defer c.Unlock()
	C.gio_makeCurrentContext(c.ctx)
	return nil
}

func (w *window) NewContext() (Context, error) {
	return newContext(w)
}
