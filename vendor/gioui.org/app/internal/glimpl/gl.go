// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin linux freebsd openbsd

package glimpl

import (
	"runtime"
	"strings"
	"unsafe"

	"gioui.org/gpu/gl"
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

#ifdef __APPLE__
	#include "TargetConditionals.h"
	#if TARGET_OS_IPHONE
	#include <OpenGLES/ES3/gl.h>
	#else
	#include <OpenGL/gl3.h>
	#endif
#else
#define __USE_GNU
#include <dlfcn.h>
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
	// gl.Framebuffer invalidation is just a hint and can safely be ignored.
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

__attribute__((constructor)) static void gio_loadGLFunctions() {
#ifdef __APPLE__
	#if TARGET_OS_IPHONE
	_glInvalidateFramebuffer = glInvalidateFramebuffer;
	_glBeginQuery = glBeginQuery;
	_glDeleteQueries = glDeleteQueries;
	_glEndQuery = glEndQuery;
	_glGenQueries = glGenQueries;
	_glGetQueryObjectuiv = glGetQueryObjectuiv;
	#endif
	_glBindBufferBase = glBindBufferBase;
	_glGetUniformBlockIndex = glGetUniformBlockIndex;
	_glUniformBlockBinding = glUniformBlockBinding;
	_glGetStringi = glGetStringi;
#else
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
#endif
}
*/
import "C"

type Functions struct {
	// gl.Query caches.
	uints [100]C.GLuint
	ints  [100]C.GLint
}

func (f *Functions) ActiveTexture(texture gl.Enum) {
	C.glActiveTexture(C.GLenum(texture))
}

func (f *Functions) AttachShader(p gl.Program, s gl.Shader) {
	C.glAttachShader(C.GLuint(p.V), C.GLuint(s.V))
}

func (f *Functions) BeginQuery(target gl.Enum, query gl.Query) {
	C.gio_glBeginQuery(C.GLenum(target), C.GLenum(query.V))
}

func (f *Functions) BindAttribLocation(p gl.Program, a gl.Attrib, name string) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.glBindAttribLocation(C.GLuint(p.V), C.GLuint(a), cname)
}

func (f *Functions) BindBufferBase(target gl.Enum, index int, b gl.Buffer) {
	C.gio_glBindBufferBase(C.GLenum(target), C.GLuint(index), C.GLuint(b.V))
}

func (f *Functions) BindBuffer(target gl.Enum, b gl.Buffer) {
	C.glBindBuffer(C.GLenum(target), C.GLuint(b.V))
}

func (f *Functions) BindFramebuffer(target gl.Enum, fb gl.Framebuffer) {
	C.glBindFramebuffer(C.GLenum(target), C.GLuint(fb.V))
}

func (f *Functions) BindRenderbuffer(target gl.Enum, fb gl.Renderbuffer) {
	C.glBindRenderbuffer(C.GLenum(target), C.GLuint(fb.V))
}

func (f *Functions) BindTexture(target gl.Enum, t gl.Texture) {
	C.glBindTexture(C.GLenum(target), C.GLuint(t.V))
}

func (f *Functions) BlendEquation(mode gl.Enum) {
	C.glBlendEquation(C.GLenum(mode))
}

func (f *Functions) BlendFunc(sfactor, dfactor gl.Enum) {
	C.glBlendFunc(C.GLenum(sfactor), C.GLenum(dfactor))
}

func (f *Functions) BufferData(target gl.Enum, src []byte, usage gl.Enum) {
	var p unsafe.Pointer
	if len(src) > 0 {
		p = unsafe.Pointer(&src[0])
	}
	C.glBufferData(C.GLenum(target), C.GLsizeiptr(len(src)), p, C.GLenum(usage))
}

func (f *Functions) CheckFramebufferStatus(target gl.Enum) gl.Enum {
	return gl.Enum(C.glCheckFramebufferStatus(C.GLenum(target)))
}

func (f *Functions) Clear(mask gl.Enum) {
	C.glClear(C.GLbitfield(mask))
}

