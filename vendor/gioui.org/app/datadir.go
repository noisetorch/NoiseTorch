// SPDX-License-Identifier: Unlicense OR MIT

// +build !android

package app

import "os"

func dataDir() (string, error) {
	return os.UserConfigDir()
}
