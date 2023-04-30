// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"time"
	"unsafe"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/gl"
)

// Backend implements driver.Device.
type Backend struct {
	funcs *gl.Functions

	state glstate

	glver [2]int
	gles  bool
	ubo   bool
	feats driver.Caps
	// floatTriple holds the settings for floating point
	// textures.
	floatTriple textureTriple
	// Single channel alpha textures.
	alphaTriple textureTriple
	srgbaTriple textureTriple
}

// State tracking.
type glstate struct {
	// nattr is the current number of enabled vertex arrays.
	nattr    int
	prog     *gpuProgram
	texUnits [4]*gpuTexture
	layout   *gpuInputLayout
	buffer   bufferBinding
}

type bufferBinding struct {
	buf    *gpuBuffer
	offset int
	stride int
}

type gpuTimer struct {
	funcs *gl.Functions
	obj   gl.Query
}

type gpuTexture struct {
	backend *Backend
	obj     gl.Texture
	triple  textureTriple
	width   int
	height  int
}

type gpuFramebuffer struct {
	backend  *Backend
	obj      gl.Framebuffer
	hasDepth bool
	depthBuf gl.Renderbuffer
	foreign  bool
}

type gpuBuffer struct {
	backend   *Backend
	hasBuffer bool
	obj       gl.Buffer
	typ       driver.BufferBinding
	size      int
	immutable bool
	version   int
	// For emulation of uniform buffers.
	data []byte
}

type gpuProgram struct {
	backend      *Backend
	obj          gl.Program
	nattr        int
	vertUniforms uniformsTracker
	fragUniforms uniformsTracker
	storage      [storageBindings]*gpuBuffer
}

type uniformsTracker struct {
	locs    []uniformLocation
	size    int
	buf     *gpuBuffer
	version int
}

type uniformLocation struct {
	uniform gl.Uniform
	offset  int
	typ     driver.DataType
	size    int
}

type gpuInputLayout struct {
	inputs []driver.InputLocation
	layout []driver.InputDesc
}

// textureTriple holds the type settings for
// a TexImage2D call.
type textureTriple struct {
	internalFormat gl.Enum
	format         gl.Enum
	typ            gl.Enum
}

type Context = gl.Context

const (
	storageBindings = 32
)

func init() {
	driver.NewOpenGLDevice = newOpenGLDevice
}

func newOpenGLDevice(api driver.OpenGL) (driver.Device, error) {
	f, err := gl.NewFunctions(api.Context)
	if err != nil {
		return nil, err
	}
	exts := strings.Split(f.GetString(gl.EXTENSIONS), " ")
	glVer := f.GetString(gl.VERSION)
	ver, gles, err := gl.ParseGLVersion(glVer)
	if err != nil {
		return nil, err
	}
	floatTriple, ffboErr := floatTripleFor(f, ver, exts)
	srgbaTriple, err := srgbaTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	gles30 := gles && ver[0] >= 3
	gles31 := gles && (ver[0] > 3 || (ver[0] == 3 && ver[1] >= 1))
	b := &Backend{
		glver:       ver,
		gles:        gles,
		ubo:         gles30,
		funcs:       f,
		floatTriple: floatTriple,
		alphaTriple: alphaTripleFor(ver),
		srgbaTriple: srgbaTriple,
	}
	b.feats.BottomLeftOrigin = true
	if ffboErr == nil {
		b.feats.Features |= driver.FeatureFloatRenderTargets
	}
	if gles31 {
		b.feats.Features |= driver.FeatureCompute
	}
	if hasExtension(exts, "GL_EXT_disjoint_timer_query_webgl2") || hasExtension(exts, "GL_EXT_disjoint_timer_query") {
		b.feats.Features |= driver.FeatureTimers
	}
	b.feats.MaxTextureSize = f.GetInteger(gl.MAX_TEXTURE_SIZE)
	return b, nil
}

func (b *Backend) BeginFrame() driver.Framebuffer {
	// Assume GL state is reset between frames.
	b.state = glstate{}
	fboID := gl.Framebuffer(b.funcs.GetBinding(gl.FRAMEBUFFER_BINDING))
	return &gpuFramebuffer{backend: b, obj: fboID, foreign: true}
}

