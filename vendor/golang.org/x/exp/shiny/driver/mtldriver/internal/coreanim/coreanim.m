// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

#import <QuartzCore/QuartzCore.h>
#include "coreanim.h"

void * MakeMetalLayer() {
	return [[CAMetalLayer alloc] init];
}

uint16_t MetalLayer_PixelFormat(void * metalLayer) {
	return ((CAMetalLayer *)metalLayer).pixelFormat;
}

void MetalLayer_SetDevice(void * metalLayer, void * device) {
	((CAMetalLayer *)metalLayer).device = (id<MTLDevice>)device;
}

const char * MetalLayer_SetPixelFormat(void * metalLayer, uint16_t pixelFormat) {
	@try {
		((CAMetalLayer *)metalLayer).pixelFormat = (MTLPixelFormat)pixelFormat;
	}
	@catch (NSException * exception) {
		return exception.reason.UTF8String;
	}
	return NULL;
}

const char * MetalLayer_SetMaximumDrawableCount(void * metalLayer, uint_t maximumDrawableCount) {
	if (@available(macOS 10.13.2, *)) {
		@try {
			((CAMetalLayer *)metalLayer).maximumDrawableCount = (NSUInteger)maximumDrawableCount;
		}
		@catch (NSException * exception) {
			return exception.reason.UTF8String;
		}
	}
	return NULL;
}

void MetalLayer_SetDisplaySyncEnabled(void * metalLayer, BOOL displaySyncEnabled) {
	((CAMetalLayer *)metalLayer).displaySyncEnabled = displaySyncEnabled;
}

void MetalLayer_SetDrawableSize(void * metalLayer, double width, double height) {
	((CAMetalLayer *)metalLayer).drawableSize = (CGSize){width, height};
}

void * MetalLayer_NextDrawable(void * metalLayer) {
	return [(CAMetalLayer *)metalLayer nextDrawable];
}

void * MetalDrawable_Texture(void * metalDrawable) {
	return ((id<CAMetalDrawable>)metalDrawable).texture;
}
