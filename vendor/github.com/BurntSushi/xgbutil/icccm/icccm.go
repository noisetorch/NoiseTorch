package icccm

import (
	"fmt"

	"github.com/BurntSushi/xgb/xproto"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xprop"
)

const (
	HintInput = (1 << iota)
	HintState
	HintIconPixmap
	HintIconWindow
	HintIconPosition
	HintIconMask
	HintWindowGroup
	HintMessage
	HintUrgency
)

const (
	SizeHintUSPosition = (1 << iota)
	SizeHintUSSize
	SizeHintPPosition
	SizeHintPSize
	SizeHintPMinSize
	SizeHintPMaxSize
	SizeHintPResizeInc
	SizeHintPAspect
	SizeHintPBaseSize
	SizeHintPWinGravity
)

const (
	StateWithdrawn = iota
	StateNormal
	StateZoomed
	StateIconic
	StateInactive
)

// WM_NAME get
func WmNameGet(xu *xgbutil.XUtil, win xproto.Window) (string, error) {
	return xprop.PropValStr(xprop.GetProperty(xu, win, "WM_NAME"))
}

// WM_NAME set
func WmNameSet(xu *xgbutil.XUtil, win xproto.Window, name string) error {
	return xprop.ChangeProp(xu, win, 8, "WM_NAME", "STRING", ([]byte)(name))
}

// WM_ICON_NAME get
func WmIconNameGet(xu *xgbutil.XUtil, win xproto.Window) (string, error) {
	return xprop.PropValStr(xprop.GetProperty(xu, win, "WM_ICON_NAME"))
}

// WM_ICON_NAME set
func WmIconNameSet(xu *xgbutil.XUtil, win xproto.Window, name string) error {
	return xprop.ChangeProp(xu, win, 8, "WM_ICON_NAME", "STRING",
		([]byte)(name))
}

// NormalHints is a struct that organizes the information related to the
// WM_NORMAL_HINTS property. Please see the ICCCM spec for more details.
type NormalHints struct {
	Flags                                                   uint
	X, Y                                                    int
	Width, Height, MinWidth, MinHeight, MaxWidth, MaxHeight uint
	WidthInc, HeightInc                                     uint
	MinAspectNum, MinAspectDen, MaxAspectNum, MaxAspectDen  uint
	BaseWidth, BaseHeight, WinGravity                       uint
}

// WM_NORMAL_HINTS get
func WmNormalHintsGet(xu *xgbutil.XUtil,
	win xproto.Window) (nh *NormalHints, err error) {

	lenExpect := 18
	hints, err := xprop.PropValNums(xprop.GetProperty(xu, win,
		"WM_NORMAL_HINTS"))
	if err != nil {
		return nil, err
	}
	if len(hints) != lenExpect {
		return nil,
			fmt.Errorf("WmNormalHint: There are %d fields in WM_NORMAL_HINTS, "+
				"but xgbutil expects %d.", len(hints), lenExpect)
	}

	nh = &NormalHints{}
	nh.Flags = hints[0]
	nh.X = int(hints[1])
	nh.Y = int(hints[2])
	nh.Width = hints[3]
	nh.Height = hints[4]
	nh.MinWidth = hints[5]
	nh.MinHeight = hints[6]
	nh.MaxWidth = hints[7]
	nh.MaxHeight = hints[8]
	nh.WidthInc = hints[9]
	nh.HeightInc = hints[10]
	nh.MinAspectNum = hints[11]
	nh.MinAspectDen = hints[12]
	nh.MaxAspectNum = hints[13]
	nh.MaxAspectDen = hints[14]
	nh.BaseWidth = hints[15]
	nh.BaseHeight = hints[16]
	nh.WinGravity = hints[17]

	if nh.WinGravity <= 0 {
		nh.WinGravity = xproto.GravityNorthWest
	}

	return nh, nil
}