func (b *Backend) EndFrame() {
	b.funcs.ActiveTexture(gl.TEXTURE0)
}

func (b *Backend) Caps() driver.Caps {
	return b.feats
}

func (b *Backend) NewTimer() driver.Timer {
	return &gpuTimer{
		funcs: b.funcs,
		obj:   b.funcs.CreateQuery(),
	}
}

func (b *Backend) IsTimeContinuous() bool {
	return b.funcs.GetInteger(gl.GPU_DISJOINT_EXT) == gl.FALSE
}

func (b *Backend) NewFramebuffer(tex driver.Texture, depthBits int) (driver.Framebuffer, error) {
	glErr(b.funcs)
	gltex := tex.(*gpuTexture)
	fb := b.funcs.CreateFramebuffer()
	fbo := &gpuFramebuffer{backend: b, obj: fb}
	b.BindFramebuffer(fbo)
	if err := glErr(b.funcs); err != nil {
		fbo.Release()
		return nil, err
	}
	b.funcs.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, gltex.obj, 0)
	if depthBits > 0 {
		size := gl.Enum(gl.DEPTH_COMPONENT16)
		switch {
		case depthBits > 24:
			size = gl.DEPTH_COMPONENT32F
		case depthBits > 16:
			size = gl.DEPTH_COMPONENT24
		}
		depthBuf := b.funcs.CreateRenderbuffer()
		b.funcs.BindRenderbuffer(gl.RENDERBUFFER, depthBuf)
		b.funcs.RenderbufferStorage(gl.RENDERBUFFER, size, gltex.width, gltex.height)
		b.funcs.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depthBuf)
		fbo.depthBuf = depthBuf
		fbo.hasDepth = true
		if err := glErr(b.funcs); err != nil {
			fbo.Release()
			return nil, err
		}
	}
	if st := b.funcs.CheckFramebufferStatus(gl.FRAMEBUFFER); st != gl.FRAMEBUFFER_COMPLETE {
		fbo.Release()
		return nil, fmt.Errorf("incomplete framebuffer, status = 0x%x, err = %d", st, b.funcs.GetError())
	}
	return fbo, nil
}

func (b *Backend) NewTexture(format driver.TextureFormat, width, height int, minFilter, magFilter driver.TextureFilter, binding driver.BufferBinding) (driver.Texture, error) {
	glErr(b.funcs)
	tex := &gpuTexture{backend: b, obj: b.funcs.CreateTexture(), width: width, height: height}
	switch format {
	case driver.TextureFormatFloat:
		tex.triple = b.floatTriple
	case driver.TextureFormatSRGB:
		tex.triple = b.srgbaTriple
	case driver.TextureFormatRGBA8:
		tex.triple = textureTriple{gl.RGBA8, gl.RGBA, gl.UNSIGNED_BYTE}
	default:
		return nil, errors.New("unsupported texture format")
	}
	b.BindTexture(0, tex)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, toTexFilter(magFilter))
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, toTexFilter(minFilter))
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	if b.gles && b.glver[0] >= 3 {
		// Immutable textures are required for BindImageTexture, and can't hurt otherwise.
		b.funcs.TexStorage2D(gl.TEXTURE_2D, 1, tex.triple.internalFormat, width, height)
	} else {
		b.funcs.TexImage2D(gl.TEXTURE_2D, 0, tex.triple.internalFormat, width, height, tex.triple.format, tex.triple.typ)
	}
	if err := glErr(b.funcs); err != nil {
		tex.Release()
		return nil, err
	}
	return tex, nil
}

func (b *Backend) NewBuffer(typ driver.BufferBinding, size int) (driver.Buffer, error) {
	glErr(b.funcs)
	buf := &gpuBuffer{backend: b, typ: typ, size: size}
	if typ&driver.BufferBindingUniforms != 0 {
		if typ != driver.BufferBindingUniforms {
			return nil, errors.New("uniforms buffers cannot be bound as anything else")
		}
		if !b.ubo {
			// GLES 2 doesn't support uniform buffers.
			buf.data = make([]byte, size)
		}
	}
	if typ&^driver.BufferBindingUniforms != 0 || b.ubo {
		buf.hasBuffer = true
		buf.obj = b.funcs.CreateBuffer()
		if err := glErr(b.funcs); err != nil {
			buf.Release()
			return nil, err
		}
		firstBinding := firstBufferType(typ)
		b.funcs.BindBuffer(firstBinding, buf.obj)
		b.funcs.BufferData(firstBinding, size, gl.DYNAMIC_DRAW)
	}
	return buf, nil
}

