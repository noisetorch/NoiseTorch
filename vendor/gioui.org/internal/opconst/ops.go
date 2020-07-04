// SPDX-License-Identifier: Unlicense OR MIT

package opconst

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacro OpType = iota + firstOpIndex
	TypeCall
	TypeTransform
	TypeLayer
	TypeInvalidate
	TypeImage
	TypePaint
	TypeColor
	TypeArea
	TypePointerInput
	TypePass
	TypeKeyInput
	TypeHideInput
	TypePush
	TypePop
	TypeAux
	TypeClip
	TypeProfile
)

const (
	TypeMacroLen        = 1 + 4 + 4
	TypeCallLen         = 1 + 4 + 4
	TypeTransformLen    = 1 + 4*6
	TypeLayerLen        = 1
	TypeRedrawLen       = 1 + 8
	TypeImageLen        = 1 + 4*4
	TypePaintLen        = 1 + 4*4
	TypeColorLen        = 1 + 4
	TypeAreaLen         = 1 + 1 + 4*4
	TypePointerInputLen = 1 + 1 + 1
	TypePassLen         = 1 + 1
	TypeKeyInputLen     = 1 + 1
	TypeHideInputLen    = 1
	TypePushLen         = 1
	TypePopLen          = 1
	TypeAuxLen          = 1
	TypeClipLen         = 1 + 4*4
	TypeProfileLen      = 1
)

func (t OpType) Size() int {
	return [...]int{
		TypeMacroLen,
		TypeCallLen,
		TypeTransformLen,
		TypeLayerLen,
		TypeRedrawLen,
		TypeImageLen,
		TypePaintLen,
		TypeColorLen,
		TypeAreaLen,
		TypePointerInputLen,
		TypePassLen,
		TypeKeyInputLen,
		TypeHideInputLen,
		TypePushLen,
		TypePopLen,
		TypeAuxLen,
		TypeClipLen,
		TypeProfileLen,
	}[t-firstOpIndex]
}

func (t OpType) NumRefs() int {
	switch t {
	case TypeKeyInput, TypePointerInput, TypeProfile, TypeCall:
		return 1
	case TypeImage:
		return 2
	default:
		return 0
	}
}
