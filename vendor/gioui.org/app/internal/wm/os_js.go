// SPDX-License-Identifier: Unlicense OR MIT

package wm

import (
	"image"
	"strings"
	"sync"
	"syscall/js"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type window struct {
	window                js.Value
	document              js.Value
	clipboard             js.Value
	cnv                   js.Value
	tarea                 js.Value
	w                     Callbacks
	redraw                js.Func
	clipboardCallback     js.Func
	requestAnimationFrame js.Value
	browserHistory        js.Value
	visualViewport        js.Value
	cleanfuncs            []func()
	touches               []js.Value
	composing             bool
	requestFocus          bool

	chanAnimation chan struct{}
	chanRedraw    chan struct{}

	mu        sync.Mutex
	size      f32.Point
	inset     f32.Point
	scale     float32
	animating bool
	// animRequested tracks whether a requestAnimationFrame callback
	// is pending.
	animRequested bool
}

func NewWindow(win Callbacks, opts *Options) error {
	doc := js.Global().Get("document")
	cont := getContainer(doc)
	cnv := createCanvas(doc)
	cont.Call("appendChild", cnv)
	tarea := createTextArea(doc)
	cont.Call("appendChild", tarea)
	w := &window{
		cnv:       cnv,
		document:  doc,
		tarea:     tarea,
		window:    js.Global().Get("window"),
		clipboard: js.Global().Get("navigator").Get("clipboard"),
	}
	w.requestAnimationFrame = w.window.Get("requestAnimationFrame")
	w.browserHistory = w.window.Get("history")
	w.visualViewport = w.window.Get("visualViewport")
	if w.visualViewport.IsUndefined() {
		w.visualViewport = w.window
	}
	w.chanAnimation = make(chan struct{}, 1)
	w.chanRedraw = make(chan struct{}, 1)
	w.redraw = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		w.chanAnimation <- struct{}{}
		return nil
	})
	w.clipboardCallback = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		content := args[0].String()
		win.Event(clipboard.Event{Text: content})
		return nil
	})
	w.addEventListeners()
	w.addHistory()
	w.Option(opts)
	w.w = win

	go func() {
		defer w.cleanup()
		w.w.SetDriver(w)
		w.blur()
		w.w.Event(system.StageEvent{Stage: system.StageRunning})
		w.resize()
		w.draw(true)
		for {
			select {
			case <-w.chanAnimation:
				w.animCallback()
			case <-w.chanRedraw:
				w.draw(true)
			}
		}
	}()
	return nil
}

func getContainer(doc js.Value) js.Value {
	cont := doc.Call("getElementById", "giowindow")
	if !cont.IsNull() {
		return cont
	}
	cont = doc.Call("createElement", "DIV")
	doc.Get("body").Call("appendChild", cont)
	return cont
}

func createTextArea(doc js.Value) js.Value {
	tarea := doc.Call("createElement", "input")
	style := tarea.Get("style")
	style.Set("width", "1px")
	style.Set("height", "1px")
	style.Set("opacity", "0")
	style.Set("border", "0")
	style.Set("padding", "0")
	tarea.Set("autocomplete", "off")
	tarea.Set("autocorrect", "off")
	tarea.Set("autocapitalize", "off")
	tarea.Set("spellcheck", false)
	return tarea
}

func createCanvas(doc js.Value) js.Value {
	cnv := doc.Call("createElement", "canvas")
	style := cnv.Get("style")
	style.Set("position", "fixed")
	style.Set("width", "100%")
	style.Set("height", "100%")
	return cnv
}

func (w *window) cleanup() {
	// Cleanup in the opposite order of
	// construction.
	for i := len(w.cleanfuncs) - 1; i >= 0; i-- {
		w.cleanfuncs[i]()
	}
	w.cleanfuncs = nil
}

