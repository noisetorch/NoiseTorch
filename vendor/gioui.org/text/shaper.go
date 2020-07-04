// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"io"
	"strings"

	"gioui.org/op"
	"golang.org/x/image/math/fixed"
)

// Shaper implements layout and shaping of text.
type Shaper interface {
	// Layout a text according to a set of options.
	Layout(font Font, size fixed.Int26_6, maxWidth int, txt io.Reader) ([]Line, error)
	// Shape a line of text and return a clipping operation for its outline.
	Shape(font Font, size fixed.Int26_6, layout []Glyph) op.CallOp

	// LayoutString is like Layout, but for strings.
	LayoutString(font Font, size fixed.Int26_6, maxWidth int, str string) []Line
	// ShapeString is like Shape for lines previously laid out by LayoutString.
	ShapeString(font Font, size fixed.Int26_6, str string, layout []Glyph) op.CallOp
}

// A FontFace is a Font and a matching Face.
type FontFace struct {
	Font Font
	Face Face
}

// Cache implements cached layout and shaping of text from a set of
// registered fonts.
//
// If a font matches no registered shape, Cache falls back to the
// first registered face.
//
// The LayoutString and ShapeString results are cached and re-used if
// possible.
type Cache struct {
	def   Typeface
	faces map[Font]*faceCache
}

type faceCache struct {
	face        Face
	layoutCache layoutCache
	pathCache   pathCache
}

func (c *Cache) lookup(font Font) *faceCache {
	var f *faceCache
	f = c.faceForStyle(font)
	if f == nil {
		font.Typeface = c.def
		f = c.faceForStyle(font)
	}
	return f
}

func (c *Cache) faceForStyle(font Font) *faceCache {
	tf := c.faces[font]
	if tf == nil {
		font := font
		font.Weight = Normal
		tf = c.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		tf = c.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		font.Weight = Normal
		tf = c.faces[font]
	}
	return tf
}

func NewCache(collection []FontFace) *Cache {
	c := &Cache{
		faces: make(map[Font]*faceCache),
	}
	for i, ff := range collection {
		if ff.Font.Weight == 0 {
			ff.Font.Weight = Normal
		}
		if i == 0 {
			c.def = ff.Font.Typeface
		}
		c.faces[ff.Font] = &faceCache{face: ff.Face}
	}
	return c
}

func (s *Cache) Layout(font Font, size fixed.Int26_6, maxWidth int, txt io.Reader) ([]Line, error) {
	cache := s.lookup(font)
	return cache.face.Layout(size, maxWidth, txt)
}

func (s *Cache) Shape(font Font, size fixed.Int26_6, layout []Glyph) op.CallOp {
	cache := s.lookup(font)
	return cache.face.Shape(size, layout)
}

func (s *Cache) LayoutString(font Font, size fixed.Int26_6, maxWidth int, str string) []Line {
	cache := s.lookup(font)
	return cache.layout(size, maxWidth, str)
}

func (s *Cache) ShapeString(font Font, size fixed.Int26_6, str string, layout []Glyph) op.CallOp {
	cache := s.lookup(font)
	return cache.shape(size, str, layout)
}

func (f *faceCache) layout(ppem fixed.Int26_6, maxWidth int, str string) []Line {
	if f == nil {
		return nil
	}
	lk := layoutKey{
		ppem:     ppem,
		maxWidth: maxWidth,
		str:      str,
	}
	if l, ok := f.layoutCache.Get(lk); ok {
		return l
	}
	l, _ := f.face.Layout(ppem, maxWidth, strings.NewReader(str))
	f.layoutCache.Put(lk, l)
	return l
}

func (f *faceCache) shape(ppem fixed.Int26_6, str string, layout []Glyph) op.CallOp {
	if f == nil {
		return op.CallOp{}
	}
	pk := pathKey{
		ppem: ppem,
		str:  str,
	}
	if clip, ok := f.pathCache.Get(pk); ok {
		return clip
	}
	clip := f.face.Shape(ppem, layout)
	f.pathCache.Put(pk, clip)
	return clip
}
