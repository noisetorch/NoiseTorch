// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"io"

	"golang.org/x/image/math/fixed"

	"gioui.org/op"
)

// A Line contains the measurements of a line of text.
type Line struct {
	Layout Layout
	// Width is the width of the line.
	Width fixed.Int26_6
	// Ascent is the height above the baseline.
	Ascent fixed.Int26_6
	// Descent is the height below the baseline, including
	// the line gap.
	Descent fixed.Int26_6
	// Bounds is the visible bounds of the line.
	Bounds fixed.Rectangle26_6
}

type Layout struct {
	Text     string
	Advances []fixed.Int26_6
}

// Style is the font style.
type Style int

// Weight is a font weight, in CSS units subtracted 400 so the zero value
// is normal text weight.
type Weight int

// Font specify a particular typeface variant, style and weight.
type Font struct {
	Typeface Typeface
	Variant  Variant
	Style    Style
	// Weight is the text weight. If zero, Normal is used instead.
	Weight Weight
}

// Face implements text layout and shaping for a particular font. All
// methods must be safe for concurrent use.
type Face interface {
	Layout(ppem fixed.Int26_6, maxWidth int, txt io.Reader) ([]Line, error)
	Shape(ppem fixed.Int26_6, str Layout) op.CallOp
}

// Typeface identifies a particular typeface design. The empty
// string denotes the default typeface.
type Typeface string

// Variant denotes a typeface variant such as "Mono" or "Smallcaps".
type Variant string

type Alignment uint8

const (
	Start Alignment = iota
	End
	Middle
)

const (
	Regular Style = iota
	Italic
)

const (
	Normal Weight = 400 - 400
	Medium Weight = 500 - 400
	Bold   Weight = 600 - 400
)

func (a Alignment) String() string {
	switch a {
	case Start:
		return "Start"
	case End:
		return "End"
	case Middle:
		return "Middle"
	default:
		panic("unreachable")
	}
}