// WM_NORMAL_HINTS set
// Make sure to set the flags in the NormalHints struct correctly!
func WmNormalHintsSet(xu *xgbutil.XUtil, win xproto.Window,
	nh *NormalHints) error {

	raw := []uint{
		nh.Flags,
		uint(nh.X), uint(nh.Y), nh.Width, nh.Height,
		nh.MinWidth, nh.MinHeight,
		nh.MaxWidth, nh.MaxHeight,
		nh.WidthInc, nh.HeightInc,
		nh.MinAspectNum, nh.MinAspectDen,
		nh.MaxAspectNum, nh.MaxAspectDen,
		nh.BaseWidth, nh.BaseHeight,
		nh.WinGravity,
	}
	return xprop.ChangeProp32(xu, win, "WM_NORMAL_HINTS", "WM_SIZE_HINTS",
		raw...)
}

// Hints is a struct that organizes information related to the WM_HINTS
// property. Once again, I refer you to the ICCCM spec for documentation.
type Hints struct {
	Flags                   uint
	Input, InitialState     uint
	IconX, IconY            int
	IconPixmap, IconMask    xproto.Pixmap
	WindowGroup, IconWindow xproto.Window
}

// WM_HINTS get
func WmHintsGet(xu *xgbutil.XUtil,
	win xproto.Window) (hints *Hints, err error) {

	lenExpect := 9
	raw, err := xprop.PropValNums(xprop.GetProperty(xu, win, "WM_HINTS"))
	if err != nil {
		return nil, err
	}
	if len(raw) != lenExpect {
		return nil,
			fmt.Errorf("WmHints: There are %d fields in "+
				"WM_HINTS, but xgbutil expects %d.", len(raw), lenExpect)
	}

	hints = &Hints{}
	hints.Flags = raw[0]
	hints.Input = raw[1]
	hints.InitialState = raw[2]
	hints.IconPixmap = xproto.Pixmap(raw[3])
	hints.IconWindow = xproto.Window(raw[4])
	hints.IconX = int(raw[5])
	hints.IconY = int(raw[6])
	hints.IconMask = xproto.Pixmap(raw[7])
	hints.WindowGroup = xproto.Window(raw[8])

	return hints, nil
}

// WM_HINTS set
// Make sure to set the flags in the Hints struct correctly!
func WmHintsSet(xu *xgbutil.XUtil, win xproto.Window, hints *Hints) error {
	raw := []uint{
		hints.Flags, hints.Input, hints.InitialState,
		uint(hints.IconPixmap), uint(hints.IconWindow),
		uint(hints.IconX), uint(hints.IconY),
		uint(hints.IconMask),
		uint(hints.WindowGroup),
	}
	return xprop.ChangeProp32(xu, win, "WM_HINTS", "WM_HINTS", raw...)
}

// WmClass struct contains two data points:
// the instance and a class of a window.
type WmClass struct {
	Instance, Class string
}

// WM_CLASS get
func WmClassGet(xu *xgbutil.XUtil, win xproto.Window) (*WmClass, error) {
	raw, err := xprop.PropValStrs(xprop.GetProperty(xu, win, "WM_CLASS"))
	if err != nil {
		return nil, err
	}
	if len(raw) != 2 {
		return nil,
			fmt.Errorf("WmClass: Two string make up WM_CLASS, but "+
				"xgbutil found %d in '%v'.", len(raw), raw)
	}

	return &WmClass{
		Instance: raw[0],
		Class:    raw[1],
	}, nil
}

// WM_CLASS set
func WmClassSet(xu *xgbutil.XUtil, win xproto.Window, class *WmClass) error {
	raw := make([]byte, len(class.Instance)+len(class.Class)+2)
	copy(raw, class.Instance)
	copy(raw[(len(class.Instance)+1):], class.Class)

	return xprop.ChangeProp(xu, win, 8, "WM_CLASS", "STRING", raw)
}

// WM_TRANSIENT_FOR get
func WmTransientForGet(xu *xgbutil.XUtil,
	win xproto.Window) (xproto.Window, error) {

	return xprop.PropValWindow(xprop.GetProperty(xu, win, "WM_TRANSIENT_FOR"))
}

// WM_TRANSIENT_FOR set
func WmTransientForSet(xu *xgbutil.XUtil, win xproto.Window,
	transient xproto.Window) error {

	return xprop.ChangeProp32(xu, win, "WM_TRANSIENT_FOR", "WINDOW",
		uint(transient))
}

// WM_PROTOCOLS get
func WmProtocolsGet(xu *xgbutil.XUtil, win xproto.Window) ([]string, error) {
	raw, err := xprop.GetProperty(xu, win, "WM_PROTOCOLS")
	return xprop.PropValAtoms(xu, raw, err)
}

