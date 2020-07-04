// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android freebsd openbsd

package window

import (
	"errors"
)

func Main() {
	select {}
}

// instead of creating files with build tags for each combination of wayland +/- x11
// let each driver initialize these variables with their own version of createWindow.
var wlDriver, x11Driver func(Callbacks, *Options) error

func NewWindow(window Callbacks, opts *Options) error {
	var errFirst, err error
	if wlDriver != nil {
		if err = wlDriver(window, opts); err == nil {
			return nil
		}
		errFirst = err
	}
	if x11Driver != nil {
		if err = x11Driver(window, opts); err == nil {
			return nil
		}
		if errFirst == nil {
			errFirst = err
		}
	}
	if errFirst != nil {
		return errFirst
	}
	return errors.New("app: no window driver available")
}
