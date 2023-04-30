// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

@import Foundation;

#include "_cgo_export.h"

void nslog(char *str) {
	NSLog(@"%@", @(str));
}
