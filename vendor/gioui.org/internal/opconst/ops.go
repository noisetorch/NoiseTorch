// SPDX-License-Identifier: Unlicense OR MIT

package opconst

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacro OpType = iota + firstOpIndex
	TypeCall
	TypeDefer
	TypeTransform
	TypeInvalidate
	TypeImage
	TypePaint
	TypeColor
	TypeLinearGradient
	TypeArea
	TypePointerInput
	TypePass
	TypeClipboardRead
	TypeClipboardWrite
	TypeKeyInput
	TypeKeyFocus
	TypeKeySoftKeyboard
	TypeSave
	TypeLoad
	TypeAux
	TypeClip
	TypeProfile
	TypeCursor
	TypePath
	TypeStroke
)

const (
	TypeMacroLen           = 1 + 4 + 4
	TypeCallLen            = 1 + 4 + 4
	TypeDeferLen           = 1
	TypeTransformLen       = 1 + 4*6
	TypeRedrawLen          = 1 + 8
	TypeImageLen           = 1
	TypePaintLen           = 1
	TypeColorLen           = 1 + 4
	TypeLinearGradientLen  = 1 + 8*2 + 4*2
	TypeAreaLen            = 1 + 1 + 4*4
	TypePointerInputLen    = 1 + 1 + 1 + 2*4 + 2*4
	TypePassLen            = 1 + 1
	TypeClipboardReadLen   = 1
	TypeClipboardWriteLen  = 1
	TypeKeyInputLen        = 1
	TypeKeyFocusLen        = 1
	TypeKeySoftKeyboardLen = 1 + 1
	TypeSaveLen            = 1 + 4
	TypeLoadLen            = 1 + 1 + 4
	TypeAuxLen             = 1
	TypeClipLen            = 1 + 4*4 + 1
	TypeProfileLen         = 1
	TypeCursorLen          = 1 + 1
	TypePathLen            = 1
	TypeStrokeLen          = 1 + 4
)

// StateMask is a bitmask of state types a load operation
// should restore.
type StateMask uint8

const (
	TransformState StateMask = 1 << iota

	AllState = ^StateMask(0)
)

// InitialStateID is the ID for saving and loading
// the initial operation state.
const InitialStateID = 0

func (t OpType) Size() int {
	return [...]int{
		TypeMacroLen,
		TypeCallLen,
		TypeDeferLen,
		TypeTransformLen,
		TypeRedrawLen,
		TypeImageLen,
		TypePaintLen,
		TypeColorLen,
		TypeLinearGradientLen,
		TypeAreaLen,
		TypePointerInputLen,
		TypePassLen,
		TypeClipboardReadLen,
		TypeClipboardWriteLen,
		TypeKeyInputLen,
		TypeKeyFocusLen,
		TypeKeySoftKeyboardLen,
		TypeSaveLen,
		TypeLoadLen,
		TypeAuxLen,
		TypeClipLen,
		TypeProfileLen,
		TypeCursorLen,
		TypePathLen,
		TypeStrokeLen,
	}[t-firstOpIndex]
}

func (t OpType) NumRefs() int {
	switch t {
	case TypeKeyInput, TypeKeyFocus, TypePointerInput, TypeProfile, TypeCall, TypeClipboardRead, TypeClipboardWrite, TypeCursor:
		return 1
	case TypeImage:
		return 2
	default:
		return 0
	}
}