// WM_PROTOCOLS set
func WmProtocolsSet(xu *xgbutil.XUtil, win xproto.Window,
	atomNames []string) error {

	atoms, err := xprop.StrToAtoms(xu, atomNames)
	if err != nil {
		return err
	}
	return xprop.ChangeProp32(xu, win, "WM_PROTOCOLS", "ATOM", atoms...)
}

// WM_COLORMAP_WINDOWS get
func WmColormapWindowsGet(xu *xgbutil.XUtil,
	win xproto.Window) ([]xproto.Window, error) {

	return xprop.PropValWindows(xprop.GetProperty(xu, win,
		"WM_COLORMAP_WINDOWS"))
}

// WM_COLORMAP_WINDOWS set
func WmColormapWindowsSet(xu *xgbutil.XUtil, win xproto.Window,
	windows []xproto.Window) error {

	return xprop.ChangeProp32(xu, win, "WM_COLORMAP_WINDOWS", "WINDOW",
		xprop.WindowToInt(windows)...)
}

// WM_CLIENT_MACHINE get
func WmClientMachineGet(xu *xgbutil.XUtil, win xproto.Window) (string, error) {
	return xprop.PropValStr(xprop.GetProperty(xu, win, "WM_CLIENT_MACHINE"))
}

// WM_CLIENT_MACHINE set
func WmClientMachineSet(xu *xgbutil.XUtil, win xproto.Window,
	client string) error {

	return xprop.ChangeProp(xu, win, 8, "WM_CLIENT_MACHINE", "STRING",
		([]byte)(client))
}

// WmState is a struct that organizes information related to the WM_STATE
// property. Namely, the state (corresponding to a State* constant in this file)
// and the icon window (probably not used).
type WmState struct {
	State uint
	Icon  xproto.Window
}

// WM_STATE get
func WmStateGet(xu *xgbutil.XUtil, win xproto.Window) (*WmState, error) {
	raw, err := xprop.PropValNums(xprop.GetProperty(xu, win, "WM_STATE"))
	if err != nil {
		return nil, err
	}
	if len(raw) != 2 {
		return nil,
			fmt.Errorf("WmState: Expected two integers in WM_STATE property "+
				"but xgbutil found %d in '%v'.", len(raw), raw)
	}

	return &WmState{
		State: raw[0],
		Icon:  xproto.Window(raw[1]),
	}, nil
}

// WM_STATE set
func WmStateSet(xu *xgbutil.XUtil, win xproto.Window, state *WmState) error {
	raw := []uint{
		state.State,
		uint(state.Icon),
	}

	return xprop.ChangeProp32(xu, win, "WM_STATE", "WM_STATE", raw...)
}

// IconSize is a struct the organizes information related to the WM_ICON_SIZE
// property. Mostly info about its dimensions.
type IconSize struct {
	MinWidth, MinHeight, MaxWidth, MaxHeight, WidthInc, HeightInc uint
}

// WM_ICON_SIZE get
func WmIconSizeGet(xu *xgbutil.XUtil, win xproto.Window) (*IconSize, error) {
	raw, err := xprop.PropValNums(xprop.GetProperty(xu, win, "WM_ICON_SIZE"))
	if err != nil {
		return nil, err
	}
	if len(raw) != 6 {
		return nil,
			fmt.Errorf("WmIconSize: Expected six integers in WM_ICON_SIZE "+
				"property, but xgbutil found "+"%d in '%v'.", len(raw), raw)
	}

	return &IconSize{
		MinWidth: raw[0], MinHeight: raw[1],
		MaxWidth: raw[2], MaxHeight: raw[3],
		WidthInc: raw[4], HeightInc: raw[5],
	}, nil
}

// WM_ICON_SIZE set
func WmIconSizeSet(xu *xgbutil.XUtil, win xproto.Window,
	icondim *IconSize) error {

	raw := []uint{
		icondim.MinWidth, icondim.MinHeight,
		icondim.MaxWidth, icondim.MaxHeight,
		icondim.WidthInc, icondim.HeightInc,
	}

	return xprop.ChangeProp32(xu, win, "WM_ICON_SIZE", "WM_ICON_SIZE", raw...)
}
