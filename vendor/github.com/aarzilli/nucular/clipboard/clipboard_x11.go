// +build linux,!android freebsd

package clipboard

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"os"
	"time"
)

const debugClipboardRequests = false

var X *xgb.Conn
var win xproto.Window
var clipboardText string
var selnotify chan bool

var clipboardAtom, primaryAtom, textAtom, targetsAtom, atomAtom xproto.Atom
var targetAtoms []xproto.Atom
var clipboardAtomCache = map[xproto.Atom]string{}

func Start() {
	var err error
	X, err = xgb.NewConnDisplay("")
	if err != nil {
		panic(err)
	}

	selnotify = make(chan bool, 1)

	win, err = xproto.NewWindowId(X)
	if err != nil {
		panic(err)
	}

	setup := xproto.Setup(X)
	s := setup.DefaultScreen(X)
	err = xproto.CreateWindowChecked(X, s.RootDepth, win, s.Root, 100, 100, 1, 1, 0, xproto.WindowClassInputOutput, s.RootVisual, 0, []uint32{}).Check()
	if err != nil {
		panic(err)
	}

	clipboardAtom = internAtom(X, "CLIPBOARD")
	primaryAtom = internAtom(X, "PRIMARY")
	textAtom = internAtom(X, "UTF8_STRING")
	targetsAtom = internAtom(X, "TARGETS")
	atomAtom = internAtom(X, "ATOM")

	targetAtoms = []xproto.Atom{targetsAtom, textAtom}

	go eventLoop()
}

func Set(text string) {
	clipboardText = text
	ssoc := xproto.SetSelectionOwnerChecked(X, win, clipboardAtom, xproto.TimeCurrentTime)
	if err := ssoc.Check(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting clipboard: %v", err)
	}
	ssoc = xproto.SetSelectionOwnerChecked(X, win, primaryAtom, xproto.TimeCurrentTime)
	if err := ssoc.Check(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting primary selection: %v", err)
	}
}

func Get() string {
	return getSelection(clipboardAtom)
}

func GetPrimary() string {
	return getSelection(primaryAtom)
}

func getSelection(selAtom xproto.Atom) string {
	csc := xproto.ConvertSelectionChecked(X, win, selAtom, textAtom, selAtom, xproto.TimeCurrentTime)
	err := csc.Check()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	select {
	case r := <-selnotify:
		if !r {
			return ""
		}
		gpc := xproto.GetProperty(X, true, win, selAtom, textAtom, 0, 5*1024*1024)
		gpr, err := gpc.Reply()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return ""
		}
		if gpr.BytesAfter != 0 {
			fmt.Fprintln(os.Stderr, "Clipboard too large")
			return ""
		}
		return string(gpr.Value[:gpr.ValueLen])
	case <-time.After(1 * time.Second):
		fmt.Fprintln(os.Stderr, "Clipboard retrieval failed, timeout")
		return ""
	}
}

func eventLoop() {
	for {
		e, err := X.WaitForEvent()
		if err != nil {
			continue
		}

		switch e := e.(type) {
		case xproto.SelectionRequestEvent:
			if debugClipboardRequests {
				tgtname := lookupAtom(e.Target)
				fmt.Fprintln(os.Stderr, "SelectionRequest", e, textAtom, tgtname, "isPrimary:", e.Selection == primaryAtom, "isClipboard:", e.Selection == clipboardAtom)
			}
			t := clipboardText

			switch e.Target {
			case textAtom:
				if debugClipboardRequests {
					fmt.Fprintln(os.Stderr, "Sending as text")
				}
				cpc := xproto.ChangePropertyChecked(X, xproto.PropModeReplace, e.Requestor, e.Property, textAtom, 8, uint32(len(t)), []byte(t))
				err := cpc.Check()
				if err == nil {
					sendSelectionNotify(e)
				} else {
					fmt.Fprintln(os.Stderr, err)
				}

			case targetsAtom:
				if debugClipboardRequests {
					fmt.Fprintln(os.Stderr, "Sending targets")
				}
				buf := make([]byte, len(targetAtoms)*4)
				for i, atom := range targetAtoms {
					xgb.Put32(buf[i*4:], uint32(atom))
				}

				xproto.ChangePropertyChecked(X, xproto.PropModeReplace, e.Requestor, e.Property, atomAtom, 32, uint32(len(targetAtoms)), buf).Check()
				if err == nil {
					sendSelectionNotify(e)
				} else {
					fmt.Fprintln(os.Stderr, err)
				}

			default:
				if debugClipboardRequests {
					fmt.Fprintln(os.Stderr, "Skipping")
				}
				e.Property = 0
				sendSelectionNotify(e)
			}

		case xproto.SelectionNotifyEvent:
			selnotify <- (e.Property == clipboardAtom) || (e.Property == primaryAtom)
		}
	}
}

func lookupAtom(at xproto.Atom) string {
	if s, ok := clipboardAtomCache[at]; ok {
		return s
	}

	reply, err := xproto.GetAtomName(X, at).Reply()
	if err != nil {
		panic(err)
	}

	// If we're here, it means we didn't have the ATOM id cached. So cache it.
	atomName := string(reply.Name)
	clipboardAtomCache[at] = atomName
	return atomName
}

func sendSelectionNotify(e xproto.SelectionRequestEvent) {
	sn := xproto.SelectionNotifyEvent{
		Time:      xproto.TimeCurrentTime,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    e.Target,
		Property:  e.Property}
	sec := xproto.SendEventChecked(X, false, e.Requestor, 0, string(sn.Bytes()))
	err := sec.Check()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func internAtom(conn *xgb.Conn, n string) xproto.Atom {
	iac := xproto.InternAtom(conn, true, uint16(len(n)), n)
	iar, err := iac.Reply()
	if err != nil {
		panic(err)
	}
	return iar.Atom
}
