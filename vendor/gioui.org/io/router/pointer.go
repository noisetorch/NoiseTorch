// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"encoding/binary"
	"image"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

type pointerQueue struct {
	hitTree  []hitNode
	areas    []areaNode
	cursors  []cursorNode
	cursor   pointer.CursorName
	handlers map[event.Tag]*pointerHandler
	pointers []pointerInfo
	reader   ops.Reader

	// states holds the storage for save/restore ops.
	states  []collectState
	scratch []event.Tag
}

type hitNode struct {
	next int
	area int
	// Pass tracks the most recent PassOp mode.
	pass bool

	// For handler nodes.
	tag event.Tag
}

type cursorNode struct {
	name pointer.CursorName
	area int
}

type pointerInfo struct {
	id       pointer.ID
	pressed  bool
	handlers []event.Tag
	// last tracks the last pointer event received,
	// used while processing frame events.
	last pointer.Event

	// entered tracks the tags that contain the pointer.
	entered []event.Tag
}

type pointerHandler struct {
	area      int
	active    bool
	wantsGrab bool
	types     pointer.Type
	// min and max horizontal/vertical scroll
	scrollRange image.Rectangle
}

type areaOp struct {
	kind areaKind
	rect f32.Rectangle
}

type areaNode struct {
	trans f32.Affine2D
	next  int
	area  areaOp
}

type areaKind uint8

// collectState represents the state for collectHandlers
type collectState struct {
	t    f32.Affine2D
	area int
	node int
	pass bool
}

const (
	areaRect areaKind = iota
	areaEllipse
)

func (q *pointerQueue) save(id int, state collectState) {
	if extra := id - len(q.states) + 1; extra > 0 {
		q.states = append(q.states, make([]collectState, extra)...)
	}
	q.states[id] = state
}

func (q *pointerQueue) collectHandlers(r *ops.Reader, events *handlerEvents) {
	state := collectState{
		area: -1,
		node: -1,
	}
	q.save(opconst.InitialStateID, state)
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			q.save(id, state)
		case opconst.TypeLoad:
			id, mask := ops.DecodeLoad(encOp.Data)
			s := q.states[id]
			if mask&opconst.TransformState != 0 {
				state.t = s.t
			}
			if mask&^opconst.TransformState != 0 {
				state = s
			}
		case opconst.TypePass:
			state.pass = encOp.Data[1] != 0
		case opconst.TypeArea:
			var op areaOp
			op.Decode(encOp.Data)
			q.areas = append(q.areas, areaNode{trans: state.t, next: state.area, area: op})
			state.area = len(q.areas) - 1
			q.hitTree = append(q.hitTree, hitNode{
				next: state.node,
				area: state.area,
				pass: state.pass,
			})
			state.node = len(q.hitTree) - 1
		case opconst.TypeTransform:
			dop := ops.DecodeTransform(encOp.Data)
			state.t = state.t.Mul(dop)
		case opconst.TypePointerInput:
			op := pointer.InputOp{
				Tag:   encOp.Refs[0].(event.Tag),
				Grab:  encOp.Data[1] != 0,
				Types: pointer.Type(encOp.Data[2]),
			}
			q.hitTree = append(q.hitTree, hitNode{
				next: state.node,
				area: state.area,
				pass: state.pass,
				tag:  op.Tag,
			})
			state.node = len(q.hitTree) - 1
			h, ok := q.handlers[op.Tag]
			if !ok {
				h = new(pointerHandler)
				q.handlers[op.Tag] = h
				// Cancel handlers on (each) first appearance, but don't
				// trigger redraw.
				events.AddNoRedraw(op.Tag, pointer.Event{Type: pointer.Cancel})
			}
			h.active = true
			h.area = state.area
			h.wantsGrab = h.wantsGrab || op.Grab
			h.types = h.types | op.Types
			bo := binary.LittleEndian.Uint32
			h.scrollRange = image.Rectangle{
				Min: image.Point{
					X: int(int32(bo(encOp.Data[3:]))),
					Y: int(int32(bo(encOp.Data[7:]))),
				},
				Max: image.Point{
					X: int(int32(bo(encOp.Data[11:]))),
					Y: int(int32(bo(encOp.Data[15:]))),
				},
			}
		case opconst.TypeCursor:
			q.cursors = append(q.cursors, cursorNode{
				name: encOp.Refs[0].(pointer.CursorName),
				area: len(q.areas) - 1,
			})
		}
	}
}