func (b *Backend) NewImmutableBuffer(typ driver.BufferBinding, data []byte) (driver.Buffer, error) {
	glErr(b.funcs)
	obj := b.funcs.CreateBuffer()
	buf := &gpuBuffer{backend: b, obj: obj, typ: typ, size: len(data), hasBuffer: true}
	firstBinding := firstBufferType(typ)
	b.funcs.BindBuffer(firstBinding, buf.obj)
	b.funcs.BufferData(firstBinding, len(data), gl.STATIC_DRAW)
	buf.Upload(data)
	buf.immutable = true
	if err := glErr(b.funcs); err != nil {
		buf.Release()
		return nil, err
	}
	return buf, nil
}

func glErr(f *gl.Functions) error {
	if st := f.GetError(); st != gl.NO_ERROR {
		return fmt.Errorf("glGetError: %#x", st)
	}
	return nil
}

func (b *Backend) Release() {
}

func (b *Backend) MemoryBarrier() {
	b.funcs.MemoryBarrier(gl.ALL_BARRIER_BITS)
}

func (b *Backend) DispatchCompute(x, y, z int) {
	if p := b.state.prog; p != nil {
		for binding, buf := range p.storage {
			if buf != nil {
				b.funcs.BindBufferBase(gl.SHADER_STORAGE_BUFFER, binding, buf.obj)
			}
		}
	}
	b.funcs.DispatchCompute(x, y, z)
}

func (b *Backend) BindImageTexture(unit int, tex driver.Texture, access driver.AccessBits, f driver.TextureFormat) {
	t := tex.(*gpuTexture)
	var acc gl.Enum
	switch access {
	case driver.AccessWrite:
		acc = gl.WRITE_ONLY
	case driver.AccessRead:
		acc = gl.READ_ONLY
	default:
		panic("unsupported access bits")
	}
	var format gl.Enum
	switch f {
	case driver.TextureFormatRGBA8:
		format = gl.RGBA8
	default:
		panic("unsupported format")
	}
	b.funcs.BindImageTexture(unit, t.obj, 0, false, 0, acc, format)
}

func (b *Backend) bindTexture(unit int, t *gpuTexture) {
	if b.state.texUnits[unit] != t {
		b.funcs.ActiveTexture(gl.TEXTURE0 + gl.Enum(unit))
		b.funcs.BindTexture(gl.TEXTURE_2D, t.obj)
		b.state.texUnits[unit] = t
	}
}

func (b *Backend) useProgram(p *gpuProgram) {
	if b.state.prog != p {
		p.backend.funcs.UseProgram(p.obj)
		b.state.prog = p
	}
}

func (b *Backend) enableVertexArrays(n int) {
	// Enable needed arrays.
	for i := b.state.nattr; i < n; i++ {
		b.funcs.EnableVertexAttribArray(gl.Attrib(i))
	}
	// Disable extra arrays.
	for i := n; i < b.state.nattr; i++ {
		b.funcs.DisableVertexAttribArray(gl.Attrib(i))
	}
	b.state.nattr = n
}

func (b *Backend) SetDepthTest(enable bool) {
	if enable {
		b.funcs.Enable(gl.DEPTH_TEST)
	} else {
		b.funcs.Disable(gl.DEPTH_TEST)
	}
}

func (b *Backend) BlendFunc(sfactor, dfactor driver.BlendFactor) {
	b.funcs.BlendFunc(toGLBlendFactor(sfactor), toGLBlendFactor(dfactor))
}

func toGLBlendFactor(f driver.BlendFactor) gl.Enum {
	switch f {
	case driver.BlendFactorOne:
		return gl.ONE
	case driver.BlendFactorOneMinusSrcAlpha:
		return gl.ONE_MINUS_SRC_ALPHA
	case driver.BlendFactorZero:
		return gl.ZERO
	case driver.BlendFactorDstColor:
		return gl.DST_COLOR
	default:
		panic("unsupported blend factor")
	}
}

