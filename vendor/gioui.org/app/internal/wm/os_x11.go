// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nox11 freebsd openbsd

package wm

/*
#cgo openbsd CFLAGS: -I/usr/X11R6/include -I/usr/local/include
#cgo openbsd LDFLAGS: -L/usr/X11R6/lib -L/usr/local/lib
#cgo freebsd openbsd LDFLAGS: -lX11 -lxkbcommon -lxkbcommon-x11 -lX11-xcb -lXcursor -lXfixes
#cgo linux pkg-config: x11 xkbcommon xkbcommon-x11 x11-xcb xcursor xfixes

#include <stdlib.h>
#include <locale.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/Xresource.h>
#include <X11/XKBlib.h>
#include <X11/Xlib-xcb.h>
#include <X11/extensions/Xfixes.h>
#include <X11/Xcursor/Xcursor.h>
#include <xkbcommon/xkbcommon-x11.h>

*/
import "C"
import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"

	syscall "golang.org/x/sys/unix"

	"gioui.org/app/internal/xkb"
)

type x11Window struct {
	w            Callbacks
	x            *C.Display
	xkb          *xkb.Context
	xkbEventBase C.int
	xw           C.Window

	atoms struct {
		// "UTF8_STRING".
		utf8string C.Atom
		// "text/plain;charset=utf-8".
		plaintext C.Atom
		// "TARGETS"
		targets C.Atom
		// "CLIPBOARD".
		clipboard C.Atom
		// "CLIPBOARD_CONTENT", the clipboard destination property.
		clipboardContent C.Atom
		// "WM_DELETE_WINDOW"
		evDelWindow C.Atom
		// "ATOM"
		atom C.Atom
		// "GTK_TEXT_BUFFER_CONTENTS"
		gtk_text_buffer_contents C.Atom
		// "_NET_WM_NAME"
		wmName C.Atom
		// "_NET_WM_STATE"
		wmState C.Atom
		// _NET_WM_STATE_FULLSCREEN"
		wmStateFullscreen C.Atom
	}
	stage  system.Stage
	cfg    unit.Metric
	width  int
	height int
	notify struct {
		read, write int
	}
	dead bool

	mu        sync.Mutex
	animating bool
	opts      *Options

	pointerBtns pointer.Buttons

	clipboard struct {
		read    bool
		write   *string
		content []byte
	}
	cursor pointer.CursorName
	mode   WindowMode
}

func (w *x11Window) SetAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		w.wakeup()
	}
}

func (w *x11Window) ReadClipboard() {
	w.mu.Lock()
	w.clipboard.read = true
	w.mu.Unlock()
	w.wakeup()
}

func (w *x11Window) WriteClipboard(s string) {
	w.mu.Lock()
	w.clipboard.write = &s
	w.mu.Unlock()
	w.wakeup()
}

func (w *x11Window) Option(opts *Options) {
	w.mu.Lock()
	w.opts = opts
	w.mu.Unlock()
	w.wakeup()
}

func (w *x11Window) setOptions() {
	w.mu.Lock()
	opts := w.opts
	w.opts = nil
	w.mu.Unlock()
	if opts == nil {
		return
	}
	var shints C.XSizeHints
	if o := opts.MinSize; o != nil {
		shints.min_width = C.int(w.cfg.Px(o.Width))
		shints.min_height = C.int(w.cfg.Px(o.Height))
		shints.flags = C.PMinSize
	}
	if o := opts.MaxSize; o != nil {
		shints.max_width = C.int(w.cfg.Px(o.Width))
		shints.max_height = C.int(w.cfg.Px(o.Height))
		shints.flags = shints.flags | C.PMaxSize
	}
	if shints.flags != 0 {
		C.XSetWMNormalHints(w.x, w.xw, &shints)
	}

	var title string
	if o := opts.Title; o != nil {
		title = *o
	}
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	C.XStoreName(w.x, w.xw, ctitle)
	// set _NET_WM_NAME as well for UTF-8 support in window title.
	C.XSetTextProperty(w.x, w.xw,
		&C.XTextProperty{
			value:    (*C.uchar)(unsafe.Pointer(ctitle)),
			encoding: w.atoms.utf8string,
			format:   8,
			nitems:   C.ulong(len(title)),
		},
		w.atoms.wmName)

	if o := opts.WindowMode; o != nil {
		w.SetWindowMode(*o)
	}
}

