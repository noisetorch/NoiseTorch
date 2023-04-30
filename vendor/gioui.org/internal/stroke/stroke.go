// SPDX-License-Identifier: Unlicense OR MIT

// Most of the algorithms to compute strokes and their offsets have been
// extracted, adapted from (and used as a reference implementation):
//  - github.com/tdewolff/canvas (Licensed under MIT)
//
// These algorithms have been implemented from:
//  Fast, precise flattening of cubic Bézier path and offset curves
//   Thomas F. Hain, et al.
//
// An electronic version is available at:
//  https://seant23.files.wordpress.com/2010/11/fastpreciseflatteningofbeziercurve.pdf
//
// Possible improvements (in term of speed and/or accuracy) on these
// algorithms are:
//
//  - Polar Stroking: New Theory and Methods for Stroking Paths,
//    M. Kilgard
//    https://arxiv.org/pdf/2007.00308.pdf
//
//  - https://raphlinus.github.io/graphics/curves/2019/12/23/flatten-quadbez.html
//    R. Levien

// Package stroke implements conversion of strokes to filled outlines. It is used as a
// fallback for stroke configurations not natively supported by the renderer.
package stroke

import (
	"encoding/binary"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/ops"
	"gioui.org/internal/scene"
)

// The following are copies of types from op/clip to avoid a circular import of
// that package.
// TODO: when the old renderer is gone, this package can be merged with
// op/clip, eliminating the duplicate types.
type StrokeStyle struct {
	Width float32
	Miter float32
	Cap   StrokeCap
	Join  StrokeJoin
}

type StrokeCap uint8

const (
	RoundCap StrokeCap = iota
	FlatCap
	SquareCap
)

type StrokeJoin uint8

const (
	RoundJoin StrokeJoin = iota
	BevelJoin
)

// strokeTolerance is used to reconcile rounding errors arising
// when splitting quads into smaller and smaller segments to approximate
// them into straight lines, and when joining back segments.
//
// The magic value of 0.01 was found by striking a compromise between
// aesthetic looking (curves did look like curves, even after linearization)
// and speed.
const strokeTolerance = 0.01

type QuadSegment struct {
	From, Ctrl, To f32.Point
}

type StrokeQuad struct {
	Contour uint32
	Quad    QuadSegment
}

type strokeState struct {
	p0, p1 f32.Point // p0 is the start point, p1 the end point.
	n0, n1 f32.Point // n0 is the normal vector at the start point, n1 at the end point.
	r0, r1 float32   // r0 is the curvature at the start point, r1 at the end point.
	ctl    f32.Point // ctl is the control point of the quadratic Bézier segment.
}

type StrokeQuads []StrokeQuad

func (qs *StrokeQuads) setContour(n uint32) {
	for i := range *qs {
		(*qs)[i].Contour = n
	}
}

func (qs *StrokeQuads) pen() f32.Point {
	return (*qs)[len(*qs)-1].Quad.To
}

func (qs *StrokeQuads) closed() bool {
	beg := (*qs)[0].Quad.From
	end := (*qs)[len(*qs)-1].Quad.To
	return f32Eq(beg.X, end.X) && f32Eq(beg.Y, end.Y)
}

func (qs *StrokeQuads) lineTo(pt f32.Point) {
	end := qs.pen()
	*qs = append(*qs, StrokeQuad{
		Quad: QuadSegment{
			From: end,
			Ctrl: end.Add(pt).Mul(0.5),
			To:   pt,
		},
	})
}

func (qs *StrokeQuads) arc(f1, f2 f32.Point, angle float32) {
	const segments = 16
	pen := qs.pen()
	m := ArcTransform(pen, f1.Add(pen), f2.Add(pen), angle, segments)
	for i := 0; i < segments; i++ {
		p0 := qs.pen()
		p1 := m.Transform(p0)
		p2 := m.Transform(p1)
		ctl := p1.Mul(2).Sub(p0.Add(p2).Mul(.5))
		*qs = append(*qs, StrokeQuad{
			Quad: QuadSegment{
				From: p0, Ctrl: ctl, To: p2,
			},
		})
	}
}

