// SPDX-License-Identifier: Unlicense OR MIT

package glimpl

import (
	"math"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"gioui.org/gpu/gl"
	gunsafe "gioui.org/internal/unsafe"
)

var (
	LibGLESv2                             = windows.NewLazyDLL("libGLESv2.dll")
	_glActiveTexture                      = LibGLESv2.NewProc("glActiveTexture")
	_glAttachShader                       = LibGLESv2.NewProc("glAttachShader")
	_glBeginQuery                         = LibGLESv2.NewProc("glBeginQuery")
	_glBindAttribLocation                 = LibGLESv2.NewProc("glBindAttribLocation")
	_glBindBuffer                         = LibGLESv2.NewProc("glBindBuffer")
	_glBindBufferBase                     = LibGLESv2.NewProc("glBindBufferBase")
	_glBindFramebuffer                    = LibGLESv2.NewProc("glBindFramebuffer")
	_glBindRenderbuffer                   = LibGLESv2.NewProc("glBindRenderbuffer")
	_glBindTexture                        = LibGLESv2.NewProc("glBindTexture")
	_glBlendEquation                      = LibGLESv2.NewProc("glBlendEquation")
	_glBlendFunc                          = LibGLESv2.NewProc("glBlendFunc")
	_glBufferData                         = LibGLESv2.NewProc("glBufferData")
	_glCheckFramebufferStatus             = LibGLESv2.NewProc("glCheckFramebufferStatus")
	_glClear                              = LibGLESv2.NewProc("glClear")
	_glClearColor                         = LibGLESv2.NewProc("glClearColor")
	_glClearDepthf                        = LibGLESv2.NewProc("glClearDepthf")
	_glDeleteQueries                      = LibGLESv2.NewProc("glDeleteQueries")
	_glCompileShader                      = LibGLESv2.NewProc("glCompileShader")
	_glGenBuffers                         = LibGLESv2.NewProc("glGenBuffers")
	_glGenFramebuffers                    = LibGLESv2.NewProc("glGenFramebuffers")
	_glGetUniformBlockIndex               = LibGLESv2.NewProc("glGetUniformBlockIndex")
	_glCreateProgram                      = LibGLESv2.NewProc("glCreateProgram")
	_glGenRenderbuffers                   = LibGLESv2.NewProc("glGenRenderbuffers")
	_glCreateShader                       = LibGLESv2.NewProc("glCreateShader")
	_glGenTextures                        = LibGLESv2.NewProc("glGenTextures")
	_glDeleteBuffers                      = LibGLESv2.NewProc("glDeleteBuffers")
	_glDeleteFramebuffers                 = LibGLESv2.NewProc("glDeleteFramebuffers")
	_glDeleteProgram                      = LibGLESv2.NewProc("glDeleteProgram")
	_glDeleteShader                       = LibGLESv2.NewProc("glDeleteShader")
	_glDeleteRenderbuffers                = LibGLESv2.NewProc("glDeleteRenderbuffers")
	_glDeleteTextures                     = LibGLESv2.NewProc("glDeleteTextures")
	_glDepthFunc                          = LibGLESv2.NewProc("glDepthFunc")
	_glDepthMask                          = LibGLESv2.NewProc("glDepthMask")
	_glDisableVertexAttribArray           = LibGLESv2.NewProc("glDisableVertexAttribArray")
	_glDisable                            = LibGLESv2.NewProc("glDisable")
	_glDrawArrays                         = LibGLESv2.NewProc("glDrawArrays")
	_glDrawElements                       = LibGLESv2.NewProc("glDrawElements")
	_glEnable                             = LibGLESv2.NewProc("glEnable")
	_glEnableVertexAttribArray            = LibGLESv2.NewProc("glEnableVertexAttribArray")
	_glEndQuery                           = LibGLESv2.NewProc("glEndQuery")
	_glFinish                             = LibGLESv2.NewProc("glFinish")
	_glFramebufferRenderbuffer            = LibGLESv2.NewProc("glFramebufferRenderbuffer")
	_glFramebufferTexture2D               = LibGLESv2.NewProc("glFramebufferTexture2D")
	_glGenQueries                         = LibGLESv2.NewProc("glGenQueries")
	_glGetError                           = LibGLESv2.NewProc("glGetError")
	_glGetRenderbufferParameteri          = LibGLESv2.NewProc("glGetRenderbufferParameteri")
	_glGetFramebufferAttachmentParameteri = LibGLESv2.NewProc("glGetFramebufferAttachmentParameteri")
	_glGetIntegerv                        = LibGLESv2.NewProc("glGetIntegerv")
	_glGetProgramiv                       = LibGLESv2.NewProc("glGetProgramiv")
	_glGetProgramInfoLog                  = LibGLESv2.NewProc("glGetProgramInfoLog")
	_glGetQueryObjectuiv                  = LibGLESv2.NewProc("glGetQueryObjectuiv")
	_glGetShaderiv                        = LibGLESv2.NewProc("glGetShaderiv")
	_glGetShaderInfoLog                   = LibGLESv2.NewProc("glGetShaderInfoLog")
	_glGetString                          = LibGLESv2.NewProc("glGetString")
	_glGetUniformLocation                 = LibGLESv2.NewProc("glGetUniformLocation")
	_glInvalidateFramebuffer              = LibGLESv2.NewProc("glInvalidateFramebuffer")
	_glLinkProgram                        = LibGLESv2.NewProc("glLinkProgram")
	_glPixelStorei                        = LibGLESv2.NewProc("glPixelStorei")
	_glReadPixels                         = LibGLESv2.NewProc("glReadPixels")
	_glRenderbufferStorage                = LibGLESv2.NewProc("glRenderbufferStorage")
	_glScissor                            = LibGLESv2.NewProc("glScissor")
	_glShaderSource                       = LibGLESv2.NewProc("glShaderSource")
	_glTexImage2D                         = LibGLESv2.NewProc("glTexImage2D")
	_glTexSubImage2D                      = LibGLESv2.NewProc("glTexSubImage2D")
	_glTexParameteri                      = LibGLESv2.NewProc("glTexParameteri")
	_glUniformBlockBinding                = LibGLESv2.NewProc("glUniformBlockBinding")
	_glUniform1f                          = LibGLESv2.NewProc("glUniform1f")
	_glUniform1i                          = LibGLESv2.NewProc("glUniform1i")
	_glUniform2f                          = LibGLESv2.NewProc("glUniform2f")
	_glUniform3f                          = LibGLESv2.NewProc("glUniform3f")
	_glUniform4f                          = LibGLESv2.NewProc("glUniform4f")
	_glUseProgram                         = LibGLESv2.NewProc("glUseProgram")
	_glVertexAttribPointer                = LibGLESv2.NewProc("glVertexAttribPointer")
	_glViewport                           = LibGLESv2.NewProc("glViewport")
)

