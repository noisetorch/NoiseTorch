package font

import (
	"github.com/aarzilli/nucular/internal/assets"
)

// Returns default font (DroidSansMono) with specified size and scaling
func DefaultFont(size int, scaling float64) Face {
	fontData, _ := assets.Asset("DroidSansMono.ttf")
	face, err := NewFace(fontData, int(float64(size)*scaling))
	if err != nil {
		panic(err)
	}
	return face
}
