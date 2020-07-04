// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;

#include <CoreFoundation/CoreFoundation.h>
#include <OpenGL/OpenGL.h>
#include <OpenGL/gl3.h>
#include "_cgo_export.h"

static void handleMouse(NSView *view, NSEvent *event, int typ, CGFloat dx, CGFloat dy) {
	NSPoint p = [view convertPoint:[event locationInWindow] fromView:nil];
	if (!event.hasPreciseScrollingDeltas) {
		// dx and dy are in rows and columns.
		dx *= 10;
		dy *= 10;
	}
	gio_onMouse((__bridge CFTypeRef)view, typ, [NSEvent pressedMouseButtons], p.x, p.y, dx, dy, [event timestamp], [event modifierFlags]);
}

@interface GioView : NSOpenGLView 
@end

@implementation GioView
- (instancetype)initWithFrame:(NSRect)frameRect
				  pixelFormat:(NSOpenGLPixelFormat *)format {
	return [super initWithFrame:frameRect pixelFormat:format];
}
- (void)prepareOpenGL {
	[super prepareOpenGL];
	// Bind a default VBA to emulate OpenGL ES 2.
	GLuint defVBA;
	glGenVertexArrays(1, &defVBA);
	glBindVertexArray(defVBA);
	glEnable(GL_FRAMEBUFFER_SRGB);
}
- (BOOL)isFlipped {
	return YES;
}
- (void)update {
	[super update];
	[self setNeedsDisplay:YES];
}
- (void)drawRect:(NSRect)r {
	gio_onDraw((__bridge CFTypeRef)self);
}
- (void)mouseDown:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_DOWN, 0, 0);
}
- (void)mouseUp:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_UP, 0, 0);
}
- (void)middleMouseDown:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_DOWN, 0, 0);
}
- (void)middletMouseUp:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_UP, 0, 0);
}
- (void)rightMouseDown:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_DOWN, 0, 0);
}
- (void)rightMouseUp:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_UP, 0, 0);
}
- (void)mouseMoved:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_MOVE, 0, 0);
}
- (void)mouseDragged:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_MOVE, 0, 0);
}
- (void)scrollWheel:(NSEvent *)event {
	CGFloat dx = -event.scrollingDeltaX;
	CGFloat dy = -event.scrollingDeltaY;
	handleMouse(self, event, GIO_MOUSE_SCROLL, dx, dy);
}
- (void)keyDown:(NSEvent *)event {
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys((__bridge CFTypeRef)self, (char *)[keys UTF8String], [event timestamp], [event modifierFlags]);
	[self interpretKeyEvents:[NSArray arrayWithObject:event]];
}
- (void)insertText:(id)string {
	const char *utf8 = [string UTF8String];
	gio_onText((__bridge CFTypeRef)self, (char *)utf8);
}
- (void)doCommandBySelector:(SEL)sel {
	// Don't pass commands up the responder chain.
	// They will end up in a beep.
}
@end

CFTypeRef gio_createGLView(void) {
	@autoreleasepool {
		NSOpenGLPixelFormatAttribute attr[] = {
			NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
			NSOpenGLPFAColorSize,     24,
			NSOpenGLPFADepthSize,     16,
			NSOpenGLPFAAccelerated,
			// Opt-in to automatic GPU switching. CGL-only property.
			kCGLPFASupportsAutomaticGraphicsSwitching,
			NSOpenGLPFAAllowOfflineRenderers,
			0
		};
		id pixFormat = [[NSOpenGLPixelFormat alloc] initWithAttributes:attr];

		NSRect frame = NSMakeRect(0, 0, 0, 0);
		GioView* view = [[GioView alloc] initWithFrame:frame pixelFormat:pixFormat];

		[view setWantsBestResolutionOpenGLSurface:YES];
		[view setWantsLayer:YES]; // The default in Mojave.

		return CFBridgingRetain(view);
	}
}

void gio_setNeedsDisplay(CFTypeRef viewRef) {
	NSOpenGLView *view = (__bridge NSOpenGLView *)viewRef;
	[view setNeedsDisplay:YES];
}

CFTypeRef gio_contextForView(CFTypeRef viewRef) {
	NSOpenGLView *view = (__bridge NSOpenGLView *)viewRef;
	return (__bridge CFTypeRef)view.openGLContext;
}

void gio_clearCurrentContext(void) {
	[NSOpenGLContext clearCurrentContext];
}

void gio_makeCurrentContext(CFTypeRef ctxRef) {
	NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
	[ctx makeCurrentContext];
}

void gio_lockContext(CFTypeRef ctxRef) {
	NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
	CGLLockContext([ctx CGLContextObj]);
}

void gio_unlockContext(CFTypeRef ctxRef) {
	NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
	CGLUnlockContext([ctx CGLContextObj]);
}