type Functions struct {
	// gl.Query caches.
	int32s [100]int32
}

func (c *Functions) ActiveTexture(t gl.Enum) {
	syscall.Syscall(_glActiveTexture.Addr(), 1, uintptr(t), 0, 0)
}
func (c *Functions) AttachShader(p gl.Program, s gl.Shader) {
	syscall.Syscall(_glAttachShader.Addr(), 2, uintptr(p.V), uintptr(s.V), 0)
}
func (f *Functions) BeginQuery(target gl.Enum, query gl.Query) {
	syscall.Syscall(_glBeginQuery.Addr(), 2, uintptr(target), uintptr(query.V), 0)
}
func (c *Functions) BindAttribLocation(p gl.Program, a gl.Attrib, name string) {
	cname := cString(name)
	c0 := &cname[0]
	syscall.Syscall(_glBindAttribLocation.Addr(), 3, uintptr(p.V), uintptr(a), uintptr(unsafe.Pointer(c0)))
	issue34474KeepAlive(c)
}
func (c *Functions) BindBuffer(target gl.Enum, b gl.Buffer) {
	syscall.Syscall(_glBindBuffer.Addr(), 2, uintptr(target), uintptr(b.V), 0)
}
func (c *Functions) BindBufferBase(target gl.Enum, index int, b gl.Buffer) {
	syscall.Syscall(_glBindBufferBase.Addr(), 3, uintptr(target), uintptr(index), uintptr(b.V))
}
func (c *Functions) BindFramebuffer(target gl.Enum, fb gl.Framebuffer) {
	syscall.Syscall(_glBindFramebuffer.Addr(), 2, uintptr(target), uintptr(fb.V), 0)
}
func (c *Functions) BindRenderbuffer(target gl.Enum, rb gl.Renderbuffer) {
	syscall.Syscall(_glBindRenderbuffer.Addr(), 2, uintptr(target), uintptr(rb.V), 0)
}
func (c *Functions) BindTexture(target gl.Enum, t gl.Texture) {
	syscall.Syscall(_glBindTexture.Addr(), 2, uintptr(target), uintptr(t.V), 0)
}
func (c *Functions) BlendEquation(mode gl.Enum) {
	syscall.Syscall(_glBlendEquation.Addr(), 1, uintptr(mode), 0, 0)
}
func (c *Functions) BlendFunc(sfactor, dfactor gl.Enum) {
	syscall.Syscall(_glBlendFunc.Addr(), 2, uintptr(sfactor), uintptr(dfactor), 0)
}
func (c *Functions) BufferData(target gl.Enum, src []byte, usage gl.Enum) {
	if n := len(src); n == 0 {
		syscall.Syscall6(_glBufferData.Addr(), 4, uintptr(target), 0, 0, uintptr(usage), 0, 0)
	} else {
		s0 := &src[0]
		syscall.Syscall6(_glBufferData.Addr(), 4, uintptr(target), uintptr(n), uintptr(unsafe.Pointer(s0)), uintptr(usage), 0, 0)
		issue34474KeepAlive(s0)
	}
}
func (c *Functions) CheckFramebufferStatus(target gl.Enum) gl.Enum {
	s, _, _ := syscall.Syscall(_glCheckFramebufferStatus.Addr(), 1, uintptr(target), 0, 0)
	return gl.Enum(s)
}
func (c *Functions) Clear(mask gl.Enum) {
	syscall.Syscall(_glClear.Addr(), 1, uintptr(mask), 0, 0)
}
func (c *Functions) ClearColor(red, green, blue, alpha float32) {
	syscall.Syscall6(_glClearColor.Addr(), 4, uintptr(math.Float32bits(red)), uintptr(math.Float32bits(green)), uintptr(math.Float32bits(blue)), uintptr(math.Float32bits(alpha)), 0, 0)
}
func (c *Functions) ClearDepthf(d float32) {
	syscall.Syscall(_glClearDepthf.Addr(), 1, uintptr(math.Float32bits(d)), 0, 0)
}
func (c *Functions) CompileShader(s gl.Shader) {
	syscall.Syscall(_glCompileShader.Addr(), 1, uintptr(s.V), 0, 0)
}
func (c *Functions) CreateBuffer() gl.Buffer {
	var buf uintptr
	syscall.Syscall(_glGenBuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&buf)), 0)
	return gl.Buffer{uint(buf)}
}
func (c *Functions) CreateFramebuffer() gl.Framebuffer {
	var fb uintptr
	syscall.Syscall(_glGenFramebuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&fb)), 0)
	return gl.Framebuffer{uint(fb)}
}
func (c *Functions) CreateProgram() gl.Program {
	p, _, _ := syscall.Syscall(_glCreateProgram.Addr(), 0, 0, 0, 0)
	return gl.Program{uint(p)}
}
func (f *Functions) CreateQuery() gl.Query {
	var q uintptr
	syscall.Syscall(_glGenQueries.Addr(), 2, 1, uintptr(unsafe.Pointer(&q)), 0)
	return gl.Query{uint(q)}
}
func (c *Functions) CreateRenderbuffer() gl.Renderbuffer {
	var rb uintptr
	syscall.Syscall(_glGenRenderbuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&rb)), 0)
	return gl.Renderbuffer{uint(rb)}
}
func (c *Functions) CreateShader(ty gl.Enum) gl.Shader {
	s, _, _ := syscall.Syscall(_glCreateShader.Addr(), 1, uintptr(ty), 0, 0)
	return gl.Shader{uint(s)}
}
func (c *Functions) CreateTexture() gl.Texture {
	var t uintptr
	syscall.Syscall(_glGenTextures.Addr(), 2, 1, uintptr(unsafe.Pointer(&t)), 0)
	return gl.Texture{uint(t)}
}
func (c *Functions) DeleteBuffer(v gl.Buffer) {
	syscall.Syscall(_glDeleteBuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&v)), 0)
}
func (c *Functions) DeleteFramebuffer(v gl.Framebuffer) {
	syscall.Syscall(_glDeleteFramebuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&v.V)), 0)
}
func (c *Functions) DeleteProgram(p gl.Program) {
	syscall.Syscall(_glDeleteProgram.Addr(), 1, uintptr(p.V), 0, 0)
}
func (f *Functions) DeleteQuery(query gl.Query) {
	syscall.Syscall(_glDeleteQueries.Addr(), 2, 1, uintptr(unsafe.Pointer(&query.V)), 0)
}
func (c *Functions) DeleteShader(s gl.Shader) {
	syscall.Syscall(_glDeleteShader.Addr(), 1, uintptr(s.V), 0, 0)
}
func (c *Functions) DeleteRenderbuffer(v gl.Renderbuffer) {
	syscall.Syscall(_glDeleteRenderbuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&v.V)), 0)
}
func (c *Functions) DeleteTexture(v gl.Texture) {
	syscall.Syscall(_glDeleteTextures.Addr(), 2, 1, uintptr(unsafe.Pointer(&v.V)), 0)
}
func (c *Functions) DepthFunc(f gl.Enum) {
	syscall.Syscall(_glDepthFunc.Addr(), 1, uintptr(f), 0, 0)
}
func (c *Functions) DepthMask(mask bool) {
	var m uintptr
	if mask {
		m = 1
	}
	syscall.Syscall(_glDepthMask.Addr(), 1, m, 0, 0)
}
func (c *Functions) DisableVertexAttribArray(a gl.Attrib) {
	syscall.Syscall(_glDisableVertexAttribArray.Addr(), 1, uintptr(a), 0, 0)
}
func (c *Functions) Disable(cap gl.Enum) {
	syscall.Syscall(_glDisable.Addr(), 1, uintptr(cap), 0, 0)
}
func (c *Functions) DrawArrays(mode gl.Enum, first, count int) {
	syscall.Syscall(_glDrawArrays.Addr(), 3, uintptr(mode), uintptr(first), uintptr(count))
}
func (c *Functions) DrawElements(mode gl.Enum, count int, ty gl.Enum, offset int) {
	syscall.Syscall6(_glDrawElements.Addr(), 4, uintptr(mode), uintptr(count), uintptr(ty), uintptr(offset), 0, 0)
}
func (c *Functions) Enable(cap gl.Enum) {
	syscall.Syscall(_glEnable.Addr(), 1, uintptr(cap), 0, 0)
}
func (c *Functions) EnableVertexAttribArray(a gl.Attrib) {
	syscall.Syscall(_glEnableVertexAttribArray.Addr(), 1, uintptr(a), 0, 0)
}
func (f *Functions) EndQuery(target gl.Enum) {
	syscall.Syscall(_glEndQuery.Addr(), 1, uintptr(target), 0, 0)
}
func (c *Functions) Finish() {
	syscall.Syscall(_glFinish.Addr(), 0, 0, 0, 0)
}
func (c *Functions) FramebufferRenderbuffer(target, attachment, renderbuffertarget gl.Enum, renderbuffer gl.Renderbuffer) {
	syscall.Syscall6(_glFramebufferRenderbuffer.Addr(), 4, uintptr(target), uintptr(attachment), uintptr(renderbuffertarget), uintptr(renderbuffer.V), 0, 0)
}
func (c *Functions) FramebufferTexture2D(target, attachment, texTarget gl.Enum, t gl.Texture, level int) {
	syscall.Syscall6(_glFramebufferTexture2D.Addr(), 5, uintptr(target), uintptr(attachment), uintptr(texTarget), uintptr(t.V), uintptr(level), 0)
}
func (f *Functions) GetUniformBlockIndex(p gl.Program, name string) uint {
	cname := cString(name)
	c0 := &cname[0]
	u, _, _ := syscall.Syscall(_glGetUniformBlockIndex.Addr(), 2, uintptr(p.V), uintptr(unsafe.Pointer(c0)), 0)
	issue34474KeepAlive(c0)
	return uint(u)
}
func (c *Functions) GetBinding(pname gl.Enum) gl.Object {
	return gl.Object{uint(c.GetInteger(pname))}
}
func (c *Functions) GetError() gl.Enum {
	e, _, _ := syscall.Syscall(_glGetError.Addr(), 0, 0, 0, 0)
	return gl.Enum(e)
}
func (c *Functions) GetRenderbufferParameteri(target, pname gl.Enum) int {
	p, _, _ := syscall.Syscall(_glGetRenderbufferParameteri.Addr(), 2, uintptr(target), uintptr(pname), 0)
	return int(p)
}
func (c *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname gl.Enum) int {
	p, _, _ := syscall.Syscall(_glGetFramebufferAttachmentParameteri.Addr(), 3, uintptr(target), uintptr(attachment), uintptr(pname))
	return int(p)
}
func (c *Functions) GetInteger(pname gl.Enum) int {
	syscall.Syscall(_glGetIntegerv.Addr(), 2, uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])), 0)
	return int(c.int32s[0])
}
func (c *Functions) GetProgrami(p gl.Program, pname gl.Enum) int {
	syscall.Syscall(_glGetProgramiv.Addr(), 3, uintptr(p.V), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}
func (c *Functions) GetProgramInfoLog(p gl.Program) string {
	n := c.GetProgrami(p, gl.INFO_LOG_LENGTH)
	buf := make([]byte, n)
	syscall.Syscall6(_glGetProgramInfoLog.Addr(), 4, uintptr(p.V), uintptr(len(buf)), 0, uintptr(unsafe.Pointer(&buf[0])), 0, 0)
	return string(buf)
}
func (c *Functions) GetQueryObjectuiv(query gl.Query, pname gl.Enum) uint {
	syscall.Syscall(_glGetQueryObjectuiv.Addr(), 3, uintptr(query.V), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return uint(c.int32s[0])
}
func (c *Functions) GetShaderi(s gl.Shader, pname gl.Enum) int {
	syscall.Syscall(_glGetShaderiv.Addr(), 3, uintptr(s.V), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}
func (c *Functions) GetShaderInfoLog(s gl.Shader) string {
	n := c.GetShaderi(s, gl.INFO_LOG_LENGTH)
	buf := make([]byte, n)
	syscall.Syscall6(_glGetShaderInfoLog.Addr(), 4, uintptr(s.V), uintptr(len(buf)), 0, uintptr(unsafe.Pointer(&buf[0])), 0, 0)
	return string(buf)
}
func (c *Functions) GetString(pname gl.Enum) string {
	s, _, _ := syscall.Syscall(_glGetString.Addr(), 1, uintptr(pname), 0, 0)
	return gunsafe.GoString(gunsafe.SliceOf(s))
}
func (c *Functions) GetUniformLocation(p gl.Program, name string) gl.Uniform {
	cname := cString(name)
	c0 := &cname[0]
	u, _, _ := syscall.Syscall(_glGetUniformLocation.Addr(), 2, uintptr(p.V), uintptr(unsafe.Pointer(c0)), 0)
	issue34474KeepAlive(c0)
	return gl.Uniform{int(u)}
}
func (c *Functions) InvalidateFramebuffer(target, attachment gl.Enum) {
	addr := _glInvalidateFramebuffer.Addr()
	if addr == 0 {
		// InvalidateFramebuffer is just a hint. Skip it if not supported.
		return
	}
	syscall.Syscall(addr, 3, uintptr(target), 1, uintptr(unsafe.Pointer(&attachment)))
}
func (c *Functions) LinkProgram(p gl.Program) {
	syscall.Syscall(_glLinkProgram.Addr(), 1, uintptr(p.V), 0, 0)
}
func (c *Functions) PixelStorei(pname gl.Enum, param int32) {
	syscall.Syscall(_glPixelStorei.Addr(), 2, uintptr(pname), uintptr(param), 0)
}
func (f *Functions) ReadPixels(x, y, width, height int, format, ty gl.Enum, data []byte) {
	d0 := &data[0]
	syscall.Syscall9(_glReadPixels.Addr(), 7, uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(format), uintptr(ty), uintptr(unsafe.Pointer(d0)), 0, 0)
	issue34474KeepAlive(d0)
}
func (c *Functions) RenderbufferStorage(target, internalformat gl.Enum, width, height int) {
	syscall.Syscall6(_glRenderbufferStorage.Addr(), 4, uintptr(target), uintptr(internalformat), uintptr(width), uintptr(height), 0, 0)
}
func (c *Functions) Scissor(x, y, width, height int32) {
	syscall.Syscall6(_glScissor.Addr(), 4, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0)
}
func (c *Functions) ShaderSource(s gl.Shader, src string) {
	var n uintptr = uintptr(len(src))
	psrc := &src
	syscall.Syscall6(_glShaderSource.Addr(), 4, uintptr(s.V), 1, uintptr(unsafe.Pointer(psrc)), uintptr(unsafe.Pointer(&n)), 0, 0)
	issue34474KeepAlive(psrc)
}
func (c *Functions) TexImage2D(target gl.Enum, level int, internalFormat int, width, height int, format, ty gl.Enum, data []byte) {
	if len(data) == 0 {
		syscall.Syscall9(_glTexImage2D.Addr(), 9, uintptr(target), uintptr(level), uintptr(internalFormat), uintptr(width), uintptr(height), 0, uintptr(format), uintptr(ty), 0)
	} else {
		d0 := &data[0]
		syscall.Syscall9(_glTexImage2D.Addr(), 9, uintptr(target), uintptr(level), uintptr(internalFormat), uintptr(width), uintptr(height), 0, uintptr(format), uintptr(ty), uintptr(unsafe.Pointer(d0)))
		issue34474KeepAlive(d0)
	}
}
func (c *Functions) TexSubImage2D(target gl.Enum, level int, x, y, width, height int, format, ty gl.Enum, data []byte) {
	d0 := &data[0]
	syscall.Syscall9(_glTexSubImage2D.Addr(), 9, uintptr(target), uintptr(level), uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(format), uintptr(ty), uintptr(unsafe.Pointer(d0)))
	issue34474KeepAlive(d0)
}
func (c *Functions) TexParameteri(target, pname gl.Enum, param int) {
	syscall.Syscall(_glTexParameteri.Addr(), 3, uintptr(target), uintptr(pname), uintptr(param))
}
func (f *Functions) UniformBlockBinding(p gl.Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	syscall.Syscall(_glUniformBlockBinding.Addr(), 3, uintptr(p.V), uintptr(uniformBlockIndex), uintptr(uniformBlockBinding))
}
func (c *Functions) Uniform1f(dst gl.Uniform, v float32) {
	syscall.Syscall(_glUniform1f.Addr(), 2, uintptr(dst.V), uintptr(math.Float32bits(v)), 0)
}
func (c *Functions) Uniform1i(dst gl.Uniform, v int) {
	syscall.Syscall(_glUniform1i.Addr(), 2, uintptr(dst.V), uintptr(v), 0)
}
func (c *Functions) Uniform2f(dst gl.Uniform, v0, v1 float32) {
	syscall.Syscall(_glUniform2f.Addr(), 3, uintptr(dst.V), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)))
}
func (c *Functions) Uniform3f(dst gl.Uniform, v0, v1, v2 float32) {
	syscall.Syscall6(_glUniform3f.Addr(), 4, uintptr(dst.V), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)), uintptr(math.Float32bits(v2)), 0, 0)
}
func (c *Functions) Uniform4f(dst gl.Uniform, v0, v1, v2, v3 float32) {
	syscall.Syscall6(_glUniform4f.Addr(), 5, uintptr(dst.V), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)), uintptr(math.Float32bits(v2)), uintptr(math.Float32bits(v3)), 0)
}
func (c *Functions) UseProgram(p gl.Program) {
	syscall.Syscall(_glUseProgram.Addr(), 1, uintptr(p.V), 0, 0)
}
func (c *Functions) VertexAttribPointer(dst gl.Attrib, size int, ty gl.Enum, normalized bool, stride, offset int) {
	var norm uintptr
	if normalized {
		norm = 1
	}
	syscall.Syscall6(_glVertexAttribPointer.Addr(), 6, uintptr(dst), uintptr(size), uintptr(ty), norm, uintptr(stride), uintptr(offset))
}
func (c *Functions) Viewport(x, y, width, height int) {
	syscall.Syscall6(_glViewport.Addr(), 4, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0)
}

func cString(s string) []byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return b
}

// issue34474KeepAlive calls runtime.KeepAlive as a
// workaround for golang.org/issue/34474.
func issue34474KeepAlive(v interface{}) {
	runtime.KeepAlive(v)
}
