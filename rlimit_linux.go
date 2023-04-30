//go:build linux
// +build linux

package main

import (
	"syscall"
	"unsafe"
)

func pRlimit(pid int, limit uintptr, new *syscall.Rlimit, old *syscall.Rlimit) error {
	_, _, errno := syscall.RawSyscall6(syscall.SYS_PRLIMIT64,
		uintptr(pid),
		limit,
		uintptr(unsafe.Pointer(new)),
		uintptr(unsafe.Pointer(old)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
