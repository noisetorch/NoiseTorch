// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

package window

import (
	"errors"
	"image"
	"runtime"
	"time"
	"unicode"
	"unicode/utf16"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"

	_ "gioui.org/app/internal/cocoainit"
)

/*
#cgo CFLAGS: -DGL_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include <AppKit/AppKit.h>

#define GIO_MOUSE_MOVE 1
#define GIO_MOUSE_UP 2
#define GIO_MOUSE_DOWN 3
#define GIO_MOUSE_SCROLL 4

__attribute__ ((visibility ("hidden"))) void gio_main(void);
__attribute__ ((visibility ("hidden"))) CGFloat gio_viewWidth(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) CGFloat gio_viewHeight(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) CGFloat gio_getViewBackingScale(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) CGFloat gio_getScreenBackingScale(void);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_readClipboard(void);
__attribute__ ((visibility ("hidden"))) void gio_writeClipboard(unichar *chars, NSUInteger length);
__attribute__ ((visibility ("hidden"))) void gio_setNeedsDisplay(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createWindow(CFTypeRef viewRef, const char *title, CGFloat width, CGFloat height, CGFloat minWidth, CGFloat minHeight, CGFloat maxWidth, CGFloat maxHeight);
__attribute__ ((visibility ("hidden"))) void gio_makeKeyAndOrderFront(CFTypeRef windowRef);
__attribute__ ((visibility ("hidden"))) NSPoint gio_cascadeTopLeftFromPoint(CFTypeRef windowRef, NSPoint topLeft);
__attribute__ ((visibility ("hidden"))) void gio_close(CFTypeRef windowRef);
*/
import "C"

func init() {
	// Darwin requires that UI operations happen on the main thread only.
	runtime.LockOSThread()
}

type window struct {
	view        C.CFTypeRef
	window      C.CFTypeRef
	w           Callbacks
	stage       system.Stage
	displayLink *displayLink

	scale float32
}

// viewMap is the mapping from Cocoa NSViews to Go windows.
var viewMap = make(map[C.CFTypeRef]*window)

var viewFactory func() C.CFTypeRef

// launched is closed when applicationDidFinishLaunching is called.
var launched = make(chan struct{})

// nextTopLeft is the offset to use for the next window's call to
// cascadeTopLeftFromPoint.
var nextTopLeft C.NSPoint

// mustView is like lookoupView, except that it panics
// if the view isn't mapped.
func mustView(view C.CFTypeRef) *window {
	w, ok := lookupView(view)
	if !ok {
		panic("no window for view")
	}
	return w
}

func lookupView(view C.CFTypeRef) (*window, bool) {
	w, exists := viewMap[view]
	if !exists {
		return nil, false
	}
	return w, true
}

func deleteView(view C.CFTypeRef) {
	delete(viewMap, view)
}

func insertView(view C.CFTypeRef, w *window) {
	viewMap[view] = w
}

func (w *window) contextView() C.CFTypeRef {
	return w.view
}

func (w *window) ReadClipboard() {
	runOnMain(func() {
		content := nsstringToString(C.gio_readClipboard())
		w.w.Event(system.ClipboardEvent{Text: content})
	})
}

func (w *window) WriteClipboard(s string) {
	u16 := utf16.Encode([]rune(s))
	runOnMain(func() {
		var chars *C.unichar
		if len(u16) > 0 {
			chars = (*C.unichar)(unsafe.Pointer(&u16[0]))
		}
		C.gio_writeClipboard(chars, C.NSUInteger(len(u16)))
	})
}

func (w *window) ShowTextInput(show bool) {}

func (w *window) SetAnimating(anim bool) {
	if anim {
		w.displayLink.Start()
	} else {
		w.displayLink.Stop()
	}
}

func (w *window) Close() {
	runOnMain(func() {
		// Make sure the view is still valid. The window might've been closed
		// during the switch to the main thread.
		if w.view != 0 {
			C.gio_close(w.window)
		}
	})
}

func (w *window) setStage(stage system.Stage) {
	if stage == w.stage {
		return
	}
	w.stage = stage
	w.w.Event(system.StageEvent{Stage: stage})
}

//export gio_onKeys
func gio_onKeys(view C.CFTypeRef, cstr *C.char, ti C.double, mods C.NSUInteger) {
	str := C.GoString(cstr)
	kmods := convertMods(mods)
	w := mustView(view)
	for _, k := range str {
		if n, ok := convertKey(k); ok {
			w.w.Event(key.Event{
				Name:      n,
				Modifiers: kmods,
			})
		}
	}
}

