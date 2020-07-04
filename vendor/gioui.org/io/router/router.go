// SPDX-License-Identifier: Unlicense OR MIT

/*
Package router implements Router, a event.Queue implementation
that that disambiguates and routes events to handlers declared
in operation lists.

Router is used by app.Window and is otherwise only useful for
using Gio with external window implementations.
*/
package router

import (
	"encoding/binary"
	"time"

	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/profile"
	"gioui.org/op"
)

// Router is a Queue implementation that routes events
// to handlers declared in operation lists.
type Router struct {
	pqueue pointerQueue
	kqueue keyQueue

	handlers handlerEvents

	reader ops.Reader

	// InvalidateOp summary.
	wakeup     bool
	wakeupTime time.Time

	// ProfileOp summary.
	profiling    bool
	profHandlers map[event.Tag]struct{}
	profile      profile.Event
}

type handlerEvents struct {
	handlers  map[event.Tag][]event.Event
	hadEvents bool
}

// Events returns the available events for the handler key.
func (q *Router) Events(k event.Tag) []event.Event {
	events := q.handlers.Events(k)
	if _, isprof := q.profHandlers[k]; isprof {
		delete(q.profHandlers, k)
		events = append(events, q.profile)
	}
	return events
}

// Frame replaces the declared handlers from the supplied
// operation list. The text input state, wakeup time and whether
// there are active profile handlers is also saved.
func (q *Router) Frame(ops *op.Ops) {
	q.handlers.Clear()
	q.wakeup = false
	q.profiling = false
	for k := range q.profHandlers {
		delete(q.profHandlers, k)
	}
	q.reader.Reset(ops)
	q.collect()

	q.pqueue.Frame(ops, &q.handlers)
	q.kqueue.Frame(ops, &q.handlers)
	if q.handlers.HadEvents() {
		q.wakeup = true
		q.wakeupTime = time.Time{}
	}
}

func (q *Router) Add(events ...event.Event) bool {
	for _, e := range events {
		switch e := e.(type) {
		case profile.Event:
			q.profile = e
		case pointer.Event:
			q.pqueue.Push(e, &q.handlers)
		case key.EditEvent, key.Event, key.FocusEvent:
			q.kqueue.Push(e, &q.handlers)
		}
	}
	return q.handlers.HadEvents()
}

// TextInputState returns the input state from the most recent
// call to Frame.
func (q *Router) TextInputState() TextInputState {
	return q.kqueue.InputState()
}

func (q *Router) collect() {
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeInvalidate:
			op := decodeInvalidateOp(encOp.Data)
			if !q.wakeup || op.At.Before(q.wakeupTime) {
				q.wakeup = true
				q.wakeupTime = op.At
			}
		case opconst.TypeProfile:
			op := decodeProfileOp(encOp.Data, encOp.Refs)
			if q.profHandlers == nil {
				q.profHandlers = make(map[event.Tag]struct{})
			}
			q.profiling = true
			q.profHandlers[op.Tag] = struct{}{}
		}
	}
}

// Profiling reports whether there was profile handlers in the
// most recent Frame call.
func (q *Router) Profiling() bool {
	return q.profiling
}

// WakeupTime returns the most recent time for doing another frame,
// as determined from the last call to Frame.
func (q *Router) WakeupTime() (time.Time, bool) {
	return q.wakeupTime, q.wakeup
}

func (h *handlerEvents) init() {
	if h.handlers == nil {
		h.handlers = make(map[event.Tag][]event.Event)
	}
}

func (h *handlerEvents) Add(k event.Tag, e event.Event) {
	h.init()
	h.handlers[k] = append(h.handlers[k], e)
	h.hadEvents = true
}

func (h *handlerEvents) HadEvents() bool {
	u := h.hadEvents
	h.hadEvents = false
	return u
}

func (h *handlerEvents) Events(k event.Tag) []event.Event {
	if events, ok := h.handlers[k]; ok {
		h.handlers[k] = h.handlers[k][:0]
		// Schedule another frame if we delivered events to the user
		// to flush half-updated state. This is important when an
		// event changes UI state that has already been laid out. In
		// the worst case, we waste a frame, increasing power usage.
		//
		// Gio is expected to grow the ability to construct
		// frame-to-frame differences and only render to changed
		// areas. In that case, the waste of a spurious frame should
		// be minimal.
		h.hadEvents = h.hadEvents || len(events) > 0
		return events
	}
	return nil
}

func (h *handlerEvents) Clear() {
	for k := range h.handlers {
		delete(h.handlers, k)
	}
}

func decodeProfileOp(d []byte, refs []interface{}) profile.Op {
	if opconst.OpType(d[0]) != opconst.TypeProfile {
		panic("invalid op")
	}
	return profile.Op{
		Tag: refs[0].(event.Tag),
	}
}

func decodeInvalidateOp(d []byte) op.InvalidateOp {
	bo := binary.LittleEndian
	if opconst.OpType(d[0]) != opconst.TypeInvalidate {
		panic("invalid op")
	}
	var o op.InvalidateOp
	if nanos := bo.Uint64(d[1:]); nanos > 0 {
		o.At = time.Unix(0, int64(nanos))
	}
	return o
}
