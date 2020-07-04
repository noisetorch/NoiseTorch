// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"image"
	"strings"
	"sync"
	"syscall/js"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type window struct {
	window                js.Value
	clipboard             js.Value
	cnv                   js.Value
	tarea                 js.Value
	w                     Callbacks
	redraw                js.Func
	clipboardCallback     js.Func
	requestAnimationFrame js.Value
	cleanfuncs            []func()
	touches               []js.Value
	composing             bool

	mu        sync.Mutex
	scale     float32
	animating bool
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
		tarea:     tarea,
		window:    js.Global().Get("window"),
		clipboard: js.Global().Get("navigator").Get("clipboard"),
	}
	w.requestAnimationFrame = w.window.Get("requestAnimationFrame")
	w.redraw = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		w.animCallback()
		return nil
	})
	w.clipboardCallback = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		content := args[0].String()
		win.Event(system.ClipboardEvent{Text: content})
		return nil
	})
	w.addEventListeners()
	w.w = win
	go func() {
		w.w.SetDriver(w)
		w.focus()
		w.w.Event(system.StageEvent{Stage: system.StageRunning})
		w.draw(true)
		select {}
		w.cleanup()
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
	w.addEventListener(w.window, "resize", func(this js.Value, args []js.Value) interface{} {
		w.draw(true)
		return nil
	})
	w.addEventListener(w.cnv, "mousemove", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Move, 0, 0, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "mousedown", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Press, 0, 0, args[0])
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
		return nil
	})
	w.addEventListener(w.tarea, "keydown", func(this js.Value, args []js.Value) interface{} {
		w.keyEvent(args[0])
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
}

func (w *window) flushInput() {
	val := w.tarea.Get("value").String()
	w.tarea.Set("value", "")
	w.w.Event(key.EditEvent{Text: string(val)})
}

func (w *window) blur() {
	w.tarea.Call("blur")
}

func (w *window) focus() {
	w.tarea.Call("focus")
}

func (w *window) keyEvent(e js.Value) {
	k := e.Get("key").String()
	if n, ok := translateKey(k); ok {
		cmd := key.Event{
			Name:      n,
			Modifiers: modifiersFor(e),
		}
		w.w.Event(cmd)
	}
}

// modifiersFor returns the modifier set for a DOM MouseEvent or
// KeyEvent.
func modifiersFor(e js.Value) key.Modifiers {
	var mods key.Modifiers
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
			Modifiers: modifiersFor(e),
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
		btns |= pointer.ButtonLeft
	}
	if jbtns&2 != 0 {
		btns |= pointer.ButtonRight
	}
	if jbtns&4 != 0 {
		btns |= pointer.ButtonMiddle
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
// functions to be released up.
func (w *window) funcOf(f func(this js.Value, args []js.Value) interface{}) js.Func {
	jsf := js.FuncOf(f)
	w.cleanfuncs = append(w.cleanfuncs, jsf.Release)
	return jsf
}

func (w *window) animCallback() {
	w.mu.Lock()
	anim := w.animating
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
	if anim && !w.animating {
		w.requestAnimationFrame.Invoke(w.redraw)
	}
	w.animating = anim
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

func (w *window) draw(sync bool) {
	width, height, scale, cfg := w.config()
	if cfg == (unit.Metric{}) || width == 0 || height == 0 {
		return
	}
	w.mu.Lock()
	w.scale = float32(scale)
	w.mu.Unlock()
	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: width,
				Y: height,
			},
			Metric: cfg,
		},
		Sync: sync,
	})
}

func (w *window) config() (int, int, float32, unit.Metric) {
	rect := w.cnv.Call("getBoundingClientRect")
	width, height := rect.Get("width").Float(), rect.Get("height").Float()
	scale := w.window.Get("devicePixelRatio").Float()
	width *= scale
	height *= scale
	iw, ih := int(width+.5), int(height+.5)
	// Adjust internal size of canvas if necessary.
	if cw, ch := w.cnv.Get("width").Int(), w.cnv.Get("height").Int(); iw != cw || ih != ch {
		w.cnv.Set("width", iw)
		w.cnv.Set("height", ih)
	}
	return iw, ih, float32(scale), unit.Metric{
		PxPerDp: float32(scale),
		PxPerSp: float32(scale),
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
		n = "Space"
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