// split splits a slice of quads into slices of quads grouped
// by contours (ie: splitted at move-to boundaries).
func (qs StrokeQuads) split() []StrokeQuads {
	if len(qs) == 0 {
		return nil
	}

	var (
		c uint32
		o []StrokeQuads
		i = len(o)
	)
	for _, q := range qs {
		if q.Contour != c {
			c = q.Contour
			i = len(o)
			o = append(o, StrokeQuads{})
		}
		o[i] = append(o[i], q)
	}

	return o
}

func (qs StrokeQuads) stroke(stroke StrokeStyle, dashes DashOp) StrokeQuads {
	if !IsSolidLine(dashes) {
		qs = qs.dash(dashes)
	}

	var (
		o  StrokeQuads
		hw = 0.5 * stroke.Width
	)

	for _, ps := range qs.split() {
		rhs, lhs := ps.offset(hw, stroke)
		switch lhs {
		case nil:
			o = o.append(rhs)
		default:
			// Closed path.
			// Inner path should go opposite direction to cancel outer path.
			switch {
			case ps.ccw():
				lhs = lhs.reverse()
				o = o.append(rhs)
				o = o.append(lhs)
			default:
				rhs = rhs.reverse()
				o = o.append(lhs)
				o = o.append(rhs)
			}
		}
	}

	return o
}

// offset returns the right-hand and left-hand sides of the path, offset by
// the half-width hw.
// The stroke handles how segments are joined and ends are capped.
func (qs StrokeQuads) offset(hw float32, stroke StrokeStyle) (rhs, lhs StrokeQuads) {
	var (
		states []strokeState
		beg    = qs[0].Quad.From
		end    = qs[len(qs)-1].Quad.To
		closed = beg == end
	)
	for i := range qs {
		q := qs[i].Quad

		var (
			n0 = strokePathNorm(q.From, q.Ctrl, q.To, 0, hw)
			n1 = strokePathNorm(q.From, q.Ctrl, q.To, 1, hw)
			r0 = strokePathCurv(q.From, q.Ctrl, q.To, 0)
			r1 = strokePathCurv(q.From, q.Ctrl, q.To, 1)
		)
		states = append(states, strokeState{
			p0:  q.From,
			p1:  q.To,
			n0:  n0,
			n1:  n1,
			r0:  r0,
			r1:  r1,
			ctl: q.Ctrl,
		})
	}

	for i, state := range states {
		rhs = rhs.append(strokeQuadBezier(state, +hw, strokeTolerance))
		lhs = lhs.append(strokeQuadBezier(state, -hw, strokeTolerance))

		// join the current and next segments
		if hasNext := i+1 < len(states); hasNext || closed {
			var next strokeState
			switch {
			case hasNext:
				next = states[i+1]
			case closed:
				next = states[0]
			}
			if state.n1 != next.n0 {
				strokePathJoin(stroke, &rhs, &lhs, hw, state.p1, state.n1, next.n0, state.r1, next.r0)
			}
		}
	}

	if closed {
		rhs.close()
		lhs.close()
		return rhs, lhs
	}

	qbeg := &states[0]
	qend := &states[len(states)-1]

	// Default to counter-clockwise direction.
	lhs = lhs.reverse()
	strokePathCap(stroke, &rhs, hw, qend.p1, qend.n1)

	rhs = rhs.append(lhs)
	strokePathCap(stroke, &rhs, hw, qbeg.p0, qbeg.n0.Mul(-1))

	rhs.close()

	return rhs, nil
}

func (qs *StrokeQuads) close() {
	p0 := (*qs)[len(*qs)-1].Quad.To
	p1 := (*qs)[0].Quad.From

	if p1 == p0 {
		return
	}

	*qs = append(*qs, StrokeQuad{
		Quad: QuadSegment{
			From: p0,
			Ctrl: p0.Add(p1).Mul(0.5),
			To:   p1,
		},
	})
}

// ccw returns whether the path is counter-clockwise.
func (qs StrokeQuads) ccw() bool {
	// Use the Shoelace formula:
	//  https://en.wikipedia.org/wiki/Shoelace_formula
	var area float32
	for _, ps := range qs.split() {
		for i := 1; i < len(ps); i++ {
			pi := ps[i].Quad.To
			pj := ps[i-1].Quad.To
			area += (pi.X - pj.X) * (pi.Y + pj.Y)
		}
	}
	return area <= 0.0
}

