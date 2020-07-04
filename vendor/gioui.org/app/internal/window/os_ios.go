// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

package window

/*
#cgo CFLAGS: -DGLES_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include <CoreGraphics/CoreGraphics.h>
#include <UIKit/UIKit.h>
#include <stdint.h>

struct drawParams {
	CGFloat dpi, sdpi;
	CGFloat width, height;
	CGFloat top, right, bottom, left;
};

__attribute__ ((visibility ("hidden"))) void gio_showTextInput(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_hideTextInput(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_addLayerToView(CFTypeRef viewRef, CFTypeRef layerRef);
__attribute__ ((visibility ("hidden"))) void gio_updateView(CFTypeRef viewRef, CFTypeRef layerRef);
__attribute__ ((visibility ("hidden"))) void gio_removeLayer(CFTypeRef layerRef);
__attribute__ ((visibility ("hidden"))) struct drawParams gio_viewDrawParams(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_readClipboard(void);
__attribute__ ((visibility ("hidden"))) void gio_writeClipboard(unichar *chars, NSUInteger length);
*/
import "C"

import (
	"image"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type window struct {
	view        C.CFTypeRef
	w           Callbacks
	displayLink *displayLink

	layer   C.CFTypeRef
	visible atomic.Value

	pointerMap []C.CFTypeRef
}

var mainWindow = newWindowRendezvous()

var layerFactory func() uintptr

var views = make(map[C.CFTypeRef]*window)

func init() {
	// Darwin requires UI operations happen on the main thread only.
	runtime.LockOSThread()
}

//export onCreate
func onCreate(view C.CFTypeRef) {
	w := &window{
		view: view,
	}
	dl, err := NewDisplayLink(func() {
		w.draw(false)
	})
	if err != nil {
		panic(err)
	}
	w.displayLink = dl
	wopts := <-mainWindow.out
	w.w = wopts.window
	w.w.SetDriver(w)
	w.visible.Store(false)
	w.layer = C.CFTypeRef(layerFactory())
	C.gio_addLayerToView(view, w.layer)
	views[view] = w
	w.w.Event(system.StageEvent{Stage: system.StagePaused})
}

//export gio_onDraw
func gio_onDraw(view C.CFTypeRef) {
	w := views[view]
	w.draw(true)
}

func (w *window) draw(sync bool) {
	params := C.gio_viewDrawParams(w.view)
	if params.width == 0 || params.height == 0 {
		return
	}
	wasVisible := w.isVisible()
	w.visible.Store(true)
	C.gio_updateView(w.view, w.layer)
	if !wasVisible {
		w.w.Event(system.StageEvent{Stage: system.StageRunning})
	}
	const inchPrDp = 1.0 / 163
	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: int(params.width + .5),
				Y: int(params.height + .5),
			},
			Insets: system.Insets{
				Top:    unit.Px(float32(params.top)),
				Right:  unit.Px(float32(params.right)),
				Bottom: unit.Px(float32(params.bottom)),
				Left:   unit.Px(float32(params.left)),
			},
			Metric: unit.Metric{
				PxPerDp: float32(params.dpi) * inchPrDp,
				PxPerSp: float32(params.sdpi) * inchPrDp,
			},
		},
		Sync: sync,
	})
}

//export onStop
func onStop(view C.CFTypeRef) {
	w := views[view]
	w.visible.Store(false)
	w.w.Event(system.StageEvent{Stage: system.StagePaused})
}

//export onDestroy
func onDestroy(view C.CFTypeRef) {
	w := views[view]
	delete(views, view)
	w.w.Event(system.DestroyEvent{})
	w.displayLink.Close()
	C.gio_removeLayer(w.layer)
	C.CFRelease(w.layer)
	w.layer = 0
	w.view = 0
}

//export onFocus
func onFocus(view C.CFTypeRef, focus int) {
	w := views[view]
	w.w.Event(key.FocusEvent{Focus: focus != 0})
}

//export onLowMemory
func onLowMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}

//export onUpArrow
func onUpArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameUpArrow)
}

//export onDownArrow
func onDownArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameDownArrow)
}

//export onLeftArrow
func onLeftArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameLeftArrow)
}

//export onRightArrow
func onRightArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameRightArrow)
}

//export onDeleteBackward
func onDeleteBackward(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameDeleteBackward)
}

//export onText
func onText(view C.CFTypeRef, str *C.char) {
	w := views[view]
	w.w.Event(key.EditEvent{
		Text: C.GoString(str),
	})
}

//export onTouch
func onTouch(last C.int, view, touchRef C.CFTypeRef, phase C.NSInteger, x, y C.CGFloat, ti C.double) {
	var typ pointer.Type
	switch phase {
	case C.UITouchPhaseBegan:
		typ = pointer.Press
	case C.UITouchPhaseMoved:
		typ = pointer.Move
	case C.UITouchPhaseEnded:
		typ = pointer.Release
	case C.UITouchPhaseCancelled:
		typ = pointer.Cancel
	default:
		return
	}
	w := views[view]
	t := time.Duration(float64(ti) * float64(time.Second))
	p := f32.Point{X: float32(x), Y: float32(y)}
	w.w.Event(pointer.Event{
		Type:      typ,
		Source:    pointer.Touch,
		PointerID: w.lookupTouch(last != 0, touchRef),
		Position:  p,
		Time:      t,
	})
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

func (w *window) SetAnimating(anim bool) {
	v := w.view
	if v == 0 {
		return
	}
	if anim {
		w.displayLink.Start()
	} else {
		w.displayLink.Stop()
	}
}

func (w *window) onKeyCommand(name string) {
	w.w.Event(key.Event{
		Name: name,
	})
}

// lookupTouch maps an UITouch pointer value to an index. If
// last is set, the map is cleared.
func (w *window) lookupTouch(last bool, touch C.CFTypeRef) pointer.ID {
	id := -1
	for i, ref := range w.pointerMap {
		if ref == touch {
			id = i
			break
		}
	}
	if id == -1 {
		id = len(w.pointerMap)
		w.pointerMap = append(w.pointerMap, touch)
	}
	if last {
		w.pointerMap = w.pointerMap[:0]
	}
	return pointer.ID(id)
}

func (w *window) contextLayer() uintptr {
	return uintptr(w.layer)
}

func (w *window) isVisible() bool {
	return w.visible.Load().(bool)
}

func (w *window) ShowTextInput(show bool) {
	v := w.view
	if v == 0 {
		return
	}
	C.CFRetain(v)
	runOnMain(func() {
		defer C.CFRelease(v)
		if show {
			C.gio_showTextInput(w.view)
		} else {
			C.gio_hideTextInput(w.view)
		}
	})
}

// Close the window. Not implemented for iOS.
func (w *window) Close() {}

func NewWindow(win Callbacks, opts *Options) error {
	mainWindow.in <- windowAndOptions{win, opts}
	return <-mainWindow.errs
}

func Main() {
}

//export gio_runMain
func gio_runMain() {
	runMain()
}
