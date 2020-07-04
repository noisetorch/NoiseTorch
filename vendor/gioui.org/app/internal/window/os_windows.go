// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
	"unsafe"

	syscall "golang.org/x/sys/windows"

	"gioui.org/app/internal/windows"
	"gioui.org/unit"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
)

type winConstraints struct {
	minWidth, minHeight int32
	maxWidth, maxHeight int32
}

type winDeltas struct {
	width  int32
	height int32
}

type window struct {
	hwnd        syscall.Handle
	hdc         syscall.Handle
	w           Callbacks
	width       int
	height      int
	stage       system.Stage
	dead        bool
	pointerBtns pointer.Buttons

	mu        sync.Mutex
	animating bool

	minmax winConstraints
	deltas winDeltas
	opts   *Options
}

const _WM_REDRAW = windows.WM_USER + 0

type gpuAPI struct {
	priority    int
	initializer func(w *window) (Context, error)
}

// backends is the list of potential Context
// implementations.
var backends []gpuAPI

// winMap maps win32 HWNDs to *windows.
var winMap sync.Map

var resources struct {
	once sync.Once
	// handle is the module handle from GetModuleHandle.
	handle syscall.Handle
	// class is the Gio window class from RegisterClassEx.
	class uint16
	// cursor is the arrow cursor resource
	cursor syscall.Handle
}

func Main() {
	select {}
}

func NewWindow(window Callbacks, opts *Options) error {
	cerr := make(chan error)
	go func() {
		// Call win32 API from a single OS thread.
		runtime.LockOSThread()
		w, err := createNativeWindow(opts)
		if err != nil {
			cerr <- err
			return
		}
		defer w.destroy()
		cerr <- nil
		winMap.Store(w.hwnd, w)
		defer winMap.Delete(w.hwnd)
		w.w = window
		w.w.SetDriver(w)
		defer w.w.Event(system.DestroyEvent{})
		windows.ShowWindow(w.hwnd, windows.SW_SHOWDEFAULT)
		windows.SetForegroundWindow(w.hwnd)
		windows.SetFocus(w.hwnd)
		if err := w.loop(); err != nil {
			panic(err)
		}
	}()
	return <-cerr
}

// initResources initializes the resources global.
func initResources() error {
	windows.SetProcessDPIAware()
	hInst, err := windows.GetModuleHandle()
	if err != nil {
		return err
	}
	resources.handle = hInst
	curs, err := windows.LoadCursor(windows.IDC_ARROW)
	if err != nil {
		return err
	}
	resources.cursor = curs
	wcls := windows.WndClassEx{
		CbSize:        uint32(unsafe.Sizeof(windows.WndClassEx{})),
		Style:         windows.CS_HREDRAW | windows.CS_VREDRAW | windows.CS_OWNDC,
		LpfnWndProc:   syscall.NewCallback(windowProc),
		HInstance:     hInst,
		HCursor:       curs,
		LpszClassName: syscall.StringToUTF16Ptr("GioWindow"),
	}
	cls, err := windows.RegisterClassEx(&wcls)
	if err != nil {
		return err
	}
	resources.class = cls
	return nil
}

func getWindowConstraints(cfg unit.Metric, opts *Options, d winDeltas) winConstraints {
	var minmax winConstraints
	minmax.minWidth = int32(cfg.Px(opts.MinWidth))
	minmax.minHeight = int32(cfg.Px(opts.MinHeight))
	minmax.maxWidth = int32(cfg.Px(opts.MaxWidth))
	minmax.maxHeight = int32(cfg.Px(opts.MaxHeight))
	return minmax
}

