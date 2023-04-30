//go:build darwin
// +build darwin

package main

import (
	"syscall"
)

func pRlimit(pid int, limit uintptr, new *syscall.Rlimit, old *syscall.Rlimit) error {
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, old)
	if err != nil {
		return err
	}
	// Modify new.Rlimit based on desired changes
	// ...
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, new)
	if err != nil {
		return err
	}
	return nil
}
