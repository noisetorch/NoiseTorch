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
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/ops"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/profile"
	"gioui.org/io/semantic"
	"gioui.org/io/transfer"
	"gioui.org/op"
)

// Router is a Queue implementation that routes events
// to handlers declared in operation lists.
type Router struct {
	savedTrans []f32.Affine2D
	transStack []f32.Affine2D
	pointer    struct {
		queue     pointerQueue
		collector pointerCollector
	}
	key struct {
		queue     keyQueue
		collector keyCollector
	}
	cqueue clipboardQueue

	handlers handlerEvents

	reader ops.Reader

	// InvalidateOp summary.
	wakeup     bool
	wakeupTime time.Time

	// ProfileOp summary.
	profHandlers map[event.Tag]struct{}
	profile      profile.Event
}

// SemanticNode represents a node in the tree describing the components
// contained in a frame.
type SemanticNode struct {
	ID       SemanticID
	ParentID SemanticID
	Children []SemanticNode
	Desc     SemanticDesc

	areaIdx int
}

// SemanticDesc provides a semantic description of a UI component.
type SemanticDesc struct {
	Class       semantic.ClassOp
	Description string
	Label       string
	Selected    bool
	Disabled    bool
	Gestures    SemanticGestures
	Bounds      f32.Rectangle
}

// SemanticGestures is a bit-set of supported gestures.
type SemanticGestures int

const (
	ClickGesture SemanticGestures = 1 << iota
)

// SemanticID uniquely identifies a SemanticDescription.
//
// By convention, the zero value denotes the non-existent ID.
type SemanticID uint64

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
func (q *Router) Frame(frame *op.Ops) {
	q.handlers.Clear()
	q.wakeup = false
	for k := range q.profHandlers {
		delete(q.profHandlers, k)
	}
	var ops *ops.Ops
	if frame != nil {
		ops = &frame.Internal
	}
	q.reader.Reset(ops)
	q.collect()

	q.pointer.queue.Frame(&q.handlers)
	q.key.queue.Frame(&q.handlers, q.key.collector)
	if q.handlers.HadEvents() {
		q.wakeup = true
		q.wakeupTime = time.Time{}
	}
}

// Queue an event and report whether at least one handler had an event queued.
func (q *Router) Queue(events ...event.Event) bool {
	for _, e := range events {
		switch e := e.(type) {
		case profile.Event:
			q.profile = e
		case pointer.Event:
			q.pointer.queue.Push(e, &q.handlers)
		case key.EditEvent, key.Event, key.FocusEvent:
			q.key.queue.Push(e, &q.handlers)
		case clipboard.Event:
			q.cqueue.Push(e, &q.handlers)
		}
	}
	return q.handlers.HadEvents()
}

// TextInputState returns the input state from the most recent
// call to Frame.
func (q *Router) TextInputState() TextInputState {
	return q.key.queue.InputState()
}

// TextInputHint returns the input mode from the most recent key.InputOp.
func (q *Router) TextInputHint() (key.InputHint, bool) {
	return q.key.queue.InputHint()
}

// WriteClipboard returns the most recent text to be copied
// to the clipboard, if any.
func (q *Router) WriteClipboard() (string, bool) {
	return q.cqueue.WriteClipboard()
}

// ReadClipboard reports if any new handler is waiting
// to read the clipboard.
func (q *Router) ReadClipboard() bool {
	return q.cqueue.ReadClipboard()
}

// Cursor returns the last cursor set.
func (q *Router) Cursor() pointer.CursorName {
	return q.pointer.queue.cursor
}

// SemanticAt returns the first semantic description under pos, if any.
func (q *Router) SemanticAt(pos f32.Point) (SemanticID, bool) {
	return q.pointer.queue.SemanticAt(pos)
}

// AppendSemantics appends the semantic tree to nodes, and returns the result.
// The root node is the first added.
func (q *Router) AppendSemantics(nodes []SemanticNode) []SemanticNode {
	q.pointer.collector.q = &q.pointer.queue
	q.pointer.collector.ensureRoot()
	return q.pointer.queue.AppendSemantics(nodes)
}

