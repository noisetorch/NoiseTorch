// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"encoding/binary"
	"math"

	"gioui.org/internal/opconst"
	"gioui.org/op"
)

// Stroke represents a stroked path.
type Stroke struct {
	Path  PathSpec
	Style StrokeStyle

	// Dashes specify the dashes of the stroke.
	// The empty value denotes no dashes.
	Dashes DashSpec
}

// Op returns a clip operation representing the stroke.
func (s Stroke) Op() Op {
	return Op{
		path:   s.Path,
		stroke: s.Style,
		dashes: s.Dashes,
	}
}

// StrokeStyle describes how a path should be stroked.
type StrokeStyle struct {
	Width float32 // Width of the stroked path.

	// Miter is the limit to apply to a miter joint.
	// The zero Miter disables the miter joint; setting Miter to +∞
	// unconditionally enables the miter joint.
	Miter float32
	Cap   StrokeCap  // Cap describes the head or tail of a stroked path.
	Join  StrokeJoin // Join describes how stroked paths are collated.
}

// StrokeCap describes the head or tail of a stroked path.
type StrokeCap uint8

const (
	// RoundCap caps stroked paths with a round cap, joining the right-hand and
	// left-hand sides of a stroked path with a half disc of diameter the
	// stroked path's width.
	RoundCap StrokeCap = iota

	// FlatCap caps stroked paths with a flat cap, joining the right-hand
	// and left-hand sides of a stroked path with a straight line.
	FlatCap

	// SquareCap caps stroked paths with a square cap, joining the right-hand
	// and left-hand sides of a stroked path with a half square of length
	// the stroked path's width.
	SquareCap
)

// StrokeJoin describes how stroked paths are collated.
type StrokeJoin uint8

const (
	// RoundJoin joins path segments with a round segment.
	RoundJoin StrokeJoin = iota

	// BevelJoin joins path segments with sharp bevels.
	BevelJoin
)

// Dash records dashes' lengths and phase for a stroked path.
type Dash struct {
	ops   *op.Ops
	macro op.MacroOp
	phase float32
	size  uint8 // size of the pattern
}

func (d *Dash) Begin(ops *op.Ops) {
	d.ops = ops
	d.macro = op.Record(ops)
	// Write the TypeAux opcode
	data := ops.Write(opconst.TypeAuxLen)
	data[0] = byte(opconst.TypeAux)
}

func (d *Dash) Phase(v float32) {
	d.phase = v
}

func (d *Dash) Dash(length float32) {
	if d.size == math.MaxUint8 {
		panic("clip: dash pattern too large")
	}
	data := d.ops.Write(4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], math.Float32bits(length))
	d.size++
}

func (d *Dash) End() DashSpec {
	c := d.macro.Stop()
	return DashSpec{
		spec:  c,
		phase: d.phase,
		size:  d.size,
	}
}

// DashSpec describes a dashed pattern.
type DashSpec struct {
	spec  op.CallOp
	phase float32
	size  uint8 // size of the pattern
}
