// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
#include <Foundation/Foundation.h>

__attribute__ ((visibility ("hidden"))) void gio_wakeupMainThread(void);
__attribute__ ((visibility ("hidden"))) NSUInteger gio_nsstringLength(CFTypeRef str);
__attribute__ ((visibility ("hidden"))) void gio_nsstringGetCharacters(CFTypeRef str, unichar *chars, NSUInteger loc, NSUInteger length);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createDisplayLink(void);
__attribute__ ((visibility ("hidden"))) void gio_releaseDisplayLink(CFTypeRef dl);
__attribute__ ((visibility ("hidden"))) int gio_startDisplayLink(CFTypeRef dl);
__attribute__ ((visibility ("hidden"))) int gio_stopDisplayLink(CFTypeRef dl);
__attribute__ ((visibility ("hidden"))) void gio_setDisplayLinkDisplay(CFTypeRef dl, uint64_t did);
*/
import "C"
import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unsafe"
)

// displayLink is the state for a display link (CVDisplayLinkRef on macOS,
// CADisplayLink on iOS). It runs a state-machine goroutine that keeps the
// display link running for a while after being stopped to avoid the thread
// start/stop overhead and because the CVDisplayLink sometimes fails to
// start, stop and start again within a short duration.
type displayLink struct {
	callback func()
	// states is for starting or stopping the display link.
	states chan bool
	// done is closed when the display link is destroyed.
	done chan struct{}
	// dids receives the display id when the callback owner is moved
	// to a different screen.
	dids chan uint64
	// running tracks the desired state of the link. running is accessed
	// with atomic.
	running uint32
}

// displayLinks maps CFTypeRefs to *displayLinks.
var displayLinks sync.Map

var mainFuncs = make(chan func(), 1)

// runOnMain runs the function on the main thread.
func runOnMain(f func()) {
	go func() {
		mainFuncs <- f
		C.gio_wakeupMainThread()
	}()
}

//export gio_dispatchMainFuncs
func gio_dispatchMainFuncs() {
	for {
		select {
		case f := <-mainFuncs:
			f()
		default:
			return
		}
	}
}

// nsstringToString converts a NSString to a Go string, and
// releases the original string.
func nsstringToString(str C.CFTypeRef) string {
	defer C.CFRelease(str)
	n := C.gio_nsstringLength(str)
	if n == 0 {
		return ""
	}
	chars := make([]uint16, n)
	C.gio_nsstringGetCharacters(str, (*C.unichar)(unsafe.Pointer(&chars[0])), 0, n)
	utf8 := utf16.Decode(chars)
	return string(utf8)
}

func NewDisplayLink(callback func()) (*displayLink, error) {
	d := &displayLink{
		callback: callback,
		done:     make(chan struct{}),
		states:   make(chan bool),
		dids:     make(chan uint64),
	}
	dl := C.gio_createDisplayLink()
	if dl == 0 {
		return nil, errors.New("app: failed to create display link")
	}
	go d.run(dl)
	return d, nil
}

func (d *displayLink) run(dl C.CFTypeRef) {
	defer C.gio_releaseDisplayLink(dl)
	displayLinks.Store(dl, d)
	defer displayLinks.Delete(dl)
	var stopTimer *time.Timer
	var tchan <-chan time.Time
	started := false
	for {
		select {
		case <-tchan:
			tchan = nil
			started = false
			C.gio_stopDisplayLink(dl)
		case start := <-d.states:
			switch {
			case !start && tchan == nil:
				// stopTimeout is the delay before stopping the display link to
				// avoid the overhead of frequently starting and stopping the
				// link thread.
				const stopTimeout = 500 * time.Millisecond
				if stopTimer == nil {
					stopTimer = time.NewTimer(stopTimeout)
				} else {
					// stopTimer is always drained when tchan == nil.
					stopTimer.Reset(stopTimeout)
				}
				tchan = stopTimer.C
				atomic.StoreUint32(&d.running, 0)
			case start:
				if tchan != nil && !stopTimer.Stop() {
					<-tchan
				}
				tchan = nil
				atomic.StoreUint32(&d.running, 1)
				if !started {
					started = true
					C.gio_startDisplayLink(dl)
				}
			}
		case did := <-d.dids:
			C.gio_setDisplayLinkDisplay(dl, C.uint64_t(did))
		case <-d.done:
			return
		}
	}
}

func (d *displayLink) Start() {
	d.states <- true
}

func (d *displayLink) Stop() {
	d.states <- false
}

func (d *displayLink) Close() {
	close(d.done)
}

func (d *displayLink) SetDisplayID(did uint64) {
	d.dids <- did
}

//export gio_onFrameCallback
func gio_onFrameCallback(dl C.CFTypeRef) {
	if d, exists := displayLinks.Load(dl); exists {
		d := d.(*displayLink)
		if atomic.LoadUint32(&d.running) != 0 {
			d.callback()
		}
	}
}
