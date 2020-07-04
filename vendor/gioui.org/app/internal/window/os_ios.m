// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

@import UIKit;

#include <stdint.h>
#include "_cgo_export.h"
#include "framework_ios.h"

@interface GioView: UIView <UIKeyInput>
@end

@implementation GioViewController

CGFloat _keyboardHeight;

- (void)loadView {
	gio_runMain();

	CGRect zeroFrame = CGRectMake(0, 0, 0, 0);
	self.view = [[UIView alloc] initWithFrame:zeroFrame];
	self.view.layoutMargins = UIEdgeInsetsMake(0, 0, 0, 0);
	UIView *drawView = [[GioView alloc] initWithFrame:zeroFrame];
	[self.view addSubview: drawView];
#ifndef TARGET_OS_TV
	drawView.multipleTouchEnabled = YES;
#endif
	drawView.preservesSuperviewLayoutMargins = YES;
	drawView.layoutMargins = UIEdgeInsetsMake(0, 0, 0, 0);
	onCreate((__bridge CFTypeRef)drawView);
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(keyboardWillChange:)
												 name:UIKeyboardWillShowNotification
											   object:nil];
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(keyboardWillChange:)
												 name:UIKeyboardWillChangeFrameNotification
											   object:nil];
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(keyboardWillHide:)
												 name:UIKeyboardWillHideNotification
											   object:nil];
	[[NSNotificationCenter defaultCenter] addObserver: self
											 selector: @selector(applicationDidEnterBackground:)
												 name: UIApplicationDidEnterBackgroundNotification
											   object: nil];
	[[NSNotificationCenter defaultCenter] addObserver: self
											 selector: @selector(applicationWillEnterForeground:)
												 name: UIApplicationWillEnterForegroundNotification
											   object: nil];
}

- (void)applicationWillEnterForeground:(UIApplication *)application {
	UIView *drawView = self.view.subviews[0];
	if (drawView != nil) {
		gio_onDraw((__bridge CFTypeRef)drawView);
	}
}

- (void)applicationDidEnterBackground:(UIApplication *)application {
	UIView *drawView = self.view.subviews[0];
	if (drawView != nil) {
		onStop((__bridge CFTypeRef)drawView);
	}
}

- (void)viewDidDisappear:(BOOL)animated {
	[super viewDidDisappear:animated];
	CFTypeRef viewRef = (__bridge CFTypeRef)self.view.subviews[0];
	onDestroy(viewRef);
}

- (void)viewDidLayoutSubviews {
	[super viewDidLayoutSubviews];
	UIView *view = self.view.subviews[0];
	CGRect frame = self.view.bounds;
	// Adjust view bounds to make room for the keyboard.
	frame.size.height -= _keyboardHeight;
	view.frame = frame;
	gio_onDraw((__bridge CFTypeRef)view);
}

- (void)didReceiveMemoryWarning {
	onLowMemory();
	[super didReceiveMemoryWarning];
}

- (void)keyboardWillChange:(NSNotification *)note {
	NSDictionary *userInfo = note.userInfo;
	CGRect f = [userInfo[UIKeyboardFrameEndUserInfoKey] CGRectValue];
	_keyboardHeight = f.size.height;
	[self.view setNeedsLayout];
}

- (void)keyboardWillHide:(NSNotification *)note {
	_keyboardHeight = 0.0;
	[self.view setNeedsLayout];
}
@end

static void handleTouches(int last, UIView *view, NSSet<UITouch *> *touches, UIEvent *event) {
	CGFloat scale = view.contentScaleFactor;
	NSUInteger i = 0;
	NSUInteger n = [touches count];
	CFTypeRef viewRef = (__bridge CFTypeRef)view;
	for (UITouch *touch in touches) {
		CFTypeRef touchRef = (__bridge CFTypeRef)touch;
		i++;
		NSArray<UITouch *> *coalescedTouches = [event coalescedTouchesForTouch:touch];
		NSUInteger j = 0;
		NSUInteger m = [coalescedTouches count];
		for (UITouch *coalescedTouch in [event coalescedTouchesForTouch:touch]) {
			CGPoint loc = [coalescedTouch locationInView:view];
			j++;
			int lastTouch = last && i == n && j == m;
			onTouch(lastTouch, viewRef, touchRef, touch.phase, loc.x*scale, loc.y*scale, [coalescedTouch timestamp]);
		}
	}
}

@implementation GioView
NSArray<UIKeyCommand *> *_keyCommands;
+ (void)onFrameCallback:(CADisplayLink *)link {
	gio_onFrameCallback((__bridge CFTypeRef)link);
}

- (void)willMoveToWindow:(UIWindow *)newWindow {
	if (self.window != nil) {
		[[NSNotificationCenter defaultCenter] removeObserver:self
														name:UIWindowDidBecomeKeyNotification
													  object:self.window];
		[[NSNotificationCenter defaultCenter] removeObserver:self
														name:UIWindowDidResignKeyNotification
													  object:self.window];
	}
	self.contentScaleFactor = newWindow.screen.nativeScale;
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(onWindowDidBecomeKey:)
												 name:UIWindowDidBecomeKeyNotification
											   object:newWindow];
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(onWindowDidResignKey:)
												 name:UIWindowDidResignKeyNotification
											   object:newWindow];
}

- (void)onWindowDidBecomeKey:(NSNotification *)note {
	if (self.isFirstResponder) {
		onFocus((__bridge CFTypeRef)self, YES);
	}
}

- (void)onWindowDidResignKey:(NSNotification *)note {
	if (self.isFirstResponder) {
		onFocus((__bridge CFTypeRef)self, NO);
	}
}

