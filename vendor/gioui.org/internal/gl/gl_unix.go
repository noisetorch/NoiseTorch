// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin linux freebsd openbsd

package gl

import (
	"runtime"
	"strings"
	"unsafe"
)

/*
#cgo CFLAGS: -Werror
#cgo linux,!android pkg-config: glesv2
#cgo linux freebsd LDFLAGS: -ldl
#cgo freebsd openbsd android LDFLAGS: -lGLESv2
#cgo freebsd CFLAGS: -I/usr/local/include
#cgo freebsd LDFLAGS: -L/usr/local/lib
#cgo openbsd CFLAGS: -I/usr/X11R6/include
#cgo openbsd LDFLAGS: -L/usr/X11R6/lib
#cgo darwin,!ios CFLAGS: -DGL_SILENCE_DEPRECATION
#cgo darwin,!ios LDFLAGS: -framework OpenGL
#cgo darwin,ios CFLAGS: -DGLES_SILENCE_DEPRECATION
#cgo darwin,ios LDFLAGS: -framework OpenGLES

#include <stdlib.h>
#define __USE_GNU
#include <dlfcn.h>

#ifdef __APPLE__
	#include "TargetConditionals.h"
	#if TARGET_OS_IPHONE
	#include <OpenGLES/ES3/gl.h>
	#else
	#include <OpenGL/gl3.h>
	#endif
#else
#include <GLES2/gl2.h>
#include <GLES3/gl3.h>
#endif

static void (*_glBindBufferBase)(GLenum target, GLuint index, GLuint buffer);
static GLuint (*_glGetUniformBlockIndex)(GLuint program, const GLchar *uniformBlockName);
static void (*_glUniformBlockBinding)(GLuint program, GLuint uniformBlockIndex, GLuint uniformBlockBinding);
static void (*_glInvalidateFramebuffer)(GLenum target, GLsizei numAttachments, const GLenum *attachments);

static void (*_glBeginQuery)(GLenum target, GLuint id);
static void (*_glDeleteQueries)(GLsizei n, const GLuint *ids);
static void (*_glEndQuery)(GLenum target);
static void (*_glGenQueries)(GLsizei n, GLuint *ids);
static void (*_glGetQueryObjectuiv)(GLuint id, GLenum pname, GLuint *params);
static const GLubyte* (*_glGetStringi)(GLenum name, GLuint index);
static void (*_glMemoryBarrier)(GLbitfield barriers);
static void (*_glDispatchCompute)(GLuint x, GLuint y, GLuint z);
static void* (*_glMapBufferRange)(GLenum target, GLintptr offset, GLsizeiptr length, GLbitfield access);
static GLboolean (*_glUnmapBuffer)(GLenum target);
static void (*_glBindImageTexture)(GLuint unit, GLuint texture, GLint level, GLboolean layered, GLint layer, GLenum access, GLenum format);
static void (*_glTexStorage2D)(GLenum target, GLsizei levels, GLenum internalformat, GLsizei width, GLsizei height);
static void (*_glBlitFramebuffer)(GLint srcX0, GLint srcY0, GLint srcX1, GLint srcY1, GLint dstX0, GLint dstY0, GLint dstX1, GLint dstY1, GLbitfield mask, GLenum filter);

// The pointer-free version of glVertexAttribPointer, to avoid the Cgo pointer checks.
__attribute__ ((visibility ("hidden"))) void gio_glVertexAttribPointer(GLuint index, GLint size, GLenum type, GLboolean normalized, GLsizei stride, uintptr_t offset) {
	glVertexAttribPointer(index, size, type, normalized, stride, (const GLvoid *)offset);
}

// The pointer-free version of glDrawElements, to avoid the Cgo pointer checks.
__attribute__ ((visibility ("hidden"))) void gio_glDrawElements(GLenum mode, GLsizei count, GLenum type, const uintptr_t offset) {
	glDrawElements(mode, count, type, (const GLvoid *)offset);
}

__attribute__ ((visibility ("hidden"))) void gio_glBindBufferBase(GLenum target, GLuint index, GLuint buffer) {
	_glBindBufferBase(target, index, buffer);
}

__attribute__ ((visibility ("hidden"))) void gio_glUniformBlockBinding(GLuint program, GLuint uniformBlockIndex, GLuint uniformBlockBinding) {
	_glUniformBlockBinding(program, uniformBlockIndex, uniformBlockBinding);
}

__attribute__ ((visibility ("hidden"))) GLuint gio_glGetUniformBlockIndex(GLuint program, const GLchar *uniformBlockName) {
	return _glGetUniformBlockIndex(program, uniformBlockName);
}

__attribute__ ((visibility ("hidden"))) void gio_glInvalidateFramebuffer(GLenum target, GLenum attachment) {
	// Framebuffer invalidation is just a hint and can safely be ignored.
	if (_glInvalidateFramebuffer != NULL) {
		_glInvalidateFramebuffer(target, 1, &attachment);
	}
}

__attribute__ ((visibility ("hidden"))) void gio_glBeginQuery(GLenum target, GLenum attachment) {
	_glBeginQuery(target, attachment);
}

__attribute__ ((visibility ("hidden"))) void gio_glDeleteQueries(GLsizei n, const GLuint *ids) {
	_glDeleteQueries(n, ids);
}

__attribute__ ((visibility ("hidden"))) void gio_glEndQuery(GLenum target) {
	_glEndQuery(target);
}

__attribute__ ((visibility ("hidden"))) const GLubyte* gio_glGetStringi(GLenum name, GLuint index) {
	if (_glGetStringi == NULL) {
		return NULL;
	}
	return _glGetStringi(name, index);
}

__attribute__ ((visibility ("hidden"))) void gio_glGenQueries(GLsizei n, GLuint *ids) {
	_glGenQueries(n, ids);
}

__attribute__ ((visibility ("hidden"))) void gio_glGetQueryObjectuiv(GLuint id, GLenum pname, GLuint *params) {
	_glGetQueryObjectuiv(id, pname, params);
}

__attribute__ ((visibility ("hidden"))) void gio_glMemoryBarrier(GLbitfield barriers) {
	_glMemoryBarrier(barriers);
}

__attribute__ ((visibility ("hidden"))) void gio_glDispatchCompute(GLuint x, GLuint y, GLuint z) {
	_glDispatchCompute(x, y, z);
}

__attribute__ ((visibility ("hidden"))) void *gio_glMapBufferRange(GLenum target, GLintptr offset, GLsizeiptr length, GLbitfield access) {
	return _glMapBufferRange(target, offset, length, access);
}

__attribute__ ((visibility ("hidden"))) GLboolean gio_glUnmapBuffer(GLenum target) {
	return _glUnmapBuffer(target);
}

__attribute__ ((visibility ("hidden"))) void gio_glBindImageTexture(GLuint unit, GLuint texture, GLint level, GLboolean layered, GLint layer, GLenum access, GLenum format) {
	_glBindImageTexture(unit, texture, level, layered, layer, access, format);
}

__attribute__ ((visibility ("hidden"))) void gio_glTexStorage2D(GLenum target, GLsizei levels, GLenum internalFormat, GLsizei width, GLsizei height) {
	_glTexStorage2D(target, levels, internalFormat, width, height);
}

__attribute__ ((visibility ("hidden"))) void gio_glBlitFramebuffer(GLint srcX0, GLint srcY0, GLint srcX1, GLint srcY1, GLint dstX0, GLint dstY0, GLint dstX1, GLint dstY1, GLbitfield mask, GLenum filter) {
	_glBlitFramebuffer(srcX0, srcY0, srcX1, srcY1, dstX0, dstY0, dstX1, dstY1, mask, filter);
}

__attribute__((constructor)) static void gio_loadGLFunctions() {
	// Load libGLESv3 if available.
	dlopen("libGLESv3.so", RTLD_NOW | RTLD_GLOBAL);

	_glBindBufferBase = dlsym(RTLD_DEFAULT, "glBindBufferBase");
	_glGetUniformBlockIndex = dlsym(RTLD_DEFAULT, "glGetUniformBlockIndex");
	_glUniformBlockBinding = dlsym(RTLD_DEFAULT, "glUniformBlockBinding");
	_glInvalidateFramebuffer = dlsym(RTLD_DEFAULT, "glInvalidateFramebuffer");
	_glGetStringi = dlsym(RTLD_DEFAULT, "glGetStringi");
	// Fall back to EXT_invalidate_framebuffer if available.
	if (_glInvalidateFramebuffer == NULL) {
		_glInvalidateFramebuffer = dlsym(RTLD_DEFAULT, "glDiscardFramebufferEXT");
	}

	_glBeginQuery = dlsym(RTLD_DEFAULT, "glBeginQuery");
	if (_glBeginQuery == NULL)
		_glBeginQuery = dlsym(RTLD_DEFAULT, "glBeginQueryEXT");
	_glDeleteQueries = dlsym(RTLD_DEFAULT, "glDeleteQueries");
	if (_glDeleteQueries == NULL)
		_glDeleteQueries = dlsym(RTLD_DEFAULT, "glDeleteQueriesEXT");
	_glEndQuery = dlsym(RTLD_DEFAULT, "glEndQuery");
	if (_glEndQuery == NULL)
		_glEndQuery = dlsym(RTLD_DEFAULT, "glEndQueryEXT");
	_glGenQueries = dlsym(RTLD_DEFAULT, "glGenQueries");
	if (_glGenQueries == NULL)
		_glGenQueries = dlsym(RTLD_DEFAULT, "glGenQueriesEXT");
	_glGetQueryObjectuiv = dlsym(RTLD_DEFAULT, "glGetQueryObjectuiv");
	if (_glGetQueryObjectuiv == NULL)
		_glGetQueryObjectuiv = dlsym(RTLD_DEFAULT, "glGetQueryObjectuivEXT");

	_glMemoryBarrier = dlsym(RTLD_DEFAULT, "glMemoryBarrier");
	_glDispatchCompute = dlsym(RTLD_DEFAULT, "glDispatchCompute");
	_glMapBufferRange = dlsym(RTLD_DEFAULT, "glMapBufferRange");
	_glUnmapBuffer = dlsym(RTLD_DEFAULT, "glUnmapBuffer");
	_glBindImageTexture = dlsym(RTLD_DEFAULT, "glBindImageTexture");
	_glTexStorage2D = dlsym(RTLD_DEFAULT, "glTexStorage2D");
	_glBlitFramebuffer = dlsym(RTLD_DEFAULT, "glBlitFramebuffer");
}
*/
import "C"