func (qs StrokeQuads) reverse() StrokeQuads {
	if len(qs) == 0 {
		return nil
	}

	ps := make(StrokeQuads, 0, len(qs))
	for i := range qs {
		q := qs[len(qs)-1-i]
		q.Quad.To, q.Quad.From = q.Quad.From, q.Quad.To
		ps = append(ps, q)
	}

	return ps
}

func (qs StrokeQuads) append(ps StrokeQuads) StrokeQuads {
	switch {
	case len(ps) == 0:
		return qs
	case len(qs) == 0:
		return ps
	}

	// Consolidate quads and smooth out rounding errors.
	// We need to also check for the strokeTolerance to correctly handle
	// join/cap points or on-purpose disjoint quads.
	p0 := qs[len(qs)-1].Quad.To
	p1 := ps[0].Quad.From
	if p0 != p1 && lenPt(p0.Sub(p1)) < strokeTolerance {
		qs = append(qs, StrokeQuad{
			Quad: QuadSegment{
				From: p0,
				Ctrl: p0.Add(p1).Mul(0.5),
				To:   p1,
			},
		})
	}
	return append(qs, ps...)
}

func (q QuadSegment) Transform(t f32.Affine2D) QuadSegment {
	q.From = t.Transform(q.From)
	q.Ctrl = t.Transform(q.Ctrl)
	q.To = t.Transform(q.To)
	return q
}

// strokePathNorm returns the normal vector at t.
func strokePathNorm(p0, p1, p2 f32.Point, t, d float32) f32.Point {
	switch t {
	case 0:
		n := p1.Sub(p0)
		if n.X == 0 && n.Y == 0 {
			return f32.Point{}
		}
		n = rot90CW(n)
		return normPt(n, d)
	case 1:
		n := p2.Sub(p1)
		if n.X == 0 && n.Y == 0 {
			return f32.Point{}
		}
		n = rot90CW(n)
		return normPt(n, d)
	}
	panic("impossible")
}

func rot90CW(p f32.Point) f32.Point  { return f32.Pt(+p.Y, -p.X) }
func rot90CCW(p f32.Point) f32.Point { return f32.Pt(-p.Y, +p.X) }

// cosPt returns the cosine of the opening angle between p and q.
func cosPt(p, q f32.Point) float32 {
	np := math.Hypot(float64(p.X), float64(p.Y))
	nq := math.Hypot(float64(q.X), float64(q.Y))
	return dotPt(p, q) / float32(np*nq)
}

func normPt(p f32.Point, l float32) f32.Point {
	d := math.Hypot(float64(p.X), float64(p.Y))
	l64 := float64(l)
	if math.Abs(d-l64) < 1e-10 {
		return f32.Point{}
	}
	n := float32(l64 / d)
	return f32.Point{X: p.X * n, Y: p.Y * n}
}

func lenPt(p f32.Point) float32 {
	return float32(math.Hypot(float64(p.X), float64(p.Y)))
}

func dotPt(p, q f32.Point) float32 {
	return p.X*q.X + p.Y*q.Y
}

func perpDot(p, q f32.Point) float32 {
	return p.X*q.Y - p.Y*q.X
}

// strokePathCurv returns the curvature at t, along the quadratic Bézier
// curve defined by the triplet (beg, ctl, end).
func strokePathCurv(beg, ctl, end f32.Point, t float32) float32 {
	var (
		d1p = quadBezierD1(beg, ctl, end, t)
		d2p = quadBezierD2(beg, ctl, end, t)

		// Negative when bending right, ie: the curve is CW at this point.
		a = float64(perpDot(d1p, d2p))
	)

	// We check early that the segment isn't too line-like and
	// save a costly call to math.Pow that will be discarded by dividing
	// with a too small 'a'.
	if math.Abs(a) < 1e-10 {
		return float32(math.NaN())
	}
	return float32(math.Pow(float64(d1p.X*d1p.X+d1p.Y*d1p.Y), 1.5) / a)
}

// quadBezierSample returns the point on the Bézier curve at t.
//  B(t) = (1-t)^2 P0 + 2(1-t)t P1 + t^2 P2
func quadBezierSample(p0, p1, p2 f32.Point, t float32) f32.Point {
	t1 := 1 - t
	c0 := t1 * t1
	c1 := 2 * t1 * t
	c2 := t * t

	o := p0.Mul(c0)
	o = o.Add(p1.Mul(c1))
	o = o.Add(p2.Mul(c2))
	return o
}

