// +build !release

package main

import "errors"

type updateui struct {
	serverVersion string
	available     bool
	triggered     bool
	updatingText  string
}

func updateCheck(ctx *ntcontext) {

}

func update(ctx *ntcontext) {

}

func fetchFile(file string) ([]byte, error) {
	return make([]byte, 0), errors.New("Disabled by build flags")
}

func publickey() []byte {
	return make([]byte, 0)
}
