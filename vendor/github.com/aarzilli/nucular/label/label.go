package label

import (
	"image"
	"image/color"
)

type Label struct {
	Kind   LabelKind
	Text   string
	Img    *image.RGBA
	Color  color.RGBA
	Symbol SymbolType
	Align  Align
}

type LabelKind int

const (
	ColorLabel LabelKind = iota
	ImageLabel
	ImageTextLabel
	SymbolLabel
	SymbolTextLabel
	TextLabel
)

// Text alignment.
// A two character string, the first character is horizontal alignment, the second character vertical alignment.
// For the first character: L (left), C (centered), R (right)
// For the second character: T (top), C (centered), B (bottom)
type Align string

type SymbolType int

const (
	SymbolNone SymbolType = iota
	SymbolX
	SymbolUnderscore
	SymbolCircle
	SymbolCircleFilled
	SymbolRect
	SymbolRectFilled
	SymbolTriangleUp
	SymbolTriangleDown
	SymbolTriangleLeft
	SymbolTriangleRight
	SymbolPlus
	SymbolMinus
)

func T(text string) Label {
	return Label{Kind: TextLabel, Text: text, Align: ""}
}

func TA(text string, align Align) Label {
	return Label{Kind: TextLabel, Text: text, Align: align}
}

func C(color color.RGBA) Label {
	return Label{Kind: ColorLabel, Color: color}
}

func I(img *image.RGBA) Label {
	return Label{Kind: ImageLabel, Img: img}
}

func IT(img *image.RGBA, text string, align Align) Label {
	return Label{Kind: ImageTextLabel, Img: img, Text: text, Align: align}
}

func S(s SymbolType) Label {
	return Label{Kind: SymbolLabel, Symbol: s}
}

func ST(s SymbolType, text string, align Align) Label {
	return Label{Kind: SymbolTextLabel, Symbol: s, Text: text, Align: align}
}
