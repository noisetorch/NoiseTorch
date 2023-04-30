// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/op"
)

// Reader parses an ops list.
type Reader struct {
	pc        PC
	stack     []macro
	ops       *op.Ops
	deferOps  op.Ops
	deferDone bool
}

// EncodedOp represents an encoded op returned by
// Reader.
type EncodedOp struct {
	Key  Key
	Data []byte
	Refs []interface{}
}

// Key is a unique key for a given op.
type Key struct {
	ops            *op.Ops
	pc             int
	version        int
	sx, hx, sy, hy float32
}

// Shadow of op.MacroOp.
type macroOp struct {
	ops *op.Ops
	pc  PC
}

// PC is an instruction counter for an operation list.
type PC struct {
	data int
	refs int
}

type macro struct {
	ops   *op.Ops
	retPC PC
	endPC PC
}

type opMacroDef struct {
	endpc PC
}

// Reset start reading from the beginning of ops.
func (r *Reader) Reset(ops *op.Ops) {
	r.ResetAt(ops, PC{})
}

// ResetAt is like Reset, except it starts reading from pc.
func (r *Reader) ResetAt(ops *op.Ops, pc PC) {
	r.stack = r.stack[:0]
	r.deferOps.Reset()
	r.deferDone = false
	r.pc = pc
	r.ops = ops
}

// NewPC returns a PC representing the current instruction counter of
// ops.
func NewPC(ops *op.Ops) PC {
	return PC{
		data: len(ops.Data()),
		refs: len(ops.Refs()),
	}
}

func (k Key) SetTransform(t f32.Affine2D) Key {
	sx, hx, _, hy, sy, _ := t.Elems()
	k.sx = sx
	k.hx = hx
	k.hy = hy
	k.sy = sy
	return k
}

func (r *Reader) Decode() (EncodedOp, bool) {
	if r.ops == nil {
		return EncodedOp{}, false
	}
	deferring := false
	for {
		if len(r.stack) > 0 {
			b := r.stack[len(r.stack)-1]
			if r.pc == b.endPC {
				r.ops = b.ops
				r.pc = b.retPC
				r.stack = r.stack[:len(r.stack)-1]
				continue
			}
		}
		data := r.ops.Data()
		data = data[r.pc.data:]
		refs := r.ops.Refs()
		if len(data) == 0 {
			if r.deferDone {
				return EncodedOp{}, false
			}
			r.deferDone = true
			// Execute deferred macros.
			r.ops = &r.deferOps
			r.pc = PC{}
			continue
		}
		key := Key{ops: r.ops, pc: r.pc.data, version: r.ops.Version()}
		t := opconst.OpType(data[0])
		n := t.Size()
		nrefs := t.NumRefs()
		data = data[:n]
		refs = refs[r.pc.refs:]
		refs = refs[:nrefs]
		switch t {
		case opconst.TypeDefer:
			deferring = true
			r.pc.data += n
			r.pc.refs += nrefs
			continue
		case opconst.TypeAux:
			// An Aux operations is always wrapped in a macro, and
			// its length is the remaining space.
			block := r.stack[len(r.stack)-1]
			n += block.endPC.data - r.pc.data - opconst.TypeAuxLen
			data = data[:n]
		case opconst.TypeCall:
			if deferring {
				deferring = false
				// Copy macro for deferred execution.
				if t.NumRefs() != 1 {
					panic("internal error: unexpected number of macro refs")
				}
				deferData := r.deferOps.Write1(t.Size(), refs[0])
				copy(deferData, data)
				continue
			}
			var op macroOp
			op.decode(data, refs)
			macroData := op.ops.Data()[op.pc.data:]
			if opconst.OpType(macroData[0]) != opconst.TypeMacro {
				panic("invalid macro reference")
			}
			var opDef opMacroDef
			opDef.decode(macroData[:opconst.TypeMacro.Size()])
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, macro{
				ops:   r.ops,
				retPC: retPC,
				endPC: opDef.endpc,
			})
			r.ops = op.ops
			r.pc = op.pc
			r.pc.data += opconst.TypeMacro.Size()
			r.pc.refs += opconst.TypeMacro.NumRefs()
			continue
		case opconst.TypeMacro:
			var op opMacroDef
			op.decode(data)
			r.pc = op.endpc
			continue
		}
		r.pc.data += n
		r.pc.refs += nrefs
		return EncodedOp{Key: key, Data: data, Refs: refs}, true
	}
}

func (op *opMacroDef) decode(data []byte) {
	if opconst.OpType(data[0]) != opconst.TypeMacro {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	data = data[:9]
	dataIdx := int(int32(bo.Uint32(data[1:])))
	refsIdx := int(int32(bo.Uint32(data[5:])))
	*op = opMacroDef{
		endpc: PC{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}

func (m *macroOp) decode(data []byte, refs []interface{}) {
	if opconst.OpType(data[0]) != opconst.TypeCall {
		panic("invalid op")
	}
	data = data[:9]
	bo := binary.LittleEndian
	dataIdx := int(int32(bo.Uint32(data[1:])))
	refsIdx := int(int32(bo.Uint32(data[5:])))
	*m = macroOp{
		ops: refs[0].(*op.Ops),
		pc: PC{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}