// quadBezierD1 returns the first derivative of the Bézier curve with respect to t.
//  B'(t) = 2(1-t)(P1 - P0) + 2t(P2 - P1)
func quadBezierD1(p0, p1, p2 f32.Point, t float32) f32.Point {
	p10 := p1.Sub(p0).Mul(2 * (1 - t))
	p21 := p2.Sub(p1).Mul(2 * t)

	return p10.Add(p21)
}

// quadBezierD2 returns the second derivative of the Bézier curve with respect to t:
//  B''(t) = 2(P2 - 2P1 + P0)
func quadBezierD2(p0, p1, p2 f32.Point, t float32) f32.Point {
	p := p2.Sub(p1.Mul(2)).Add(p0)
	return p.Mul(2)
}

// quadBezierLen returns the length of the Bézier curve.
// See:
//  https://malczak.linuxpl.com/blog/quadratic-bezier-curve-length/
func quadBezierLen(p0, p1, p2 f32.Point) float32 {
	a := p0.Sub(p1.Mul(2)).Add(p2)
	b := p1.Mul(2).Sub(p0.Mul(2))
	A := float64(4 * dotPt(a, a))
	B := float64(4 * dotPt(a, b))
	C := float64(dotPt(b, b))
	if f64Eq(A, 0.0) {
		// p1 is in the middle between p0 and p2,
		// so it is a straight line from p0 to p2.
		return lenPt(p2.Sub(p0))
	}

	Sabc := 2 * math.Sqrt(A+B+C)
	A2 := math.Sqrt(A)
	A32 := 2 * A * A2
	C2 := 2 * math.Sqrt(C)
	BA := B / A2
	return float32((A32*Sabc + A2*B*(Sabc-C2) + (4*C*A-B*B)*math.Log((2*A2+BA+Sabc)/(BA+C2))) / (4 * A32))
}

func strokeQuadBezier(state strokeState, d, flatness float32) StrokeQuads {
	// Gio strokes are only quadratic Bézier curves, w/o any inflection point.
	// So we just have to flatten them.
	var qs StrokeQuads
	return flattenQuadBezier(qs, state.p0, state.ctl, state.p1, d, flatness)
}

// flattenQuadBezier splits a Bézier quadratic curve into linear sub-segments,
// themselves also encoded as Bézier (degenerate, flat) quadratic curves.
func flattenQuadBezier(qs StrokeQuads, p0, p1, p2 f32.Point, d, flatness float32) StrokeQuads {
	var (
		t      float32
		flat64 = float64(flatness)
	)
	for t < 1 {
		s2 := float64((p2.X-p0.X)*(p1.Y-p0.Y) - (p2.Y-p0.Y)*(p1.X-p0.X))
		den := math.Hypot(float64(p1.X-p0.X), float64(p1.Y-p0.Y))
		if s2*den == 0.0 {
			break
		}

		s2 /= den
		t = 2.0 * float32(math.Sqrt(flat64/3.0/math.Abs(s2)))
		if t >= 1.0 {
			break
		}
		var q0, q1, q2 f32.Point
		q0, q1, q2, p0, p1, p2 = quadBezierSplit(p0, p1, p2, t)
		qs.addLine(q0, q1, q2, 0, d)
	}
	qs.addLine(p0, p1, p2, 1, d)
	return qs
}

func (qs *StrokeQuads) addLine(p0, ctrl, p1 f32.Point, t, d float32) {

	switch i := len(*qs); i {
	case 0:
		p0 = p0.Add(strokePathNorm(p0, ctrl, p1, 0, d))
	default:
		// Address possible rounding errors and use previous point.
		p0 = (*qs)[i-1].Quad.To
	}

	p1 = p1.Add(strokePathNorm(p0, ctrl, p1, 1, d))

	*qs = append(*qs,
		StrokeQuad{
			Quad: QuadSegment{
				From: p0,
				Ctrl: p0.Add(p1).Mul(0.5),
				To:   p1,
			},
		},
	)
}

// quadInterp returns the interpolated point at t.
func quadInterp(p, q f32.Point, t float32) f32.Point {
	return f32.Pt(
		(1-t)*p.X+t*q.X,
		(1-t)*p.Y+t*q.Y,
	)
}

