// SPDX-License-Identifier: Unlicense OR MIT

@import Dispatch;
@import Foundation;

#include "_cgo_export.h"

void gio_wakeupMainThread(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		gio_dispatchMainFuncs();
	});
}

NSUInteger gio_nsstringLength(CFTypeRef cstr) {
	NSString *str = (__bridge NSString *)cstr;
	return [str length];
}

void gio_nsstringGetCharacters(CFTypeRef cstr, unichar *chars, NSUInteger loc, NSUInteger length) {
	NSString *str = (__bridge NSString *)cstr;
	[str getCharacters:chars range:NSMakeRange(loc, length)];
}