type Context interface{}

type Functions struct {
	// Query caches.
	uints [100]C.GLuint
	ints  [100]C.GLint
}

func NewFunctions(ctx Context) (*Functions, error) {
	if ctx != nil {
		panic("non-nil context")
	}
	return new(Functions), nil
}

func (f *Functions) ActiveTexture(texture Enum) {
	C.glActiveTexture(C.GLenum(texture))
}

func (f *Functions) AttachShader(p Program, s Shader) {
	C.glAttachShader(C.GLuint(p.V), C.GLuint(s.V))
}

func (f *Functions) BeginQuery(target Enum, query Query) {
	C.gio_glBeginQuery(C.GLenum(target), C.GLenum(query.V))
}

func (f *Functions) BindAttribLocation(p Program, a Attrib, name string) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.glBindAttribLocation(C.GLuint(p.V), C.GLuint(a), cname)
}

func (f *Functions) BindBufferBase(target Enum, index int, b Buffer) {
	C.gio_glBindBufferBase(C.GLenum(target), C.GLuint(index), C.GLuint(b.V))
}

func (f *Functions) BindBuffer(target Enum, b Buffer) {
	C.glBindBuffer(C.GLenum(target), C.GLuint(b.V))
}

func (f *Functions) BindFramebuffer(target Enum, fb Framebuffer) {
	C.glBindFramebuffer(C.GLenum(target), C.GLuint(fb.V))
}