// quadBezierSplit returns the pair of triplets (from,ctrl,to) Bézier curve,
// split before (resp. after) the provided parametric t value.
func quadBezierSplit(p0, p1, p2 f32.Point, t float32) (f32.Point, f32.Point, f32.Point, f32.Point, f32.Point, f32.Point) {

	var (
		b0 = p0
		b1 = quadInterp(p0, p1, t)
		b2 = quadBezierSample(p0, p1, p2, t)

		a0 = b2
		a1 = quadInterp(p1, p2, t)
		a2 = p2
	)

	return b0, b1, b2, a0, a1, a2
}

// strokePathJoin joins the two paths rhs and lhs, according to the provided
// stroke operation.
func strokePathJoin(stroke StrokeStyle, rhs, lhs *StrokeQuads, hw float32, pivot, n0, n1 f32.Point, r0, r1 float32) {
	if stroke.Miter > 0 {
		strokePathMiterJoin(stroke, rhs, lhs, hw, pivot, n0, n1, r0, r1)
		return
	}
	switch stroke.Join {
	case BevelJoin:
		strokePathBevelJoin(rhs, lhs, hw, pivot, n0, n1, r0, r1)
	case RoundJoin:
		strokePathRoundJoin(rhs, lhs, hw, pivot, n0, n1, r0, r1)
	default:
		panic("impossible")
	}
}

func strokePathBevelJoin(rhs, lhs *StrokeQuads, hw float32, pivot, n0, n1 f32.Point, r0, r1 float32) {

	rp := pivot.Add(n1)
	lp := pivot.Sub(n1)

	rhs.lineTo(rp)
	lhs.lineTo(lp)
}

func strokePathRoundJoin(rhs, lhs *StrokeQuads, hw float32, pivot, n0, n1 f32.Point, r0, r1 float32) {
	rp := pivot.Add(n1)
	lp := pivot.Sub(n1)
	cw := dotPt(rot90CW(n0), n1) >= 0.0
	switch {
	case cw:
		// Path bends to the right, ie. CW (or 180 degree turn).
		c := pivot.Sub(lhs.pen())
		angle := -math.Acos(float64(cosPt(n0, n1)))
		lhs.arc(c, c, float32(angle))
		lhs.lineTo(lp) // Add a line to accommodate for rounding errors.
		rhs.lineTo(rp)
	default:
		// Path bends to the left, ie. CCW.
		angle := math.Acos(float64(cosPt(n0, n1)))
		c := pivot.Sub(rhs.pen())
		rhs.arc(c, c, float32(angle))
		rhs.lineTo(rp) // Add a line to accommodate for rounding errors.
		lhs.lineTo(lp)
	}
}

func strokePathMiterJoin(stroke StrokeStyle, rhs, lhs *StrokeQuads, hw float32, pivot, n0, n1 f32.Point, r0, r1 float32) {
	if n0 == n1.Mul(-1) {
		strokePathBevelJoin(rhs, lhs, hw, pivot, n0, n1, r0, r1)
		return
	}

	// This is to handle nearly linear joints that would be clipped otherwise.
	limit := math.Max(float64(stroke.Miter), 1.001)

	cw := dotPt(rot90CW(n0), n1) >= 0.0
	if cw {
		// hw is used to calculate |R|.
		// When running CW, n0 and n1 point the other way,
		// so the sign of r0 and r1 is negated.
		hw = -hw
	}
	hw64 := float64(hw)

	cos := math.Sqrt(0.5 * (1 + float64(cosPt(n0, n1))))
	d := hw64 / cos
	if math.Abs(limit*hw64) < math.Abs(d) {
		stroke.Miter = 0 // Set miter to zero to disable the miter joint.
		strokePathJoin(stroke, rhs, lhs, hw, pivot, n0, n1, r0, r1)
		return
	}
	mid := pivot.Add(normPt(n0.Add(n1), float32(d)))

	rp := pivot.Add(n1)
	lp := pivot.Sub(n1)
	switch {
	case cw:
		// Path bends to the right, ie. CW.
		lhs.lineTo(mid)
	default:
		// Path bends to the left, ie. CCW.
		rhs.lineTo(mid)
	}
	rhs.lineTo(rp)
	lhs.lineTo(lp)
}

