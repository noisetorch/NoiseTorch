// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"encoding/binary"
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/op"
)

// Path constructs a Op clip path described by lines and
// Bézier curves, where drawing outside the Path is discarded.
// The inside-ness of a pixel is determines by the even-odd rule,
// similar to the SVG rule of the same name.
//
// Path generates no garbage and can be used for dynamic paths; path
// data is stored directly in the Ops list supplied to Begin.
type Path struct {
	ops     *op.Ops
	contour int
	pen     f32.Point
	macro   op.MacroOp
	start   f32.Point
}

// Op sets the current clip to the intersection of
// the existing clip with this clip.
//
// If you need to reset the clip to its previous values after
// applying a Op, use op.StackOp.
type Op struct {
	call   op.CallOp
	bounds f32.Rectangle
}

func (p Op) Add(o *op.Ops) {
	p.call.Add(o)
	data := o.Write(opconst.TypeClipLen)
	data[0] = byte(opconst.TypeClip)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(p.bounds.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(p.bounds.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(p.bounds.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(p.bounds.Max.Y))
}

// Begin the path, storing the path data and final Op into ops.
func (p *Path) Begin(ops *op.Ops) {
	p.ops = ops
	p.macro = op.Record(ops)
	// Write the TypeAux opcode
	data := ops.Write(opconst.TypeAuxLen)
	data[0] = byte(opconst.TypeAux)
}

// MoveTo moves the pen to the given position.
func (p *Path) Move(to f32.Point) {
	to = to.Add(p.pen)
	p.end()
	p.pen = to
	p.start = to
}

// end completes the current contour.
func (p *Path) end() {
	if p.pen != p.start {
		p.lineTo(p.start)
	}
	p.contour++
}

// Line moves the pen by the amount specified by delta, recording a line.
func (p *Path) Line(delta f32.Point) {
	to := delta.Add(p.pen)
	p.lineTo(to)
}

func (p *Path) lineTo(to f32.Point) {
	// Model lines as degenerate quadratic Béziers.
	p.quadTo(to.Add(p.pen).Mul(.5), to)
}

// Quad records a quadratic Bézier from the pen to end
// with the control point ctrl.
func (p *Path) Quad(ctrl, to f32.Point) {
	ctrl = ctrl.Add(p.pen)
	to = to.Add(p.pen)
	p.quadTo(ctrl, to)
}

func (p *Path) quadTo(ctrl, to f32.Point) {
	data := p.ops.Write(ops.QuadSize + 4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	ops.EncodeQuad(data[4:], ops.Quad{
		From: p.pen,
		Ctrl: ctrl,
		To:   to,
	})
	p.pen = to
}

// Cube records a cubic Bézier from the pen through
// two control points ending in to.
func (p *Path) Cube(ctrl0, ctrl1, to f32.Point) {
	ctrl0 = ctrl0.Add(p.pen)
	ctrl1 = ctrl1.Add(p.pen)
	to = to.Add(p.pen)
	// Set the maximum distance proportionally to the longest side
	// of the bounding rectangle.
	hull := f32.Rectangle{
		Min: p.pen,
		Max: ctrl0,
	}.Canon().Add(ctrl1).Add(to)
	l := hull.Dx()
	if h := hull.Dy(); h > l {
		l = h
	}
	p.approxCubeTo(0, l*0.001, ctrl0, ctrl1, to)
}

// approxCube approximates a cubic Bézier by a series of quadratic
// curves.
func (p *Path) approxCubeTo(splits int, maxDist float32, ctrl0, ctrl1, to f32.Point) int {
	// The idea is from
	// https://caffeineowl.com/graphics/2d/vectorial/cubic2quad01.html
	// where a quadratic approximates a cubic by eliminating its t³ term
	// from its polynomial expression anchored at the starting point:
	//
	// P(t) = pen + 3t(ctrl0 - pen) + 3t²(ctrl1 - 2ctrl0 + pen) + t³(to - 3ctrl1 + 3ctrl0 - pen)
	//
	// The control point for the new quadratic Q1 that shares starting point, pen, with P is
	//
	// C1 = (3ctrl0 - pen)/2
	//
	// The reverse cubic anchored at the end point has the polynomial
	//
	// P'(t) = to + 3t(ctrl1 - to) + 3t²(ctrl0 - 2ctrl1 + to) + t³(pen - 3ctrl0 + 3ctrl1 - to)
	//
	// The corresponding quadratic Q2 that shares the end point, to, with P has control
	// point
	//
	// C2 = (3ctrl1 - to)/2
	//
	// The combined quadratic Bézier, Q, shares both start and end points with its cubic
	// and use the midpoint between the two curves Q1 and Q2 as control point:
	//
	// C = (3ctrl0 - pen + 3ctrl1 - to)/4
	c := ctrl0.Mul(3).Sub(p.pen).Add(ctrl1.Mul(3)).Sub(to).Mul(1.0 / 4.0)
	const maxSplits = 32
	if splits >= maxSplits {
		p.quadTo(c, to)
		return splits
	}
	// The maximum distance between the cubic P and its approximation Q given t
	// can be shown to be
	//
	// d = sqrt(3)/36*|to - 3ctrl1 + 3ctrl0 - pen|
	//
	// To save a square root, compare d² with the squared tolerance.
	v := to.Sub(ctrl1.Mul(3)).Add(ctrl0.Mul(3)).Sub(p.pen)
	d2 := (v.X*v.X + v.Y*v.Y) * 3 / (36 * 36)
	if d2 <= maxDist*maxDist {
		p.quadTo(c, to)
		return splits
	}
	// De Casteljau split the curve and approximate the halves.
	t := float32(0.5)
	c0 := p.pen.Add(ctrl0.Sub(p.pen).Mul(t))
	c1 := ctrl0.Add(ctrl1.Sub(ctrl0).Mul(t))
	c2 := ctrl1.Add(to.Sub(ctrl1).Mul(t))
	c01 := c0.Add(c1.Sub(c0).Mul(t))
	c12 := c1.Add(c2.Sub(c1).Mul(t))
	c0112 := c01.Add(c12.Sub(c01).Mul(t))
	splits++
	splits = p.approxCubeTo(splits, maxDist, c0, c01, c0112)
	splits = p.approxCubeTo(splits, maxDist, c12, c2, to)
	return splits
}

// End the path and return a clip operation that represents it.
func (p *Path) End() Op {
	p.end()
	c := p.macro.Stop()
	return Op{
		call: c,
	}
}

// Rect represents the clip area of a rectangle with rounded
// corners.The origin is in the upper left
// corner.
// Specify a square with corner radii equal to half the square size to
// construct a circular clip area.
type Rect struct {
	Rect f32.Rectangle
	// The corner radii.
	SE, SW, NW, NE float32
}

// Op returns the Op for the rectangle.
func (rr Rect) Op(ops *op.Ops) Op {
	r := rr.Rect
	// Optimize for the common pixel aligned rectangle with no
	// corner rounding.
	if rr.SE == 0 && rr.SW == 0 && rr.NW == 0 && rr.NE == 0 {
		ri := image.Rectangle{
			Min: image.Point{X: int(r.Min.X), Y: int(r.Min.Y)},
			Max: image.Point{X: int(r.Max.X), Y: int(r.Max.Y)},
		}
		// Optimize pixel-aligned rectangles to just its bounds.
		if r == fRect(ri) {
			return Op{bounds: r}
		}
	}
	return roundRect(ops, r, rr.SE, rr.SW, rr.NW, rr.NE)
}

// Add is a shorthand for Op(ops).Add(ops).
func (rr Rect) Add(ops *op.Ops) {
	rr.Op(ops).Add(ops)
}

// roundRect returns the clip area of a rectangle with rounded
// corners defined by their radii.
func roundRect(ops *op.Ops, r f32.Rectangle, se, sw, nw, ne float32) Op {
	size := r.Size()
	// https://pomax.github.io/bezierinfo/#circles_cubic.
	w, h := float32(size.X), float32(size.Y)
	const c = 0.55228475 // 4*(sqrt(2)-1)/3
	var p Path
	p.Begin(ops)
	p.Move(r.Min)

	p.Move(f32.Point{X: w, Y: h - se})
	p.Cube(f32.Point{X: 0, Y: se * c}, f32.Point{X: -se + se*c, Y: se}, f32.Point{X: -se, Y: se}) // SE
	p.Line(f32.Point{X: sw - w + se, Y: 0})
	p.Cube(f32.Point{X: -sw * c, Y: 0}, f32.Point{X: -sw, Y: -sw + sw*c}, f32.Point{X: -sw, Y: -sw}) // SW
	p.Line(f32.Point{X: 0, Y: nw - h + sw})
	p.Cube(f32.Point{X: 0, Y: -nw * c}, f32.Point{X: nw - nw*c, Y: -nw}, f32.Point{X: nw, Y: -nw}) // NW
	p.Line(f32.Point{X: w - ne - nw, Y: 0})
	p.Cube(f32.Point{X: ne * c, Y: 0}, f32.Point{X: ne, Y: ne - ne*c}, f32.Point{X: ne, Y: ne}) // NE
	return p.End()
}

// fRect converts a rectangle to a f32.Rectangle.
func fRect(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: fPt(r.Min), Max: fPt(r.Max),
	}
}

// fPt converts an point to a f32.Point.
func fPt(p image.Point) f32.Point {
	return f32.Point{
		X: float32(p.X), Y: float32(p.Y),
	}
}