- (void)dealloc {
}

- (void)touchesBegan:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(0, self, touches, event);
}

- (void)touchesMoved:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(0, self, touches, event);
}

- (void)touchesEnded:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(1, self, touches, event);
}

- (void)touchesCancelled:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(1, self, touches, event);
}

- (void)insertText:(NSString *)text {
	onText((__bridge CFTypeRef)self, (char *)text.UTF8String);
}

- (BOOL)canBecomeFirstResponder {
	return YES;
}

- (BOOL)hasText {
	return YES;
}

- (void)deleteBackward {
	onDeleteBackward((__bridge CFTypeRef)self);
}

- (void)onUpArrow {
	onUpArrow((__bridge CFTypeRef)self);
}

- (void)onDownArrow {
	onDownArrow((__bridge CFTypeRef)self);
}

- (void)onLeftArrow {
	onLeftArrow((__bridge CFTypeRef)self);
}

- (void)onRightArrow {
	onRightArrow((__bridge CFTypeRef)self);
}

- (NSArray<UIKeyCommand *> *)keyCommands {
	if (_keyCommands == nil) {
		_keyCommands = @[
			[UIKeyCommand keyCommandWithInput:UIKeyInputUpArrow
								modifierFlags:0
									   action:@selector(onUpArrow)],
			[UIKeyCommand keyCommandWithInput:UIKeyInputDownArrow
								modifierFlags:0
									   action:@selector(onDownArrow)],
			[UIKeyCommand keyCommandWithInput:UIKeyInputLeftArrow
								modifierFlags:0
									   action:@selector(onLeftArrow)],
			[UIKeyCommand keyCommandWithInput:UIKeyInputRightArrow
								modifierFlags:0
									   action:@selector(onRightArrow)]
		];
	}
	return _keyCommands;
}
@end

void gio_writeClipboard(unichar *chars, NSUInteger length) {
	@autoreleasepool {
		NSString *s = [NSString string];
		if (length > 0) {
			s = [NSString stringWithCharacters:chars length:length];
		}
		UIPasteboard *p = UIPasteboard.generalPasteboard;
		p.string = s;
	}
}

CFTypeRef gio_readClipboard(void) {
	@autoreleasepool {
		UIPasteboard *p = UIPasteboard.generalPasteboard;
		return (__bridge_retained CFTypeRef)p.string;
	}
}

void gio_showTextInput(CFTypeRef viewRef) {
	UIView *view = (__bridge UIView *)viewRef;
	[view becomeFirstResponder];
}

void gio_hideTextInput(CFTypeRef viewRef) {
	UIView *view = (__bridge UIView *)viewRef;
	[view resignFirstResponder];
}

void gio_addLayerToView(CFTypeRef viewRef, CFTypeRef layerRef) {
	UIView *view = (__bridge UIView *)viewRef;
	CALayer *layer = (__bridge CALayer *)layerRef;
	[view.layer addSublayer:layer];
}

void gio_updateView(CFTypeRef viewRef, CFTypeRef layerRef) {
	UIView *view = (__bridge UIView *)viewRef;
	CAEAGLLayer *layer = (__bridge CAEAGLLayer *)layerRef;
	layer.contentsScale = view.contentScaleFactor;
	layer.bounds = view.bounds;
}

void gio_removeLayer(CFTypeRef layerRef) {
	CALayer *layer = (__bridge CALayer *)layerRef;
	[layer removeFromSuperlayer];
}

struct drawParams gio_viewDrawParams(CFTypeRef viewRef) {
	UIView *v = (__bridge UIView *)viewRef;
	struct drawParams params;
	CGFloat scale = v.layer.contentsScale;
	// Use 163 as the standard ppi on iOS.
	params.dpi = 163*scale;
	params.sdpi = params.dpi;
	UIEdgeInsets insets = v.layoutMargins;
	if (@available(iOS 11.0, tvOS 11.0, *)) {
		UIFontMetrics *metrics = [UIFontMetrics defaultMetrics];
		params.sdpi = [metrics scaledValueForValue:params.sdpi];
		insets = v.safeAreaInsets;
	}
	params.width = v.bounds.size.width*scale;
	params.height = v.bounds.size.height*scale;
	params.top = insets.top*scale;
	params.right = insets.right*scale;
	params.bottom = insets.bottom*scale;
	params.left = insets.left*scale;
	return params;
}

CFTypeRef gio_createDisplayLink(void) {
	CADisplayLink *dl = [CADisplayLink displayLinkWithTarget:[GioView class] selector:@selector(onFrameCallback:)];
	dl.paused = YES;
	NSRunLoop *runLoop = [NSRunLoop mainRunLoop];
	[dl addToRunLoop:runLoop forMode:[runLoop currentMode]];
	return (__bridge_retained CFTypeRef)dl;
}

int gio_startDisplayLink(CFTypeRef dlref) {
	CADisplayLink *dl = (__bridge CADisplayLink *)dlref;
	dl.paused = NO;
	return 0;
}

int gio_stopDisplayLink(CFTypeRef dlref) {
	CADisplayLink *dl = (__bridge CADisplayLink *)dlref;
	dl.paused = YES;
	return 0;
}

void gio_releaseDisplayLink(CFTypeRef dlref) {
	CADisplayLink *dl = (__bridge CADisplayLink *)dlref;
	[dl invalidate];
	CFRelease(dlref);
}

void gio_setDisplayLinkDisplay(CFTypeRef dl, uint64_t did) {
	// Nothing to do on iOS.
}
