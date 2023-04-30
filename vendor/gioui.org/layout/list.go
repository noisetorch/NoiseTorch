// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

type scrollChild struct {
	size image.Point
	call op.CallOp
}

// List displays a subsection of a potentially infinitely
// large underlying list. List accepts user input to scroll
// the subsection.
type List struct {
	Axis Axis
	// ScrollToEnd instructs the list to stay scrolled to the far end position
	// once reached. A List with ScrollToEnd == true and Position.BeforeEnd ==
	// false draws its content with the last item at the bottom of the list
	// area.
	ScrollToEnd bool
	// Alignment is the cross axis alignment of list elements.
	Alignment Alignment

	cs          Constraints
	scroll      gesture.Scroll
	scrollDelta int

	// Position is updated during Layout. To save the list scroll position,
	// just save Position after Layout finishes. To scroll the list
	// programmatically, update Position (e.g. restore it from a saved value)
	// before calling Layout.
	Position Position

	len int

	// maxSize is the total size of visible children.
	maxSize  int
	children []scrollChild
	dir      iterationDir
}

// ListElement is a function that computes the dimensions of
// a list element.
type ListElement func(gtx Context, index int) Dimensions

type iterationDir uint8

// Position is a List scroll offset represented as an offset from the top edge
// of a child element.
type Position struct {
	// BeforeEnd tracks whether the List position is before the very end. We
	// use "before end" instead of "at end" so that the zero value of a
	// Position struct is useful.
	//
	// When laying out a list, if ScrollToEnd is true and BeforeEnd is false,
	// then First and Offset are ignored, and the list is drawn with the last
	// item at the bottom. If ScrollToEnd is false then BeforeEnd is ignored.
	BeforeEnd bool
	// First is the index of the first visible child.
	First int
	// Offset is the distance in pixels from the top edge to the child at index
	// First.
	Offset int
	// OffsetLast is the signed distance in pixels from the bottom edge to the
	// bottom edge of the child at index First+Count.
	OffsetLast int
	// Count is the number of visible children.
	Count int
}

const (
	iterateNone iterationDir = iota
	iterateForward
	iterateBackward
)

const inf = 1e6

// init prepares the list for iterating through its children with next.
func (l *List) init(gtx Context, len int) {
	if l.more() {
		panic("unfinished child")
	}
	l.cs = gtx.Constraints
	l.maxSize = 0
	l.children = l.children[:0]
	l.len = len
	l.update(gtx)
	if l.scrollToEnd() || l.Position.First > len {
		l.Position.Offset = 0
		l.Position.First = len
	}
}

// Layout the List.
func (l *List) Layout(gtx Context, len int, w ListElement) Dimensions {
	l.init(gtx, len)
	crossMin, crossMax := l.Axis.crossConstraint(gtx.Constraints)
	gtx.Constraints = l.Axis.constraints(0, inf, crossMin, crossMax)
	macro := op.Record(gtx.Ops)
	for l.next(); l.more(); l.next() {
		child := op.Record(gtx.Ops)
		dims := w(gtx, l.index())
		call := child.Stop()
		l.end(dims, call)
	}
	return l.layout(gtx.Ops, macro)
}

func (l *List) scrollToEnd() bool {
	return l.ScrollToEnd && !l.Position.BeforeEnd
}

// Dragging reports whether the List is being dragged.
func (l *List) Dragging() bool {
	return l.scroll.State() == gesture.StateDragging
}

func (l *List) update(gtx Context) {
	d := l.scroll.Scroll(gtx.Metric, gtx, gtx.Now, gesture.Axis(l.Axis))
	l.scrollDelta = d
	l.Position.Offset += d
}

// next advances to the next child.
func (l *List) next() {
	l.dir = l.nextDir()
	// The user scroll offset is applied after scrolling to
	// list end.
	if l.scrollToEnd() && !l.more() && l.scrollDelta < 0 {
		l.Position.BeforeEnd = true
		l.Position.Offset += l.scrollDelta
		l.dir = l.nextDir()
	}
}

// index is current child's position in the underlying list.
func (l *List) index() int {
	switch l.dir {
	case iterateBackward:
		return l.Position.First - 1
	case iterateForward:
		return l.Position.First + len(l.children)
	default:
		panic("Index called before Next")
	}
}

// more reports whether more children are needed.
func (l *List) more() bool {
	return l.dir != iterateNone
}

