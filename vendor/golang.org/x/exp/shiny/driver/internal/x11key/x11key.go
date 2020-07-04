// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go

// x11key contains X11 numeric codes for the keyboard and mouse.
package x11key // import "golang.org/x/exp/shiny/driver/internal/x11key"

import (
	"unicode"

	"golang.org/x/mobile/event/key"
)

// These constants come from /usr/include/X11/X.h
const (
	ShiftMask   = 1 << 0
	LockMask    = 1 << 1
	ControlMask = 1 << 2
	Mod1Mask    = 1 << 3
	Mod2Mask    = 1 << 4
	Mod3Mask    = 1 << 5
	Mod4Mask    = 1 << 6
	Mod5Mask    = 1 << 7
	Button1Mask = 1 << 8
	Button2Mask = 1 << 9
	Button3Mask = 1 << 10
	Button4Mask = 1 << 11
	Button5Mask = 1 << 12
)

type KeysymTable struct {
	Table [256][6]uint32

	NumLockMod, ModeSwitchMod, ISOLevel3ShiftMod uint16
}

func (t *KeysymTable) Lookup(detail uint8, state uint16) (rune, key.Code) {
	te := t.Table[detail][0:2]
	if state&t.ModeSwitchMod != 0 {
		te = t.Table[detail][2:4]
	}
	if state&t.ISOLevel3ShiftMod != 0 {
		te = t.Table[detail][4:6]
	}

	// The key event's rune depends on whether the shift key is down.
	unshifted := rune(te[0])
	r := unshifted
	if state&t.NumLockMod != 0 && isKeypad(te[1]) {
		if state&ShiftMask == 0 {
			r = rune(te[1])
		}
	} else if state&ShiftMask != 0 {
		r = rune(te[1])
		// In X11, a zero keysym when shift is down means to use what the
		// keysym is when shift is up.
		if r == 0 {
			r = unshifted
		}
	}

	// The key event's code is independent of whether the shift key is down.
	var c key.Code
	if 0 <= unshifted && unshifted < 0x80 {
		c = asciiKeycodes[unshifted]
		if state&LockMask != 0 {
			r = unicode.ToUpper(r)
		}
	} else if kk, isKeypad := keypadKeysyms[r]; isKeypad {
		r, c = kk.rune, kk.code
	} else if nuk := nonUnicodeKeycodes[unshifted]; nuk != key.CodeUnknown {
		r, c = -1, nuk
	} else {
		r = keysymCodePoints[r]
		if state&LockMask != 0 {
			r = unicode.ToUpper(r)
		}
	}

	return r, c
}

func isKeypad(keysym uint32) bool {
	return keysym >= 0xff80 && keysym <= 0xffbd
}

func KeyModifiers(state uint16) (m key.Modifiers) {
	if state&ShiftMask != 0 {
		m |= key.ModShift
	}
	if state&ControlMask != 0 {
		m |= key.ModControl
	}
	if state&Mod1Mask != 0 {
		m |= key.ModAlt
	}
	if state&Mod4Mask != 0 {
		m |= key.ModMeta
	}
	return m
}