func (q *pointerQueue) opHit(handlers *[]event.Tag, pos f32.Point) {
	// Track whether we're passing through hits.
	pass := true
	idx := len(q.hitTree) - 1
	for idx >= 0 {
		n := &q.hitTree[idx]
		if !q.hit(n.area, pos) {
			idx--
			continue
		}
		pass = pass && n.pass
		if pass {
			idx--
		} else {
			idx = n.next
		}
		if n.tag != nil {
			if _, exists := q.handlers[n.tag]; exists {
				*handlers = append(*handlers, n.tag)
			}
		}
	}
}

func (q *pointerQueue) invTransform(areaIdx int, p f32.Point) f32.Point {
	if areaIdx == -1 {
		return p
	}
	return q.areas[areaIdx].trans.Invert().Transform(p)
}

func (q *pointerQueue) hit(areaIdx int, p f32.Point) bool {
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		p := a.trans.Invert().Transform(p)
		if !a.area.Hit(p) {
			return false
		}
		areaIdx = a.next
	}
	return true
}

func (q *pointerQueue) reset() {
	if q.handlers == nil {
		q.handlers = make(map[event.Tag]*pointerHandler)
	}
}

func (q *pointerQueue) Frame(root *op.Ops, events *handlerEvents) {
	q.reset()
	for _, h := range q.handlers {
		// Reset handler.
		h.active = false
		h.wantsGrab = false
		h.types = 0
	}
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.cursors = q.cursors[:0]
	q.reader.Reset(root)
	q.collectHandlers(&q.reader, events)
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandlers(events, k)
			delete(q.handlers, k)
		}
		if h.wantsGrab {
			for _, p := range q.pointers {
				if !p.pressed {
					continue
				}
				for i, k2 := range p.handlers {
					if k2 == k {
						// Drop other handlers that lost their grab.
						dropped := make([]event.Tag, 0, len(p.handlers)-1)
						dropped = append(dropped, p.handlers[:i]...)
						dropped = append(dropped, p.handlers[i+1:]...)
						cancelHandlers(events, dropped...)
						q.dropHandlers(events, dropped...)
						break
					}
				}
			}
		}
	}
	for i := range q.pointers {
		p := &q.pointers[i]
		q.deliverEnterLeaveEvents(p, events, p.last)
	}
}

func cancelHandlers(events *handlerEvents, tags ...event.Tag) {
	for _, k := range tags {
		events.Add(k, pointer.Event{Type: pointer.Cancel})
	}
}

func (q *pointerQueue) dropHandlers(events *handlerEvents, tags ...event.Tag) {
	for _, k := range tags {
		for i := range q.pointers {
			p := &q.pointers[i]
			for i := len(p.handlers) - 1; i >= 0; i-- {
				if p.handlers[i] == k {
					p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
				}
			}
			for i := len(p.entered) - 1; i >= 0; i-- {
				if p.entered[i] == k {
					p.entered = append(p.entered[:i], p.entered[i+1:]...)
				}
			}
		}
	}
}