func createNativeWindow(opts *Options) (*window, error) {
	var resErr error
	resources.once.Do(func() {
		resErr = initResources()
	})
	if resErr != nil {
		return nil, resErr
	}
	cfg := configForDC()
	wr := windows.Rect{
		Right:  int32(cfg.Px(opts.Width)),
		Bottom: int32(cfg.Px(opts.Height)),
	}
	dwStyle := uint32(windows.WS_OVERLAPPEDWINDOW)
	dwExStyle := uint32(windows.WS_EX_APPWINDOW | windows.WS_EX_WINDOWEDGE)
	deltas := winDeltas{
		width:  wr.Right,
		height: wr.Bottom,
	}
	windows.AdjustWindowRectEx(&wr, dwStyle, 0, dwExStyle)
	deltas.width = wr.Right - wr.Left - deltas.width
	deltas.height = wr.Bottom - wr.Top - deltas.height

	hwnd, err := windows.CreateWindowEx(dwExStyle,
		resources.class,
		opts.Title,
		dwStyle|windows.WS_CLIPSIBLINGS|windows.WS_CLIPCHILDREN,
		windows.CW_USEDEFAULT, windows.CW_USEDEFAULT,
		wr.Right-wr.Left,
		wr.Bottom-wr.Top,
		0,
		0,
		resources.handle,
		0)
	if err != nil {
		return nil, err
	}
	w := &window{
		hwnd:   hwnd,
		minmax: getWindowConstraints(cfg, opts, deltas),
		deltas: deltas,
		opts:   opts,
	}
	w.hdc, err = windows.GetDC(hwnd)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func windowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	win, exists := winMap.Load(hwnd)
	if !exists {
		return windows.DefWindowProc(hwnd, msg, wParam, lParam)
	}

	w := win.(*window)

	switch msg {
	case windows.WM_UNICHAR:
		if wParam == windows.UNICODE_NOCHAR {
			// Tell the system that we accept WM_UNICHAR messages.
			return 1
		}
		fallthrough
	case windows.WM_CHAR:
		if r := rune(wParam); unicode.IsPrint(r) {
			w.w.Event(key.EditEvent{Text: string(r)})
		}
		// The message is processed.
		return 1
	case windows.WM_DPICHANGED:
		// Let Windows know we're prepared for runtime DPI changes.
		return 1
	case windows.WM_ERASEBKGND:
		// Avoid flickering between GPU content and background color.
		return 1
	case windows.WM_KEYDOWN, windows.WM_SYSKEYDOWN:
		if n, ok := convertKeyCode(wParam); ok {
			w.w.Event(key.Event{Name: n, Modifiers: getModifiers()})
		}
	case windows.WM_LBUTTONDOWN:
		w.pointerButton(pointer.ButtonLeft, true, lParam, getModifiers())
	case windows.WM_LBUTTONUP:
		w.pointerButton(pointer.ButtonLeft, false, lParam, getModifiers())
	case windows.WM_RBUTTONDOWN:
		w.pointerButton(pointer.ButtonRight, true, lParam, getModifiers())
	case windows.WM_RBUTTONUP:
		w.pointerButton(pointer.ButtonRight, false, lParam, getModifiers())
	case windows.WM_MBUTTONDOWN:
		w.pointerButton(pointer.ButtonMiddle, true, lParam, getModifiers())
	case windows.WM_MBUTTONUP:
		w.pointerButton(pointer.ButtonMiddle, false, lParam, getModifiers())
	case windows.WM_CANCELMODE:
		w.w.Event(pointer.Event{
			Type: pointer.Cancel,
		})
	case windows.WM_SETFOCUS:
		w.w.Event(key.FocusEvent{Focus: true})
	case windows.WM_KILLFOCUS:
		w.w.Event(key.FocusEvent{Focus: false})
	case windows.WM_MOUSEMOVE:
		x, y := coordsFromlParam(lParam)
		p := f32.Point{X: float32(x), Y: float32(y)}
		w.w.Event(pointer.Event{
			Type:     pointer.Move,
			Source:   pointer.Mouse,
			Position: p,
			Buttons:  w.pointerBtns,
			Time:     windows.GetMessageTime(),
		})
	case windows.WM_MOUSEWHEEL:
		w.scrollEvent(wParam, lParam)
	case windows.WM_DESTROY:
		w.dead = true
	case windows.WM_PAINT:
		w.draw(true)
	case windows.WM_SIZE:
		switch wParam {
		case windows.SIZE_MINIMIZED:
			w.setStage(system.StagePaused)
		case windows.SIZE_MAXIMIZED, windows.SIZE_RESTORED:
			w.setStage(system.StageRunning)
		}
	case windows.WM_GETMINMAXINFO:
		mm := (*windows.MinMaxInfo)(unsafe.Pointer(uintptr(lParam)))
		if w.minmax.minWidth > 0 || w.minmax.minHeight > 0 {
			mm.PtMinTrackSize = windows.Point{
				w.minmax.minWidth+w.deltas.width,
				w.minmax.minHeight+w.deltas.height,
			}
		}
		if w.minmax.maxWidth > 0 || w.minmax.maxHeight > 0 {
			mm.PtMaxTrackSize = windows.Point{
				w.minmax.maxWidth+w.deltas.width,
				w.minmax.maxHeight+w.deltas.height,
			}
		}
	}

	return windows.DefWindowProc(hwnd, msg, wParam, lParam)
}

func getModifiers() key.Modifiers {
	var kmods key.Modifiers
	if windows.GetKeyState(windows.VK_LWIN)&0x1000 != 0 || windows.GetKeyState(windows.VK_RWIN)&0x1000 != 0 {
		kmods |= key.ModSuper
	}
	if windows.GetKeyState(windows.VK_MENU)&0x1000 != 0 {
		kmods |= key.ModAlt
	}
	if windows.GetKeyState(windows.VK_CONTROL)&0x1000 != 0 {
		kmods |= key.ModCtrl
	}
	if windows.GetKeyState(windows.VK_SHIFT)&0x1000 != 0 {
		kmods |= key.ModShift
	}
	return kmods
}