//export gio_onText
func gio_onText(view C.CFTypeRef, cstr *C.char) {
	str := C.GoString(cstr)
	w := mustView(view)
	w.w.Event(key.EditEvent{Text: str})
}

//export gio_onMouse
func gio_onMouse(view C.CFTypeRef, cdir C.int, cbtns C.NSUInteger, x, y, dx, dy C.CGFloat, ti C.double, mods C.NSUInteger) {
	var typ pointer.Type
	switch cdir {
	case C.GIO_MOUSE_MOVE:
		typ = pointer.Move
	case C.GIO_MOUSE_UP:
		typ = pointer.Release
	case C.GIO_MOUSE_DOWN:
		typ = pointer.Press
	case C.GIO_MOUSE_SCROLL:
		typ = pointer.Scroll
	default:
		panic("invalid direction")
	}
	var btns pointer.Buttons
	if cbtns&(1<<0) != 0 {
		btns |= pointer.ButtonLeft
	}
	if cbtns&(1<<1) != 0 {
		btns |= pointer.ButtonRight
	}
	if cbtns&(1<<2) != 0 {
		btns |= pointer.ButtonMiddle
	}
	t := time.Duration(float64(ti)*float64(time.Second) + .5)
	w := mustView(view)
	xf, yf := float32(x)*w.scale, float32(y)*w.scale
	dxf, dyf := float32(dx)*w.scale, float32(dy)*w.scale
	w.w.Event(pointer.Event{
		Type:      typ,
		Source:    pointer.Mouse,
		Time:      t,
		Buttons:   btns,
		Position:  f32.Point{X: xf, Y: yf},
		Scroll:    f32.Point{X: dxf, Y: dyf},
		Modifiers: convertMods(mods),
	})
}

//export gio_onDraw
func gio_onDraw(view C.CFTypeRef) {
	w := mustView(view)
	w.draw()
}

//export gio_onFocus
func gio_onFocus(view C.CFTypeRef, focus C.BOOL) {
	w := mustView(view)
	w.w.Event(key.FocusEvent{Focus: focus == C.YES})
}

//export gio_onChangeScreen
func gio_onChangeScreen(view C.CFTypeRef, did uint64) {
	w := mustView(view)
	w.displayLink.SetDisplayID(did)
}

func (w *window) draw() {
	w.scale = float32(C.gio_getViewBackingScale(w.view))
	wf, hf := float32(C.gio_viewWidth(w.view)), float32(C.gio_viewHeight(w.view))
	if wf == 0 || hf == 0 {
		return
	}
	width := int(wf*w.scale + .5)
	height := int(hf*w.scale + .5)
	cfg := configFor(w.scale)
	w.setStage(system.StageRunning)
	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: width,
				Y: height,
			},
			Metric: cfg,
		},
		Sync: true,
	})
}

func configFor(scale float32) unit.Metric {
	return unit.Metric{
		PxPerDp: scale,
		PxPerSp: scale,
	}
}

//export gio_onClose
func gio_onClose(view C.CFTypeRef) {
	w := mustView(view)
	w.displayLink.Close()
	deleteView(view)
	w.w.Event(system.DestroyEvent{})
	C.CFRelease(w.view)
	w.view = 0
	C.CFRelease(w.window)
	w.window = 0
}

//export gio_onHide
func gio_onHide(view C.CFTypeRef) {
	w := mustView(view)
	w.setStage(system.StagePaused)
}

//export gio_onShow
func gio_onShow(view C.CFTypeRef) {
	w := mustView(view)
	w.setStage(system.StageRunning)
}

//export gio_onAppHide
func gio_onAppHide() {
	for _, w := range viewMap {
		w.setStage(system.StagePaused)
	}
}

//export gio_onAppShow
func gio_onAppShow() {
	for _, w := range viewMap {
		w.setStage(system.StageRunning)
	}
}

//export gio_onFinishLaunching
func gio_onFinishLaunching() {
	close(launched)
}

