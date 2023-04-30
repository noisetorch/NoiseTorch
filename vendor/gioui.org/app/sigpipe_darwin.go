// SPDX-License-Identifier: Unlicense OR MIT

// +build !go1.14

// Work around golang.org/issue/33384, fixed in CL 191785,
// to be released in Go 1.14.

package app

import (
	"os"
	"os/signal"
	"syscall"
)

func init() {
	signal.Notify(make(chan os.Signal), syscall.SIGPIPE)
}