func (f *Functions) BindRenderbuffer(target Enum, fb Renderbuffer) {
	C.glBindRenderbuffer(C.GLenum(target), C.GLuint(fb.V))
}

func (f *Functions) BindImageTexture(unit int, t Texture, level int, layered bool, layer int, access, format Enum) {
	l := C.GLboolean(C.GL_FALSE)
	if layered {
		l = C.GL_TRUE
	}
	C.gio_glBindImageTexture(C.GLuint(unit), C.GLuint(t.V), C.GLint(level), l, C.GLint(layer), C.GLenum(access), C.GLenum(format))
}

func (f *Functions) BindTexture(target Enum, t Texture) {
	C.glBindTexture(C.GLenum(target), C.GLuint(t.V))
}

func (f *Functions) BlendEquation(mode Enum) {
	C.glBlendEquation(C.GLenum(mode))
}

func (f *Functions) BlendFunc(sfactor, dfactor Enum) {
	C.glBlendFunc(C.GLenum(sfactor), C.GLenum(dfactor))
}

func (f *Functions) BlitFramebuffer(sx0, sy0, sx1, sy1, dx0, dy0, dx1, dy1 int, mask Enum, filter Enum) {
	C.gio_glBlitFramebuffer(
		C.GLint(sx0), C.GLint(sy0), C.GLint(sx1), C.GLint(sy1),
		C.GLint(dx0), C.GLint(dy0), C.GLint(dx1), C.GLint(dy1),
		C.GLenum(mask), C.GLenum(filter),
	)
}