func (w *x11Window) SetCursor(name pointer.CursorName) {
	switch name {
	case pointer.CursorNone:
		w.cursor = name
		C.XFixesHideCursor(w.x, w.xw)
		return
	case pointer.CursorGrab:
		name = "hand1"
	}
	if w.cursor == pointer.CursorNone {
		C.XFixesShowCursor(w.x, w.xw)
	}
	cname := C.CString(string(name))
	defer C.free(unsafe.Pointer(cname))
	c := C.XcursorLibraryLoadCursor(w.x, cname)
	if c == 0 {
		name = pointer.CursorDefault
	}
	w.cursor = name
	// If c if null (i.e. name was not found),
	// XDefineCursor will use the default cursor.
	C.XDefineCursor(w.x, w.xw, c)
}

func (w *x11Window) SetWindowMode(mode WindowMode) {
	switch mode {
	case w.mode:
		return
	case Windowed:
		C.XDeleteProperty(w.x, w.xw, w.atoms.wmStateFullscreen)
	case Fullscreen:
		C.XChangeProperty(w.x, w.xw, w.atoms.wmState, C.XA_ATOM,
			32, C.PropModeReplace,
			(*C.uchar)(unsafe.Pointer(&w.atoms.wmStateFullscreen)), 1,
		)
	default:
		return
	}
	w.mode = mode
	// "A Client wishing to change the state of a window MUST send
	//  a _NET_WM_STATE client message to the root window (see below)."
	var xev C.XEvent
	ev := (*C.XClientMessageEvent)(unsafe.Pointer(&xev))
	*ev = C.XClientMessageEvent{
		_type:        C.ClientMessage,
		display:      w.x,
		window:       w.xw,
		message_type: w.atoms.wmState,
		format:       32,
	}
	arr := (*[5]C.long)(unsafe.Pointer(&ev.data))
	arr[0] = 2 // _NET_WM_STATE_TOGGLE
	arr[1] = C.long(w.atoms.wmStateFullscreen)
	arr[2] = 0
	arr[3] = 1 // application
	arr[4] = 0
	C.XSendEvent(
		w.x,
		C.XDefaultRootWindow(w.x), // MUST be the root window
		C.False,
		C.SubstructureNotifyMask|C.SubstructureRedirectMask,
		&xev,
	)
}

func (w *x11Window) ShowTextInput(show bool) {}

// Close the window.
func (w *x11Window) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	var xev C.XEvent
	ev := (*C.XClientMessageEvent)(unsafe.Pointer(&xev))
	*ev = C.XClientMessageEvent{
		_type:        C.ClientMessage,
		display:      w.x,
		window:       w.xw,
		message_type: w.atom("WM_PROTOCOLS", true),
		format:       32,
	}
	arr := (*[5]C.long)(unsafe.Pointer(&ev.data))
	arr[0] = C.long(w.atoms.evDelWindow)
	arr[1] = C.CurrentTime
	C.XSendEvent(w.x, w.xw, C.False, C.NoEventMask, &xev)
}

var x11OneByte = make([]byte, 1)

func (w *x11Window) wakeup() {
	if _, err := syscall.Write(w.notify.write, x11OneByte); err != nil && err != syscall.EAGAIN {
		panic(fmt.Errorf("failed to write to pipe: %v", err))
	}
}

func (w *x11Window) display() *C.Display {
	return w.x
}

func (w *x11Window) window() (C.Window, int, int) {
	return w.xw, w.width, w.height
}

func (w *x11Window) setStage(s system.Stage) {
	if s == w.stage {
		return
	}
	w.stage = s
	w.w.Event(system.StageEvent{Stage: s})
}