// strokePathCap caps the provided path qs, according to the provided stroke operation.
func strokePathCap(stroke StrokeStyle, qs *StrokeQuads, hw float32, pivot, n0 f32.Point) {
	switch stroke.Cap {
	case FlatCap:
		strokePathFlatCap(qs, hw, pivot, n0)
	case SquareCap:
		strokePathSquareCap(qs, hw, pivot, n0)
	case RoundCap:
		strokePathRoundCap(qs, hw, pivot, n0)
	default:
		panic("impossible")
	}
}

// strokePathFlatCap caps the start or end of a path with a flat cap.
func strokePathFlatCap(qs *StrokeQuads, hw float32, pivot, n0 f32.Point) {
	end := pivot.Sub(n0)
	qs.lineTo(end)
}

// strokePathSquareCap caps the start or end of a path with a square cap.
func strokePathSquareCap(qs *StrokeQuads, hw float32, pivot, n0 f32.Point) {
	var (
		e       = pivot.Add(rot90CCW(n0))
		corner1 = e.Add(n0)
		corner2 = e.Sub(n0)
		end     = pivot.Sub(n0)
	)

	qs.lineTo(corner1)
	qs.lineTo(corner2)
	qs.lineTo(end)
}

// strokePathRoundCap caps the start or end of a path with a round cap.
func strokePathRoundCap(qs *StrokeQuads, hw float32, pivot, n0 f32.Point) {
	c := pivot.Sub(qs.pen())
	qs.arc(c, c, math.Pi)
}

// ArcTransform computes a transformation that can be used for generating quadratic bézier
// curve approximations for an arc.
//
// The math is extracted from the following paper:
//  "Drawing an elliptical arc using polylines, quadratic or
//   cubic Bezier curves", L. Maisonobe
// An electronic version may be found at:
//  http://spaceroots.org/documents/ellipse/elliptical-arc.pdf
func ArcTransform(p, f1, f2 f32.Point, angle float32, segments int) f32.Affine2D {
	c := f32.Point{
		X: 0.5 * (f1.X + f2.X),
		Y: 0.5 * (f1.Y + f2.Y),
	}

	// semi-major axis: 2a = |PF1| + |PF2|
	a := 0.5 * (dist(f1, p) + dist(f2, p))

	// semi-minor axis: c^2 = a^2+b^2 (c: focal distance)
	f := dist(f1, c)
	b := math.Sqrt(a*a - f*f)

	var rx, ry, alpha, start float64
	switch {
	case a > b:
		rx = a
		ry = b
	default:
		rx = b
		ry = a
	}

	var x float64
	switch {
	case f1 == c || f2 == c:
		// degenerate case of a circle.
		alpha = 0
	default:
		switch {
		case f1.X > c.X:
			x = float64(f1.X - c.X)
			alpha = math.Acos(x / f)
		case f1.X < c.X:
			x = float64(f2.X - c.X)
			alpha = math.Acos(x / f)
		case f1.X == c.X:
			// special case of a "vertical" ellipse.
			alpha = math.Pi / 2
			if f1.Y < c.Y {
				alpha = -alpha
			}
		}
	}

	start = math.Acos(float64(p.X-c.X) / dist(c, p))
	if c.Y > p.Y {
		start = -start
	}
	start -= alpha

	var (
		θ   = angle / float32(segments)
		ref f32.Affine2D // transform from absolute frame to ellipse-based one
		rot f32.Affine2D // rotation matrix for each segment
		inv f32.Affine2D // transform from ellipse-based frame to absolute one
	)
	ref = ref.Offset(f32.Point{}.Sub(c))
	ref = ref.Rotate(f32.Point{}, float32(-alpha))
	ref = ref.Scale(f32.Point{}, f32.Point{
		X: float32(1 / rx),
		Y: float32(1 / ry),
	})
	inv = ref.Invert()
	rot = rot.Rotate(f32.Point{}, float32(0.5*θ))

	// Instead of invoking math.Sincos for every segment, compute a rotation
	// matrix once and apply for each segment.
	// Before applying the rotation matrix rot, transform the coordinates
	// to a frame centered to the ellipse (and warped into a unit circle), then rotate.
	// Finally, transform back into the original frame.
	return inv.Mul(rot).Mul(ref)
}

