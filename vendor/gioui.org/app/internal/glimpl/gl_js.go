// SPDX-License-Identifier: Unlicense OR MIT

package glimpl

import (
	"errors"
	"strings"
	"syscall/js"

	"gioui.org/gpu/gl"
)

type Functions struct {
	Ctx                             js.Value
	EXT_disjoint_timer_query        js.Value
	EXT_disjoint_timer_query_webgl2 js.Value

	// Cached JS arrays.
	byteBuf  js.Value
	int32Buf js.Value
}

func (f *Functions) Init(version int) error {
	if version < 2 {
		f.EXT_disjoint_timer_query = f.getExtension("EXT_disjoint_timer_query")
		if f.getExtension("OES_texture_half_float").IsNull() && f.getExtension("OES_texture_float").IsNull() {
			return errors.New("gl: no support for neither OES_texture_half_float nor OES_texture_float")
		}
		if f.getExtension("EXT_sRGB").IsNull() {
			return errors.New("gl: EXT_sRGB not supported")
		}
	} else {
		// WebGL2 extensions.
		f.EXT_disjoint_timer_query_webgl2 = f.getExtension("EXT_disjoint_timer_query_webgl2")
		if f.getExtension("EXT_color_buffer_half_float").IsNull() && f.getExtension("EXT_color_buffer_float").IsNull() {
			return errors.New("gl: no support for neither EXT_color_buffer_half_float nor EXT_color_buffer_float")
		}
	}
	return nil
}

func (f *Functions) getExtension(name string) js.Value {
	return f.Ctx.Call("getExtension", name)
}

