// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux
// +build 386 ppc64 ppc64le s390x

package x11driver

import (
	"fmt"
	"syscall"
	"unsafe"
)

// These constants are from /usr/include/linux/ipc.h
const (
	ipcPrivate = 0
	ipcRmID    = 0

	shmAt  = 21
	shmDt  = 22
	shmGet = 23
	shmCtl = 24
)

func shmOpen(size int) (shmid uintptr, addr unsafe.Pointer, err error) {
	shmid, _, errno0 := syscall.RawSyscall6(syscall.SYS_IPC, shmGet, ipcPrivate, uintptr(size), 0600, 0, 0)
	if errno0 != 0 {
		return 0, unsafe.Pointer(uintptr(0)), fmt.Errorf("shmget: %v", errno0)
	}
	_, _, errno1 := syscall.RawSyscall6(syscall.SYS_IPC, shmAt, shmid, 0, uintptr(unsafe.Pointer(&addr)), 0, 0)
	_, _, errno2 := syscall.RawSyscall6(syscall.SYS_IPC, shmCtl, shmid, ipcRmID, 0, 0, 0)
	if errno1 != 0 {
		return 0, unsafe.Pointer(uintptr(0)), fmt.Errorf("shmat: %v", errno1)
	}
	if errno2 != 0 {
		return 0, unsafe.Pointer(uintptr(0)), fmt.Errorf("shmctl: %v", errno2)
	}
	return shmid, addr, nil
}

func shmClose(p unsafe.Pointer) error {
	_, _, errno := syscall.RawSyscall6(syscall.SYS_IPC, shmDt, 0, 0, 0, uintptr(p), 0)
	if errno != 0 {
		return fmt.Errorf("shmdt: %v", errno)
	}
	return nil
}
