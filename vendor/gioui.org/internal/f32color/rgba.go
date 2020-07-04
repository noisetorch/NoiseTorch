// SPDX-License-Identifier: Unlicense OR MIT

package f32color

import (
	"image/color"
	"math"
)

// RGBA is a 32 bit floating point linear space color.
type RGBA struct {
	R, G, B, A float32
}

// Array returns rgba values in a [4]float32 array.
func (rgba RGBA) Array() [4]float32 {
	return [4]float32{rgba.R, rgba.G, rgba.B, rgba.A}
}

// Float32 returns r, g, b, a values.
func (col RGBA) Float32() (r, g, b, a float32) {
	return col.R, col.G, col.B, col.A
}

// SRGBA converts from linear to sRGB color space.
func (col RGBA) SRGB() color.RGBA {
	return color.RGBA{
		R: uint8(linearTosRGB(col.R)*255 + .5),
		G: uint8(linearTosRGB(col.G)*255 + .5),
		B: uint8(linearTosRGB(col.B)*255 + .5),
		A: uint8(col.A*255 + .5),
	}
}

// Opaque returns the color without alpha component.
func (col RGBA) Opaque() RGBA {
	col.A = 1.0
	return col
}

// RGBAFromSRGB converts from SRGBA to RGBA.
func RGBAFromSRGB(col color.RGBA) RGBA {
	r, g, b, a := col.RGBA()
	return RGBA{
		R: sRGBToLinear(float32(r) / 0xffff),
		G: sRGBToLinear(float32(g) / 0xffff),
		B: sRGBToLinear(float32(b) / 0xffff),
		A: float32(a) / 0xFFFF,
	}
}

// linearTosRGB transforms color value from linear to sRGB.
func linearTosRGB(c float32) float32 {
	// Formula from EXT_sRGB.
	switch {
	case c <= 0:
		return 0
	case 0 < c && c < 0.0031308:
		return 12.92 * c
	case 0.0031308 <= c && c < 1:
		return 1.055*float32(math.Pow(float64(c), 0.41666)) - 0.055
	}

	return 1
}

// sRGBToLinear transforms color value from sRGB to linear.
func sRGBToLinear(c float32) float32 {
	// Formula from EXT_sRGB.
	if c <= 0.04045 {
		return c / 12.92
	} else {
		return float32(math.Pow(float64((c+0.055)/1.055), 2.4))
	}
}