func (f *Functions) BufferData(target Enum, size int, usage Enum) {
	C.glBufferData(C.GLenum(target), C.GLsizeiptr(size), nil, C.GLenum(usage))
}

func (f *Functions) BufferSubData(target Enum, offset int, src []byte) {
	var p unsafe.Pointer
	if len(src) > 0 {
		p = unsafe.Pointer(&src[0])
	}
	C.glBufferSubData(C.GLenum(target), C.GLintptr(offset), C.GLsizeiptr(len(src)), p)
}

func (f *Functions) CheckFramebufferStatus(target Enum) Enum {
	return Enum(C.glCheckFramebufferStatus(C.GLenum(target)))
}

func (f *Functions) Clear(mask Enum) {
	C.glClear(C.GLbitfield(mask))
}

func (f *Functions) ClearColor(red float32, green float32, blue float32, alpha float32) {
	C.glClearColor(C.GLfloat(red), C.GLfloat(green), C.GLfloat(blue), C.GLfloat(alpha))
}

func (f *Functions) ClearDepthf(d float32) {
	C.glClearDepthf(C.GLfloat(d))
}

func (f *Functions) CompileShader(s Shader) {
	C.glCompileShader(C.GLuint(s.V))
}

func (f *Functions) CreateBuffer() Buffer {
	C.glGenBuffers(1, &f.uints[0])
	return Buffer{uint(f.uints[0])}
}

func (f *Functions) CreateFramebuffer() Framebuffer {
	C.glGenFramebuffers(1, &f.uints[0])
	return Framebuffer{uint(f.uints[0])}
}

func (f *Functions) CreateProgram() Program {
	return Program{uint(C.glCreateProgram())}
}

func (f *Functions) CreateQuery() Query {
	C.gio_glGenQueries(1, &f.uints[0])
	return Query{uint(f.uints[0])}
}

func (f *Functions) CreateRenderbuffer() Renderbuffer {
	C.glGenRenderbuffers(1, &f.uints[0])
	return Renderbuffer{uint(f.uints[0])}
}

func (f *Functions) CreateShader(ty Enum) Shader {
	return Shader{uint(C.glCreateShader(C.GLenum(ty)))}
}

func (f *Functions) CreateTexture() Texture {
	C.glGenTextures(1, &f.uints[0])
	return Texture{uint(f.uints[0])}
}

func (f *Functions) DeleteBuffer(v Buffer) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteBuffers(1, &f.uints[0])
}

func (f *Functions) DeleteFramebuffer(v Framebuffer) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteFramebuffers(1, &f.uints[0])
}

func (f *Functions) DeleteProgram(p Program) {
	C.glDeleteProgram(C.GLuint(p.V))
}

func (f *Functions) DeleteQuery(query Query) {
	f.uints[0] = C.GLuint(query.V)
	C.gio_glDeleteQueries(1, &f.uints[0])
}

func (f *Functions) DeleteRenderbuffer(v Renderbuffer) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteRenderbuffers(1, &f.uints[0])
}

func (f *Functions) DeleteShader(s Shader) {
	C.glDeleteShader(C.GLuint(s.V))
}

func (f *Functions) DeleteTexture(v Texture) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteTextures(1, &f.uints[0])
}

func (f *Functions) DepthFunc(v Enum) {
	C.glDepthFunc(C.GLenum(v))
}

func (f *Functions) DepthMask(mask bool) {
	m := C.GLboolean(C.GL_FALSE)
	if mask {
		m = C.GLboolean(C.GL_TRUE)
	}
	C.glDepthMask(m)
}

func (f *Functions) DisableVertexAttribArray(a Attrib) {
	C.glDisableVertexAttribArray(C.GLuint(a))
}

func (f *Functions) Disable(cap Enum) {
	C.glDisable(C.GLenum(cap))
}

func (f *Functions) DrawArrays(mode Enum, first int, count int) {
	C.glDrawArrays(C.GLenum(mode), C.GLint(first), C.GLsizei(count))
}

func (f *Functions) DrawElements(mode Enum, count int, ty Enum, offset int) {
	C.gio_glDrawElements(C.GLenum(mode), C.GLsizei(count), C.GLenum(ty), C.uintptr_t(offset))
}

