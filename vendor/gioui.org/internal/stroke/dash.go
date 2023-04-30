// SPDX-License-Identifier: Unlicense OR MIT

// The algorithms to compute dashes have been extracted, adapted from
// (and used as a reference implementation):
//  - github.com/tdewolff/canvas (Licensed under MIT)

package stroke

import (
	"math"
	"sort"

	"gioui.org/f32"
)

type DashOp struct {
	Phase  float32
	Dashes []float32
}

func IsSolidLine(sty DashOp) bool {
	return sty.Phase == 0 && len(sty.Dashes) == 0
}

func (qs StrokeQuads) dash(sty DashOp) StrokeQuads {
	sty = dashCanonical(sty)

	switch {
	case len(sty.Dashes) == 0:
		return qs
	case len(sty.Dashes) == 1 && sty.Dashes[0] == 0.0:
		return StrokeQuads{}
	}

	if len(sty.Dashes)%2 == 1 {
		// If the dash pattern is of uneven length, dash and space lengths
		// alternate. The following duplicates the pattern so that uneven
		// indices are always spaces.
		sty.Dashes = append(sty.Dashes, sty.Dashes...)
	}

	var (
		i0, pos0 = dashStart(sty)
		out      StrokeQuads

		contour uint32 = 1
	)

	for _, ps := range qs.split() {
		var (
			i      = i0
			pos    = pos0
			t      []float64
			length = ps.len()
		)
		for pos+sty.Dashes[i] < length {
			pos += sty.Dashes[i]
			if 0.0 < pos {
				t = append(t, float64(pos))
			}
			i++
			if i == len(sty.Dashes) {
				i = 0
			}
		}

		j0 := 0
		endsInDash := i%2 == 0
		if len(t)%2 == 1 && endsInDash || len(t)%2 == 0 && !endsInDash {
			j0 = 1
		}

		var (
			qd StrokeQuads
			pd = ps.splitAt(&contour, t...)
		)
		for j := j0; j < len(pd)-1; j += 2 {
			qd = qd.append(pd[j])
		}
		if endsInDash {
			if ps.closed() {
				qd = pd[len(pd)-1].append(qd)
			} else {
				qd = qd.append(pd[len(pd)-1])
			}
		}
		out = out.append(qd)
		contour++
	}
	return out
}

func dashCanonical(sty DashOp) DashOp {
	var (
		o  = sty
		ds = o.Dashes
	)

	if len(sty.Dashes) == 0 {
		return sty
	}

	// Remove zeros except first and last.
	for i := 1; i < len(ds)-1; i++ {
		if f32Eq(ds[i], 0.0) {
			ds[i-1] += ds[i+1]
			ds = append(ds[:i], ds[i+2:]...)
			i--
		}
	}

	// Remove first zero, collapse with second and last.
	if f32Eq(ds[0], 0.0) {
		if len(ds) < 3 {
			return DashOp{
				Phase:  0.0,
				Dashes: []float32{0.0},
			}
		}
		o.Phase -= ds[1]
		ds[len(ds)-1] += ds[1]
		ds = ds[2:]
	}

	// Remove last zero, collapse with fist and second to last.
	if f32Eq(ds[len(ds)-1], 0.0) {
		if len(ds) < 3 {
			return DashOp{}
		}
		o.Phase += ds[len(ds)-2]
		ds[0] += ds[len(ds)-2]
		ds = ds[:len(ds)-2]
	}

	// If there are zeros or negatives, don't draw dashes.
	for i := 0; i < len(ds); i++ {
		if ds[i] < 0.0 || f32Eq(ds[i], 0.0) {
			return DashOp{
				Phase:  0.0,
				Dashes: []float32{0.0},
			}
		}
	}

	// Remove repeated patterns.
loop:
	for len(ds)%2 == 0 {
		mid := len(ds) / 2
		for i := 0; i < mid; i++ {
			if !f32Eq(ds[i], ds[mid+i]) {
				break loop
			}
		}
		ds = ds[:mid]
	}
	return o
}

func dashStart(sty DashOp) (int, float32) {
	i0 := 0 // i0 is the index into dashes.
	for sty.Dashes[i0] <= sty.Phase {
		sty.Phase -= sty.Dashes[i0]
		i0++
		if i0 == len(sty.Dashes) {
			i0 = 0
		}
	}
	// pos0 may be negative if the offset lands halfway into dash.
	pos0 := -sty.Phase
	if sty.Phase < 0.0 {
		var sum float32
		for _, d := range sty.Dashes {
			sum += d
		}
		pos0 = -(sum + sty.Phase) // handle negative offsets
	}
	return i0, pos0
}

func (qs StrokeQuads) len() float32 {
	var sum float32
	for i := range qs {
		q := qs[i].Quad
		sum += quadBezierLen(q.From, q.Ctrl, q.To)
	}
	return sum
}