func (w *window) addEventListeners() {
	w.addEventListener(w.visualViewport, "resize", func(this js.Value, args []js.Value) interface{} {
		w.resize()
		w.chanRedraw <- struct{}{}
		return nil
	})
	w.addEventListener(w.window, "contextmenu", func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		return nil
	})
	w.addEventListener(w.window, "popstate", func(this js.Value, args []js.Value) interface{} {
		ev := &system.CommandEvent{Type: system.CommandBack}
		w.w.Event(ev)
		if ev.Cancel {
			return w.browserHistory.Call("forward")
		}

		return w.browserHistory.Call("back")
	})
	w.addEventListener(w.document, "visibilitychange", func(this js.Value, args []js.Value) interface{} {
		ev := system.StageEvent{}
		switch w.document.Get("visibilityState").String() {
		case "hidden", "prerender", "unloaded":
			ev.Stage = system.StagePaused
		default:
			ev.Stage = system.StageRunning
		}
		w.w.Event(ev)
		return nil
	})
	w.addEventListener(w.cnv, "mousemove", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Move, 0, 0, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "mousedown", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Press, 0, 0, args[0])
		if w.requestFocus {
			w.focus()
			w.requestFocus = false
		}
		return nil
	})
	w.addEventListener(w.cnv, "mouseup", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Release, 0, 0, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "wheel", func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		dx, dy := e.Get("deltaX").Float(), e.Get("deltaY").Float()
		mode := e.Get("deltaMode").Int()
		switch mode {
		case 0x01: // DOM_DELTA_LINE
			dx *= 10
			dy *= 10
		case 0x02: // DOM_DELTA_PAGE
			dx *= 120
			dy *= 120
		}
		w.pointerEvent(pointer.Scroll, float32(dx), float32(dy), e)
		return nil
	})
	w.addEventListener(w.cnv, "touchstart", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Press, args[0])
		if w.requestFocus {
			w.focus() // iOS can only focus inside a Touch event.
			w.requestFocus = false
		}
		return nil
	})
	w.addEventListener(w.cnv, "touchend", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Release, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "touchmove", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Move, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "touchcancel", func(this js.Value, args []js.Value) interface{} {
		// Cancel all touches even if only one touch was cancelled.
		for i := range w.touches {
			w.touches[i] = js.Null()
		}
		w.touches = w.touches[:0]
		w.w.Event(pointer.Event{
			Type:   pointer.Cancel,
			Source: pointer.Touch,
		})
		return nil
	})
	w.addEventListener(w.tarea, "focus", func(this js.Value, args []js.Value) interface{} {
		w.w.Event(key.FocusEvent{Focus: true})
		return nil
	})
	w.addEventListener(w.tarea, "blur", func(this js.Value, args []js.Value) interface{} {
		w.w.Event(key.FocusEvent{Focus: false})
		w.blur()
		return nil
	})
	w.addEventListener(w.tarea, "keydown", func(this js.Value, args []js.Value) interface{} {
		w.keyEvent(args[0], key.Press)
		return nil
	})
	w.addEventListener(w.tarea, "keyup", func(this js.Value, args []js.Value) interface{} {
		w.keyEvent(args[0], key.Release)
		return nil
	})
	w.addEventListener(w.tarea, "compositionstart", func(this js.Value, args []js.Value) interface{} {
		w.composing = true
		return nil
	})
	w.addEventListener(w.tarea, "compositionend", func(this js.Value, args []js.Value) interface{} {
		w.composing = false
		w.flushInput()
		return nil
	})
	w.addEventListener(w.tarea, "input", func(this js.Value, args []js.Value) interface{} {
		if w.composing {
			return nil
		}
		w.flushInput()
		return nil
	})
	w.addEventListener(w.tarea, "paste", func(this js.Value, args []js.Value) interface{} {
		if w.clipboard.IsUndefined() {
			return nil
		}
		// Prevents duplicated-paste, since "paste" is already handled through Clipboard API.
		args[0].Call("preventDefault")
		return nil
	})
}

func (w *window) addHistory() {
	w.browserHistory.Call("pushState", nil, nil, w.window.Get("location").Get("href"))
}

func (w *window) flushInput() {
	val := w.tarea.Get("value").String()
	w.tarea.Set("value", "")
	w.w.Event(key.EditEvent{Text: string(val)})
}

func (w *window) blur() {
	w.tarea.Call("blur")
	w.requestFocus = false
}

func (w *window) focus() {
	w.tarea.Call("focus")
	w.requestFocus = true
}

func (w *window) keyEvent(e js.Value, ks key.State) {
	k := e.Get("key").String()
	if n, ok := translateKey(k); ok {
		cmd := key.Event{
			Name:      n,
			Modifiers: modifiersFor(e),
			State:     ks,
		}
		w.w.Event(cmd)
	}
}