func (f *Functions) DispatchCompute(x, y, z int) {
	C.gio_glDispatchCompute(C.GLuint(x), C.GLuint(y), C.GLuint(z))
}

func (f *Functions) Enable(cap Enum) {
	C.glEnable(C.GLenum(cap))
}

func (f *Functions) EndQuery(target Enum) {
	C.gio_glEndQuery(C.GLenum(target))
}

func (f *Functions) EnableVertexAttribArray(a Attrib) {
	C.glEnableVertexAttribArray(C.GLuint(a))
}

func (f *Functions) Finish() {
	C.glFinish()
}

func (f *Functions) FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer) {
	C.glFramebufferRenderbuffer(C.GLenum(target), C.GLenum(attachment), C.GLenum(renderbuffertarget), C.GLuint(renderbuffer.V))
}

func (f *Functions) FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int) {
	C.glFramebufferTexture2D(C.GLenum(target), C.GLenum(attachment), C.GLenum(texTarget), C.GLuint(t.V), C.GLint(level))
}

func (c *Functions) GetBinding(pname Enum) Object {
	return Object{uint(c.GetInteger(pname))}
}

func (f *Functions) GetError() Enum {
	return Enum(C.glGetError())
}

func (f *Functions) GetRenderbufferParameteri(target, pname Enum) int {
	C.glGetRenderbufferParameteriv(C.GLenum(target), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int {
	C.glGetFramebufferAttachmentParameteriv(C.GLenum(target), C.GLenum(attachment), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetInteger(pname Enum) int {
	C.glGetIntegerv(C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetProgrami(p Program, pname Enum) int {
	C.glGetProgramiv(C.GLuint(p.V), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetProgramInfoLog(p Program) string {
	n := f.GetProgrami(p, INFO_LOG_LENGTH)
	buf := make([]byte, n)
	C.glGetProgramInfoLog(C.GLuint(p.V), C.GLsizei(len(buf)), nil, (*C.GLchar)(unsafe.Pointer(&buf[0])))
	return string(buf)
}

func (f *Functions) GetQueryObjectuiv(query Query, pname Enum) uint {
	C.gio_glGetQueryObjectuiv(C.GLuint(query.V), C.GLenum(pname), &f.uints[0])
	return uint(f.uints[0])
}

func (f *Functions) GetShaderi(s Shader, pname Enum) int {
	C.glGetShaderiv(C.GLuint(s.V), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetShaderInfoLog(s Shader) string {
	n := f.GetShaderi(s, INFO_LOG_LENGTH)
	buf := make([]byte, n)
	C.glGetShaderInfoLog(C.GLuint(s.V), C.GLsizei(len(buf)), nil, (*C.GLchar)(unsafe.Pointer(&buf[0])))
	return string(buf)
}

func (f *Functions) GetStringi(pname Enum, index int) string {
	str := C.gio_glGetStringi(C.GLenum(pname), C.GLuint(index))
	if str == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(str)))
}

func (f *Functions) GetString(pname Enum) string {
	switch {
	case runtime.GOOS == "darwin" && pname == EXTENSIONS:
		// macOS OpenGL 3 core profile doesn't support glGetString(GL_EXTENSIONS).
		// Use glGetStringi(GL_EXTENSIONS, <index>).
		var exts []string
		nexts := f.GetInteger(NUM_EXTENSIONS)
		for i := 0; i < nexts; i++ {
			ext := f.GetStringi(EXTENSIONS, i)
			exts = append(exts, ext)
		}
		return strings.Join(exts, " ")
	default:
		str := C.glGetString(C.GLenum(pname))
		return C.GoString((*C.char)(unsafe.Pointer(str)))
	}
}

func (f *Functions) GetUniformBlockIndex(p Program, name string) uint {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return uint(C.gio_glGetUniformBlockIndex(C.GLuint(p.V), cname))
}

func (f *Functions) GetUniformLocation(p Program, name string) Uniform {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return Uniform{int(C.glGetUniformLocation(C.GLuint(p.V), cname))}
}

func (f *Functions) InvalidateFramebuffer(target, attachment Enum) {
	C.gio_glInvalidateFramebuffer(C.GLenum(target), C.GLenum(attachment))
}

func (f *Functions) LinkProgram(p Program) {
	C.glLinkProgram(C.GLuint(p.V))
}

func (f *Functions) PixelStorei(pname Enum, param int32) {
	C.glPixelStorei(C.GLenum(pname), C.GLint(param))
}

func (f *Functions) MemoryBarrier(barriers Enum) {
	C.gio_glMemoryBarrier(C.GLbitfield(barriers))
}

func (f *Functions) MapBufferRange(target Enum, offset, length int, access Enum) []byte {
	p := C.gio_glMapBufferRange(C.GLenum(target), C.GLintptr(offset), C.GLsizeiptr(length), C.GLbitfield(access))
	if p == nil {
		return nil
	}
	return (*[1 << 30]byte)(p)[:length:length]
}

func (f *Functions) Scissor(x, y, width, height int32) {
	C.glScissor(C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height))
}

func (f *Functions) ReadPixels(x, y, width, height int, format, ty Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	C.glReadPixels(C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height), C.GLenum(format), C.GLenum(ty), p)
}

func (f *Functions) RenderbufferStorage(target, internalformat Enum, width, height int) {
	C.glRenderbufferStorage(C.GLenum(target), C.GLenum(internalformat), C.GLsizei(width), C.GLsizei(height))
}

func (f *Functions) ShaderSource(s Shader, src string) {
	csrc := C.CString(src)
	defer C.free(unsafe.Pointer(csrc))
	strlen := C.GLint(len(src))
	C.glShaderSource(C.GLuint(s.V), 1, &csrc, &strlen)
}

func (f *Functions) TexImage2D(target Enum, level int, internalFormat Enum, width int, height int, format Enum, ty Enum) {
	C.glTexImage2D(C.GLenum(target), C.GLint(level), C.GLint(internalFormat), C.GLsizei(width), C.GLsizei(height), 0, C.GLenum(format), C.GLenum(ty), nil)
}

func (f *Functions) TexStorage2D(target Enum, levels int, internalFormat Enum, width, height int) {
	C.gio_glTexStorage2D(C.GLenum(target), C.GLsizei(levels), C.GLenum(internalFormat), C.GLsizei(width), C.GLsizei(height))
}

func (f *Functions) TexSubImage2D(target Enum, level int, x int, y int, width int, height int, format Enum, ty Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	C.glTexSubImage2D(C.GLenum(target), C.GLint(level), C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height), C.GLenum(format), C.GLenum(ty), p)
}

func (f *Functions) TexParameteri(target, pname Enum, param int) {
	C.glTexParameteri(C.GLenum(target), C.GLenum(pname), C.GLint(param))
}

func (f *Functions) UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	C.gio_glUniformBlockBinding(C.GLuint(p.V), C.GLuint(uniformBlockIndex), C.GLuint(uniformBlockBinding))
}

func (f *Functions) Uniform1f(dst Uniform, v float32) {
	C.glUniform1f(C.GLint(dst.V), C.GLfloat(v))
}

func (f *Functions) Uniform1i(dst Uniform, v int) {
	C.glUniform1i(C.GLint(dst.V), C.GLint(v))
}

func (f *Functions) Uniform2f(dst Uniform, v0 float32, v1 float32) {
	C.glUniform2f(C.GLint(dst.V), C.GLfloat(v0), C.GLfloat(v1))
}

func (f *Functions) Uniform3f(dst Uniform, v0 float32, v1 float32, v2 float32) {
	C.glUniform3f(C.GLint(dst.V), C.GLfloat(v0), C.GLfloat(v1), C.GLfloat(v2))
}

func (f *Functions) Uniform4f(dst Uniform, v0 float32, v1 float32, v2 float32, v3 float32) {
	C.glUniform4f(C.GLint(dst.V), C.GLfloat(v0), C.GLfloat(v1), C.GLfloat(v2), C.GLfloat(v3))
}

func (f *Functions) UseProgram(p Program) {
	C.glUseProgram(C.GLuint(p.V))
}

func (f *Functions) UnmapBuffer(target Enum) bool {
	r := C.gio_glUnmapBuffer(C.GLenum(target))
	return r == C.GL_TRUE
}

func (f *Functions) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride int, offset int) {
	var n C.GLboolean = C.GL_FALSE
	if normalized {
		n = C.GL_TRUE
	}
	C.gio_glVertexAttribPointer(C.GLuint(dst), C.GLint(size), C.GLenum(ty), n, C.GLsizei(stride), C.uintptr_t(offset))
}

func (f *Functions) Viewport(x int, y int, width int, height int) {
	C.glViewport(C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height))
}
