package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const rlimitRTTime = 15

func getPulsePid() (int, error) {
	pulsepidfile := filepath.Join(xdgOrFallback("XDG_RUNTIME_DIR", fmt.Sprintf("/run/user/%d", os.Getuid())), "pulse/pid")
	pidbuf, err := ioutil.ReadFile(pulsepidfile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidbuf)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func getRlimit(pid int) (syscall.Rlimit, error) {
	var res syscall.Rlimit
	err := pRlimit(pid, rlimitRTTime, nil, &res)
	return res, err
}

func setRlimit(pid int, new *syscall.Rlimit) error {
	var junk syscall.Rlimit
	err := pRlimit(pid, rlimitRTTime, new, &junk)
	return err
}

func removeRlimitAsRoot(pid int) {
	self, err := os.Executable()
	if err != nil {
		log.Printf("Couldn't find path to own binary, trying PATH\n")
		self = "noisetorch" //try PATH and hope for the best
	}

	cmd := exec.Command("pkexec", self, "-removerlimit", strconv.Itoa(pid))
	log.Printf("Calling: %s\n", cmd.String())
	err = cmd.Run()
	if err != nil {
		log.Printf("Couldn't remove rlimit as root: %v\n", err)
	}
}

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