func (w *x11Window) loop() {
	h := x11EventHandler{w: w, xev: new(C.XEvent), text: make([]byte, 4)}
	xfd := C.XConnectionNumber(w.x)

	// Poll for events and notifications.
	pollfds := []syscall.PollFd{
		{Fd: int32(xfd), Events: syscall.POLLIN | syscall.POLLERR},
		{Fd: int32(w.notify.read), Events: syscall.POLLIN | syscall.POLLERR},
	}
	xEvents := &pollfds[0].Revents
	// Plenty of room for a backlog of notifications.
	buf := make([]byte, 100)

loop:
	for !w.dead {
		var syn, anim bool
		// Check for pending draw events before checking animation or blocking.
		// This fixes an issue on Xephyr where on startup XPending() > 0 but
		// poll will still block. This also prevents no-op calls to poll.
		if syn = h.handleEvents(); !syn {
			w.mu.Lock()
			anim = w.animating
			w.mu.Unlock()
			if !anim {
				// Clear poll events.
				*xEvents = 0
				// Wait for X event or gio notification.
				if _, err := syscall.Poll(pollfds, -1); err != nil && err != syscall.EINTR {
					panic(fmt.Errorf("x11 loop: poll failed: %w", err))
				}
				switch {
				case *xEvents&syscall.POLLIN != 0:
					syn = h.handleEvents()
					if w.dead {
						break loop
					}
				case *xEvents&(syscall.POLLERR|syscall.POLLHUP) != 0:
					break loop
				}
			}
		}
		w.setOptions()
		// Clear notifications.
		for {
			_, err := syscall.Read(w.notify.read, buf)
			if err == syscall.EAGAIN {
				break
			}
			if err != nil {
				panic(fmt.Errorf("x11 loop: read from notify pipe failed: %w", err))
			}
		}

		if anim || syn {
			w.w.Event(FrameEvent{
				FrameEvent: system.FrameEvent{
					Now: time.Now(),
					Size: image.Point{
						X: w.width,
						Y: w.height,
					},
					Metric: w.cfg,
				},
				Sync: syn,
			})
		}
		w.mu.Lock()
		readClipboard := w.clipboard.read
		writeClipboard := w.clipboard.write
		w.clipboard.read = false
		w.clipboard.write = nil
		w.mu.Unlock()
		if readClipboard {
			C.XDeleteProperty(w.x, w.xw, w.atoms.clipboardContent)
			C.XConvertSelection(w.x, w.atoms.clipboard, w.atoms.utf8string, w.atoms.clipboardContent, w.xw, C.CurrentTime)
		}
		if writeClipboard != nil {
			w.clipboard.content = []byte(*writeClipboard)
			C.XSetSelectionOwner(w.x, w.atoms.clipboard, w.xw, C.CurrentTime)
		}
	}
	w.w.Event(system.DestroyEvent{Err: nil})
}

func (w *x11Window) destroy() {
	if w.notify.write != 0 {
		syscall.Close(w.notify.write)
		w.notify.write = 0
	}
	if w.notify.read != 0 {
		syscall.Close(w.notify.read)
		w.notify.read = 0
	}
	if w.xkb != nil {
		w.xkb.Destroy()
		w.xkb = nil
	}
	C.XDestroyWindow(w.x, w.xw)
	C.XCloseDisplay(w.x)
}

// atom is a wrapper around XInternAtom. Callers should cache the result
// in order to limit round-trips to the X server.
//
func (w *x11Window) atom(name string, onlyIfExists bool) C.Atom {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	flag := C.Bool(C.False)
	if onlyIfExists {
		flag = C.True
	}
	return C.XInternAtom(w.x, cname, flag)
}

// x11EventHandler wraps static variables for the main event loop.
// Its sole purpose is to prevent heap allocation and reduce clutter
// in x11window.loop.
//
type x11EventHandler struct {
	w    *x11Window
	text []byte
	xev  *C.XEvent
}

