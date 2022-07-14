// This file is part of the program "NoiseTorch-ng".
// Please see the LICENSE file for copyright information.

package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/syndtr/gocapability/capability"
)

func getCurrentCaps() *capability.Capabilities {
	caps, err := capability.NewPid2(0)
	if err != nil {
		log.Fatalf("Could not get self caps: %+v\n", err)
	}

	err = caps.Load()
	if err != nil {
		log.Fatalf("Could not load self caps: %+v\n", err)
	}

	return &caps
}

func getSelfFileCaps() *capability.Capabilities {
	self, err := os.Executable()
	log.Printf("Getting caps for: %s\n", self)
	if err != nil {
		log.Fatalf("Could not get path to own executable: %+v\n", err)
	}
	caps, err := capability.NewFile2(self)
	if err != nil {
		log.Fatalf("Could not get file caps: %+v\n", err)
	}

	err = caps.Load()
	if err != nil {
		log.Fatalf("Could not load file caps: %+v\n", err)
	}

	return &caps
}

func hasCapSysResource(caps *capability.Capabilities) bool {
	return (*caps).Get(capability.EFFECTIVE, capability.CAP_SYS_RESOURCE)
}

func makeBinarySetcapped() error {
	fileCaps := *getSelfFileCaps()
	if !hasCapSysResource(&fileCaps) {
		fileCaps.Set(capability.EFFECTIVE|capability.PERMITTED|capability.INHERITABLE, capability.CAP_SYS_RESOURCE)
		err := fileCaps.Apply(capability.EFFECTIVE | capability.PERMITTED | capability.INHERITABLE)
		if err != nil {
			return err
		}
	}
	return nil
}

func pkexecSetcapSelf() error {
	self, err := os.Executable()
	if err != nil {
		log.Fatalf("Couldn't find path to own binary\n")
		return err
	}

	cmd := exec.Command("pkexec", self, "-setcap")
	log.Printf("Calling: %s\n", cmd.String())
	err = cmd.Run()
	if err != nil {
		log.Printf("Couldn't setcap self as root: %v\n", err)
		return err
	}

	return nil
}