func (b *Backend) DepthMask(mask bool) {
	b.funcs.DepthMask(mask)
}

func (b *Backend) SetBlend(enable bool) {
	if enable {
		b.funcs.Enable(gl.BLEND)
	} else {
		b.funcs.Disable(gl.BLEND)
	}
}

func (b *Backend) DrawElements(mode driver.DrawMode, off, count int) {
	b.prepareDraw()
	// off is in 16-bit indices, but DrawElements take a byte offset.
	byteOff := off * 2
	b.funcs.DrawElements(toGLDrawMode(mode), count, gl.UNSIGNED_SHORT, byteOff)
}

func (b *Backend) DrawArrays(mode driver.DrawMode, off, count int) {
	b.prepareDraw()
	b.funcs.DrawArrays(toGLDrawMode(mode), off, count)
}

func (b *Backend) prepareDraw() {
	nattr := b.state.prog.nattr
	b.enableVertexArrays(nattr)
	if nattr > 0 {
		b.setupVertexArrays()
	}
	if p := b.state.prog; p != nil {
		p.updateUniforms()
	}
}

func toGLDrawMode(mode driver.DrawMode) gl.Enum {
	switch mode {
	case driver.DrawModeTriangleStrip:
		return gl.TRIANGLE_STRIP
	case driver.DrawModeTriangles:
		return gl.TRIANGLES
	default:
		panic("unsupported draw mode")
	}
}

func (b *Backend) Viewport(x, y, width, height int) {
	b.funcs.Viewport(x, y, width, height)
}

func (b *Backend) Clear(colR, colG, colB, colA float32) {
	b.funcs.ClearColor(colR, colG, colB, colA)
	b.funcs.Clear(gl.COLOR_BUFFER_BIT)
}

func (b *Backend) ClearDepth(d float32) {
	b.funcs.ClearDepthf(d)
	b.funcs.Clear(gl.DEPTH_BUFFER_BIT)
}

func (b *Backend) DepthFunc(f driver.DepthFunc) {
	var glfunc gl.Enum
	switch f {
	case driver.DepthFuncGreater:
		glfunc = gl.GREATER
	case driver.DepthFuncGreaterEqual:
		glfunc = gl.GEQUAL
	default:
		panic("unsupported depth func")
	}
	b.funcs.DepthFunc(glfunc)
}

func (b *Backend) NewInputLayout(vs driver.ShaderSources, layout []driver.InputDesc) (driver.InputLayout, error) {
	if len(vs.Inputs) != len(layout) {
		return nil, fmt.Errorf("NewInputLayout: got %d inputs, expected %d", len(layout), len(vs.Inputs))
	}
	for i, inp := range vs.Inputs {
		if exp, got := inp.Size, layout[i].Size; exp != got {
			return nil, fmt.Errorf("NewInputLayout: data size mismatch for %q: got %d expected %d", inp.Name, got, exp)
		}
	}
	return &gpuInputLayout{
		inputs: vs.Inputs,
		layout: layout,
	}, nil
}

func (b *Backend) NewComputeProgram(src driver.ShaderSources) (driver.Program, error) {
	p, err := gl.CreateComputeProgram(b.funcs, src.GLSL310ES)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", src.Name, err)
	}
	gpuProg := &gpuProgram{
		backend: b,
		obj:     p,
	}
	return gpuProg, nil
}

