package icccm

import (
	"github.com/BurntSushi/xgb/xproto"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
)

// IsDeleteProtocol checks whether a ClientMessage event satisfies the
// WM_DELETE_WINDOW protocol. Namely, the format must be 32, the type must
// be the WM_PROTOCOLS atom, and the first data item must be the atom
// WM_DELETE_WINDOW.
//
// Note that if you're using the xwindow package, you should use the
// WMGracefulClose method instead of directly using IsDeleteProtocol.
func IsDeleteProtocol(X *xgbutil.XUtil, ev xevent.ClientMessageEvent) bool {
	// Make sure the Format is 32. (Meaning that each data item is
	// 32 bits.)
	if ev.Format != 32 {
		return false
	}

	// Check to make sure the Type atom is WM_PROTOCOLS.
	typeName, err := xprop.AtomName(X, ev.Type)
	if err != nil || typeName != "WM_PROTOCOLS" { // not what we want
		return false
	}

	// Check to make sure the first data item is WM_DELETE_WINDOW.
	protocolType, err := xprop.AtomName(X,
		xproto.Atom(ev.Data.Data32[0]))
	if err != nil || protocolType != "WM_DELETE_WINDOW" {
		return false
	}

	return true
}

// IsFocusProtocol checks whether a ClientMessage event satisfies the
// WM_TAKE_FOCUS protocol.
func IsFocusProtocol(X *xgbutil.XUtil, ev xevent.ClientMessageEvent) bool {
	// Make sure the Format is 32. (Meaning that each data item is
	// 32 bits.)
	if ev.Format != 32 {
		return false
	}

	// Check to make sure the Type atom is WM_PROTOCOLS.
	typeName, err := xprop.AtomName(X, ev.Type)
	if err != nil || typeName != "WM_PROTOCOLS" { // not what we want
		return false
	}

	// Check to make sure the first data item is WM_TAKE_FOCUS.
	protocolType, err := xprop.AtomName(X,
		xproto.Atom(ev.Data.Data32[0]))
	if err != nil || protocolType != "WM_TAKE_FOCUS" {
		return false
	}

	return true
}
