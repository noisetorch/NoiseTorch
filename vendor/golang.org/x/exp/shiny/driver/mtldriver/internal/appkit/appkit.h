// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

typedef signed char BOOL;

void * Window_ContentView(void * window);

void View_SetLayer(void * view, void * layer);
void View_SetWantsLayer(void * view, BOOL wantsLayer);