func (b *Backend) NewProgram(vertShader, fragShader driver.ShaderSources) (driver.Program, error) {
	attr := make([]string, len(vertShader.Inputs))
	for _, inp := range vertShader.Inputs {
		attr[inp.Location] = inp.Name
	}
	vsrc, fsrc := vertShader.GLSL100ES, fragShader.GLSL100ES
	if b.glver[0] >= 3 {
		// OpenGL (ES) 3.0.
		switch {
		case b.gles:
			vsrc, fsrc = vertShader.GLSL300ES, fragShader.GLSL300ES
		case b.glver[0] >= 4 || b.glver[1] >= 2:
			// OpenGL 3.2 Core only accepts glsl 1.50 or newer.
			vsrc, fsrc = vertShader.GLSL150, fragShader.GLSL150
		default:
			vsrc, fsrc = vertShader.GLSL130, fragShader.GLSL130
		}
	}
	p, err := gl.CreateProgram(b.funcs, vsrc, fsrc, attr)
	if err != nil {
		return nil, err
	}
	gpuProg := &gpuProgram{
		backend: b,
		obj:     p,
		nattr:   len(attr),
	}
	b.BindProgram(gpuProg)
	// Bind texture uniforms.
	for _, tex := range vertShader.Textures {
		u := b.funcs.GetUniformLocation(p, tex.Name)
		if u.Valid() {
			b.funcs.Uniform1i(u, tex.Binding)
		}
	}
	for _, tex := range fragShader.Textures {
		u := b.funcs.GetUniformLocation(p, tex.Name)
		if u.Valid() {
			b.funcs.Uniform1i(u, tex.Binding)
		}
	}
	if b.ubo {
		for _, block := range vertShader.Uniforms.Blocks {
			blockIdx := b.funcs.GetUniformBlockIndex(p, block.Name)
			if blockIdx != gl.INVALID_INDEX {
				b.funcs.UniformBlockBinding(p, blockIdx, uint(block.Binding))
			}
		}
		// To match Direct3D 11 with separate vertex and fragment
		// shader uniform buffers, offset all fragment blocks to be
		// located after the vertex blocks.
		off := len(vertShader.Uniforms.Blocks)
		for _, block := range fragShader.Uniforms.Blocks {
			blockIdx := b.funcs.GetUniformBlockIndex(p, block.Name)
			if blockIdx != gl.INVALID_INDEX {
				b.funcs.UniformBlockBinding(p, blockIdx, uint(block.Binding+off))
			}
		}
	} else {
		gpuProg.vertUniforms.setup(b.funcs, p, vertShader.Uniforms.Size, vertShader.Uniforms.Locations)
		gpuProg.fragUniforms.setup(b.funcs, p, fragShader.Uniforms.Size, fragShader.Uniforms.Locations)
	}
	return gpuProg, nil
}

func lookupUniform(funcs *gl.Functions, p gl.Program, loc driver.UniformLocation) uniformLocation {
	u := funcs.GetUniformLocation(p, loc.Name)
	return uniformLocation{uniform: u, offset: loc.Offset, typ: loc.Type, size: loc.Size}
}

func (p *gpuProgram) SetStorageBuffer(binding int, buffer driver.Buffer) {
	buf := buffer.(*gpuBuffer)
	if buf.typ&driver.BufferBindingShaderStorage == 0 {
		panic("not a shader storage buffer")
	}
	p.storage[binding] = buf
}

func (p *gpuProgram) SetVertexUniforms(buffer driver.Buffer) {
	p.vertUniforms.setBuffer(buffer)
}

func (p *gpuProgram) SetFragmentUniforms(buffer driver.Buffer) {
	p.fragUniforms.setBuffer(buffer)
}

func (p *gpuProgram) updateUniforms() {
	f := p.backend.funcs
	if p.backend.ubo {
		if b := p.vertUniforms.buf; b != nil {
			f.BindBufferBase(gl.UNIFORM_BUFFER, 0, b.obj)
		}
		if b := p.fragUniforms.buf; b != nil {
			f.BindBufferBase(gl.UNIFORM_BUFFER, 1, b.obj)
		}
	} else {
		p.vertUniforms.update(f)
		p.fragUniforms.update(f)
	}
}

func (b *Backend) BindProgram(prog driver.Program) {
	p := prog.(*gpuProgram)
	b.useProgram(p)
}

func (p *gpuProgram) Release() {
	p.backend.funcs.DeleteProgram(p.obj)
}

func (u *uniformsTracker) setup(funcs *gl.Functions, p gl.Program, uniformSize int, uniforms []driver.UniformLocation) {
	u.locs = make([]uniformLocation, len(uniforms))
	for i, uniform := range uniforms {
		u.locs[i] = lookupUniform(funcs, p, uniform)
	}
	u.size = uniformSize
}

