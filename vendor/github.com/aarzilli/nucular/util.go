package nucular

import (
	"github.com/aarzilli/nucular/font"
	nstyle "github.com/aarzilli/nucular/style"
	"image"

	"golang.org/x/mobile/event/mouse"

	"github.com/aarzilli/nucular/rect"
)

type Heading int

const (
	Up Heading = iota
	Right
	Down
	Left
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func triangleFromDirection(r rect.Rect, pad_x, pad_y int, direction Heading) (result [3]image.Point) {
	var w_half int
	var h_half int

	r.W = max(2*pad_x, r.W)
	r.H = max(2*pad_y, r.H)
	r.W = r.W - 2*pad_x
	r.H = r.H - 2*pad_y

	r.X = r.X + pad_x
	r.Y = r.Y + pad_y

	w_half = r.W / 2.0
	h_half = r.H / 2.0

	if direction == Up {
		result[0] = image.Point{r.X + w_half, r.Y}
		result[1] = image.Point{r.X + r.W, r.Y + r.H}
		result[2] = image.Point{r.X, r.Y + r.H}
	} else if direction == Right {
		result[0] = image.Point{r.X, r.Y}
		result[1] = image.Point{r.X + r.W, r.Y + h_half}
		result[2] = image.Point{r.X, r.Y + r.H}
	} else if direction == Down {
		result[0] = image.Point{r.X, r.Y}
		result[1] = image.Point{r.X + r.W, r.Y}
		result[2] = image.Point{r.X + w_half, r.Y + r.H}
	} else {
		result[0] = image.Point{r.X, r.Y + h_half}
		result[1] = image.Point{r.X + r.W, r.Y}
		result[2] = image.Point{r.X + r.W, r.Y + r.H}
	}
	return
}

func minFloat(x, y float64) float64 {
	if x < y {
		return x
	}
	return y
}

func maxFloat(x, y float64) float64 {
	if x > y {
		return x
	}
	return y
}

func clampFloat(i, v, x float64) float64 {
	if v < i {
		v = i
	}
	if v > x {
		v = x
	}
	return v
}

func clampInt(i, v, x int) int {
	if v < i {
		v = i
	}
	if v > x {
		v = x
	}
	return v
}

func saturateFloat(x float64) float64 {
	return maxFloat(0.0, minFloat(1.0, x))
}

func basicWidgetStateControl(state *nstyle.WidgetStates, in *Input, bounds rect.Rect) nstyle.WidgetStates {
	if in == nil {
		*state = nstyle.WidgetStateInactive
		return nstyle.WidgetStateInactive
	}

	hovering := in.Mouse.HoveringRect(bounds)

	if *state == nstyle.WidgetStateInactive && hovering {
		*state = nstyle.WidgetStateHovered
	}

	if *state == nstyle.WidgetStateHovered && !hovering {
		*state = nstyle.WidgetStateInactive
	}

	if *state == nstyle.WidgetStateHovered && in.Mouse.HasClickInRect(mouse.ButtonLeft, bounds) {
		*state = nstyle.WidgetStateActive
	}

	if hovering {
		return nstyle.WidgetStateHovered
	} else {
		return nstyle.WidgetStateInactive
	}
}

func shrinkRect(r rect.Rect, amount int) rect.Rect {
	var res rect.Rect
	r.W = max(r.W, 2*amount)
	r.H = max(r.H, 2*amount)
	res.X = r.X + amount
	res.Y = r.Y + amount
	res.W = r.W - 2*amount
	res.H = r.H - 2*amount
	return res
}

func FontHeight(f font.Face) int {
	return f.Metrics().Ascent.Ceil() + f.Metrics().Descent.Ceil()
}

func unify(a rect.Rect, b rect.Rect) (clip rect.Rect) {
	clip.X = max(a.X, b.X)
	clip.Y = max(a.Y, b.Y)
	clip.W = min(a.X+a.W, b.X+b.W) - clip.X
	clip.H = min(a.Y+a.H, b.Y+b.H) - clip.Y
	clip.W = max(0.0, clip.W)
	clip.H = max(0.0, clip.H)
	return
}

func padRect(r rect.Rect, pad image.Point) rect.Rect {
	r.W = max(r.W, 2*pad.X)
	r.H = max(r.H, 2*pad.Y)
	r.X += pad.X
	r.Y += pad.Y
	r.W -= 2 * pad.X
	r.H -= 2 * pad.Y
	return r
}