// splitAt splits the path into separate paths at the specified intervals
// along the path.
// splitAt updates the provided contour counter as it splits the segments.
func (qs StrokeQuads) splitAt(contour *uint32, ts ...float64) []StrokeQuads {
	if len(ts) == 0 {
		qs.setContour(*contour)
		return []StrokeQuads{qs}
	}

	sort.Float64s(ts)
	if ts[0] == 0 {
		ts = ts[1:]
	}

	var (
		j int     // index into ts
		t float64 // current position along curve
	)

	var oo []StrokeQuads
	var oi StrokeQuads
	push := func() {
		oo = append(oo, oi)
		oi = nil
	}

	for _, ps := range qs.split() {
		for _, q := range ps {
			if j == len(ts) {
				oi = append(oi, q)
				continue
			}
			speed := func(t float64) float64 {
				return float64(lenPt(quadBezierD1(q.Quad.From, q.Quad.Ctrl, q.Quad.To, float32(t))))
			}
			invL, dt := invSpeedPolynomialChebyshevApprox(20, gaussLegendre7, speed, 0, 1)

			var (
				t0 float64
				r0 = q.Quad.From
				r1 = q.Quad.Ctrl
				r2 = q.Quad.To

				// from keeps track of the start of the 'running' segment.
				from = r0
			)
			for j < len(ts) && t < ts[j] && ts[j] <= t+dt {
				tj := invL(ts[j] - t)
				tsub := (tj - t0) / (1.0 - t0)
				t0 = tj

				var q1 f32.Point
				_, q1, _, r0, r1, r2 = quadBezierSplit(r0, r1, r2, float32(tsub))

				oi = append(oi, StrokeQuad{
					Contour: *contour,
					Quad: QuadSegment{
						From: from,
						Ctrl: q1,
						To:   r0,
					},
				})
				push()
				(*contour)++

				from = r0
				j++
			}
			if !f64Eq(t0, 1) {
				if len(oi) > 0 {
					r0 = oi.pen()
				}
				oi = append(oi, StrokeQuad{
					Contour: *contour,
					Quad: QuadSegment{
						From: r0,
						Ctrl: r1,
						To:   r2,
					},
				})
			}
			t += dt
		}
	}
	if len(oi) > 0 {
		push()
		(*contour)++
	}

	return oo
}

func f32Eq(a, b float32) bool {
	const epsilon = 1e-10
	return math.Abs(float64(a-b)) < epsilon
}

func f64Eq(a, b float64) bool {
	const epsilon = 1e-10
	return math.Abs(a-b) < epsilon
}

func invSpeedPolynomialChebyshevApprox(N int, gaussLegendre gaussLegendreFunc, fp func(float64) float64, tmin, tmax float64) (func(float64) float64, float64) {
	// The TODOs below are copied verbatim from tdewolff/canvas:
	//
	// TODO: find better way to determine N. For Arc 10 seems fine, for some
	// Quads 10 is too low, for Cube depending on inflection points is
	// maybe not the best indicator
	//
	// TODO: track efficiency, how many times is fp called?
	// Does a look-up table make more sense?
	fLength := func(t float64) float64 {
		return math.Abs(gaussLegendre(fp, tmin, t))
	}
	totalLength := fLength(tmax)
	t := func(L float64) float64 {
		return bisectionMethod(fLength, L, tmin, tmax)
	}
	return polynomialChebyshevApprox(N, t, 0.0, totalLength, tmin, tmax), totalLength
}

func polynomialChebyshevApprox(N int, f func(float64) float64, xmin, xmax, ymin, ymax float64) func(float64) float64 {
	var (
		invN = 1.0 / float64(N)
		fs   = make([]float64, N)
	)
	for k := 0; k < N; k++ {
		u := math.Cos(math.Pi * (float64(k+1) - 0.5) * invN)
		fs[k] = f(xmin + 0.5*(xmax-xmin)*(u+1))
	}

	c := make([]float64, N)
	for j := 0; j < N; j++ {
		var a float64
		for k := 0; k < N; k++ {
			a += fs[k] * math.Cos(float64(j)*math.Pi*(float64(k+1)-0.5)/float64(N))
		}
		c[j] = 2 * invN * a
	}

	if ymax < ymin {
		ymin, ymax = ymax, ymin
	}
	return func(x float64) float64 {
		x = math.Min(xmax, math.Max(xmin, x))
		u := (x-xmin)/(xmax-xmin)*2 - 1
		var a float64
		for j := 0; j < N; j++ {
			a += c[j] * math.Cos(float64(j)*math.Acos(u))
		}
		y := -0.5*c[0] + a
		if !math.IsNaN(ymin) && !math.IsNaN(ymax) {
			y = math.Min(ymax, math.Max(ymin, y))
		}
		return y
	}
}

// bisectionMethod finds the value x for which f(x) = y in the interval x
// in [xmin, xmax] using the bisection method.
func bisectionMethod(f func(float64) float64, y, xmin, xmax float64) float64 {
	const (
		maxIter   = 100
		tolerance = 0.001 // 0.1%
	)

	var (
		n    = 0
		x    float64
		tolX = math.Abs(xmax-xmin) * tolerance
		tolY = math.Abs(f(xmax)-f(xmin)) * tolerance
	)
	for {
		x = 0.5 * (xmin + xmax)
		if n >= maxIter {
			return x
		}

		dy := f(x) - y
		switch {
		case math.Abs(dy) < tolY, math.Abs(0.5*(xmax-xmin)) < tolX:
			return x
		case dy > 0:
			xmax = x
		default:
			xmin = x
		}
		n++
	}
}

type gaussLegendreFunc func(func(float64) float64, float64, float64) float64

// Gauss-Legendre quadrature integration from a to b with n=7
func gaussLegendre7(f func(float64) float64, a, b float64) float64 {
	c := 0.5 * (b - a)
	d := 0.5 * (a + b)
	Qd1 := f(-0.949108*c + d)
	Qd2 := f(-0.741531*c + d)
	Qd3 := f(-0.405845*c + d)
	Qd4 := f(d)
	Qd5 := f(0.405845*c + d)
	Qd6 := f(0.741531*c + d)
	Qd7 := f(0.949108*c + d)
	return c * (0.129485*(Qd1+Qd7) + 0.279705*(Qd2+Qd6) + 0.381830*(Qd3+Qd5) + 0.417959*Qd4)
}
