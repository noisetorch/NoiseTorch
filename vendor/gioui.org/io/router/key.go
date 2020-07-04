// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/op"
)

type TextInputState uint8

type keyQueue struct {
	focus    event.Tag
	handlers map[event.Tag]*keyHandler
	reader   ops.Reader
	state    TextInputState
}

type keyHandler struct {
	active bool
}

type listenerPriority uint8

const (
	priNone listenerPriority = iota
	priDefault
	priCurrentFocus
	priNewFocus
)

const (
	TextInputKeep TextInputState = iota
	TextInputClose
	TextInputOpen
)

// InputState returns the last text input state as
// determined in Frame.
func (q *keyQueue) InputState() TextInputState {
	return q.state
}

func (q *keyQueue) Frame(root *op.Ops, events *handlerEvents) {
	if q.handlers == nil {
		q.handlers = make(map[event.Tag]*keyHandler)
	}
	for _, h := range q.handlers {
		h.active = false
	}
	q.reader.Reset(root)
	focus, pri, hide := q.resolveFocus(events)
	for k, h := range q.handlers {
		if !h.active {
			delete(q.handlers, k)
			if q.focus == k {
				q.focus = nil
				hide = true
			}
		}
	}
	if focus != q.focus {
		if q.focus != nil {
			events.Add(q.focus, key.FocusEvent{Focus: false})
		}
		q.focus = focus
		if q.focus != nil {
			events.Add(q.focus, key.FocusEvent{Focus: true})
		} else {
			hide = true
		}
	}
	switch {
	case pri == priNewFocus:
		q.state = TextInputOpen
	case hide:
		q.state = TextInputClose
	default:
		q.state = TextInputKeep
	}
}

func (q *keyQueue) Push(e event.Event, events *handlerEvents) {
	if q.focus != nil {
		events.Add(q.focus, e)
	}
}

func (q *keyQueue) resolveFocus(events *handlerEvents) (event.Tag, listenerPriority, bool) {
	var k event.Tag
	var pri listenerPriority
	var hide bool
loop:
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeKeyInput:
			op := decodeKeyInputOp(encOp.Data, encOp.Refs)
			var newPri listenerPriority
			switch {
			case op.Focus:
				newPri = priNewFocus
			case op.Tag == q.focus:
				newPri = priCurrentFocus
			default:
				newPri = priDefault
			}
			// Switch focus if higher priority or if focus requested.
			if newPri.replaces(pri) {
				k, pri = op.Tag, newPri
			}
			h, ok := q.handlers[op.Tag]
			if !ok {
				h = new(keyHandler)
				q.handlers[op.Tag] = h
				// Reset the handler on (each) first appearance.
				events.Add(op.Tag, key.FocusEvent{Focus: false})
			}
			h.active = true
		case opconst.TypeHideInput:
			hide = true
		case opconst.TypePush:
			newK, newPri, h := q.resolveFocus(events)
			hide = hide || h
			if newPri.replaces(pri) {
				k, pri = newK, newPri
			}
		case opconst.TypePop:
			break loop
		}
	}
	return k, pri, hide
}

func (p listenerPriority) replaces(p2 listenerPriority) bool {
	// Favor earliest default focus or latest requested focus.
	return p > p2 || p == p2 && p == priNewFocus
}

func decodeKeyInputOp(d []byte, refs []interface{}) key.InputOp {
	if opconst.OpType(d[0]) != opconst.TypeKeyInput {
		panic("invalid op")
	}
	return key.InputOp{
		Tag:   refs[0].(event.Tag),
		Focus: d[1] != 0,
	}
}
