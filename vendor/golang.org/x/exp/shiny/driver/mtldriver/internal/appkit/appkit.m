// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

#import <Cocoa/Cocoa.h>
#include "appkit.h"

void * Window_ContentView(void * window) {
	return ((NSWindow *)window).contentView;
}

void View_SetLayer(void * view, void * layer) {
	((NSView *)view).layer = (CALayer *)layer;
}

void View_SetWantsLayer(void * view, BOOL wantsLayer) {
	((NSView *)view).wantsLayer = wantsLayer;
}