func NewWindow(win Callbacks, opts *Options) error {
	<-launched
	errch := make(chan error)
	runOnMain(func() {
		w, err := newWindow(opts)
		if err != nil {
			errch <- err
			return
		}
		screenScale := float32(C.gio_getScreenBackingScale())
		cfg := configFor(screenScale)
		width := cfg.Px(opts.Width)
		height := cfg.Px(opts.Height)
		// Window sizes is in unscaled screen coordinates, not device pixels.
		width = int(float32(width) / screenScale)
		height = int(float32(height) / screenScale)
		minWidth := cfg.Px(opts.MinWidth)
		minHeight := cfg.Px(opts.MinHeight)
		minWidth = int(float32(minWidth) / screenScale)
		minHeight = int(float32(minHeight) / screenScale)
		maxWidth := cfg.Px(opts.MaxWidth)
		maxHeight := cfg.Px(opts.MaxHeight)
		maxWidth = int(float32(maxWidth) / screenScale)
		maxHeight = int(float32(maxHeight) / screenScale)
		title := C.CString(opts.Title)
		defer C.free(unsafe.Pointer(title))
		errch <- nil
		win.SetDriver(w)
		w.w = win
		w.window = C.gio_createWindow(w.view, title, C.CGFloat(width), C.CGFloat(height),
			C.CGFloat(minWidth), C.CGFloat(minHeight), C.CGFloat(maxWidth), C.CGFloat(maxHeight))
		if nextTopLeft.x == 0 && nextTopLeft.y == 0 {
			// cascadeTopLeftFromPoint treats (0, 0) as a no-op,
			// and just returns the offset we need for the first window.
			nextTopLeft = C.gio_cascadeTopLeftFromPoint(w.window, nextTopLeft)
		}
		nextTopLeft = C.gio_cascadeTopLeftFromPoint(w.window, nextTopLeft)
		C.gio_makeKeyAndOrderFront(w.window)
	})
	return <-errch
}

func newWindow(opts *Options) (*window, error) {
	view := viewFactory()
	if view == 0 {
		return nil, errors.New("CreateWindow: failed to create view")
	}
	scale := float32(C.gio_getViewBackingScale(view))
	w := &window{
		view:  view,
		scale: scale,
	}
	dl, err := NewDisplayLink(func() {
		runOnMain(func() {
			if w.view != 0 {
				C.gio_setNeedsDisplay(w.view)
			}
		})
	})
	w.displayLink = dl
	if err != nil {
		C.CFRelease(view)
		return nil, err
	}
	insertView(view, w)
	return w, nil
}

func Main() {
	C.gio_main()
}

func convertKey(k rune) (string, bool) {
	var n string
	switch k {
	case 0x1b:
		n = key.NameEscape
	case C.NSLeftArrowFunctionKey:
		n = key.NameLeftArrow
	case C.NSRightArrowFunctionKey:
		n = key.NameRightArrow
	case C.NSUpArrowFunctionKey:
		n = key.NameUpArrow
	case C.NSDownArrowFunctionKey:
		n = key.NameDownArrow
	case 0xd:
		n = key.NameReturn
	case 0x3:
		n = key.NameEnter
	case C.NSHomeFunctionKey:
		n = key.NameHome
	case C.NSEndFunctionKey:
		n = key.NameEnd
	case 0x7f:
		n = key.NameDeleteBackward
	case C.NSDeleteFunctionKey:
		n = key.NameDeleteForward
	case C.NSPageUpFunctionKey:
		n = key.NamePageUp
	case C.NSPageDownFunctionKey:
		n = key.NamePageDown
	case C.NSF1FunctionKey:
		n = "F1"
	case C.NSF2FunctionKey:
		n = "F2"
	case C.NSF3FunctionKey:
		n = "F3"
	case C.NSF4FunctionKey:
		n = "F4"
	case C.NSF5FunctionKey:
		n = "F5"
	case C.NSF6FunctionKey:
		n = "F6"
	case C.NSF7FunctionKey:
		n = "F7"
	case C.NSF8FunctionKey:
		n = "F8"
	case C.NSF9FunctionKey:
		n = "F9"
	case C.NSF10FunctionKey:
		n = "F10"
	case C.NSF11FunctionKey:
		n = "F11"
	case C.NSF12FunctionKey:
		n = "F12"
	case 0x09, 0x19:
		n = key.NameTab
	case 0x20:
		n = "Space"
	default:
		k = unicode.ToUpper(k)
		if !unicode.IsPrint(k) {
			return "", false
		}
		n = string(k)
	}
	return n, true
}

func convertMods(mods C.NSUInteger) key.Modifiers {
	var kmods key.Modifiers
	if mods&C.NSAlternateKeyMask != 0 {
		kmods |= key.ModAlt
	}
	if mods&C.NSControlKeyMask != 0 {
		kmods |= key.ModCtrl
	}
	if mods&C.NSCommandKeyMask != 0 {
		kmods |= key.ModCommand
	}
	if mods&C.NSShiftKeyMask != 0 {
		kmods |= key.ModShift
	}
	return kmods
}
