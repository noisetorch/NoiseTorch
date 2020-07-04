// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package mtldriver

import "image"

// bufferImpl implements screen.Buffer.
type bufferImpl struct {
	rgba *image.RGBA
}

func (*bufferImpl) Release()                  {}
func (b *bufferImpl) Size() image.Point       { return b.rgba.Rect.Max }
func (b *bufferImpl) Bounds() image.Rectangle { return b.rgba.Rect }
func (b *bufferImpl) RGBA() *image.RGBA       { return b.rgba }
