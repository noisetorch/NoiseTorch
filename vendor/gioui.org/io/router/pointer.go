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
	handlers map[event.Tag]*pointerHandler
	pointers []pointerInfo
	reader   ops.Reader

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

type pointerInfo struct {
	id       pointer.ID
	pressed  bool
	handlers []event.Tag

	// entered tracks the tags that contain the pointer.
	entered []event.Tag
}

type pointerHandler struct {
	area      int
	active    bool
	wantsGrab bool
	types     pointer.Type
}

type areaOp struct {
	kind areaKind
	rect image.Rectangle
}

type areaNode struct {
	trans f32.Affine2D
	next  int
	area  areaOp
}

type areaKind uint8

const (
	areaRect areaKind = iota
	areaEllipse
)

func (q *pointerQueue) collectHandlers(r *ops.Reader, events *handlerEvents, t f32.Affine2D, area, node int, pass bool) {
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypePush:
			q.collectHandlers(r, events, t, area, node, pass)
		case opconst.TypePop:
			return
		case opconst.TypePass:
			op := decodePassOp(encOp.Data)
			pass = op.Pass
		case opconst.TypeArea:
			var op areaOp
			op.Decode(encOp.Data)
			q.areas = append(q.areas, areaNode{trans: t, next: area, area: op})
			area = len(q.areas) - 1
			q.hitTree = append(q.hitTree, hitNode{
				next: node,
				area: area,
				pass: pass,
			})
			node = len(q.hitTree) - 1
		case opconst.TypeTransform:
			dop := ops.DecodeTransform(encOp.Data)
			t = t.Mul(dop)
		case opconst.TypePointerInput:
			op := decodePointerInputOp(encOp.Data, encOp.Refs)
			q.hitTree = append(q.hitTree, hitNode{
				next: node,
				area: area,
				pass: pass,
				tag:  op.Tag,
			})
			node = len(q.hitTree) - 1
			h, ok := q.handlers[op.Tag]
			if !ok {
				h = new(pointerHandler)
				q.handlers[op.Tag] = h
				events.Add(op.Tag, pointer.Event{Type: pointer.Cancel})
			}
			h.active = true
			h.area = area
			h.wantsGrab = h.wantsGrab || op.Grab
			h.types = h.types | op.Types
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

func (q *pointerQueue) init() {
	if q.handlers == nil {
		q.handlers = make(map[event.Tag]*pointerHandler)
	}
}

func (q *pointerQueue) Frame(root *op.Ops, events *handlerEvents) {
	q.init()
	for _, h := range q.handlers {
		// Reset handler.
		h.active = false
		h.wantsGrab = false
		h.types = 0
	}
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.reader.Reset(root)
	q.collectHandlers(&q.reader, events, f32.Affine2D{}, -1, -1, false)
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
						q.dropHandlers(events, p.handlers[i+1:]...)
						q.dropHandlers(events, p.handlers[:i]...)
						break
					}
				}
			}
		}
	}
}

func (q *pointerQueue) dropHandlers(events *handlerEvents, tags ...event.Tag) {
	for _, k := range tags {
		events.Add(k, pointer.Event{Type: pointer.Cancel})
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
	q.init()
	if e.Type == pointer.Cancel {
		q.pointers = q.pointers[:0]
		for k := range q.handlers {
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

	if e.Type == pointer.Move && p.pressed {
		e.Type = pointer.Drag
	}

	if e.Type == pointer.Release {
		q.deliverEvent(p, events, e)
		p.pressed = false
	}
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
	q.deliverEnterLeaveEvents(p, q.scratch, events, e)

	if !p.pressed {
		p.handlers = append(p.handlers[:0], q.scratch...)
	}
	if e.Type == pointer.Press {
		p.pressed = true
	}
	if e.Type != pointer.Release {
		q.deliverEvent(p, events, e)
	}
	if !p.pressed && len(p.entered) == 0 {
		// No longer need to track pointer.
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
}

func (q *pointerQueue) deliverEvent(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	foremost := true
	for _, k := range p.handlers {
		h := q.handlers[k]
		e := e
		if p.pressed && len(p.handlers) == 1 {
			e.Priority = pointer.Grabbed
		} else if foremost {
			e.Priority = pointer.Foremost
		}

		e.Position = q.invTransform(h.area, e.Position)

		if e.Type&h.types == e.Type {
			foremost = false
			events.Add(k, e)
		}
	}
}

func (q *pointerQueue) deliverEnterLeaveEvents(p *pointerInfo, hits []event.Tag, events *handlerEvents, e pointer.Event) {
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
		e.Position = q.invTransform(h.area, e.Position)

		if e.Type&h.types == e.Type {
			events.Add(k, e)
		}
	}
	// Deliver Enter events.
	for _, k := range hits {
		if _, found := searchTag(p.entered, k); found {
			continue
		}
		h := q.handlers[k]
		e.Type = pointer.Enter
		e.Position = q.invTransform(h.area, e.Position)

		if e.Type&h.types == e.Type {
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

func (op *areaOp) Decode(d []byte) {
	if opconst.OpType(d[0]) != opconst.TypeArea {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	rect := image.Rectangle{
		Min: image.Point{
			X: int(int32(bo.Uint32(d[2:]))),
			Y: int(int32(bo.Uint32(d[6:]))),
		},
		Max: image.Point{
			X: int(int32(bo.Uint32(d[10:]))),
			Y: int(int32(bo.Uint32(d[14:]))),
		},
	}
	*op = areaOp{
		kind: areaKind(d[1]),
		rect: rect,
	}
}

func (op *areaOp) Hit(pos f32.Point) bool {
	min := f32.Point{
		X: float32(op.rect.Min.X),
		Y: float32(op.rect.Min.Y),
	}
	pos = pos.Sub(min)
	size := op.rect.Size()
	switch op.kind {
	case areaRect:
		return 0 <= pos.X && pos.X < float32(size.X) &&
			0 <= pos.Y && pos.Y < float32(size.Y)
	case areaEllipse:
		rx := float32(size.X) / 2
		ry := float32(size.Y) / 2
		xh := pos.X - rx
		yk := pos.Y - ry
		// The ellipse function works in all cases because
		// 0/0 is not <= 1.
		return (xh*xh)/(rx*rx)+(yk*yk)/(ry*ry) <= 1
	default:
		panic("invalid area kind")
	}
}

func decodePointerInputOp(d []byte, refs []interface{}) pointer.InputOp {
	if opconst.OpType(d[0]) != opconst.TypePointerInput {
		panic("invalid op")
	}
	return pointer.InputOp{
		Tag:   refs[0].(event.Tag),
		Grab:  d[1] != 0,
		Types: pointer.Type(d[2]),
	}
}

func decodePassOp(d []byte) pointer.PassOp {
	if opconst.OpType(d[0]) != opconst.TypePass {
		panic("invalid op")
	}
	return pointer.PassOp{
		Pass: d[1] != 0,
	}
}