// handleEvents returns true if the window needs to be redrawn.
//
func (h *x11EventHandler) handleEvents() bool {
	w := h.w
	xev := h.xev
	redraw := false
	for C.XPending(w.x) != 0 {
		C.XNextEvent(w.x, xev)
		if C.XFilterEvent(xev, C.None) == C.True {
			continue
		}
		switch _type := (*C.XAnyEvent)(unsafe.Pointer(xev))._type; _type {
		case h.w.xkbEventBase:
			xkbEvent := (*C.XkbAnyEvent)(unsafe.Pointer(xev))
			switch xkbEvent.xkb_type {
			case C.XkbNewKeyboardNotify, C.XkbMapNotify:
				if err := h.w.updateXkbKeymap(); err != nil {
					panic(err)
				}
			case C.XkbStateNotify:
				state := (*C.XkbStateNotifyEvent)(unsafe.Pointer(xev))
				h.w.xkb.UpdateMask(uint32(state.base_mods), uint32(state.latched_mods), uint32(state.locked_mods),
					uint32(state.base_group), uint32(state.latched_group), uint32(state.locked_group))
			}
		case C.KeyPress, C.KeyRelease:
			ks := key.Press
			if _type == C.KeyRelease {
				ks = key.Release
			}
			kevt := (*C.XKeyPressedEvent)(unsafe.Pointer(xev))
			for _, e := range h.w.xkb.DispatchKey(uint32(kevt.keycode), ks) {
				w.w.Event(e)
			}
		case C.ButtonPress, C.ButtonRelease:
			bevt := (*C.XButtonEvent)(unsafe.Pointer(xev))
			ev := pointer.Event{
				Type:   pointer.Press,
				Source: pointer.Mouse,
				Position: f32.Point{
					X: float32(bevt.x),
					Y: float32(bevt.y),
				},
				Time:      time.Duration(bevt.time) * time.Millisecond,
				Modifiers: w.xkb.Modifiers(),
			}
			if bevt._type == C.ButtonRelease {
				ev.Type = pointer.Release
			}
			var btn pointer.Buttons
			const scrollScale = 10
			switch bevt.button {
			case C.Button1:
				btn = pointer.ButtonPrimary
			case C.Button2:
				btn = pointer.ButtonTertiary
			case C.Button3:
				btn = pointer.ButtonSecondary
			case C.Button4:
				// scroll up
				ev.Type = pointer.Scroll
				ev.Scroll.Y = -scrollScale
			case C.Button5:
				// scroll down
				ev.Type = pointer.Scroll
				ev.Scroll.Y = +scrollScale
			case 6:
				// http://xahlee.info/linux/linux_x11_mouse_button_number.html
				// scroll left
				ev.Type = pointer.Scroll
				ev.Scroll.X = -scrollScale * 2
			case 7:
				// scroll right
				ev.Type = pointer.Scroll
				ev.Scroll.X = +scrollScale * 2
			default:
				continue
			}
			switch _type {
			case C.ButtonPress:
				w.pointerBtns |= btn
			case C.ButtonRelease:
				w.pointerBtns &^= btn
			}
			ev.Buttons = w.pointerBtns
			w.w.Event(ev)
		case C.MotionNotify:
			mevt := (*C.XMotionEvent)(unsafe.Pointer(xev))
			w.w.Event(pointer.Event{
				Type:    pointer.Move,
				Source:  pointer.Mouse,
				Buttons: w.pointerBtns,
				Position: f32.Point{
					X: float32(mevt.x),
					Y: float32(mevt.y),
				},
				Time:      time.Duration(mevt.time) * time.Millisecond,
				Modifiers: w.xkb.Modifiers(),
			})
		case C.Expose: // update
			// redraw only on the last expose event
			redraw = (*C.XExposeEvent)(unsafe.Pointer(xev)).count == 0
		case C.FocusIn:
			w.w.Event(key.FocusEvent{Focus: true})
		case C.FocusOut:
			w.w.Event(key.FocusEvent{Focus: false})
		case C.ConfigureNotify: // window configuration change
			cevt := (*C.XConfigureEvent)(unsafe.Pointer(xev))
			w.width = int(cevt.width)
			w.height = int(cevt.height)
			// redraw will be done by a later expose event
		case C.SelectionNotify:
			cevt := (*C.XSelectionEvent)(unsafe.Pointer(xev))
			prop := w.atoms.clipboardContent
			if cevt.property != prop {
				break
			}
			if cevt.selection != w.atoms.clipboard {
				break
			}
			var text C.XTextProperty
			if st := C.XGetTextProperty(w.x, w.xw, &text, prop); st == 0 {
				// Failed; ignore.
				break
			}
			if text.format != 8 || text.encoding != w.atoms.utf8string {
				// Ignore non-utf-8 encoded strings.
				break
			}
			str := C.GoStringN((*C.char)(unsafe.Pointer(text.value)), C.int(text.nitems))
			w.w.Event(clipboard.Event{Text: str})
		case C.SelectionRequest:
			cevt := (*C.XSelectionRequestEvent)(unsafe.Pointer(xev))
			if cevt.selection != w.atoms.clipboard || cevt.property == C.None {
				// Unsupported clipboard or obsolete requestor.
				break
			}
			notify := func() {
				var xev C.XEvent
				ev := (*C.XSelectionEvent)(unsafe.Pointer(&xev))
				*ev = C.XSelectionEvent{
					_type:     C.SelectionNotify,
					display:   cevt.display,
					requestor: cevt.requestor,
					selection: cevt.selection,
					target:    cevt.target,
					property:  cevt.property,
					time:      cevt.time,
				}
				C.XSendEvent(w.x, cevt.requestor, 0, 0, &xev)
			}
			switch cevt.target {
			case w.atoms.targets:
				// The requestor wants the supported clipboard
				// formats. First write the targets...
				formats := [...]C.long{
					C.long(w.atoms.targets),
					C.long(w.atoms.utf8string),
					C.long(w.atoms.plaintext),
					// GTK clients need this.
					C.long(w.atoms.gtk_text_buffer_contents),
				}
				C.XChangeProperty(w.x, cevt.requestor, cevt.property, w.atoms.atom,
					32 /* bitwidth of formats */, C.PropModeReplace,
					(*C.uchar)(unsafe.Pointer(&formats)), C.int(len(formats)),
				)
				// ...then notify the requestor.
				notify()
			case w.atoms.plaintext, w.atoms.utf8string, w.atoms.gtk_text_buffer_contents:
				content := w.clipboard.content
				var ptr *C.uchar
				if len(content) > 0 {
					ptr = (*C.uchar)(unsafe.Pointer(&content[0]))
				}
				C.XChangeProperty(w.x, cevt.requestor, cevt.property, cevt.target,
					8 /* bitwidth */, C.PropModeReplace,
					ptr, C.int(len(content)),
				)
				notify()
			}
		case C.ClientMessage: // extensions
			cevt := (*C.XClientMessageEvent)(unsafe.Pointer(xev))
			switch *(*C.long)(unsafe.Pointer(&cevt.data)) {
			case C.long(w.atoms.evDelWindow):
				w.dead = true
				return false
			}
		}
	}
	return redraw
}

