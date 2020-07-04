// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

// Package appkit provides access to Apple's AppKit API
// (https://developer.apple.com/documentation/appkit).
//
// This package is in very early stages of development.
// It's a minimal implementation with scope limited to
// supporting mtldriver.
//
// It was copied from dmitri.shuralyov.com/gpu/mtl/example/movingtriangle/internal/ns.
package appkit

import (
	"unsafe"

	"golang.org/x/exp/shiny/driver/mtldriver/internal/coreanim"
)

/*
#include "appkit.h"
*/
import "C"

// Window is a window that an app displays on the screen.
//
// Reference: https://developer.apple.com/documentation/appkit/nswindow.
type Window struct {
	window unsafe.Pointer
}

// NewWindow returns a Window that wraps an existing NSWindow * pointer.
func NewWindow(window unsafe.Pointer) Window {
	return Window{window}
}

// ContentView returns the window's content view, the highest accessible View
// in the window's view hierarchy.
//
// Reference: https://developer.apple.com/documentation/appkit/nswindow/1419160-contentview.
func (w Window) ContentView() View {
	return View{C.Window_ContentView(w.window)}
}

// View is the infrastructure for drawing, printing, and handling events in an app.
//
// Reference: https://developer.apple.com/documentation/appkit/nsview.
type View struct {
	view unsafe.Pointer
}

// SetLayer sets v.layer to l.
//
// Reference: https://developer.apple.com/documentation/appkit/nsview/1483298-layer.
func (v View) SetLayer(l coreanim.Layer) {
	C.View_SetLayer(v.view, l.Layer())
}

// SetWantsLayer sets v.wantsLayer to wantsLayer.
//
// Reference: https://developer.apple.com/documentation/appkit/nsview/1483695-wantslayer.
func (v View) SetWantsLayer(wantsLayer bool) {
	C.View_SetWantsLayer(v.view, toCBool(wantsLayer))
}

func toCBool(b bool) C.BOOL {
	if b {
		return 1
	}
	return 0
}