func (f *Functions) ActiveTexture(t gl.Enum) {
	f.Ctx.Call("activeTexture", int(t))
}
func (f *Functions) AttachShader(p gl.Program, s gl.Shader) {
	f.Ctx.Call("attachShader", js.Value(p), js.Value(s))
}
func (f *Functions) BeginQuery(target gl.Enum, query gl.Query) {
	if !f.EXT_disjoint_timer_query_webgl2.IsNull() {
		f.Ctx.Call("beginQuery", int(target), js.Value(query))
	} else {
		f.EXT_disjoint_timer_query.Call("beginQueryEXT", int(target), js.Value(query))
	}
}
func (f *Functions) BindAttribLocation(p gl.Program, a gl.Attrib, name string) {
	f.Ctx.Call("bindAttribLocation", js.Value(p), int(a), name)
}
func (f *Functions) BindBuffer(target gl.Enum, b gl.Buffer) {
	f.Ctx.Call("bindBuffer", int(target), js.Value(b))
}
func (f *Functions) BindBufferBase(target gl.Enum, index int, b gl.Buffer) {
	f.Ctx.Call("bindBufferBase", int(target), index, js.Value(b))
}
func (f *Functions) BindFramebuffer(target gl.Enum, fb gl.Framebuffer) {
	f.Ctx.Call("bindFramebuffer", int(target), js.Value(fb))
}
func (f *Functions) BindRenderbuffer(target gl.Enum, rb gl.Renderbuffer) {
	f.Ctx.Call("bindRenderbuffer", int(target), js.Value(rb))
}
func (f *Functions) BindTexture(target gl.Enum, t gl.Texture) {
	f.Ctx.Call("bindTexture", int(target), js.Value(t))
}
func (f *Functions) BlendEquation(mode gl.Enum) {
	f.Ctx.Call("blendEquation", int(mode))
}
func (f *Functions) BlendFunc(sfactor, dfactor gl.Enum) {
	f.Ctx.Call("blendFunc", int(sfactor), int(dfactor))
}
func (f *Functions) BufferData(target gl.Enum, src []byte, usage gl.Enum) {
	f.Ctx.Call("bufferData", int(target), f.byteArrayOf(src), int(usage))
}
func (f *Functions) CheckFramebufferStatus(target gl.Enum) gl.Enum {
	return gl.Enum(f.Ctx.Call("checkFramebufferStatus", int(target)).Int())
}
func (f *Functions) Clear(mask gl.Enum) {
	f.Ctx.Call("clear", int(mask))
}
func (f *Functions) ClearColor(red, green, blue, alpha float32) {
	f.Ctx.Call("clearColor", red, green, blue, alpha)
}
func (f *Functions) ClearDepthf(d float32) {
	f.Ctx.Call("clearDepth", d)
}
func (f *Functions) CompileShader(s gl.Shader) {
	f.Ctx.Call("compileShader", js.Value(s))
}
func (f *Functions) CreateBuffer() gl.Buffer {
	return gl.Buffer(f.Ctx.Call("createBuffer"))
}
func (f *Functions) CreateFramebuffer() gl.Framebuffer {
	return gl.Framebuffer(f.Ctx.Call("createFramebuffer"))
}
func (f *Functions) CreateProgram() gl.Program {
	return gl.Program(f.Ctx.Call("createProgram"))
}
func (f *Functions) CreateQuery() gl.Query {
	return gl.Query(f.Ctx.Call("createQuery"))
}
func (f *Functions) CreateRenderbuffer() gl.Renderbuffer {
	return gl.Renderbuffer(f.Ctx.Call("createRenderbuffer"))
}
func (f *Functions) CreateShader(ty gl.Enum) gl.Shader {
	return gl.Shader(f.Ctx.Call("createShader", int(ty)))
}
func (f *Functions) CreateTexture() gl.Texture {
	return gl.Texture(f.Ctx.Call("createTexture"))
}
func (f *Functions) DeleteBuffer(v gl.Buffer) {
	f.Ctx.Call("deleteBuffer", js.Value(v))
}
func (f *Functions) DeleteFramebuffer(v gl.Framebuffer) {
	f.Ctx.Call("deleteFramebuffer", js.Value(v))
}
func (f *Functions) DeleteProgram(p gl.Program) {
	f.Ctx.Call("deleteProgram", js.Value(p))
}
func (f *Functions) DeleteQuery(query gl.Query) {
	if !f.EXT_disjoint_timer_query_webgl2.IsNull() {
		f.Ctx.Call("deleteQuery", js.Value(query))
	} else {
		f.EXT_disjoint_timer_query.Call("deleteQueryEXT", js.Value(query))
	}
}
func (f *Functions) DeleteShader(s gl.Shader) {
	f.Ctx.Call("deleteShader", js.Value(s))
}
func (f *Functions) DeleteRenderbuffer(v gl.Renderbuffer) {
	f.Ctx.Call("deleteRenderbuffer", js.Value(v))
}
func (f *Functions) DeleteTexture(v gl.Texture) {
	f.Ctx.Call("deleteTexture", js.Value(v))
}
func (f *Functions) DepthFunc(fn gl.Enum) {
	f.Ctx.Call("depthFunc", int(fn))
}
func (f *Functions) DepthMask(mask bool) {
	f.Ctx.Call("depthMask", mask)
}
func (f *Functions) DisableVertexAttribArray(a gl.Attrib) {
	f.Ctx.Call("disableVertexAttribArray", int(a))
}
func (f *Functions) Disable(cap gl.Enum) {
	f.Ctx.Call("disable", int(cap))
}
func (f *Functions) DrawArrays(mode gl.Enum, first, count int) {
	f.Ctx.Call("drawArrays", int(mode), first, count)
}
func (f *Functions) DrawElements(mode gl.Enum, count int, ty gl.Enum, offset int) {
	f.Ctx.Call("drawElements", int(mode), count, int(ty), offset)
}
func (f *Functions) Enable(cap gl.Enum) {
	f.Ctx.Call("enable", int(cap))
}
func (f *Functions) EnableVertexAttribArray(a gl.Attrib) {
	f.Ctx.Call("enableVertexAttribArray", int(a))
}
func (f *Functions) EndQuery(target gl.Enum) {
	if !f.EXT_disjoint_timer_query_webgl2.IsNull() {
		f.Ctx.Call("endQuery", int(target))
	} else {
		f.EXT_disjoint_timer_query.Call("endQueryEXT", int(target))
	}
}
func (f *Functions) Finish() {
	f.Ctx.Call("finish")
}
func (f *Functions) FramebufferRenderbuffer(target, attachment, renderbuffertarget gl.Enum, renderbuffer gl.Renderbuffer) {
	f.Ctx.Call("framebufferRenderbuffer", int(target), int(attachment), int(renderbuffertarget), js.Value(renderbuffer))
}
func (f *Functions) FramebufferTexture2D(target, attachment, texTarget gl.Enum, t gl.Texture, level int) {
	f.Ctx.Call("framebufferTexture2D", int(target), int(attachment), int(texTarget), js.Value(t), level)
}
func (f *Functions) GetError() gl.Enum {
	return gl.Enum(f.Ctx.Call("getError").Int())
}
func (f *Functions) GetRenderbufferParameteri(target, pname gl.Enum) int {
	return paramVal(f.Ctx.Call("getRenderbufferParameteri", int(pname)))
}
func (f *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname gl.Enum) int {
	return paramVal(f.Ctx.Call("getFramebufferAttachmentParameter", int(target), int(attachment), int(pname)))
}
func (f *Functions) GetBinding(pname gl.Enum) gl.Object {
	return gl.Object(f.Ctx.Call("getParameter", int(pname)))
}
func (f *Functions) GetInteger(pname gl.Enum) int {
	return paramVal(f.Ctx.Call("getParameter", int(pname)))
}
func (f *Functions) GetProgrami(p gl.Program, pname gl.Enum) int {
	return paramVal(f.Ctx.Call("getProgramParameter", js.Value(p), int(pname)))
}
func (f *Functions) GetProgramInfoLog(p gl.Program) string {
	return f.Ctx.Call("getProgramInfoLog", js.Value(p)).String()
}
func (f *Functions) GetQueryObjectuiv(query gl.Query, pname gl.Enum) uint {
	if !f.EXT_disjoint_timer_query_webgl2.IsNull() {
		return uint(paramVal(f.Ctx.Call("getQueryParameter", js.Value(query), int(pname))))
	} else {
		return uint(paramVal(f.EXT_disjoint_timer_query.Call("getQueryObjectEXT", js.Value(query), int(pname))))
	}
}
func (f *Functions) GetShaderi(s gl.Shader, pname gl.Enum) int {
	return paramVal(f.Ctx.Call("getShaderParameter", js.Value(s), int(pname)))
}
func (f *Functions) GetShaderInfoLog(s gl.Shader) string {
	return f.Ctx.Call("getShaderInfoLog", js.Value(s)).String()
}
func (f *Functions) GetString(pname gl.Enum) string {
	switch pname {
	case gl.EXTENSIONS:
		extsjs := f.Ctx.Call("getSupportedExtensions")
		var exts []string
		for i := 0; i < extsjs.Length(); i++ {
			exts = append(exts, "GL_"+extsjs.Index(i).String())
		}
		return strings.Join(exts, " ")
	default:
		return f.Ctx.Call("getParameter", int(pname)).String()
	}
}
func (f *Functions) GetUniformBlockIndex(p gl.Program, name string) uint {
	return uint(paramVal(f.Ctx.Call("getUniformBlockIndex", js.Value(p), name)))
}
func (f *Functions) GetUniformLocation(p gl.Program, name string) gl.Uniform {
	return gl.Uniform(f.Ctx.Call("getUniformLocation", js.Value(p), name))
}
func (f *Functions) InvalidateFramebuffer(target, attachment gl.Enum) {
	fn := f.Ctx.Get("invalidateFramebuffer")
	if !fn.IsUndefined() {
		if f.int32Buf.IsUndefined() {
			f.int32Buf = js.Global().Get("Int32Array").New(1)
		}
		f.int32Buf.SetIndex(0, int32(attachment))
		f.Ctx.Call("invalidateFramebuffer", int(target), f.int32Buf)
	}
}
func (f *Functions) LinkProgram(p gl.Program) {
	f.Ctx.Call("linkProgram", js.Value(p))
}
func (f *Functions) PixelStorei(pname gl.Enum, param int32) {
	f.Ctx.Call("pixelStorei", int(pname), param)
}
func (f *Functions) RenderbufferStorage(target, internalformat gl.Enum, width, height int) {
	f.Ctx.Call("renderbufferStorage", int(target), int(internalformat), width, height)
}
func (f *Functions) ReadPixels(x, y, width, height int, format, ty gl.Enum, data []byte) {
	f.resizeByteBuffer(len(data))
	f.Ctx.Call("readPixels", x, y, width, height, int(format), int(ty), f.byteBuf)
	js.CopyBytesToGo(data, f.byteBuf)
}
func (f *Functions) Scissor(x, y, width, height int32) {
	f.Ctx.Call("scissor", x, y, width, height)
}
func (f *Functions) ShaderSource(s gl.Shader, src string) {
	f.Ctx.Call("shaderSource", js.Value(s), src)
}
func (f *Functions) TexImage2D(target gl.Enum, level int, internalFormat int, width, height int, format, ty gl.Enum, data []byte) {
	f.Ctx.Call("texImage2D", int(target), int(level), int(internalFormat), int(width), int(height), 0, int(format), int(ty), f.byteArrayOf(data))
}
func (f *Functions) TexSubImage2D(target gl.Enum, level int, x, y, width, height int, format, ty gl.Enum, data []byte) {
	f.Ctx.Call("texSubImage2D", int(target), level, x, y, width, height, int(format), int(ty), f.byteArrayOf(data))
}
func (f *Functions) TexParameteri(target, pname gl.Enum, param int) {
	f.Ctx.Call("texParameteri", int(target), int(pname), int(param))
}
func (f *Functions) UniformBlockBinding(p gl.Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	f.Ctx.Call("uniformBlockBinding", js.Value(p), int(uniformBlockIndex), int(uniformBlockBinding))
}
func (f *Functions) Uniform1f(dst gl.Uniform, v float32) {
	f.Ctx.Call("uniform1f", js.Value(dst), v)
}
func (f *Functions) Uniform1i(dst gl.Uniform, v int) {
	f.Ctx.Call("uniform1i", js.Value(dst), v)
}
func (f *Functions) Uniform2f(dst gl.Uniform, v0, v1 float32) {
	f.Ctx.Call("uniform2f", js.Value(dst), v0, v1)
}
func (f *Functions) Uniform3f(dst gl.Uniform, v0, v1, v2 float32) {
	f.Ctx.Call("uniform3f", js.Value(dst), v0, v1, v2)
}
func (f *Functions) Uniform4f(dst gl.Uniform, v0, v1, v2, v3 float32) {
	f.Ctx.Call("uniform4f", js.Value(dst), v0, v1, v2, v3)
}
func (f *Functions) UseProgram(p gl.Program) {
	f.Ctx.Call("useProgram", js.Value(p))
}
func (f *Functions) VertexAttribPointer(dst gl.Attrib, size int, ty gl.Enum, normalized bool, stride, offset int) {
	f.Ctx.Call("vertexAttribPointer", int(dst), size, int(ty), normalized, stride, offset)
}
func (f *Functions) Viewport(x, y, width, height int) {
	f.Ctx.Call("viewport", x, y, width, height)
}

func (f *Functions) byteArrayOf(data []byte) js.Value {
	if len(data) == 0 {
		return js.Null()
	}
	f.resizeByteBuffer(len(data))
	js.CopyBytesToJS(f.byteBuf, data)
	return f.byteBuf
}

func (f *Functions) resizeByteBuffer(n int) {
	if n == 0 {
		return
	}
	if !f.byteBuf.IsUndefined() && f.byteBuf.Length() >= n {
		return
	}
	f.byteBuf = js.Global().Get("Uint8Array").New(n)
}

func paramVal(v js.Value) int {
	switch v.Type() {
	case js.TypeBoolean:
		if b := v.Bool(); b {
			return 1
		} else {
			return 0
		}
	case js.TypeNumber:
		return v.Int()
	default:
		panic("unknown parameter type")
	}
}