var (
	x11Threads sync.Once
)

func init() {
	x11Driver = newX11Window
}

func newX11Window(gioWin Callbacks, opts *Options) error {
	var err error

	pipe := make([]int, 2)
	if err := syscall.Pipe2(pipe, syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return fmt.Errorf("NewX11Window: failed to create pipe: %w", err)
	}

	x11Threads.Do(func() {
		if C.XInitThreads() == 0 {
			err = errors.New("x11: threads init failed")
		}
		C.XrmInitialize()
	})
	if err != nil {
		return err
	}
	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return errors.New("x11: cannot connect to the X server")
	}
	var major, minor C.int = C.XkbMajorVersion, C.XkbMinorVersion
	var xkbEventBase C.int
	if C.XkbQueryExtension(dpy, nil, &xkbEventBase, nil, &major, &minor) != C.True {
		C.XCloseDisplay(dpy)
		return errors.New("x11: XkbQueryExtension failed")
	}
	const bits = C.uint(C.XkbNewKeyboardNotifyMask | C.XkbMapNotifyMask | C.XkbStateNotifyMask)
	if C.XkbSelectEvents(dpy, C.XkbUseCoreKbd, bits, bits) != C.True {
		C.XCloseDisplay(dpy)
		return errors.New("x11: XkbSelectEvents failed")
	}
	xkb, err := xkb.New()
	if err != nil {
		C.XCloseDisplay(dpy)
		return fmt.Errorf("x11: %v", err)
	}

	ppsp := x11DetectUIScale(dpy)
	cfg := unit.Metric{PxPerDp: ppsp, PxPerSp: ppsp}
	swa := C.XSetWindowAttributes{
		event_mask: C.ExposureMask | C.FocusChangeMask | // update
			C.KeyPressMask | C.KeyReleaseMask | // keyboard
			C.ButtonPressMask | C.ButtonReleaseMask | // mouse clicks
			C.PointerMotionMask | // mouse movement
			C.StructureNotifyMask, // resize
		background_pixmap: C.None,
		override_redirect: C.False,
	}
	var width, height int
	if o := opts.Size; o != nil {
		width = cfg.Px(o.Width)
		height = cfg.Px(o.Height)
	}
	win := C.XCreateWindow(dpy, C.XDefaultRootWindow(dpy),
		0, 0, C.uint(width), C.uint(height),
		0, C.CopyFromParent, C.InputOutput, nil,
		C.CWEventMask|C.CWBackPixmap|C.CWOverrideRedirect, &swa)

	w := &x11Window{
		w: gioWin, x: dpy, xw: win,
		width:        width,
		height:       height,
		cfg:          cfg,
		xkb:          xkb,
		xkbEventBase: xkbEventBase,
	}
	w.notify.read = pipe[0]
	w.notify.write = pipe[1]

	if err := w.updateXkbKeymap(); err != nil {
		w.destroy()
		return err
	}

	var hints C.XWMHints
	hints.input = C.True
	hints.flags = C.InputHint
	C.XSetWMHints(dpy, win, &hints)

	name := C.CString(filepath.Base(os.Args[0]))
	defer C.free(unsafe.Pointer(name))
	wmhints := C.XClassHint{name, name}
	C.XSetClassHint(dpy, win, &wmhints)

	w.atoms.utf8string = w.atom("UTF8_STRING", false)
	w.atoms.plaintext = w.atom("text/plain;charset=utf-8", false)
	w.atoms.gtk_text_buffer_contents = w.atom("GTK_TEXT_BUFFER_CONTENTS", false)
	w.atoms.evDelWindow = w.atom("WM_DELETE_WINDOW", false)
	w.atoms.clipboard = w.atom("CLIPBOARD", false)
	w.atoms.clipboardContent = w.atom("CLIPBOARD_CONTENT", false)
	w.atoms.atom = w.atom("ATOM", false)
	w.atoms.targets = w.atom("TARGETS", false)
	w.atoms.wmName = w.atom("_NET_WM_NAME", false)
	w.atoms.wmState = w.atom("_NET_WM_STATE", false)
	w.atoms.wmStateFullscreen = w.atom("_NET_WM_STATE_FULLSCREEN", false)

	// extensions
	C.XSetWMProtocols(dpy, win, &w.atoms.evDelWindow, 1)

	w.Option(opts)

	// make the window visible on the screen
	C.XMapWindow(dpy, win)

	go func() {
		w.w.SetDriver(w)
		w.setStage(system.StageRunning)
		w.loop()
		w.destroy()
	}()
	return nil
}

