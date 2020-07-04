package rect

import "image"

type Rect struct {
	X int
	Y int
	W int
	H int
}

func between(x int, a int, b int) bool {
	return a <= x && x <= b
}

func (rect Rect) Contains(p image.Point) bool {
	return between(p.X, rect.X, rect.X+rect.W) && between(p.Y, rect.Y, rect.Y+rect.H)
}

func (r0 *Rect) Intersect(r1 *Rect) bool {
	return r1.X <= (r0.X+r0.W) && (r1.X+r1.W) >= r0.X && r1.Y <= (r0.Y+r0.H) && (r1.Y+r1.H) >= r0.Y
}

func (rect *Rect) Min() image.Point {
	return image.Point{rect.X, rect.Y}
}

func (rect *Rect) Max() image.Point {
	return image.Point{rect.X + rect.W, rect.Y + rect.H}
}

func (rect Rect) Scaled(scaling float64) Rect {
	if scaling == 1.0 {
		return rect
	}
	return Rect{int(float64(rect.X) * scaling), int(float64(rect.Y) * scaling), int(float64(rect.W) * scaling), int(float64(rect.H) * scaling)}
}

func FromRectangle(r image.Rectangle) Rect {
	return Rect{r.Min.X, r.Min.Y, r.Dx(), r.Dy()}
}

func (r *Rect) Rectangle() image.Rectangle {
	return image.Rect(r.X, r.Y, r.X+r.W, r.Y+r.H)
}
