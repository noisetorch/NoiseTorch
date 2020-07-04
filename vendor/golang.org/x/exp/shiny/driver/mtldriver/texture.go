// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package mtldriver

import (
	"image"
	"image/color"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/draw"
)

// textureImpl implements screen.Texture.
type textureImpl struct {
	rgba *image.RGBA
}

func (*textureImpl) Release()                  {}
func (t *textureImpl) Size() image.Point       { return t.rgba.Rect.Max }
func (t *textureImpl) Bounds() image.Rectangle { return t.rgba.Rect }

func (t *textureImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	draw.Draw(t.rgba, sr.Sub(sr.Min).Add(dp), src.RGBA(), sr.Min, draw.Src)
}

func (t *textureImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {
	draw.Draw(t.rgba, dr, &image.Uniform{src}, image.Point{}, op)
}