func (w *window) pointerButton(btn pointer.Buttons, press bool, lParam uintptr, kmods key.Modifiers) {
	var typ pointer.Type
	if press {
		typ = pointer.Press
		if w.pointerBtns == 0 {
			windows.SetCapture(w.hwnd)
		}
		w.pointerBtns |= btn
	} else {
		typ = pointer.Release
		w.pointerBtns &^= btn
		if w.pointerBtns == 0 {
			windows.ReleaseCapture()
		}
	}
	x, y := coordsFromlParam(lParam)
	p := f32.Point{X: float32(x), Y: float32(y)}
	w.w.Event(pointer.Event{
		Type:      typ,
		Source:    pointer.Mouse,
		Position:  p,
		Buttons:   w.pointerBtns,
		Time:      windows.GetMessageTime(),
		Modifiers: kmods,
	})
}

func coordsFromlParam(lParam uintptr) (int, int) {
	x := int(int16(lParam & 0xffff))
	y := int(int16((lParam >> 16) & 0xffff))
	return x, y
}

func (w *window) scrollEvent(wParam, lParam uintptr) {
	x, y := coordsFromlParam(lParam)
	// The WM_MOUSEWHEEL coordinates are in screen coordinates, in contrast
	// to other mouse events.
	np := windows.Point{X: int32(x), Y: int32(y)}
	windows.ScreenToClient(w.hwnd, &np)
	p := f32.Point{X: float32(np.X), Y: float32(np.Y)}
	dist := float32(int16(wParam >> 16))
	w.w.Event(pointer.Event{
		Type:     pointer.Scroll,
		Source:   pointer.Mouse,
		Position: p,
		Buttons:  w.pointerBtns,
		Scroll:   f32.Point{Y: -dist},
		Time:     windows.GetMessageTime(),
	})
}

// Adapted from https://blogs.msdn.microsoft.com/oldnewthing/20060126-00/?p=32513/
func (w *window) loop() error {
	msg := new(windows.Msg)
	for !w.dead {
		w.mu.Lock()
		anim := w.animating
		w.mu.Unlock()
		if anim && !windows.PeekMessage(msg, w.hwnd, 0, 0, windows.PM_NOREMOVE) {
			w.draw(false)
			continue
		}
		windows.GetMessage(msg, w.hwnd, 0, 0)
		if msg.Message == windows.WM_QUIT {
			windows.PostQuitMessage(msg.WParam)
			break
		}
		windows.TranslateMessage(msg)
		windows.DispatchMessage(msg)
	}
	return nil
}

func (w *window) SetAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		w.postRedraw()
	}
}

func (w *window) postRedraw() {
	if err := windows.PostMessage(w.hwnd, _WM_REDRAW, 0, 0); err != nil {
		panic(err)
	}
}

func (w *window) setStage(s system.Stage) {
	w.stage = s
	w.w.Event(system.StageEvent{Stage: s})
}

func (w *window) draw(sync bool) {
	var r windows.Rect
	windows.GetClientRect(w.hwnd, &r)
	w.width = int(r.Right - r.Left)
	w.height = int(r.Bottom - r.Top)
	if w.width == 0 || w.height == 0 {
		return
	}
	cfg := configForDC()
	w.minmax = getWindowConstraints(cfg, w.opts, w.deltas)
	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: w.width,
				Y: w.height,
			},
			Metric: cfg,
		},
		Sync: sync,
	})
}

func (w *window) destroy() {
	if w.hdc != 0 {
		windows.ReleaseDC(w.hdc)
		w.hdc = 0
	}
	if w.hwnd != 0 {
		windows.DestroyWindow(w.hwnd)
		w.hwnd = 0
	}
}

func (w *window) NewContext() (Context, error) {
	sort.Slice(backends, func(i, j int) bool {
		return backends[i].priority < backends[j].priority
	})
	var errs []string
	for _, b := range backends {
		ctx, err := b.initializer(w)
		if err == nil {
			return ctx, nil
		}
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("NewContext: failed to create a GPU device, tried: %s", strings.Join(errs, ", "))
	}
	return nil, errors.New("NewContext: no available backends")
}

func (w *window) ReadClipboard() {
	w.readClipboard()
}

