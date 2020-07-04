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

The StackOp saves the current state to the state stack and restores it later:

	ops := new(op.Ops)
	// Save the current state, in particular the transform.
	stack := op.Push(ops)
	// Apply a transform to subsequent operations.
	op.Offset(...).Add(ops)
	...
	// Restore the previous transform.
	stack.Pop()

You can also use this one-line to save the current state and restore it at the
end of a function :

  defer op.Push(ops).Pop()

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
	// External references for operations.
	refs []interface{}

	stackStack stack
	macroStack stack
}

// StackOp saves and restores the operation state
// in a stack-like manner.
type StackOp struct {
	id      stackID
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

// stack tracks the integer identities of StackOp and MacroOp
// operations to ensure correct pairing of Push/Pop and Record/End.
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

// Push (save) the current operations state.
func Push(o *Ops) StackOp {
	s := StackOp{
		ops:     o,
		id:      o.stackStack.push(),
		macroID: o.macroStack.currentID,
	}
	data := o.Write(opconst.TypePushLen)
	data[0] = byte(opconst.TypePush)
	return s
}

// Pop (restore) a previously Pushed operations state.
func (s StackOp) Pop() {
	if s.ops.macroStack.currentID != s.macroID {
		panic("pop in a different macro than push")
	}
	s.ops.stackStack.pop(s.id)
	data := s.ops.Write(opconst.TypePopLen)
	data[0] = byte(opconst.TypePop)
}

// Reset the Ops, preparing it for re-use. Reset invalidates
// any recorded macros.
func (o *Ops) Reset() {
	o.stackStack = stack{}
	o.macroStack = stack{}
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
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
func (o *Ops) Write(n int, refs ...interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, refs...)
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
	data := o.Write(opconst.TypeCallLen, c.ops)
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