// modifiersFor returns the modifier set for a DOM MouseEvent or
// KeyEvent.
func modifiersFor(e js.Value) key.Modifiers {
	var mods key.Modifiers
	if e.Get("getModifierState").IsUndefined() {
		// Some browsers doesn't support getModifierState.
		return mods
	}
	if e.Call("getModifierState", "Alt").Bool() {
		mods |= key.ModAlt
	}
	if e.Call("getModifierState", "Control").Bool() {
		mods |= key.ModCtrl
	}
	if e.Call("getModifierState", "Shift").Bool() {
		mods |= key.ModShift
	}
	return mods
}

func (w *window) touchEvent(typ pointer.Type, e js.Value) {
	e.Call("preventDefault")
	t := time.Duration(e.Get("timeStamp").Int()) * time.Millisecond
	changedTouches := e.Get("changedTouches")
	n := changedTouches.Length()
	rect := w.cnv.Call("getBoundingClientRect")
	w.mu.Lock()
	scale := w.scale
	w.mu.Unlock()
	var mods key.Modifiers
	if e.Get("shiftKey").Bool() {
		mods |= key.ModShift
	}
	if e.Get("altKey").Bool() {
		mods |= key.ModAlt
	}
	if e.Get("ctrlKey").Bool() {
		mods |= key.ModCtrl
	}
	for i := 0; i < n; i++ {
		touch := changedTouches.Index(i)
		pid := w.touchIDFor(touch)
		x, y := touch.Get("clientX").Float(), touch.Get("clientY").Float()
		x -= rect.Get("left").Float()
		y -= rect.Get("top").Float()
		pos := f32.Point{
			X: float32(x) * scale,
			Y: float32(y) * scale,
		}
		w.w.Event(pointer.Event{
			Type:      typ,
			Source:    pointer.Touch,
			Position:  pos,
			PointerID: pid,
			Time:      t,
			Modifiers: mods,
		})
	}
}

func (w *window) touchIDFor(touch js.Value) pointer.ID {
	id := touch.Get("identifier")
	for i, id2 := range w.touches {
		if id2.Equal(id) {
			return pointer.ID(i)
		}
	}
	pid := pointer.ID(len(w.touches))
	w.touches = append(w.touches, id)
	return pid
}

func (w *window) pointerEvent(typ pointer.Type, dx, dy float32, e js.Value) {
	e.Call("preventDefault")
	x, y := e.Get("clientX").Float(), e.Get("clientY").Float()
	rect := w.cnv.Call("getBoundingClientRect")
	x -= rect.Get("left").Float()
	y -= rect.Get("top").Float()
	w.mu.Lock()
	scale := w.scale
	w.mu.Unlock()
	pos := f32.Point{
		X: float32(x) * scale,
		Y: float32(y) * scale,
	}
	scroll := f32.Point{
		X: dx * scale,
		Y: dy * scale,
	}
	t := time.Duration(e.Get("timeStamp").Int()) * time.Millisecond
	jbtns := e.Get("buttons").Int()
	var btns pointer.Buttons
	if jbtns&1 != 0 {
		btns |= pointer.ButtonPrimary
	}
	if jbtns&2 != 0 {
		btns |= pointer.ButtonSecondary
	}
	if jbtns&4 != 0 {
		btns |= pointer.ButtonTertiary
	}
	w.w.Event(pointer.Event{
		Type:      typ,
		Source:    pointer.Mouse,
		Buttons:   btns,
		Position:  pos,
		Scroll:    scroll,
		Time:      t,
		Modifiers: modifiersFor(e),
	})
}

func (w *window) addEventListener(this js.Value, event string, f func(this js.Value, args []js.Value) interface{}) {
	jsf := w.funcOf(f)
	this.Call("addEventListener", event, jsf)
	w.cleanfuncs = append(w.cleanfuncs, func() {
		this.Call("removeEventListener", event, jsf)
	})
}

// funcOf is like js.FuncOf but adds the js.Func to a list of
// functions to be released during cleanup.
func (w *window) funcOf(f func(this js.Value, args []js.Value) interface{}) js.Func {
	jsf := js.FuncOf(f)
	w.cleanfuncs = append(w.cleanfuncs, jsf.Release)
	return jsf
}

func (w *window) animCallback() {
	w.mu.Lock()
	anim := w.animating
	w.animRequested = anim
	if anim {
		w.requestAnimationFrame.Invoke(w.redraw)
	}
	w.mu.Unlock()
	if anim {
		w.draw(false)
	}
}