func (f *Functions) ClearColor(red float32, green float32, blue float32, alpha float32) {
	C.glClearColor(C.GLfloat(red), C.GLfloat(green), C.GLfloat(blue), C.GLfloat(alpha))
}

func (f *Functions) ClearDepthf(d float32) {
	C.glClearDepthf(C.GLfloat(d))
}

func (f *Functions) CompileShader(s gl.Shader) {
	C.glCompileShader(C.GLuint(s.V))
}

func (f *Functions) CreateBuffer() gl.Buffer {
	C.glGenBuffers(1, &f.uints[0])
	return gl.Buffer{uint(f.uints[0])}
}

func (f *Functions) CreateFramebuffer() gl.Framebuffer {
	C.glGenFramebuffers(1, &f.uints[0])
	return gl.Framebuffer{uint(f.uints[0])}
}

func (f *Functions) CreateProgram() gl.Program {
	return gl.Program{uint(C.glCreateProgram())}
}

func (f *Functions) CreateQuery() gl.Query {
	C.gio_glGenQueries(1, &f.uints[0])
	return gl.Query{uint(f.uints[0])}
}

func (f *Functions) CreateRenderbuffer() gl.Renderbuffer {
	C.glGenRenderbuffers(1, &f.uints[0])
	return gl.Renderbuffer{uint(f.uints[0])}
}

func (f *Functions) CreateShader(ty gl.Enum) gl.Shader {
	return gl.Shader{uint(C.glCreateShader(C.GLenum(ty)))}
}

func (f *Functions) CreateTexture() gl.Texture {
	C.glGenTextures(1, &f.uints[0])
	return gl.Texture{uint(f.uints[0])}
}

func (f *Functions) DeleteBuffer(v gl.Buffer) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteBuffers(1, &f.uints[0])
}

func (f *Functions) DeleteFramebuffer(v gl.Framebuffer) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteFramebuffers(1, &f.uints[0])
}

func (f *Functions) DeleteProgram(p gl.Program) {
	C.glDeleteProgram(C.GLuint(p.V))
}

func (f *Functions) DeleteQuery(query gl.Query) {
	f.uints[0] = C.GLuint(query.V)
	C.gio_glDeleteQueries(1, &f.uints[0])
}

func (f *Functions) DeleteRenderbuffer(v gl.Renderbuffer) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteRenderbuffers(1, &f.uints[0])
}

func (f *Functions) DeleteShader(s gl.Shader) {
	C.glDeleteShader(C.GLuint(s.V))
}

func (f *Functions) DeleteTexture(v gl.Texture) {
	f.uints[0] = C.GLuint(v.V)
	C.glDeleteTextures(1, &f.uints[0])
}

func (f *Functions) DepthFunc(v gl.Enum) {
	C.glDepthFunc(C.GLenum(v))
}

func (f *Functions) DepthMask(mask bool) {
	m := C.GLboolean(C.GL_FALSE)
	if mask {
		m = C.GLboolean(C.GL_TRUE)
	}
	C.glDepthMask(m)
}

func (f *Functions) DisableVertexAttribArray(a gl.Attrib) {
	C.glDisableVertexAttribArray(C.GLuint(a))
}

func (f *Functions) Disable(cap gl.Enum) {
	C.glDisable(C.GLenum(cap))
}

func (f *Functions) DrawArrays(mode gl.Enum, first int, count int) {
	C.glDrawArrays(C.GLenum(mode), C.GLint(first), C.GLsizei(count))
}

func (f *Functions) DrawElements(mode gl.Enum, count int, ty gl.Enum, offset int) {
	C.gio_glDrawElements(C.GLenum(mode), C.GLsizei(count), C.GLenum(ty), C.uintptr_t(offset))
}

func (f *Functions) Enable(cap gl.Enum) {
	C.glEnable(C.GLenum(cap))
}

func (f *Functions) EndQuery(target gl.Enum) {
	C.gio_glEndQuery(C.GLenum(target))
}

func (f *Functions) EnableVertexAttribArray(a gl.Attrib) {
	C.glEnableVertexAttribArray(C.GLuint(a))
}

