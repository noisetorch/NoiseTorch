// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
)

const QuadSize = 4 * 2 * 3

type Quad struct {
	From, Ctrl, To f32.Point
}

func (q Quad) Transform(t f32.Affine2D) Quad {
	q.From = t.Transform(q.From)
	q.Ctrl = t.Transform(q.Ctrl)
	q.To = t.Transform(q.To)
	return q
}

func EncodeQuad(d []byte, q Quad) {
	bo := binary.LittleEndian
	bo.PutUint32(d[0:], math.Float32bits(q.From.X))
	bo.PutUint32(d[4:], math.Float32bits(q.From.Y))
	bo.PutUint32(d[8:], math.Float32bits(q.Ctrl.X))
	bo.PutUint32(d[12:], math.Float32bits(q.Ctrl.Y))
	bo.PutUint32(d[16:], math.Float32bits(q.To.X))
	bo.PutUint32(d[20:], math.Float32bits(q.To.Y))
}

func DecodeQuad(d []byte) (q Quad) {
	bo := binary.LittleEndian
	q.From.X = math.Float32frombits(bo.Uint32(d[0:]))
	q.From.Y = math.Float32frombits(bo.Uint32(d[4:]))
	q.Ctrl.X = math.Float32frombits(bo.Uint32(d[8:]))
	q.Ctrl.Y = math.Float32frombits(bo.Uint32(d[12:]))
	q.To.X = math.Float32frombits(bo.Uint32(d[16:]))
	q.To.Y = math.Float32frombits(bo.Uint32(d[20:]))
	return
}

func DecodeTransform(d []byte) (t f32.Affine2D) {
	if opconst.OpType(d[0]) != opconst.TypeTransform {
		panic("invalid op")
	}
	if len(d) < 1+6*4 {
		panic("too short buffer")
	}
	return decodeAffine2D(d[1:])
}

func decodeAffine2D(data []byte) f32.Affine2D {
	bo := binary.LittleEndian
	a := math.Float32frombits(bo.Uint32(data))
	b := math.Float32frombits(bo.Uint32(data[4*1:]))
	c := math.Float32frombits(bo.Uint32(data[4*2:]))
	d := math.Float32frombits(bo.Uint32(data[4*3:]))
	e := math.Float32frombits(bo.Uint32(data[4*4:]))
	f := math.Float32frombits(bo.Uint32(data[4*5:]))
	return f32.NewAffine2D(a, b, c, d, e, f)
}
