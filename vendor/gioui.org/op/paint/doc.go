// SPDX-License-Identifier: Unlicense OR MIT

/*
Package paint provides drawing operations for 2D graphics.

The PaintOp operation fills the current clip with the current brush,
taking the current transformation into account.

The current brush is set by either a ColorOp for a constant color, or
ImageOp for an image, or LinearGradientOp for gradients.

All color.NRGBA values are in the sRGB color space.
*/
package paint
