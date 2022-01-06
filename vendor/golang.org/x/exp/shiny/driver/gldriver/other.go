// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!darwin || ios || !cgo) && (!linux || android || !cgo) && (!openbsd || !cgo) && !windows
// +build !darwin ios !cgo
// +build !linux android !cgo
// +build !openbsd !cgo
// +build !windows

package gldriver

import (
	"fmt"
	"runtime"

	"golang.org/x/exp/shiny/screen"
)

const useLifecycler = true
const handleSizeEventsAtChannelReceive = true

var errUnsupported = fmt.Errorf("gldriver: unsupported GOOS/GOARCH %s/%s or cgo not enabled", runtime.GOOS, runtime.GOARCH)

func newWindow(opts *screen.NewWindowOptions) (uintptr, error) { return 0, errUnsupported }

func initWindow(id *windowImpl) {}
func showWindow(id *windowImpl) {}
func closeWindow(id uintptr)    {}
func drawLoop(w *windowImpl)    {}

func surfaceCreate() error             { return errUnsupported }
func main(f func(screen.Screen)) error { return errUnsupported }
