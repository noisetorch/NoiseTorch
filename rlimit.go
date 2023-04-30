// This file is part of the program "NoiseTorch-ng".
// Please see the LICENSE file for copyright information.

package main

import (
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"syscall"

	"github.com/noisetorch/pulseaudio"
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

func removeRlimit(pid int) {
	const MaxUint = ^uint64(0)
	new := syscall.Rlimit{Cur: MaxUint, Max: MaxUint}
	err := setRlimit(pid, &new)
	if err != nil {
		log.Printf("Couldn't set rlimit with caps\n")
	}
}
