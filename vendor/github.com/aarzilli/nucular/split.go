package nucular

import (
	"github.com/aarzilli/nucular/rect"
	nstyle "github.com/aarzilli/nucular/style"

	"golang.org/x/mobile/event/mouse"
)

type ScalableSplit struct {
	Size     int
	MinSize  int
	Spacing  int
	lastsize int
	resize   bool
}

func (s *ScalableSplit) Horizontal(w *Window, bounds rect.Rect) (bounds0, bounds1 rect.Rect) {
	scaling := w.Master().Style().Scaling

	var rszbounds rect.Rect
	bounds0, bounds1, rszbounds = s.horizontalnw(bounds, scaling)

	w.LayoutSpacePushScaled(rszbounds)
	rszbounds, _ = w.Custom(nstyle.WidgetStateInactive)

	if w.Input().Mouse.IsClickDownInRect(mouse.ButtonLeft, rszbounds, true) {
		s.resize = true
	}
	if s.resize {
		if !w.Input().Mouse.Down(mouse.ButtonLeft) {
			s.resize = false
		} else {
			s.Size += int(float64(w.Input().Mouse.Delta.Y) / scaling)
			if s.Size <= s.MinSize {
				s.Size = s.MinSize
			}
		}
	}

	return
}

func (s *ScalableSplit) horizontalnw(bounds rect.Rect, scaling float64) (bounds0, bounds1, rszbounds rect.Rect) {
	if bounds.H < 0 || bounds.W < 0 {
		return
	}

	if s.lastsize == 0 {
		s.lastsize = bounds.H
	}
	if s.lastsize != bounds.H {
		diff := int(float64(bounds.H-s.lastsize) / scaling)
		s.Size += diff / 2
		s.lastsize = bounds.H
	}

	hs := int(float64(s.Spacing) * scaling)
	h := bounds.H - hs
	var h0, h1 int
	if s.Size == 0 {
		h0 = h / 2
		h1 = h - h0
		s.Size = int(float64(h0) / scaling)
	} else {
		h0 = int(float64(s.Size) * scaling)
		h1 = h - h0
	}

	minh := int(float64(s.MinSize) * scaling)
	if h1 < minh {
		h1 = minh
		h0 = h - h1
	}
	if h0 < minh {
		h0 = minh
		h1 = h - h0
	}

	bounds0 = bounds
	bounds0.H = h0

	rszbounds = bounds
	rszbounds.Y += bounds0.H
	rszbounds.H = hs

	bounds1 = bounds
	bounds1.Y = rszbounds.Y + rszbounds.H
	bounds1.H = h1

	return bounds0, bounds1, rszbounds
}

func (s *ScalableSplit) Vertical(w *Window, bounds rect.Rect) (bounds0, bounds1 rect.Rect) {
	scaling := w.Master().Style().Scaling

	var rszbounds rect.Rect
	bounds0, bounds1, rszbounds = s.verticalnw(bounds, scaling)

	w.LayoutSpacePushScaled(rszbounds)
	rszbounds, _ = w.Custom(nstyle.WidgetStateInactive)

	if w.Input().Mouse.IsClickDownInRect(mouse.ButtonLeft, rszbounds, true) {
		s.resize = true
	}
	if s.resize {
		if !w.Input().Mouse.Down(mouse.ButtonLeft) {
			s.resize = false
		} else {
			s.Size += int(float64(w.Input().Mouse.Delta.X) / scaling)
			if s.Size <= s.MinSize {
				s.Size = s.MinSize
			}
		}
	}

	return bounds0, bounds1
}

func (s *ScalableSplit) verticalnw(bounds rect.Rect, scaling float64) (bounds0, bounds1, rszbounds rect.Rect) {
	if bounds.H < 0 || bounds.W < 0 {
		return
	}

	if s.lastsize == 0 {
		s.lastsize = bounds.W
	}
	if s.lastsize != bounds.W {
		diff := int(float64(bounds.W-s.lastsize) / scaling)
		s.Size += diff / 2
		s.lastsize = bounds.W
	}

	ws := int(float64(s.Spacing) * scaling)
	wt := bounds.W - ws
	var w0, w1 int
	if s.Size == 0 {
		w0 = wt / 2
		w1 = wt - w0
		s.Size = int(float64(w0) / scaling)
	} else {
		w0 = int(float64(s.Size) * scaling)
		w1 = wt - w0
	}

	minw := int(float64(s.MinSize) * scaling)
	if w1 < minw {
		w1 = minw
		w0 = wt - w1
	}
	if w0 < minw {
		w0 = minw
		w1 = wt - w0
	}

	bounds0 = bounds
	bounds0.W = w0

	rszbounds = bounds
	rszbounds.X += bounds0.W
	rszbounds.W = ws

	bounds1 = bounds
	bounds1.X = rszbounds.X + rszbounds.W
	bounds1.W = w1

	return bounds0, bounds1, rszbounds
}
