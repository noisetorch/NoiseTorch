// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"gioui.org/app/internal/window"
)

// JavaVM returns the global JNI JavaVM.
func JavaVM() uintptr {
	return window.JavaVM()
}

// AppContext returns the global Application context as a JNI
// jobject.
func AppContext() uintptr {
	return window.AppContext()
}

// Do invokes the function with a JNI jobject handle to the underlying
// Android View. The function is invoked on the main thread, and the
// handle is invalidated after the function returns.
//
// Note: Do may deadlock if called from the same goroutine that receives from
// Events.
func (w *Window) Do(f func(view uintptr)) {
	type androidDriver interface {
		Do(f func(view uintptr)) bool
	}
	success := make(chan bool)
	for {
		driver := make(chan androidDriver, 1)
		// two-stage process: first wait for a valid driver...
		w.driverDo(func() {
			driver <- w.driver.(androidDriver)
		})
		// .. then run the function on the main thread using the
		// driver. The driver Do method returns false if the
		// view was invalidated while switching to the main thread.
		window.RunOnMain(func() {
			d := <-driver
			success <- d.Do(f)
		})
		if <-success {
			break
		}
	}
}
