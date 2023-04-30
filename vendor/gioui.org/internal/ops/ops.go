// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/opconst"
	"gioui.org/internal/scene"
)

func DecodeCommand(d []byte) scene.Command {
	var cmd scene.Command
	copy(byteslice.Uint32(cmd[:]), d)
	return cmd
}

func EncodeCommand(out []byte, cmd scene.Command) {
	copy(out, byteslice.Uint32(cmd[:]))
}

func DecodeTransform(data []byte) (t f32.Affine2D) {
	if opconst.OpType(data[0]) != opconst.TypeTransform {
		panic("invalid op")
	}
	data = data[1:]
	data = data[:4*6]

	bo := binary.LittleEndian
	a := math.Float32frombits(bo.Uint32(data))
	b := math.Float32frombits(bo.Uint32(data[4*1:]))
	c := math.Float32frombits(bo.Uint32(data[4*2:]))
	d := math.Float32frombits(bo.Uint32(data[4*3:]))
	e := math.Float32frombits(bo.Uint32(data[4*4:]))
	f := math.Float32frombits(bo.Uint32(data[4*5:]))
	return f32.NewAffine2D(a, b, c, d, e, f)
}

// DecodeSave decodes the state id of a save op.
func DecodeSave(data []byte) int {
	if opconst.OpType(data[0]) != opconst.TypeSave {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	return int(bo.Uint32(data[1:]))
}

// DecodeLoad decodes the state id and mask of a load op.
func DecodeLoad(data []byte) (int, opconst.StateMask) {
	if opconst.OpType(data[0]) != opconst.TypeLoad {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	return int(bo.Uint32(data[2:])), opconst.StateMask(data[1])
}