// detectUIScale reports the system UI scale, or 1.0 if it fails.
func x11DetectUIScale(dpy *C.Display) float32 {
	// default fixed DPI value used in most desktop UI toolkits
	const defaultDesktopDPI = 96
	var scale float32 = 1.0

	// Get actual DPI from X resource Xft.dpi (set by GTK and Qt).
	// This value is entirely based on user preferences and conflates both
	// screen (UI) scaling and font scale.
	rms := C.XResourceManagerString(dpy)
	if rms != nil {
		db := C.XrmGetStringDatabase(rms)
		if db != nil {
			var (
				t *C.char
				v C.XrmValue
			)
			if C.XrmGetResource(db, (*C.char)(unsafe.Pointer(&[]byte("Xft.dpi\x00")[0])),
				(*C.char)(unsafe.Pointer(&[]byte("Xft.Dpi\x00")[0])), &t, &v) != C.False {
				if t != nil && C.GoString(t) == "String" {
					f, err := strconv.ParseFloat(C.GoString(v.addr), 32)
					if err == nil {
						scale = float32(f) / defaultDesktopDPI
					}
				}
			}
			C.XrmDestroyDatabase(db)
		}
	}

	return scale
}

func (w *x11Window) updateXkbKeymap() error {
	w.xkb.DestroyKeymapState()
	ctx := (*C.struct_xkb_context)(unsafe.Pointer(w.xkb.Ctx))
	xcb := C.XGetXCBConnection(w.x)
	if xcb == nil {
		return errors.New("x11: XGetXCBConnection failed")
	}
	xkbDevID := C.xkb_x11_get_core_keyboard_device_id(xcb)
	if xkbDevID == -1 {
		return errors.New("x11: xkb_x11_get_core_keyboard_device_id failed")
	}
	keymap := C.xkb_x11_keymap_new_from_device(ctx, xcb, xkbDevID, C.XKB_KEYMAP_COMPILE_NO_FLAGS)
	if keymap == nil {
		return errors.New("x11: xkb_x11_keymap_new_from_device failed")
	}
	state := C.xkb_x11_state_new_from_device(keymap, xcb, xkbDevID)
	if state == nil {
		C.xkb_keymap_unref(keymap)
		return errors.New("x11: xkb_x11_keymap_new_from_device failed")
	}
	w.xkb.SetKeymap(unsafe.Pointer(keymap), unsafe.Pointer(state))
	return nil
}
