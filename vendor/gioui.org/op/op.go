// SPDX-License-Identifier: Unlicense OR MIT

/*

Package op implements operations for updating a user interface.

Gio programs use operations, or ops, for describing their user
interfaces. There are operations for drawing, defining input
handlers, changing window properties as well as operations for
controlling the execution of other operations.

Ops represents a list of operations. The most important use
for an Ops list is to describe a complete user interface update
to a ui/app.Window's Update method.

Drawing a colored square:

	import "gioui.org/unit"
	import "gioui.org/app"
	import "gioui.org/op/paint"

	var w app.Window
	var e system.FrameEvent
	ops := new(op.Ops)
	...
	ops.Reset()
	paint.ColorOp{Color: ...}.Add(ops)
	paint.PaintOp{Rect: ...}.Add(ops)
	e.Frame(ops)

State

An Ops list can be viewed as a very simple virtual machine: it has an implicit
mutable state stack and execution flow can be controlled with macros.

The Save function saves the current state for later restoring:

	ops := new(op.Ops)
	// Save the current state, in particular the transform.
	state := op.Save(ops)
	// Apply a transform to subsequent operations.
	op.Offset(...).Add(ops)
	...
	// Restore the previous transform.
	state.Load()

You can also use this one-line to save the current state and restore it at the
end of a function :

  defer op.Save(ops).Load()

The MacroOp records a list of operations to be executed later:

	ops := new(op.Ops)
	macro := op.Record(ops)
	// Record operations by adding them.
	op.InvalidateOp{}.Add(ops)
	...
	// End recording.
	call := macro.Stop()

	// replay the recorded operations:
	call.Add(ops)

*/
package op

import (
	"encoding/binary"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
)

// Ops holds a list of operations. Operations are stored in
// serialized form to avoid garbage during construction of
// the ops list.
type Ops struct {
	// version is incremented at each Reset.
	version int
	// data contains the serialized operations.
	data []byte
	// refs hold external references for operations.
	refs []interface{}
	// nextStateID is the id allocated for the next
	// StateOp.
	nextStateID int

	macroStack stack
}

// StateOp represents a saved operation snapshop to be restored
// later.
type StateOp struct {
	id      int
	macroID int
	ops     *Ops
}

// MacroOp records a list of operations for later use.
type MacroOp struct {
	ops *Ops
	id  stackID
	pc  pc
}

// CallOp invokes the operations recorded by Record.
type CallOp struct {
	// Ops is the list of operations to invoke.
	ops *Ops
	pc  pc
}

// InvalidateOp requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type InvalidateOp struct {
	At time.Time
}

// TransformOp applies a transform to the current transform. The zero value
// for TransformOp represents the identity transform.
type TransformOp struct {
	t f32.Affine2D
}

// stack tracks the integer identities of MacroOp
// operations to ensure correct pairing of Record/End.
type stack struct {
	currentID int
	nextID    int
}

type stackID struct {
	id   int
	prev int
}

type pc struct {
	data int
	refs int
}

// Defer executes c after all other operations have completed,
// including previously deferred operations.
// Defer saves the current transformation and restores it prior
// to execution. All other operation state is reset.
//
// Note that deferred operations are executed in first-in-first-out
// order, unlike the Go facility of the same name.
func Defer(o *Ops, c CallOp) {
	if c.ops == nil {
		return
	}
	state := Save(o)
	// Wrap c in a macro that loads the saved state before execution.
	m := Record(o)
	load(o, opconst.InitialStateID, opconst.AllState)
	load(o, state.id, opconst.TransformState)
	c.Add(o)
	c = m.Stop()
	// A Defer is recorded as a TypeDefer followed by the
	// wrapped macro.
	data := o.Write(opconst.TypeDeferLen)
	data[0] = byte(opconst.TypeDefer)
	c.Add(o)
}

// Save the current operations state.
func Save(o *Ops) StateOp {
	o.nextStateID++
	s := StateOp{
		ops:     o,
		id:      o.nextStateID,
		macroID: o.macroStack.currentID,
	}
	save(o, s.id)
	return s
}

// save records a save of the operations state to
// id.
func save(o *Ops, id int) {
	bo := binary.LittleEndian
	data := o.Write(opconst.TypeSaveLen)
	data[0] = byte(opconst.TypeSave)
	bo.PutUint32(data[1:], uint32(id))
}

// Load a previously saved operations state.
func (s StateOp) Load() {
	if s.ops.macroStack.currentID != s.macroID {
		panic("load in a different macro than save")
	}
	if s.id == 0 {
		panic("zero-value op")
	}
	load(s.ops, s.id, opconst.AllState)
}

