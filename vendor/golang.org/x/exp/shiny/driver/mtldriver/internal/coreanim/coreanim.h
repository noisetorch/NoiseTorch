// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

typedef signed char BOOL;
typedef unsigned long uint_t;
typedef unsigned short uint16_t;

void * MakeMetalLayer();

uint16_t     MetalLayer_PixelFormat(void * metalLayer);
void         MetalLayer_SetDevice(void * metalLayer, void * device);
const char * MetalLayer_SetPixelFormat(void * metalLayer, uint16_t pixelFormat);
const char * MetalLayer_SetMaximumDrawableCount(void * metalLayer, uint_t maximumDrawableCount);
void         MetalLayer_SetDisplaySyncEnabled(void * metalLayer, BOOL displaySyncEnabled);
void         MetalLayer_SetDrawableSize(void * metalLayer, double width, double height);
void *       MetalLayer_NextDrawable(void * metalLayer);

void * MetalDrawable_Texture(void * drawable);
