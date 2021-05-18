// +build !release

package main

type updateui struct {
	serverVersion string
	available     bool
	triggered     bool
	updatingText  string
}

func updateable() bool {
	return false
}

func updateCheck(ctx *ntcontext) {
	// noop for non-release versions
}

func update(ctx *ntcontext) {
	// noop for non-release versions
}