func (q *pointerQueue) Push(e pointer.Event, events *handlerEvents) {
	q.reset()
	if e.Type == pointer.Cancel {
		q.pointers = q.pointers[:0]
		for k := range q.handlers {
			cancelHandlers(events, k)
			q.dropHandlers(events, k)
		}
		return
	}
	pidx := -1
	for i, p := range q.pointers {
		if p.id == e.PointerID {
			pidx = i
			break
		}
	}
	if pidx == -1 {
		q.pointers = append(q.pointers, pointerInfo{id: e.PointerID})
		pidx = len(q.pointers) - 1
	}
	p := &q.pointers[pidx]
	p.last = e

	if e.Type == pointer.Move && p.pressed {
		e.Type = pointer.Drag
	}

	if e.Type == pointer.Release {
		q.deliverEvent(p, events, e)
		p.pressed = false
	}
	q.deliverEnterLeaveEvents(p, events, e)

	if !p.pressed {
		p.handlers = append(p.handlers[:0], q.scratch...)
	}
	if e.Type == pointer.Press {
		p.pressed = true
	}
	switch e.Type {
	case pointer.Release:
	case pointer.Scroll:
		q.deliverScrollEvent(p, events, e)
	default:
		q.deliverEvent(p, events, e)
	}
	if !p.pressed && len(p.entered) == 0 {
		// No longer need to track pointer.
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
}

func (q *pointerQueue) deliverEvent(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	foremost := true
	if p.pressed && len(p.handlers) == 1 {
		e.Priority = pointer.Grabbed
		foremost = false
	}
	for _, k := range p.handlers {
		h := q.handlers[k]
		if e.Type&h.types == 0 {
			continue
		}
		e := e
		if foremost {
			foremost = false
			e.Priority = pointer.Foremost
		}
		e.Position = q.invTransform(h.area, e.Position)
		events.Add(k, e)
	}
}

func (q *pointerQueue) deliverScrollEvent(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	foremost := true
	if p.pressed && len(p.handlers) == 1 {
		e.Priority = pointer.Grabbed
		foremost = false
	}
	var sx, sy = e.Scroll.X, e.Scroll.Y
	for _, k := range p.handlers {
		if sx == 0 && sy == 0 {
			return
		}
		h := q.handlers[k]
		// Distribute the scroll to the handler based on its ScrollRange.
		sx, e.Scroll.X = setScrollEvent(sx, h.scrollRange.Min.X, h.scrollRange.Max.X)
		sy, e.Scroll.Y = setScrollEvent(sy, h.scrollRange.Min.Y, h.scrollRange.Max.Y)
		e := e
		if foremost {
			foremost = false
			e.Priority = pointer.Foremost
		}
		e.Position = q.invTransform(h.area, e.Position)
		events.Add(k, e)
	}
}

func (q *pointerQueue) deliverEnterLeaveEvents(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	q.scratch = q.scratch[:0]
	q.opHit(&q.scratch, e.Position)
	if p.pressed {
		// Filter out non-participating handlers.
		for i := len(q.scratch) - 1; i >= 0; i-- {
			if _, found := searchTag(p.handlers, q.scratch[i]); !found {
				q.scratch = append(q.scratch[:i], q.scratch[i+1:]...)
			}
		}
	}
	hits := q.scratch
	if e.Source != pointer.Mouse && !p.pressed && e.Type != pointer.Press {
		// Consider non-mouse pointers leaving when they're released.
		hits = nil
	}
	// Deliver Leave events.
	for _, k := range p.entered {
		if _, found := searchTag(hits, k); found {
			continue
		}
		h := q.handlers[k]
		e.Type = pointer.Leave

		if e.Type&h.types != 0 {
			e.Position = q.invTransform(h.area, e.Position)
			events.Add(k, e)
		}
	}
	// Deliver Enter events and update cursor.
	q.cursor = pointer.CursorDefault
	for _, k := range hits {
		h := q.handlers[k]
		for i := len(q.cursors) - 1; i >= 0; i-- {
			if c := q.cursors[i]; c.area == h.area {
				q.cursor = c.name
				break
			}
		}
		if _, found := searchTag(p.entered, k); found {
			continue
		}
		e.Type = pointer.Enter

		if e.Type&h.types != 0 {
			e.Position = q.invTransform(h.area, e.Position)
			events.Add(k, e)
		}
	}
	p.entered = append(p.entered[:0], hits...)
}

func searchTag(tags []event.Tag, tag event.Tag) (int, bool) {
	for i, t := range tags {
		if t == tag {
			return i, true
		}
	}
	return 0, false
}

func opDecodeFloat32(d []byte) float32 {
	return float32(int32(binary.LittleEndian.Uint32(d)))
}

func (op *areaOp) Decode(d []byte) {
	if opconst.OpType(d[0]) != opconst.TypeArea {
		panic("invalid op")
	}
	rect := f32.Rectangle{
		Min: f32.Point{
			X: opDecodeFloat32(d[2:]),
			Y: opDecodeFloat32(d[6:]),
		},
		Max: f32.Point{
			X: opDecodeFloat32(d[10:]),
			Y: opDecodeFloat32(d[14:]),
		},
	}
	*op = areaOp{
		kind: areaKind(d[1]),
		rect: rect,
	}
}

func (op *areaOp) Hit(pos f32.Point) bool {
	pos = pos.Sub(op.rect.Min)
	size := op.rect.Size()
	switch op.kind {
	case areaRect:
		return 0 <= pos.X && pos.X < size.X &&
			0 <= pos.Y && pos.Y < size.Y
	case areaEllipse:
		rx := size.X / 2
		ry := size.Y / 2
		xh := pos.X - rx
		yk := pos.Y - ry
		// The ellipse function works in all cases because
		// 0/0 is not <= 1.
		return (xh*xh)/(rx*rx)+(yk*yk)/(ry*ry) <= 1
	default:
		panic("invalid area kind")
	}
}

func setScrollEvent(scroll float32, min, max int) (left, scrolled float32) {
	if v := float32(max); scroll > v {
		return scroll - v, v
	}
	if v := float32(min); scroll < v {
		return scroll - v, v
	}
	return 0, scroll
}
