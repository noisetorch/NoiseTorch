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
	// visible will be true if the InputOp is present
	// in the current frame.
	visible bool
	new     bool
}

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
		h.visible, h.new = false, false
	}
	q.reader.Reset(root)

	focus, changed, state := q.resolveFocus(events)
	for k, h := range q.handlers {
		if !h.visible {
			delete(q.handlers, k)
			if q.focus == k {
				// Remove the focus from the handler that is no longer visible.
				q.focus = nil
				state = TextInputClose
			}
		} else if h.new && k != focus {
			// Reset the handler on (each) first appearance, but don't trigger redraw.
			events.AddNoRedraw(k, key.FocusEvent{Focus: false})
		}
	}
	if changed && focus != nil {
		if _, exists := q.handlers[focus]; !exists {
			focus = nil
		}
	}
	if changed && focus != q.focus {
		if q.focus != nil {
			events.Add(q.focus, key.FocusEvent{Focus: false})
		}
		q.focus = focus
		if q.focus != nil {
			events.Add(q.focus, key.FocusEvent{Focus: true})
		} else {
			state = TextInputClose
		}
	}
	q.state = state
}

func (q *keyQueue) Push(e event.Event, events *handlerEvents) {
	if q.focus != nil {
		events.Add(q.focus, e)
	}
}

func (q *keyQueue) resolveFocus(events *handlerEvents) (focus event.Tag, changed bool, state TextInputState) {
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeKeyFocus:
			op := decodeFocusOp(encOp.Data, encOp.Refs)
			changed = true
			focus = op.Tag
		case opconst.TypeKeySoftKeyboard:
			op := decodeSoftKeyboardOp(encOp.Data, encOp.Refs)
			if op.Show {
				state = TextInputOpen
			} else {
				state = TextInputClose
			}
		case opconst.TypeKeyInput:
			op := decodeKeyInputOp(encOp.Data, encOp.Refs)
			h, ok := q.handlers[op.Tag]
			if !ok {
				h = &keyHandler{new: true}
				q.handlers[op.Tag] = h
			}
			h.visible = true
		}
	}
	return
}

func decodeKeyInputOp(d []byte, refs []interface{}) key.InputOp {
	if opconst.OpType(d[0]) != opconst.TypeKeyInput {
		panic("invalid op")
	}
	return key.InputOp{
		Tag: refs[0].(event.Tag),
	}
}

func decodeSoftKeyboardOp(d []byte, refs []interface{}) key.SoftKeyboardOp {
	if opconst.OpType(d[0]) != opconst.TypeKeySoftKeyboard {
		panic("invalid op")
	}
	return key.SoftKeyboardOp{
		Show: d[1] != 0,
	}
}

func decodeFocusOp(d []byte, refs []interface{}) key.FocusOp {
	if opconst.OpType(d[0]) != opconst.TypeKeyFocus {
		panic("invalid op")
	}
	return key.FocusOp{
		Tag: refs[0],
	}
}
