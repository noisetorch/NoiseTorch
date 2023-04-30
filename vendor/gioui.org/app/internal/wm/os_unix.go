// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android freebsd openbsd

package wm

import (
	"errors"
)

func Main() {
	select {}
}

type windowDriver func(Callbacks, *Options) error

// Instead of creating files with build tags for each combination of wayland +/- x11
// let each driver initialize these variables with their own version of createWindow.
var wlDriver, x11Driver windowDriver

func NewWindow(window Callbacks, opts *Options) error {
	var errFirst error
	for _, d := range []windowDriver{x11Driver, wlDriver} {
		if d == nil {
			continue
		}
		err := d(window, opts)
		if err == nil {
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