// load a previously saved operations state given
// its ID. Only state included in mask is affected.
func load(o *Ops, id int, m opconst.StateMask) {
	bo := binary.LittleEndian
	data := o.Write(opconst.TypeLoadLen)
	data[0] = byte(opconst.TypeLoad)
	data[1] = byte(m)
	bo.PutUint32(data[2:], uint32(id))
}

// Reset the Ops, preparing it for re-use. Reset invalidates
// any recorded macros.
func (o *Ops) Reset() {
	o.macroStack = stack{}
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.nextStateID = 0
	o.version++
}

// Data is for internal use only.
func (o *Ops) Data() []byte {
	return o.data
}

// Refs is for internal use only.
func (o *Ops) Refs() []interface{} {
	return o.refs
}

// Version is for internal use only.
func (o *Ops) Version() int {
	return o.version
}

// Write is for internal use only.
func (o *Ops) Write(n int) []byte {
	o.data = append(o.data, make([]byte, n)...)
	return o.data[len(o.data)-n:]
}

// Write1 is for internal use only.
func (o *Ops) Write1(n int, ref1 interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, ref1)
	return o.data[len(o.data)-n:]
}

// Write2 is for internal use only.
func (o *Ops) Write2(n int, ref1, ref2 interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, ref1, ref2)
	return o.data[len(o.data)-n:]
}

func (o *Ops) pc() pc {
	return pc{data: len(o.data), refs: len(o.refs)}
}

// Record a macro of operations.
func Record(o *Ops) MacroOp {
	m := MacroOp{
		ops: o,
		id:  o.macroStack.push(),
		pc:  o.pc(),
	}
	// Reserve room for a macro definition. Updated in Stop.
	m.ops.Write(opconst.TypeMacroLen)
	m.fill()
	return m
}

// Stop ends a previously started recording and returns an
// operation for replaying it.
func (m MacroOp) Stop() CallOp {
	m.ops.macroStack.pop(m.id)
	m.fill()
	return CallOp{
		ops: m.ops,
		pc:  m.pc,
	}
}

func (m MacroOp) fill() {
	pc := m.ops.pc()
	// Fill out the macro definition reserved in Record.
	data := m.ops.data[m.pc.data:]
	data = data[:opconst.TypeMacroLen]
	data[0] = byte(opconst.TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
}

// Add the recorded list of operations. Add
// panics if the Ops containing the recording
// has been reset.
func (c CallOp) Add(o *Ops) {
	if c.ops == nil {
		return
	}
	data := o.Write1(opconst.TypeCallLen, c.ops)
	data[0] = byte(opconst.TypeCall)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(c.pc.data))
	bo.PutUint32(data[5:], uint32(c.pc.refs))
}

func (r InvalidateOp) Add(o *Ops) {
	data := o.Write(opconst.TypeRedrawLen)
	data[0] = byte(opconst.TypeInvalidate)
	bo := binary.LittleEndian
	// UnixNano cannot represent the zero time.
	if t := r.At; !t.IsZero() {
		nanos := t.UnixNano()
		if nanos > 0 {
			bo.PutUint64(data[1:], uint64(nanos))
		}
	}
}

// Offset creates a TransformOp with the offset o.
func Offset(o f32.Point) TransformOp {
	return TransformOp{t: f32.Affine2D{}.Offset(o)}
}

// Affine creates a TransformOp representing the transformation a.
func Affine(a f32.Affine2D) TransformOp {
	return TransformOp{t: a}
}

func (t TransformOp) Add(o *Ops) {
	data := o.Write(opconst.TypeTransformLen)
	data[0] = byte(opconst.TypeTransform)
	bo := binary.LittleEndian
	a, b, c, d, e, f := t.t.Elems()
	bo.PutUint32(data[1:], math.Float32bits(a))
	bo.PutUint32(data[1+4*1:], math.Float32bits(b))
	bo.PutUint32(data[1+4*2:], math.Float32bits(c))
	bo.PutUint32(data[1+4*3:], math.Float32bits(d))
	bo.PutUint32(data[1+4*4:], math.Float32bits(e))
	bo.PutUint32(data[1+4*5:], math.Float32bits(f))
}

func (s *stack) push() stackID {
	s.nextID++
	sid := stackID{
		id:   s.nextID,
		prev: s.currentID,
	}
	s.currentID = s.nextID
	return sid
}

func (s *stack) check(sid stackID) {
	if s.currentID != sid.id {
		panic("unbalanced operation")
	}
}

func (s *stack) pop(sid stackID) {
	s.check(sid)
	s.currentID = sid.prev
}
