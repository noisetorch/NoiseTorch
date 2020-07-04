// SPDX-License-Identifier: Unlicense OR MIT

// +build android

package app

import "C"

import (
	"os"
	"path/filepath"
	"sync"

	"gioui.org/app/internal/window"
)

var (
	dataDirOnce sync.Once
	dataPath    string
)

func dataDir() (string, error) {
	dataDirOnce.Do(func() {
		dataPath = window.GetDataDir()
		// Set XDG_CACHE_HOME to make os.UserCacheDir work.
		if _, exists := os.LookupEnv("XDG_CACHE_HOME"); !exists {
			cachePath := filepath.Join(dataPath, "cache")
			os.Setenv("XDG_CACHE_HOME", cachePath)
		}
		// Set XDG_CONFIG_HOME to make os.UserConfigDir work.
		if _, exists := os.LookupEnv("XDG_CONFIG_HOME"); !exists {
			cfgPath := filepath.Join(dataPath, "config")
			os.Setenv("XDG_CONFIG_HOME", cfgPath)
		}
		// Set HOME to make os.UserHomeDir work.
		if _, exists := os.LookupEnv("HOME"); !exists {
			os.Setenv("HOME", dataPath)
		}
	})
	return dataPath, nil
}