func (f *Functions) Finish() {
	C.glFinish()
}

func (f *Functions) FramebufferRenderbuffer(target, attachment, renderbuffertarget gl.Enum, renderbuffer gl.Renderbuffer) {
	C.glFramebufferRenderbuffer(C.GLenum(target), C.GLenum(attachment), C.GLenum(renderbuffertarget), C.GLuint(renderbuffer.V))
}

func (f *Functions) FramebufferTexture2D(target, attachment, texTarget gl.Enum, t gl.Texture, level int) {
	C.glFramebufferTexture2D(C.GLenum(target), C.GLenum(attachment), C.GLenum(texTarget), C.GLuint(t.V), C.GLint(level))
}

func (c *Functions) GetBinding(pname gl.Enum) gl.Object {
	return gl.Object{uint(c.GetInteger(pname))}
}

func (f *Functions) GetError() gl.Enum {
	return gl.Enum(C.glGetError())
}

func (f *Functions) GetRenderbufferParameteri(target, pname gl.Enum) int {
	C.glGetRenderbufferParameteriv(C.GLenum(target), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname gl.Enum) int {
	C.glGetFramebufferAttachmentParameteriv(C.GLenum(target), C.GLenum(attachment), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetInteger(pname gl.Enum) int {
	C.glGetIntegerv(C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetProgrami(p gl.Program, pname gl.Enum) int {
	C.glGetProgramiv(C.GLuint(p.V), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetProgramInfoLog(p gl.Program) string {
	n := f.GetProgrami(p, gl.INFO_LOG_LENGTH)
	buf := make([]byte, n)
	C.glGetProgramInfoLog(C.GLuint(p.V), C.GLsizei(len(buf)), nil, (*C.GLchar)(unsafe.Pointer(&buf[0])))
	return string(buf)
}

func (f *Functions) GetQueryObjectuiv(query gl.Query, pname gl.Enum) uint {
	C.gio_glGetQueryObjectuiv(C.GLuint(query.V), C.GLenum(pname), &f.uints[0])
	return uint(f.uints[0])
}

func (f *Functions) GetShaderi(s gl.Shader, pname gl.Enum) int {
	C.glGetShaderiv(C.GLuint(s.V), C.GLenum(pname), &f.ints[0])
	return int(f.ints[0])
}

func (f *Functions) GetShaderInfoLog(s gl.Shader) string {
	n := f.GetShaderi(s, gl.INFO_LOG_LENGTH)
	buf := make([]byte, n)
	C.glGetShaderInfoLog(C.GLuint(s.V), C.GLsizei(len(buf)), nil, (*C.GLchar)(unsafe.Pointer(&buf[0])))
	return string(buf)
}

func (f *Functions) GetStringi(pname gl.Enum, index int) string {
	str := C.gio_glGetStringi(C.GLenum(pname), C.GLuint(index))
	if str == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(str)))
}

func (f *Functions) GetString(pname gl.Enum) string {
	switch {
	case runtime.GOOS == "darwin" && pname == gl.EXTENSIONS:
		// macOS OpenGL 3 core profile doesn't support glGetString(GL_EXTENSIONS).
		// Use glGetStringi(GL_EXTENSIONS, <index>).
		var exts []string
		nexts := f.GetInteger(gl.NUM_EXTENSIONS)
		for i := 0; i < nexts; i++ {
			ext := f.GetStringi(gl.EXTENSIONS, i)
			exts = append(exts, ext)
		}
		return strings.Join(exts, " ")
	default:
		str := C.glGetString(C.GLenum(pname))
		return C.GoString((*C.char)(unsafe.Pointer(str)))
	}
}

func (f *Functions) GetUniformBlockIndex(p gl.Program, name string) uint {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return uint(C.gio_glGetUniformBlockIndex(C.GLuint(p.V), cname))
}

func (f *Functions) GetUniformLocation(p gl.Program, name string) gl.Uniform {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return gl.Uniform{int(C.glGetUniformLocation(C.GLuint(p.V), cname))}
}

func (f *Functions) InvalidateFramebuffer(target, attachment gl.Enum) {
	C.gio_glInvalidateFramebuffer(C.GLenum(target), C.GLenum(attachment))
}

func (f *Functions) LinkProgram(p gl.Program) {
	C.glLinkProgram(C.GLuint(p.V))
}

func (f *Functions) PixelStorei(pname gl.Enum, param int32) {
	C.glPixelStorei(C.GLenum(pname), C.GLint(param))
}

func (f *Functions) Scissor(x, y, width, height int32) {
	C.glScissor(C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height))
}

func (f *Functions) ReadPixels(x, y, width, height int, format, ty gl.Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	C.glReadPixels(C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height), C.GLenum(format), C.GLenum(ty), p)
}

func (f *Functions) RenderbufferStorage(target, internalformat gl.Enum, width, height int) {
	C.glRenderbufferStorage(C.GLenum(target), C.GLenum(internalformat), C.GLsizei(width), C.GLsizei(height))
}

func (f *Functions) ShaderSource(s gl.Shader, src string) {
	csrc := C.CString(src)
	defer C.free(unsafe.Pointer(csrc))
	strlen := C.GLint(len(src))
	C.glShaderSource(C.GLuint(s.V), 1, &csrc, &strlen)
}

func (f *Functions) TexImage2D(target gl.Enum, level int, internalFormat int, width int, height int, format gl.Enum, ty gl.Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	C.glTexImage2D(C.GLenum(target), C.GLint(level), C.GLint(internalFormat), C.GLsizei(width), C.GLsizei(height), 0, C.GLenum(format), C.GLenum(ty), p)
}

func (f *Functions) TexSubImage2D(target gl.Enum, level int, x int, y int, width int, height int, format gl.Enum, ty gl.Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	C.glTexSubImage2D(C.GLenum(target), C.GLint(level), C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height), C.GLenum(format), C.GLenum(ty), p)
}

func (f *Functions) TexParameteri(target, pname gl.Enum, param int) {
	C.glTexParameteri(C.GLenum(target), C.GLenum(pname), C.GLint(param))
}

func (f *Functions) UniformBlockBinding(p gl.Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	C.gio_glUniformBlockBinding(C.GLuint(p.V), C.GLuint(uniformBlockIndex), C.GLuint(uniformBlockBinding))
}

func (f *Functions) Uniform1f(dst gl.Uniform, v float32) {
	C.glUniform1f(C.GLint(dst.V), C.GLfloat(v))
}

func (f *Functions) Uniform1i(dst gl.Uniform, v int) {
	C.glUniform1i(C.GLint(dst.V), C.GLint(v))
}

func (f *Functions) Uniform2f(dst gl.Uniform, v0 float32, v1 float32) {
	C.glUniform2f(C.GLint(dst.V), C.GLfloat(v0), C.GLfloat(v1))
}

func (f *Functions) Uniform3f(dst gl.Uniform, v0 float32, v1 float32, v2 float32) {
	C.glUniform3f(C.GLint(dst.V), C.GLfloat(v0), C.GLfloat(v1), C.GLfloat(v2))
}

func (f *Functions) Uniform4f(dst gl.Uniform, v0 float32, v1 float32, v2 float32, v3 float32) {
	C.glUniform4f(C.GLint(dst.V), C.GLfloat(v0), C.GLfloat(v1), C.GLfloat(v2), C.GLfloat(v3))
}

func (f *Functions) UseProgram(p gl.Program) {
	C.glUseProgram(C.GLuint(p.V))
}

func (f *Functions) VertexAttribPointer(dst gl.Attrib, size int, ty gl.Enum, normalized bool, stride int, offset int) {
	var n C.GLboolean = C.GL_FALSE
	if normalized {
		n = C.GL_TRUE
	}
	C.gio_glVertexAttribPointer(C.GLuint(dst), C.GLint(size), C.GLenum(ty), n, C.GLsizei(stride), C.uintptr_t(offset))
}

func (f *Functions) Viewport(x int, y int, width int, height int) {
	C.glViewport(C.GLint(x), C.GLint(y), C.GLsizei(width), C.GLsizei(height))
}
