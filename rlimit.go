package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lawl/pulseaudio"
)

const rlimitRTTime = 15

func getPulsePid() (int, error) {
	pulsepidfile, err := pulseaudio.RuntimePath("pid")
	if err != nil {
		return 0, err
	}
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

func removeRlimitAsRoot(pid int, usesudo bool) {
	self, err := os.Executable()
	if err != nil {
		log.Printf("Couldn't find path to own binary, trying PATH\n")
		self = "noisetorch" //try PATH and hope for the best
	}

	var sudocommand string
	if usesudo { // use sudo for CLI
		sudocommand = "sudo"
	} else { // use pkexec for gui
		sudocommand = "pkexec"
	}
	cmd := exec.Command(sudocommand, self, "-removerlimit", strconv.Itoa(pid))
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