func (l *List) nextDir() iterationDir {
	_, vsize := l.Axis.mainConstraint(l.cs)
	last := l.Position.First + len(l.children)
	// Clamp offset.
	if l.maxSize-l.Position.Offset < vsize && last == l.len {
		l.Position.Offset = l.maxSize - vsize
	}
	if l.Position.Offset < 0 && l.Position.First == 0 {
		l.Position.Offset = 0
	}
	switch {
	case len(l.children) == l.len:
		return iterateNone
	case l.maxSize-l.Position.Offset < vsize:
		return iterateForward
	case l.Position.Offset < 0:
		return iterateBackward
	}
	return iterateNone
}

// End the current child by specifying its dimensions.
func (l *List) end(dims Dimensions, call op.CallOp) {
	child := scrollChild{dims.Size, call}
	mainSize := l.Axis.Convert(child.size).X
	l.maxSize += mainSize
	switch l.dir {
	case iterateForward:
		l.children = append(l.children, child)
	case iterateBackward:
		l.children = append(l.children, scrollChild{})
		copy(l.children[1:], l.children)
		l.children[0] = child
		l.Position.First--
		l.Position.Offset += mainSize
	default:
		panic("call Next before End")
	}
	l.dir = iterateNone
}

// Layout the List and return its dimensions.
func (l *List) layout(ops *op.Ops, macro op.MacroOp) Dimensions {
	if l.more() {
		panic("unfinished child")
	}
	mainMin, mainMax := l.Axis.mainConstraint(l.cs)
	children := l.children
	// Skip invisible children
	for len(children) > 0 {
		sz := children[0].size
		mainSize := l.Axis.Convert(sz).X
		if l.Position.Offset < mainSize {
			// First child is partially visible.
			break
		}
		l.Position.First++
		l.Position.Offset -= mainSize
		children = children[1:]
	}
	size := -l.Position.Offset
	var maxCross int
	for i, child := range children {
		sz := l.Axis.Convert(child.size)
		if c := sz.Y; c > maxCross {
			maxCross = c
		}
		size += sz.X
		if size >= mainMax {
			children = children[:i+1]
			break
		}
	}
	l.Position.Count = len(children)
	l.Position.OffsetLast = mainMax - size
	pos := -l.Position.Offset
	// ScrollToEnd lists are end aligned.
	if space := l.Position.OffsetLast; l.ScrollToEnd && space > 0 {
		pos += space
	}
	for _, child := range children {
		sz := l.Axis.Convert(child.size)
		var cross int
		switch l.Alignment {
		case End:
			cross = maxCross - sz.Y
		case Middle:
			cross = (maxCross - sz.Y) / 2
		}
		childSize := sz.X
		max := childSize + pos
		if max > mainMax {
			max = mainMax
		}
		min := pos
		if min < 0 {
			min = 0
		}
		r := image.Rectangle{
			Min: l.Axis.Convert(image.Pt(min, -inf)),
			Max: l.Axis.Convert(image.Pt(max, inf)),
		}
		stack := op.Save(ops)
		clip.Rect(r).Add(ops)
		pt := l.Axis.Convert(image.Pt(pos, cross))
		op.Offset(FPt(pt)).Add(ops)
		child.call.Add(ops)
		stack.Load()
		pos += childSize
	}
	atStart := l.Position.First == 0 && l.Position.Offset <= 0
	atEnd := l.Position.First+len(children) == l.len && mainMax >= pos
	if atStart && l.scrollDelta < 0 || atEnd && l.scrollDelta > 0 {
		l.scroll.Stop()
	}
	l.Position.BeforeEnd = !atEnd
	if pos < mainMin {
		pos = mainMin
	}
	if pos > mainMax {
		pos = mainMax
	}
	dims := l.Axis.Convert(image.Pt(pos, maxCross))
	call := macro.Stop()
	defer op.Save(ops).Load()
	pointer.Rect(image.Rectangle{Max: dims}).Add(ops)

	var min, max int
	if o := l.Position.Offset; o > 0 {
		// Use the size of the invisible part as scroll boundary.
		min = -o
	} else if l.Position.First > 0 {
		min = -inf
	}
	if o := l.Position.OffsetLast; o < 0 {
		max = -o
	} else if l.Position.First+l.Position.Count < l.len {
		max = inf
	}
	scrollRange := image.Rectangle{
		Min: l.Axis.Convert(image.Pt(min, 0)),
		Max: l.Axis.Convert(image.Pt(max, 0)),
	}
	l.scroll.Add(ops, scrollRange)

	call.Add(ops)
	return Dimensions{Size: dims}
}