func dist(p1, p2 f32.Point) float64 {
	var (
		x1 = float64(p1.X)
		y1 = float64(p1.Y)
		x2 = float64(p2.X)
		y2 = float64(p2.Y)
		dx = x2 - x1
		dy = y2 - y1
	)
	return math.Hypot(dx, dy)
}

func StrokePathCommands(style StrokeStyle, dashes DashOp, scene []byte) StrokeQuads {
	quads := decodeToStrokeQuads(scene)
	return quads.stroke(style, dashes)
}

// decodeToStrokeQuads decodes scene commands to quads ready to stroke.
func decodeToStrokeQuads(pathData []byte) StrokeQuads {
	quads := make(StrokeQuads, 0, 2*len(pathData)/(scene.CommandSize+4))
	for len(pathData) >= scene.CommandSize+4 {
		contour := binary.LittleEndian.Uint32(pathData)
		cmd := ops.DecodeCommand(pathData[4:])
		switch cmd.Op() {
		case scene.OpLine:
			var q QuadSegment
			q.From, q.To = scene.DecodeLine(cmd)
			q.Ctrl = q.From.Add(q.To).Mul(.5)
			quad := StrokeQuad{
				Contour: contour,
				Quad:    q,
			}
			quads = append(quads, quad)
		case scene.OpQuad:
			var q QuadSegment
			q.From, q.Ctrl, q.To = scene.DecodeQuad(cmd)
			quad := StrokeQuad{
				Contour: contour,
				Quad:    q,
			}
			quads = append(quads, quad)
		case scene.OpCubic:
			for _, q := range SplitCubic(scene.DecodeCubic(cmd)) {
				quad := StrokeQuad{
					Contour: contour,
					Quad:    q,
				}
				quads = append(quads, quad)
			}
		default:
			panic("unsupported scene command")
		}
		pathData = pathData[scene.CommandSize+4:]
	}
	return quads
}

func SplitCubic(from, ctrl0, ctrl1, to f32.Point) []QuadSegment {
	quads := make([]QuadSegment, 0, 10)
	// Set the maximum distance proportionally to the longest side
	// of the bounding rectangle.
	hull := f32.Rectangle{
		Min: from,
		Max: ctrl0,
	}.Canon().Add(ctrl1).Add(to)
	l := hull.Dx()
	if h := hull.Dy(); h > l {
		l = h
	}
	approxCubeTo(&quads, 0, l*0.001, from, ctrl0, ctrl1, to)
	return quads
}

// approxCubeTo approximates a cubic Bézier by a series of quadratic
// curves.
func approxCubeTo(quads *[]QuadSegment, splits int, maxDist float32, from, ctrl0, ctrl1, to f32.Point) int {
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
	c := ctrl0.Mul(3).Sub(from).Add(ctrl1.Mul(3)).Sub(to).Mul(1.0 / 4.0)
	const maxSplits = 32
	if splits >= maxSplits {
		*quads = append(*quads, QuadSegment{From: from, Ctrl: c, To: to})
		return splits
	}
	// The maximum distance between the cubic P and its approximation Q given t
	// can be shown to be
	//
	// d = sqrt(3)/36*|to - 3ctrl1 + 3ctrl0 - pen|
	//
	// To save a square root, compare d² with the squared tolerance.
	v := to.Sub(ctrl1.Mul(3)).Add(ctrl0.Mul(3)).Sub(from)
	d2 := (v.X*v.X + v.Y*v.Y) * 3 / (36 * 36)
	if d2 <= maxDist*maxDist {
		*quads = append(*quads, QuadSegment{From: from, Ctrl: c, To: to})
		return splits
	}
	// De Casteljau split the curve and approximate the halves.
	t := float32(0.5)
	c0 := from.Add(ctrl0.Sub(from).Mul(t))
	c1 := ctrl0.Add(ctrl1.Sub(ctrl0).Mul(t))
	c2 := ctrl1.Add(to.Sub(ctrl1).Mul(t))
	c01 := c0.Add(c1.Sub(c0).Mul(t))
	c12 := c1.Add(c2.Sub(c1).Mul(t))
	c0112 := c01.Add(c12.Sub(c01).Mul(t))
	splits++
	splits = approxCubeTo(quads, splits, maxDist, from, c0, c01, c0112)
	splits = approxCubeTo(quads, splits, maxDist, c0112, c12, c2, to)
	return splits
}
