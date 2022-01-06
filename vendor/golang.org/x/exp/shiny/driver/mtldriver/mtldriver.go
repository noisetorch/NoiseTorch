// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin
// +build darwin

// Package mtldriver provides a Metal driver for accessing a screen.
//
// At this time, the Metal API is used only to present the final pixels
// to the screen. All rendering is performed on the CPU via the image/draw
// algorithms. Future work is to use mtl.Buffer, mtl.Texture, etc., and
// do more of the rendering work on the GPU.
package mtldriver

import (
	"runtime"
	"unsafe"

	"dmitri.shuralyov.com/gpu/mtl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"golang.org/x/exp/shiny/driver/internal/errscreen"
	"golang.org/x/exp/shiny/driver/mtldriver/internal/appkit"
	"golang.org/x/exp/shiny/driver/mtldriver/internal/coreanim"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

func init() {
	runtime.LockOSThread()
}

// Main is called by the program's main function to run the graphical
// application.
//
// It calls f on the Screen, possibly in a separate goroutine, as some OS-
// specific libraries require being on 'the main thread'. It returns when f
// returns.
func Main(f func(screen.Screen)) {
	if err := main(f); err != nil {
		f(errscreen.Stub(err))
	}
}

func main(f func(screen.Screen)) error {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		return err
	}
	err = glfw.Init()
	if err != nil {
		return err
	}
	defer glfw.Terminate()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	{
		// TODO(dmitshur): Delete this when https://github.com/go-gl/glfw/issues/272 is resolved.
		// Post an empty event from the main thread before it can happen in a non-main thread,
		// to work around https://github.com/glfw/glfw/issues/1649.
		glfw.PostEmptyEvent()
	}
	var (
		done            = make(chan struct{})
		newWindowCh     = make(chan newWindowReq, 1)
		releaseWindowCh = make(chan releaseWindowReq, 1)
	)
	go func() {
		f(&screenImpl{
			newWindowCh: newWindowCh,
		})
		close(done)
		glfw.PostEmptyEvent() // Break main loop out of glfw.WaitEvents so it can receive on done.
	}()
	for {
		select {
		case <-done:
			return nil
		case req := <-newWindowCh:
			w, err := newWindow(device, releaseWindowCh, req.opts)
			req.respCh <- newWindowResp{w, err}
		case req := <-releaseWindowCh:
			req.window.Destroy()
			req.respCh <- struct{}{}
		default:
			glfw.WaitEvents()
		}
	}
}

type newWindowReq struct {
	opts   *screen.NewWindowOptions
	respCh chan newWindowResp
}

type newWindowResp struct {
	w   screen.Window
	err error
}

type releaseWindowReq struct {
	window *glfw.Window
	respCh chan struct{}
}

// newWindow creates a new GLFW window.
// It must be called on the main thread.
func newWindow(device mtl.Device, releaseWindowCh chan releaseWindowReq, opts *screen.NewWindowOptions) (screen.Window, error) {
	width, height := optsSize(opts)
	window, err := glfw.CreateWindow(width, height, opts.GetTitle(), nil, nil)
	if err != nil {
		return nil, err
	}

	ml := coreanim.MakeMetalLayer()
	ml.SetDevice(device)
	ml.SetPixelFormat(mtl.PixelFormatBGRA8UNorm)
	ml.SetMaximumDrawableCount(3)
	ml.SetDisplaySyncEnabled(true)
	cv := appkit.NewWindow(unsafe.Pointer(window.GetCocoaWindow())).ContentView()
	cv.SetLayer(ml)
	cv.SetWantsLayer(true)

	w := &windowImpl{
		device:          device,
		window:          window,
		releaseWindowCh: releaseWindowCh,
		ml:              ml,
		cq:              device.MakeCommandQueue(),
	}

	// Set callbacks.
	framebufferSizeCallback := func(_ *glfw.Window, width, height int) {
		w.Send(size.Event{
			WidthPx:  width,
			HeightPx: height,
			// TODO(dmitshur): ppp,
		})
		w.Send(paint.Event{External: true})
	}
	window.SetFramebufferSizeCallback(framebufferSizeCallback)
	window.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		const scale = 2 // TODO(dmitshur): compute dynamically
		w.Send(mouse.Event{X: float32(x * scale), Y: float32(y * scale)})
	})
	window.SetMouseButtonCallback(func(_ *glfw.Window, b glfw.MouseButton, a glfw.Action, mods glfw.ModifierKey) {
		btn := glfwMouseButton(b)
		if btn == mouse.ButtonNone {
			return
		}
		const scale = 2 // TODO(dmitshur): compute dynamically
		x, y := window.GetCursorPos()
		w.Send(mouse.Event{
			X: float32(x * scale), Y: float32(y * scale),
			Button:    btn,
			Direction: glfwMouseDirection(a),
			// TODO(dmitshur): set Modifiers
		})
	})
	window.SetKeyCallback(func(_ *glfw.Window, k glfw.Key, _ int, a glfw.Action, mods glfw.ModifierKey) {
		code := glfwKeyCode(k)
		if code == key.CodeUnknown {
			return
		}
		w.Send(key.Event{
			Code:      code,
			Direction: glfwKeyDirection(a),
			// TODO(dmitshur): set Modifiers
		})
	})
	// TODO(dmitshur): set CharModsCallback to catch text (runes) that are typed,
	//                 and perhaps try to unify key pressed + character typed into single event
	window.SetCloseCallback(func(*glfw.Window) {
		w.lifecycler.SetDead(true)
		w.lifecycler.SendEvent(w, nil)
	})

	// TODO(dmitshur): more fine-grained tracking of whether window is visible and/or focused
	w.lifecycler.SetDead(false)
	w.lifecycler.SetVisible(true)
	w.lifecycler.SetFocused(true)
	w.lifecycler.SendEvent(w, nil)

	// Send the initial size and paint events.
	width, height = window.GetFramebufferSize()
	framebufferSizeCallback(window, width, height)

	return w, nil
}

func optsSize(opts *screen.NewWindowOptions) (width, height int) {
	width, height = 1024/2, 768/2
	if opts != nil {
		if opts.Width > 0 {
			width = opts.Width
		}
		if opts.Height > 0 {
			height = opts.Height
		}
	}
	return width, height
}

func glfwMouseButton(button glfw.MouseButton) mouse.Button {
	switch button {
	case glfw.MouseButtonLeft:
		return mouse.ButtonLeft
	case glfw.MouseButtonRight:
		return mouse.ButtonRight
	case glfw.MouseButtonMiddle:
		return mouse.ButtonMiddle
	default:
		return mouse.ButtonNone
	}
}

func glfwMouseDirection(action glfw.Action) mouse.Direction {
	switch action {
	case glfw.Press:
		return mouse.DirPress
	case glfw.Release:
		return mouse.DirRelease
	default:
		panic("unreachable")
	}
}

func glfwKeyCode(k glfw.Key) key.Code {
	// TODO(dmitshur): support more keys
	switch k {
	case glfw.KeyEnter:
		return key.CodeReturnEnter
	case glfw.KeyEscape:
		return key.CodeEscape
	default:
		return key.CodeUnknown
	}
}

func glfwKeyDirection(action glfw.Action) key.Direction {
	switch action {
	case glfw.Press:
		return key.DirPress
	case glfw.Release:
		return key.DirRelease
	case glfw.Repeat:
		return key.DirNone
	default:
		panic("unreachable")
	}
}