func (q *Router) collect() {
	q.transStack = q.transStack[:0]
	pc := &q.pointer.collector
	pc.q = &q.pointer.queue
	pc.reset()
	kc := &q.key.collector
	*kc = keyCollector{q: &q.key.queue}
	q.key.queue.Reset()
	var t f32.Affine2D
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypeInvalidate:
			op := decodeInvalidateOp(encOp.Data)
			if !q.wakeup || op.At.Before(q.wakeupTime) {
				q.wakeup = true
				q.wakeupTime = op.At
			}
		case ops.TypeProfile:
			op := decodeProfileOp(encOp.Data, encOp.Refs)
			if q.profHandlers == nil {
				q.profHandlers = make(map[event.Tag]struct{})
			}
			q.profHandlers[op.Tag] = struct{}{}
		case ops.TypeClipboardRead:
			q.cqueue.ProcessReadClipboard(encOp.Refs)
		case ops.TypeClipboardWrite:
			q.cqueue.ProcessWriteClipboard(encOp.Refs)
		case ops.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			if extra := id - len(q.savedTrans) + 1; extra > 0 {
				q.savedTrans = append(q.savedTrans, make([]f32.Affine2D, extra)...)
			}
			q.savedTrans[id] = t
		case ops.TypeLoad:
			id := ops.DecodeLoad(encOp.Data)
			t = q.savedTrans[id]
			pc.resetState()
			pc.setTrans(t)

		case ops.TypeClip:
			var op ops.ClipOp
			op.Decode(encOp.Data)
			pc.clip(op)
		case ops.TypePopClip:
			pc.popArea()
		case ops.TypeTransform:
			t2, push := ops.DecodeTransform(encOp.Data)
			if push {
				q.transStack = append(q.transStack, t)
			}
			t = t.Mul(t2)
			pc.setTrans(t)
		case ops.TypePopTransform:
			n := len(q.transStack)
			t = q.transStack[n-1]
			q.transStack = q.transStack[:n-1]
			pc.setTrans(t)

		// Pointer ops.
		case ops.TypePass:
			pc.pass()
		case ops.TypePopPass:
			pc.popPass()
		case ops.TypePointerInput:
			bo := binary.LittleEndian
			op := pointer.InputOp{
				Tag:   encOp.Refs[0].(event.Tag),
				Grab:  encOp.Data[1] != 0,
				Types: pointer.Type(bo.Uint16(encOp.Data[2:])),
				ScrollBounds: image.Rectangle{
					Min: image.Point{
						X: int(int32(bo.Uint32(encOp.Data[4:]))),
						Y: int(int32(bo.Uint32(encOp.Data[8:]))),
					},
					Max: image.Point{
						X: int(int32(bo.Uint32(encOp.Data[12:]))),
						Y: int(int32(bo.Uint32(encOp.Data[16:]))),
					},
				},
			}
			pc.inputOp(op, &q.handlers)
		case ops.TypeCursor:
			name := encOp.Refs[0].(pointer.CursorName)
			pc.cursor(name)
		case ops.TypeSource:
			op := transfer.SourceOp{
				Tag:  encOp.Refs[0].(event.Tag),
				Type: encOp.Refs[1].(string),
			}
			pc.sourceOp(op, &q.handlers)
		case ops.TypeTarget:
			op := transfer.TargetOp{
				Tag:  encOp.Refs[0].(event.Tag),
				Type: encOp.Refs[1].(string),
			}
			pc.targetOp(op, &q.handlers)
		case ops.TypeOffer:
			op := transfer.OfferOp{
				Tag:  encOp.Refs[0].(event.Tag),
				Type: encOp.Refs[1].(string),
				Data: encOp.Refs[2].(io.ReadCloser),
			}
			pc.offerOp(op, &q.handlers)

		// Key ops.
		case ops.TypeKeyFocus:
			tag, _ := encOp.Refs[0].(event.Tag)
			op := key.FocusOp{
				Tag: tag,
			}
			kc.focusOp(op.Tag)
		case ops.TypeKeySoftKeyboard:
			op := key.SoftKeyboardOp{
				Show: encOp.Data[1] != 0,
			}
			kc.softKeyboard(op.Show)
		case ops.TypeKeyInput:
			op := key.InputOp{
				Tag:  encOp.Refs[0].(event.Tag),
				Hint: key.InputHint(encOp.Data[1]),
			}
			kc.inputOp(op)

		// Semantic ops.
		case ops.TypeSemanticLabel:
			lbl := encOp.Refs[0].(*string)
			pc.semanticLabel(*lbl)
		case ops.TypeSemanticDesc:
			desc := encOp.Refs[0].(*string)
			pc.semanticDesc(*desc)
		case ops.TypeSemanticClass:
			class := semantic.ClassOp(encOp.Data[1])
			pc.semanticClass(class)
		case ops.TypeSemanticSelected:
			if encOp.Data[1] != 0 {
				pc.semanticSelected(true)
			} else {
				pc.semanticSelected(false)
			}
		case ops.TypeSemanticDisabled:
			if encOp.Data[1] != 0 {
				pc.semanticDisabled(true)
			} else {
				pc.semanticDisabled(false)
			}
		}
	}
}

// Profiling reports whether there was profile handlers in the
// most recent Frame call.
func (q *Router) Profiling() bool {
	return len(q.profHandlers) > 0
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

func (h *handlerEvents) AddNoRedraw(k event.Tag, e event.Event) {
	h.init()
	h.handlers[k] = append(h.handlers[k], e)
}

func (h *handlerEvents) Add(k event.Tag, e event.Event) {
	h.AddNoRedraw(k, e)
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
	if ops.OpType(d[0]) != ops.TypeProfile {
		panic("invalid op")
	}
	return profile.Op{
		Tag: refs[0].(event.Tag),
	}
}

func decodeInvalidateOp(d []byte) op.InvalidateOp {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeInvalidate {
		panic("invalid op")
	}
	var o op.InvalidateOp
	if nanos := bo.Uint64(d[1:]); nanos > 0 {
		o.At = time.Unix(0, int64(nanos))
	}
	return o
}

func (s SemanticGestures) String() string {
	var gestures []string
	if s&ClickGesture != 0 {
		gestures = append(gestures, "Click")
	}
	return strings.Join(gestures, ",")
}