// These constants come from /usr/include/X11/{keysymdef,XF86keysym}.h
const (
	xkISOLeftTab = 0xfe20
	xkBackSpace  = 0xff08
	xkTab        = 0xff09
	xkReturn     = 0xff0d
	xkEscape     = 0xff1b
	xkMultiKey   = 0xff20
	xkHome       = 0xff50
	xkLeft       = 0xff51
	xkUp         = 0xff52
	xkRight      = 0xff53
	xkDown       = 0xff54
	xkPageUp     = 0xff55
	xkPageDown   = 0xff56
	xkEnd        = 0xff57
	xkInsert     = 0xff63
	xkMenu       = 0xff67
	xkHelp       = 0xff6a

	xkNumLock        = 0xff7f
	xkKeypadEnter    = 0xff8d
	xkKeypadHome     = 0xff95
	xkKeypadLeft     = 0xff96
	xkKeypadUp       = 0xff97
	xkKeypadRight    = 0xff98
	xkKeypadDown     = 0xff99
	xkKeypadPageUp   = 0xff9a
	xkKeypadPageDown = 0xff9b
	xkKeypadEnd      = 0xff9c
	xkKeypadInsert   = 0xff9e
	xkKeypadDelete   = 0xff9f
	xkKeypadEqual    = 0xffbd
	xkKeypadMultiply = 0xffaa
	xkKeypadAdd      = 0xffab
	xkKeypadSubtract = 0xffad
	xkKeypadDecimal  = 0xffae
	xkKeypadDivide   = 0xffaf
	xkKeypad0        = 0xffb0
	xkKeypad1        = 0xffb1
	xkKeypad2        = 0xffb2
	xkKeypad3        = 0xffb3
	xkKeypad4        = 0xffb4
	xkKeypad5        = 0xffb5
	xkKeypad6        = 0xffb6
	xkKeypad7        = 0xffb7
	xkKeypad8        = 0xffb8
	xkKeypad9        = 0xffb9

	xkF1       = 0xffbe
	xkF2       = 0xffbf
	xkF3       = 0xffc0
	xkF4       = 0xffc1
	xkF5       = 0xffc2
	xkF6       = 0xffc3
	xkF7       = 0xffc4
	xkF8       = 0xffc5
	xkF9       = 0xffc6
	xkF10      = 0xffc7
	xkF11      = 0xffc8
	xkF12      = 0xffc9
	xkShiftL   = 0xffe1
	xkShiftR   = 0xffe2
	xkControlL = 0xffe3
	xkControlR = 0xffe4
	xkCapsLock = 0xffe5
	xkAltL     = 0xffe9
	xkAltR     = 0xffea
	xkSuperL   = 0xffeb
	xkSuperR   = 0xffec
	xkDelete   = 0xffff

	xf86xkAudioLowerVolume = 0x1008ff11
	xf86xkAudioMute        = 0x1008ff12
	xf86xkAudioRaiseVolume = 0x1008ff13
)

// nonUnicodeKeycodes maps from those xproto.Keysym values (converted to runes)
// that do not correspond to a Unicode code point, such as "Page Up", "F1" or
// "Left Shift", to key.Code values.
var nonUnicodeKeycodes = map[rune]key.Code{
	xkISOLeftTab: key.CodeTab,
	xkBackSpace:  key.CodeDeleteBackspace,
	xkTab:        key.CodeTab,
	xkReturn:     key.CodeReturnEnter,
	xkEscape:     key.CodeEscape,
	xkHome:       key.CodeHome,
	xkLeft:       key.CodeLeftArrow,
	xkUp:         key.CodeUpArrow,
	xkRight:      key.CodeRightArrow,
	xkDown:       key.CodeDownArrow,
	xkPageUp:     key.CodePageUp,
	xkPageDown:   key.CodePageDown,
	xkEnd:        key.CodeEnd,
	xkInsert:     key.CodeInsert,
	xkMenu:       key.CodeRightGUI, // TODO: CodeRightGUI or CodeMenu??
	xkHelp:       key.CodeHelp,
	xkNumLock:    key.CodeKeypadNumLock,
	xkMultiKey:   key.CodeCompose,

	xkKeypadEnter:    key.CodeKeypadEnter,
	xkKeypadHome:     key.CodeHome,
	xkKeypadLeft:     key.CodeLeftArrow,
	xkKeypadUp:       key.CodeUpArrow,
	xkKeypadRight:    key.CodeRightArrow,
	xkKeypadDown:     key.CodeDownArrow,
	xkKeypadPageUp:   key.CodePageUp,
	xkKeypadPageDown: key.CodePageDown,
	xkKeypadEnd:      key.CodeEnd,
	xkKeypadInsert:   key.CodeInsert,
	xkKeypadDelete:   key.CodeDeleteForward,

	xkF1:  key.CodeF1,
	xkF2:  key.CodeF2,
	xkF3:  key.CodeF3,
	xkF4:  key.CodeF4,
	xkF5:  key.CodeF5,
	xkF6:  key.CodeF6,
	xkF7:  key.CodeF7,
	xkF8:  key.CodeF8,
	xkF9:  key.CodeF9,
	xkF10: key.CodeF10,
	xkF11: key.CodeF11,
	xkF12: key.CodeF12,

	xkShiftL:   key.CodeLeftShift,
	xkShiftR:   key.CodeRightShift,
	xkControlL: key.CodeLeftControl,
	xkControlR: key.CodeRightControl,
	xkCapsLock: key.CodeCapsLock,
	xkAltL:     key.CodeLeftAlt,
	xkAltR:     key.CodeRightAlt,
	xkSuperL:   key.CodeLeftGUI,
	xkSuperR:   key.CodeRightGUI,

	xkDelete: key.CodeDeleteForward,

	xf86xkAudioRaiseVolume: key.CodeVolumeUp,
	xf86xkAudioLowerVolume: key.CodeVolumeDown,
	xf86xkAudioMute:        key.CodeMute,
}