func (u *uniformsTracker) setBuffer(buffer driver.Buffer) {
	buf := buffer.(*gpuBuffer)
	if buf.typ&driver.BufferBindingUniforms == 0 {
		panic("not a uniform buffer")
	}
	if buf.size < u.size {
		panic(fmt.Errorf("uniform buffer too small, got %d need %d", buf.size, u.size))
	}
	u.buf = buf
	// Force update.
	u.version = buf.version - 1
}

func (p *uniformsTracker) update(funcs *gl.Functions) {
	b := p.buf
	if b == nil || b.version == p.version {
		return
	}
	p.version = b.version
	data := b.data
	for _, u := range p.locs {
		data := data[u.offset:]
		switch {
		case u.typ == driver.DataTypeFloat && u.size == 1:
			data := data[:4]
			v := *(*[1]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform1f(u.uniform, v[0])
		case u.typ == driver.DataTypeFloat && u.size == 2:
			data := data[:8]
			v := *(*[2]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform2f(u.uniform, v[0], v[1])
		case u.typ == driver.DataTypeFloat && u.size == 3:
			data := data[:12]
			v := *(*[3]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform3f(u.uniform, v[0], v[1], v[2])
		case u.typ == driver.DataTypeFloat && u.size == 4:
			data := data[:16]
			v := *(*[4]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform4f(u.uniform, v[0], v[1], v[2], v[3])
		default:
			panic("unsupported uniform data type or size")
		}
	}
}

func (b *gpuBuffer) Upload(data []byte) {
	if b.immutable {
		panic("immutable buffer")
	}
	if len(data) > b.size {
		panic("buffer size overflow")
	}
	b.version++
	copy(b.data, data)
	if b.hasBuffer {
		firstBinding := firstBufferType(b.typ)
		b.backend.funcs.BindBuffer(firstBinding, b.obj)
		if len(data) == b.size {
			// the iOS GL implementation doesn't recognize when BufferSubData
			// clears the entire buffer. Tell it and avoid GPU stalls.
			// See also https://github.com/godotengine/godot/issues/23956.
			b.backend.funcs.BufferData(firstBinding, b.size, gl.DYNAMIC_DRAW)
		}
		b.backend.funcs.BufferSubData(firstBinding, 0, data)
	}
}

func (b *gpuBuffer) Download(data []byte) error {
	if len(data) > b.size {
		panic("buffer size overflow")
	}
	if !b.hasBuffer {
		copy(data, b.data)
		return nil
	}
	firstBinding := firstBufferType(b.typ)
	b.backend.funcs.BindBuffer(firstBinding, b.obj)
	bufferMap := b.backend.funcs.MapBufferRange(firstBinding, 0, len(data), gl.MAP_READ_BIT)
	if bufferMap == nil {
		return fmt.Errorf("MapBufferRange: error %#x", b.backend.funcs.GetError())
	}
	copy(data, bufferMap)
	if !b.backend.funcs.UnmapBuffer(firstBinding) {
		return driver.ErrContentLost
	}
	return nil
}

func (b *gpuBuffer) Release() {
	if b.hasBuffer {
		b.backend.funcs.DeleteBuffer(b.obj)
		b.hasBuffer = false
	}
}

func (b *Backend) BindVertexBuffer(buf driver.Buffer, stride, offset int) {
	gbuf := buf.(*gpuBuffer)
	if gbuf.typ&driver.BufferBindingVertices == 0 {
		panic("not a vertex buffer")
	}
	b.state.buffer = bufferBinding{buf: gbuf, stride: stride, offset: offset}
}

func (b *Backend) setupVertexArrays() {
	layout := b.state.layout
	if layout == nil {
		return
	}
	buf := b.state.buffer
	b.funcs.BindBuffer(gl.ARRAY_BUFFER, buf.buf.obj)
	for i, inp := range layout.inputs {
		l := layout.layout[i]
		var gltyp gl.Enum
		switch l.Type {
		case driver.DataTypeFloat:
			gltyp = gl.FLOAT
		case driver.DataTypeShort:
			gltyp = gl.SHORT
		default:
			panic("unsupported data type")
		}
		b.funcs.VertexAttribPointer(gl.Attrib(inp.Location), l.Size, gltyp, false, buf.stride, buf.offset+l.Offset)
	}
}

func (b *Backend) BindIndexBuffer(buf driver.Buffer) {
	gbuf := buf.(*gpuBuffer)
	if gbuf.typ&driver.BufferBindingIndices == 0 {
		panic("not an index buffer")
	}
	b.funcs.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, gbuf.obj)
}

func (b *Backend) BlitFramebuffer(dst, src driver.Framebuffer, srect, drect image.Rectangle) {
	b.funcs.BindFramebuffer(gl.DRAW_FRAMEBUFFER, dst.(*gpuFramebuffer).obj)
	b.funcs.BindFramebuffer(gl.READ_FRAMEBUFFER, src.(*gpuFramebuffer).obj)
	b.funcs.BlitFramebuffer(
		srect.Min.X, srect.Min.Y, srect.Max.X, srect.Max.Y,
		drect.Min.X, drect.Min.Y, drect.Max.X, drect.Max.Y,
		gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT|gl.STENCIL_BUFFER_BIT,
		gl.NEAREST)
}

func (f *gpuFramebuffer) ReadPixels(src image.Rectangle, pixels []byte) error {
	glErr(f.backend.funcs)
	f.backend.BindFramebuffer(f)
	if len(pixels) < src.Dx()*src.Dy()*4 {
		return errors.New("unexpected RGBA size")
	}
	f.backend.funcs.ReadPixels(src.Min.X, src.Min.Y, src.Dx(), src.Dy(), gl.RGBA, gl.UNSIGNED_BYTE, pixels)
	return glErr(f.backend.funcs)
}

func (b *Backend) BindFramebuffer(fbo driver.Framebuffer) {
	b.funcs.BindFramebuffer(gl.FRAMEBUFFER, fbo.(*gpuFramebuffer).obj)
}

func (f *gpuFramebuffer) Invalidate() {
	f.backend.BindFramebuffer(f)
	f.backend.funcs.InvalidateFramebuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0)
}

func (f *gpuFramebuffer) Release() {
	if f.foreign {
		panic("framebuffer not created by NewFramebuffer")
	}
	f.backend.funcs.DeleteFramebuffer(f.obj)
	if f.hasDepth {
		f.backend.funcs.DeleteRenderbuffer(f.depthBuf)
	}
}

func toTexFilter(f driver.TextureFilter) int {
	switch f {
	case driver.FilterNearest:
		return gl.NEAREST
	case driver.FilterLinear:
		return gl.LINEAR
	default:
		panic("unsupported texture filter")
	}
}

func (b *Backend) BindTexture(unit int, t driver.Texture) {
	b.bindTexture(unit, t.(*gpuTexture))
}

func (t *gpuTexture) Release() {
	t.backend.funcs.DeleteTexture(t.obj)
}

func (t *gpuTexture) Upload(offset, size image.Point, pixels []byte) {
	if min := size.X * size.Y * 4; min > len(pixels) {
		panic(fmt.Errorf("size %d larger than data %d", min, len(pixels)))
	}
	t.backend.BindTexture(0, t)
	t.backend.funcs.TexSubImage2D(gl.TEXTURE_2D, 0, offset.X, offset.Y, size.X, size.Y, t.triple.format, t.triple.typ, pixels)
}

func (t *gpuTimer) Begin() {
	t.funcs.BeginQuery(gl.TIME_ELAPSED_EXT, t.obj)
}

func (t *gpuTimer) End() {
	t.funcs.EndQuery(gl.TIME_ELAPSED_EXT)
}

func (t *gpuTimer) ready() bool {
	return t.funcs.GetQueryObjectuiv(t.obj, gl.QUERY_RESULT_AVAILABLE) == gl.TRUE
}

func (t *gpuTimer) Release() {
	t.funcs.DeleteQuery(t.obj)
}

func (t *gpuTimer) Duration() (time.Duration, bool) {
	if !t.ready() {
		return 0, false
	}
	nanos := t.funcs.GetQueryObjectuiv(t.obj, gl.QUERY_RESULT)
	return time.Duration(nanos), true
}

func (b *Backend) BindInputLayout(l driver.InputLayout) {
	b.state.layout = l.(*gpuInputLayout)
}

func (l *gpuInputLayout) Release() {}

// floatTripleFor determines the best texture triple for floating point FBOs.
func floatTripleFor(f *gl.Functions, ver [2]int, exts []string) (textureTriple, error) {
	var triples []textureTriple
	if ver[0] >= 3 {
		triples = append(triples, textureTriple{gl.R16F, gl.Enum(gl.RED), gl.Enum(gl.HALF_FLOAT)})
	}
	// According to the OES_texture_half_float specification, EXT_color_buffer_half_float is needed to
	// render to FBOs. However, the Safari WebGL1 implementation does support half-float FBOs but does not
	// report EXT_color_buffer_half_float support. The triples are verified below, so it doesn't matter if we're
	// wrong.
	if hasExtension(exts, "GL_OES_texture_half_float") || hasExtension(exts, "GL_EXT_color_buffer_half_float") {
		// Try single channel.
		triples = append(triples, textureTriple{gl.LUMINANCE, gl.Enum(gl.LUMINANCE), gl.Enum(gl.HALF_FLOAT_OES)})
		// Fallback to 4 channels.
		triples = append(triples, textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.HALF_FLOAT_OES)})
	}
	if hasExtension(exts, "GL_OES_texture_float") || hasExtension(exts, "GL_EXT_color_buffer_float") {
		triples = append(triples, textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.FLOAT)})
	}
	tex := f.CreateTexture()
	defer f.DeleteTexture(tex)
	f.BindTexture(gl.TEXTURE_2D, tex)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	fbo := f.CreateFramebuffer()
	defer f.DeleteFramebuffer(fbo)
	defFBO := gl.Framebuffer(f.GetBinding(gl.FRAMEBUFFER_BINDING))
	f.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	defer f.BindFramebuffer(gl.FRAMEBUFFER, defFBO)
	var attempts []string
	for _, tt := range triples {
		const size = 256
		f.TexImage2D(gl.TEXTURE_2D, 0, tt.internalFormat, size, size, tt.format, tt.typ)
		f.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
		st := f.CheckFramebufferStatus(gl.FRAMEBUFFER)
		if st == gl.FRAMEBUFFER_COMPLETE {
			return tt, nil
		}
		attempts = append(attempts, fmt.Sprintf("(0x%x, 0x%x, 0x%x): 0x%x", tt.internalFormat, tt.format, tt.typ, st))
	}
	return textureTriple{}, fmt.Errorf("floating point fbos not supported (attempted %s)", attempts)
}

