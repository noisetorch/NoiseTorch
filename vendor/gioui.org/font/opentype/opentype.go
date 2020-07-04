// SPDX-License-Identifier: Unlicense OR MIT

// Package opentype implements text layout and shaping for OpenType
// files.
package opentype

import (
	"io"

	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Font implements text.Face.
type Font struct {
	font *sfnt.Font
	buf  sfnt.Buffer
}

// Collection is a collection of one or more fonts.
type Collection struct {
	coll *sfnt.Collection
}

type opentype struct {
	Font    *sfnt.Font
	Hinting font.Hinting
}

// NewFont parses an SFNT font, such as TTF or OTF data, from a []byte
// data source.
func Parse(src []byte) (*Font, error) {
	fnt, err := sfnt.Parse(src)
	if err != nil {
		return nil, err
	}
	return &Font{font: fnt}, nil
}

// ParseCollection parses an SFNT font collection, such as TTC or OTC data,
// from a []byte data source.
//
// If passed data for a single font, a TTF or OTF instead of a TTC or OTC,
// it will return a collection containing 1 font.
func ParseCollection(src []byte) (*Collection, error) {
	c, err := sfnt.ParseCollection(src)
	if err != nil {
		return nil, err
	}
	return &Collection{c}, nil
}

// ParseCollectionReaderAt parses an SFNT collection, such as TTC or OTC data,
// from an io.ReaderAt data source.
//
// If passed data for a single font, a TTF or OTF instead of a TTC or OTC, it
// will return a collection containing 1 font.
func ParseCollectionReaderAt(src io.ReaderAt) (*Collection, error) {
	c, err := sfnt.ParseCollectionReaderAt(src)
	if err != nil {
		return nil, err
	}
	return &Collection{c}, nil
}

// NumFonts returns the number of fonts in the collection.
func (c *Collection) NumFonts() int {
	return c.coll.NumFonts()
}

// Font returns the i'th font in the collection.
func (c *Collection) Font(i int) (*Font, error) {
	fnt, err := c.coll.Font(i)
	if err != nil {
		return nil, err
	}
	return &Font{font: fnt}, nil
}

func (f *Font) Layout(ppem fixed.Int26_6, maxWidth int, txt io.Reader) ([]text.Line, error) {
	glyphs, err := readGlyphs(txt)
	if err != nil {
		return nil, err
	}
	return layoutText(&f.buf, ppem, maxWidth, &opentype{Font: f.font, Hinting: font.HintingFull}, glyphs)
}

func (f *Font) Shape(ppem fixed.Int26_6, str []text.Glyph) op.CallOp {
	return textPath(&f.buf, ppem, &opentype{Font: f.font, Hinting: font.HintingFull}, str)
}

func (f *Font) Metrics(ppem fixed.Int26_6) font.Metrics {
	o := &opentype{Font: f.font, Hinting: font.HintingFull}
	return o.Metrics(&f.buf, ppem)
}

func layoutText(sbuf *sfnt.Buffer, ppem fixed.Int26_6, maxWidth int, f *opentype, glyphs []text.Glyph) ([]text.Line, error) {
	m := f.Metrics(sbuf, ppem)
	lineTmpl := text.Line{
		Ascent: m.Ascent,
		// m.Height is equal to m.Ascent + m.Descent + linegap.
		// Compute the descent including the linegap.
		Descent: m.Height - m.Ascent,
		Bounds:  f.Bounds(sbuf, ppem),
	}
	var lines []text.Line
	maxDotX := fixed.I(maxWidth)
	type state struct {
		r     rune
		adv   fixed.Int26_6
		x     fixed.Int26_6
		idx   int
		len   int
		valid bool
	}
	var prev, word state
	endLine := func() {
		line := lineTmpl
		line.Layout = glyphs[:prev.idx:prev.idx]
		line.Len = prev.len
		line.Width = prev.x + prev.adv
		line.Bounds.Max.X += prev.x
		lines = append(lines, line)
		glyphs = glyphs[prev.idx:]
		prev = state{}
		word = state{}
	}
	for prev.idx < len(glyphs) {
		g := &glyphs[prev.idx]
		a, valid := f.GlyphAdvance(sbuf, ppem, g.Rune)
		next := state{
			r:     g.Rune,
			idx:   prev.idx + 1,
			len:   prev.len + utf8.RuneLen(g.Rune),
			x:     prev.x + prev.adv,
			adv:   a,
			valid: valid,
		}
		if g.Rune == '\n' {
			// The newline is zero width; use the previous
			// character for line measurements.
			prev.idx = next.idx
			prev.len = next.len
			endLine()
			continue
		}
		var k fixed.Int26_6
		if prev.valid {
			k = f.Kern(sbuf, ppem, prev.r, next.r)
		}
		// Break the line if we're out of space.
		if prev.idx > 0 && next.x+next.adv+k > maxDotX {
			// If the line contains no word breaks, break off the last rune.
			if word.idx == 0 {
				word = prev
			}
			next.x -= word.x + word.adv
			next.idx -= word.idx
			next.len -= word.len
			prev = word
			endLine()
		} else if k != 0 {
			glyphs[prev.idx-1].Advance += k
			next.x += k
		}
		g.Advance = next.adv
		if unicode.IsSpace(g.Rune) {
			word = next
		}
		prev = next
	}
	endLine()
	return lines, nil
}

func textPath(buf *sfnt.Buffer, ppem fixed.Int26_6, f *opentype, str []text.Glyph) op.CallOp {
	var lastPos f32.Point
	var builder clip.Path
	ops := new(op.Ops)
	m := op.Record(ops)
	var x fixed.Int26_6
	builder.Begin(ops)
	for _, g := range str {
		if !unicode.IsSpace(g.Rune) {
			segs, ok := f.LoadGlyph(buf, ppem, g.Rune)
			if !ok {
				continue
			}
			// Move to glyph position.
			pos := f32.Point{
				X: float32(x) / 64,
			}
			builder.Move(pos.Sub(lastPos))
			lastPos = pos
			var lastArg f32.Point
			// Convert sfnt.Segments to relative segments.
			for _, fseg := range segs {
				nargs := 1
				switch fseg.Op {
				case sfnt.SegmentOpQuadTo:
					nargs = 2
				case sfnt.SegmentOpCubeTo:
					nargs = 3
				}
				var args [3]f32.Point
				for i := 0; i < nargs; i++ {
					a := f32.Point{
						X: float32(fseg.Args[i].X) / 64,
						Y: float32(fseg.Args[i].Y) / 64,
					}
					args[i] = a.Sub(lastArg)
					if i == nargs-1 {
						lastArg = a
					}
				}
				switch fseg.Op {
				case sfnt.SegmentOpMoveTo:
					builder.Move(args[0])
				case sfnt.SegmentOpLineTo:
					builder.Line(args[0])
				case sfnt.SegmentOpQuadTo:
					builder.Quad(args[0], args[1])
				case sfnt.SegmentOpCubeTo:
					builder.Cube(args[0], args[1], args[2])
				default:
					panic("unsupported segment op")
				}
			}
			lastPos = lastPos.Add(lastArg)
		}
		x += g.Advance
	}
	builder.End().Add(ops)
	return m.Stop()
}

func readGlyphs(r io.Reader) ([]text.Glyph, error) {
	var glyphs []text.Glyph
	buf := make([]byte, 0, 1024)
	for {
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		lim := len(buf)
		// Read full runes if possible.
		if err != io.EOF {
			lim -= utf8.UTFMax - 1
		}
		i := 0
		for i < lim {
			c, s := utf8.DecodeRune(buf[i:])
			i += s
			glyphs = append(glyphs, text.Glyph{Rune: c})
		}
		n = copy(buf, buf[i:])
		buf = buf[:n]
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return glyphs, nil
}

func (f *opentype) GlyphAdvance(buf *sfnt.Buffer, ppem fixed.Int26_6, r rune) (advance fixed.Int26_6, ok bool) {
	g, err := f.Font.GlyphIndex(buf, r)
	if err != nil {
		return 0, false
	}
	adv, err := f.Font.GlyphAdvance(buf, g, ppem, f.Hinting)
	return adv, err == nil
}

func (f *opentype) Kern(buf *sfnt.Buffer, ppem fixed.Int26_6, r0, r1 rune) fixed.Int26_6 {
	g0, err := f.Font.GlyphIndex(buf, r0)
	if err != nil {
		return 0
	}
	g1, err := f.Font.GlyphIndex(buf, r1)
	if err != nil {
		return 0
	}
	adv, err := f.Font.Kern(buf, g0, g1, ppem, f.Hinting)
	if err != nil {
		return 0
	}
	return adv
}

func (f *opentype) Metrics(buf *sfnt.Buffer, ppem fixed.Int26_6) font.Metrics {
	m, _ := f.Font.Metrics(buf, ppem, f.Hinting)
	return m
}

func (f *opentype) Bounds(buf *sfnt.Buffer, ppem fixed.Int26_6) fixed.Rectangle26_6 {
	r, _ := f.Font.Bounds(buf, ppem, f.Hinting)
	return r
}

func (f *opentype) LoadGlyph(buf *sfnt.Buffer, ppem fixed.Int26_6, r rune) ([]sfnt.Segment, bool) {
	g, err := f.Font.GlyphIndex(buf, r)
	if err != nil {
		return nil, false
	}
	segs, err := f.Font.LoadGlyph(buf, g, ppem, nil)
	if err != nil {
		return nil, false
	}
	return segs, true
}
