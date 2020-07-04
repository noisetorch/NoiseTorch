// Copyright 2013 @atotto. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
)

var (
	pasteCmdArgs = "pbpaste"
	copyCmdArgs  = "pbcopy"
)

func getPasteCommand() *exec.Cmd {
	cmd := exec.Command(pasteCmdArgs)
	cmd.Env = []string{"LANG=en_US.UTF-8"}
	return cmd
}

func getCopyCommand() *exec.Cmd {
	return exec.Command(copyCmdArgs)
}

func readAll() (string, error) {
	pasteCmd := getPasteCommand()
	out, err := pasteCmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func writeAll(text string) error {
	copyCmd := getCopyCommand()
	in, err := copyCmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := copyCmd.Start(); err != nil {
		return err
	}
	if _, err := in.Write([]byte(text)); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	return copyCmd.Wait()
}

func Start() {
}

func Get() string {
	str, err := readAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	return str
}

func GetPrimary() string {
	return ""
}

func Set(text string) {
	err := writeAll(text)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