// asciiKeycodes maps lower-case ASCII runes to key.Code values.
var asciiKeycodes = [0x80]key.Code{
	'a': key.CodeA,
	'b': key.CodeB,
	'c': key.CodeC,
	'd': key.CodeD,
	'e': key.CodeE,
	'f': key.CodeF,
	'g': key.CodeG,
	'h': key.CodeH,
	'i': key.CodeI,
	'j': key.CodeJ,
	'k': key.CodeK,
	'l': key.CodeL,
	'm': key.CodeM,
	'n': key.CodeN,
	'o': key.CodeO,
	'p': key.CodeP,
	'q': key.CodeQ,
	'r': key.CodeR,
	's': key.CodeS,
	't': key.CodeT,
	'u': key.CodeU,
	'v': key.CodeV,
	'w': key.CodeW,
	'x': key.CodeX,
	'y': key.CodeY,
	'z': key.CodeZ,

	'1': key.Code1,
	'2': key.Code2,
	'3': key.Code3,
	'4': key.Code4,
	'5': key.Code5,
	'6': key.Code6,
	'7': key.Code7,
	'8': key.Code8,
	'9': key.Code9,
	'0': key.Code0,

	' ':  key.CodeSpacebar,
	'-':  key.CodeHyphenMinus,
	'=':  key.CodeEqualSign,
	'[':  key.CodeLeftSquareBracket,
	']':  key.CodeRightSquareBracket,
	'\\': key.CodeBackslash,
	';':  key.CodeSemicolon,
	'\'': key.CodeApostrophe,
	'`':  key.CodeGraveAccent,
	',':  key.CodeComma,
	'.':  key.CodeFullStop,
	'/':  key.CodeSlash,
}

type keypadKeysym struct {
	rune rune
	code key.Code
}

var keypadKeysyms = map[rune]keypadKeysym{
	xkKeypadEqual:    {'=', key.CodeKeypadEqualSign},
	xkKeypadMultiply: {'*', key.CodeKeypadAsterisk},
	xkKeypadAdd:      {'+', key.CodeKeypadPlusSign},
	xkKeypadSubtract: {'-', key.CodeKeypadHyphenMinus},
	xkKeypadDecimal:  {'.', key.CodeKeypadFullStop},
	xkKeypadDivide:   {'/', key.CodeKeypadSlash},
	xkKeypad0:        {'0', key.CodeKeypad0},
	xkKeypad1:        {'1', key.CodeKeypad1},
	xkKeypad2:        {'2', key.CodeKeypad2},
	xkKeypad3:        {'3', key.CodeKeypad3},
	xkKeypad4:        {'4', key.CodeKeypad4},
	xkKeypad5:        {'5', key.CodeKeypad5},
	xkKeypad6:        {'6', key.CodeKeypad6},
	xkKeypad7:        {'7', key.CodeKeypad7},
	xkKeypad8:        {'8', key.CodeKeypad8},
	xkKeypad9:        {'9', key.CodeKeypad9},
}