func (w *window) readClipboard() error {
	if err := windows.OpenClipboard(w.hwnd); err != nil {
		return err
	}
	defer windows.CloseClipboard()
	mem, err := windows.GetClipboardData(windows.CF_UNICODETEXT)
	if err != nil {
		return err
	}
	ptr, err := windows.GlobalLock(mem)
	if err != nil {
		return err
	}
	defer windows.GlobalUnlock(mem)
	// Look for terminating null character.
	n := 0
	for {
		ch := *(*uint16)(unsafe.Pointer(ptr + uintptr(n)*2))
		if ch == 0 {
			break
		}
		n++
	}
	var u16 []uint16
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&u16))
	hdr.Data = ptr
	hdr.Cap = n
	hdr.Len = n
	content := string(utf16.Decode(u16))
	go func() {
		w.w.Event(system.ClipboardEvent{Text: content})
	}()
	return nil
}

func (w *window) WriteClipboard(s string) {
	w.writeClipboard(s)
}

func (w *window) writeClipboard(s string) error {
	u16 := utf16.Encode([]rune(s))
	// Data must be null terminated.
	u16 = append(u16, 0)
	if err := windows.OpenClipboard(w.hwnd); err != nil {
		return err
	}
	defer windows.CloseClipboard()
	if err := windows.EmptyClipboard(); err != nil {
		return err
	}
	n := len(u16) * int(unsafe.Sizeof(u16[0]))
	mem, err := windows.GlobalAlloc(n)
	if err != nil {
		return err
	}
	ptr, err := windows.GlobalLock(mem)
	if err != nil {
		windows.GlobalFree(mem)
		return err
	}
	var u16v []uint16
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&u16v))
	hdr.Data = ptr
	hdr.Cap = len(u16)
	hdr.Len = len(u16)
	copy(u16v, u16)
	windows.GlobalUnlock(mem)
	if err := windows.SetClipboardData(windows.CF_UNICODETEXT, mem); err != nil {
		windows.GlobalFree(mem)
		return err
	}
	return nil
}

func (w *window) ShowTextInput(show bool) {}

func (w *window) HDC() syscall.Handle {
	return w.hdc
}

func (w *window) HWND() (syscall.Handle, int, int) {
	return w.hwnd, w.width, w.height
}

func (w *window) Close() {
	windows.PostMessage(w.hwnd, windows.WM_CLOSE, 0, 0)
}

func convertKeyCode(code uintptr) (string, bool) {
	if '0' <= code && code <= '9' || 'A' <= code && code <= 'Z' {
		return string(rune(code)), true
	}
	var r string
	switch code {
	case windows.VK_ESCAPE:
		r = key.NameEscape
	case windows.VK_LEFT:
		r = key.NameLeftArrow
	case windows.VK_RIGHT:
		r = key.NameRightArrow
	case windows.VK_RETURN:
		r = key.NameReturn
	case windows.VK_UP:
		r = key.NameUpArrow
	case windows.VK_DOWN:
		r = key.NameDownArrow
	case windows.VK_HOME:
		r = key.NameHome
	case windows.VK_END:
		r = key.NameEnd
	case windows.VK_BACK:
		r = key.NameDeleteBackward
	case windows.VK_DELETE:
		r = key.NameDeleteForward
	case windows.VK_PRIOR:
		r = key.NamePageUp
	case windows.VK_NEXT:
		r = key.NamePageDown
	case windows.VK_F1:
		r = "F1"
	case windows.VK_F2:
		r = "F2"
	case windows.VK_F3:
		r = "F3"
	case windows.VK_F4:
		r = "F4"
	case windows.VK_F5:
		r = "F5"
	case windows.VK_F6:
		r = "F6"
	case windows.VK_F7:
		r = "F7"
	case windows.VK_F8:
		r = "F8"
	case windows.VK_F9:
		r = "F9"
	case windows.VK_F10:
		r = "F10"
	case windows.VK_F11:
		r = "F11"
	case windows.VK_F12:
		r = "F12"
	case windows.VK_TAB:
		r = key.NameTab
	case windows.VK_SPACE:
		r = "Space"
	case windows.VK_OEM_1:
		r = ";"
	case windows.VK_OEM_PLUS:
		r = "+"
	case windows.VK_OEM_COMMA:
		r = ","
	case windows.VK_OEM_MINUS:
		r = "-"
	case windows.VK_OEM_PERIOD:
		r = "."
	case windows.VK_OEM_2:
		r = "/"
	case windows.VK_OEM_3:
		r = "`"
	case windows.VK_OEM_4:
		r = "["
	case windows.VK_OEM_5, windows.VK_OEM_102:
		r = "\\"
	case windows.VK_OEM_6:
		r = "]"
	case windows.VK_OEM_7:
		r = "'"
	default:
		return "", false
	}
	return r, true
}

func configForDC() unit.Metric {
	dpi := windows.GetSystemDPI()
	const inchPrDp = 1.0 / 96.0
	ppdp := float32(dpi) * inchPrDp
	return unit.Metric{
		PxPerDp: ppdp,
		PxPerSp: ppdp,
	}
}