func srgbaTripleFor(ver [2]int, exts []string) (textureTriple, error) {
	switch {
	case ver[0] >= 3:
		return textureTriple{gl.SRGB8_ALPHA8, gl.Enum(gl.RGBA), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	case hasExtension(exts, "GL_EXT_sRGB"):
		return textureTriple{gl.SRGB_ALPHA_EXT, gl.Enum(gl.SRGB_ALPHA_EXT), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	default:
		return textureTriple{}, errors.New("no sRGB texture formats found")
	}
}

func alphaTripleFor(ver [2]int) textureTriple {
	intf, f := gl.Enum(gl.R8), gl.Enum(gl.RED)
	if ver[0] < 3 {
		// R8, RED not supported on OpenGL ES 2.0.
		intf, f = gl.LUMINANCE, gl.Enum(gl.LUMINANCE)
	}
	return textureTriple{intf, f, gl.UNSIGNED_BYTE}
}

func hasExtension(exts []string, ext string) bool {
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func firstBufferType(typ driver.BufferBinding) gl.Enum {
	switch {
	case typ&driver.BufferBindingIndices != 0:
		return gl.ELEMENT_ARRAY_BUFFER
	case typ&driver.BufferBindingVertices != 0:
		return gl.ARRAY_BUFFER
	case typ&driver.BufferBindingUniforms != 0:
		return gl.UNIFORM_BUFFER
	case typ&driver.BufferBindingShaderStorage != 0:
		return gl.SHADER_STORAGE_BUFFER
	default:
		panic("unsupported buffer type")
	}
}