func (w *window) SetAnimating(anim bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.animating = anim
	if anim && !w.animRequested {
		w.animRequested = true
		w.requestAnimationFrame.Invoke(w.redraw)
	}
}

func (w *window) ReadClipboard() {
	if w.clipboard.IsUndefined() {
		return
	}
	if w.clipboard.Get("readText").IsUndefined() {
		return
	}
	w.clipboard.Call("readText", w.clipboard).Call("then", w.clipboardCallback)
}

func (w *window) WriteClipboard(s string) {
	if w.clipboard.IsUndefined() {
		return
	}
	if w.clipboard.Get("writeText").IsUndefined() {
		return
	}
	w.clipboard.Call("writeText", s)
}

func (w *window) Option(opts *Options) {
	if o := opts.WindowMode; o != nil {
		w.windowMode(*o)
	}
}

func (w *window) SetCursor(name pointer.CursorName) {
	style := w.cnv.Get("style")
	style.Set("cursor", string(name))
}

func (w *window) ShowTextInput(show bool) {
	// Run in a goroutine to avoid a deadlock if the
	// focus change result in an event.
	go func() {
		if show {
			w.focus()
		} else {
			w.blur()
		}
	}()
}

// Close the window. Not implemented for js.
func (w *window) Close() {}

func (w *window) resize() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.scale = float32(w.window.Get("devicePixelRatio").Float())

	rect := w.cnv.Call("getBoundingClientRect")
	w.size.X = float32(rect.Get("width").Float()) * w.scale
	w.size.Y = float32(rect.Get("height").Float()) * w.scale

	if vx, vy := w.visualViewport.Get("width"), w.visualViewport.Get("height"); !vx.IsUndefined() && !vy.IsUndefined() {
		w.inset.X = w.size.X - float32(vx.Float())*w.scale
		w.inset.Y = w.size.Y - float32(vy.Float())*w.scale
	}

	if w.size.X == 0 || w.size.Y == 0 {
		return
	}

	w.cnv.Set("width", int(w.size.X+.5))
	w.cnv.Set("height", int(w.size.Y+.5))
}

func (w *window) draw(sync bool) {
	width, height, insets, metric := w.config()
	if metric == (unit.Metric{}) || width == 0 || height == 0 {
		return
	}

	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: width,
				Y: height,
			},
			Insets: insets,
			Metric: metric,
		},
		Sync: sync,
	})
}

func (w *window) config() (int, int, system.Insets, unit.Metric) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return int(w.size.X + .5), int(w.size.Y + .5), system.Insets{
			Bottom: unit.Px(w.inset.Y),
			Right:  unit.Px(w.inset.X),
		}, unit.Metric{
			PxPerDp: w.scale,
			PxPerSp: w.scale,
		}
}

func (w *window) windowMode(mode WindowMode) {
	switch mode {
	case Windowed:
		if fs := w.document.Get("fullscreenElement"); !fs.Truthy() {
			return // Browser is already Windowed.
		}
		if !w.document.Get("exitFullscreen").Truthy() {
			return // Browser doesn't support such feature.
		}
		w.document.Call("exitFullscreen")
	case Fullscreen:
		elem := w.document.Get("documentElement")
		if !elem.Get("requestFullscreen").Truthy() {
			return // Browser doesn't support such feature.
		}
		elem.Call("requestFullscreen")
	}
}

func Main() {
	select {}
}

func translateKey(k string) (string, bool) {
	var n string
	switch k {
	case "ArrowUp":
		n = key.NameUpArrow
	case "ArrowDown":
		n = key.NameDownArrow
	case "ArrowLeft":
		n = key.NameLeftArrow
	case "ArrowRight":
		n = key.NameRightArrow
	case "Escape":
		n = key.NameEscape
	case "Enter":
		n = key.NameReturn
	case "Backspace":
		n = key.NameDeleteBackward
	case "Delete":
		n = key.NameDeleteForward
	case "Home":
		n = key.NameHome
	case "End":
		n = key.NameEnd
	case "PageUp":
		n = key.NamePageUp
	case "PageDown":
		n = key.NamePageDown
	case "Tab":
		n = key.NameTab
	case " ":
		n = key.NameSpace
	case "F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12":
		n = k
	default:
		r, s := utf8.DecodeRuneInString(k)
		// If there is exactly one printable character, return that.
		if s == len(k) && unicode.IsPrint(r) {
			return strings.ToUpper(k), true
		}
		return "", false
	}
	return n, true
}
