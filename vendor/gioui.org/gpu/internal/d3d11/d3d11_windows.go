// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"errors"
	"fmt"
	"image"
	"math"
	"reflect"
	"unsafe"

	"golang.org/x/sys/windows"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/d3d11"
)

type Backend struct {
	dev *d3d11.Device
	ctx *d3d11.DeviceContext

	// Temporary storage to avoid garbage.
	clearColor [4]float32
	viewport   d3d11.VIEWPORT
	depthState depthState
	blendState blendState

	// Current program.
	prog *Program

	caps driver.Caps

	// fbo is the currently bound fbo.
	fbo *Framebuffer

	floatFormat uint32

	// cached state objects.
	depthStates map[depthState]*d3d11.DepthStencilState
	blendStates map[blendState]*d3d11.BlendState
}

type blendState struct {
	enable  bool
	sfactor driver.BlendFactor
	dfactor driver.BlendFactor
}

type depthState struct {
	enable bool
	mask   bool
	fn     driver.DepthFunc
}

type Texture struct {
	backend  *Backend
	format   uint32
	bindings driver.BufferBinding
	tex      *d3d11.Texture2D
	sampler  *d3d11.SamplerState
	resView  *d3d11.ShaderResourceView
	width    int
	height   int
}

type Program struct {
	backend *Backend

	vert struct {
		shader   *d3d11.VertexShader
		uniforms *Buffer
	}
	frag struct {
		shader   *d3d11.PixelShader
		uniforms *Buffer
	}
}

type Framebuffer struct {
	dev          *d3d11.Device
	ctx          *d3d11.DeviceContext
	format       uint32
	resource     *d3d11.Resource
	renderTarget *d3d11.RenderTargetView
	depthView    *d3d11.DepthStencilView
	foreign      bool
}

type Buffer struct {
	backend   *Backend
	bind      uint32
	buf       *d3d11.Buffer
	immutable bool
}

type InputLayout struct {
	layout *d3d11.InputLayout
}

func init() {
	driver.NewDirect3D11Device = newDirect3D11Device
}

func detectFloatFormat(dev *d3d11.Device) (uint32, bool) {
	formats := []uint32{
		d3d11.DXGI_FORMAT_R16_FLOAT,
		d3d11.DXGI_FORMAT_R32_FLOAT,
		d3d11.DXGI_FORMAT_R16G16_FLOAT,
		d3d11.DXGI_FORMAT_R32G32_FLOAT,
		// These last two are really wasteful, but c'est la vie.
		d3d11.DXGI_FORMAT_R16G16B16A16_FLOAT,
		d3d11.DXGI_FORMAT_R32G32B32A32_FLOAT,
	}
	for _, format := range formats {
		need := uint32(d3d11.FORMAT_SUPPORT_TEXTURE2D | d3d11.FORMAT_SUPPORT_RENDER_TARGET)
		if support, _ := dev.CheckFormatSupport(format); support&need == need {
			return format, true
		}
	}
	return 0, false
}

func newDirect3D11Device(api driver.Direct3D11) (driver.Device, error) {
	dev := (*d3d11.Device)(api.Device)
	b := &Backend{
		dev: dev,
		ctx: dev.GetImmediateContext(),
		caps: driver.Caps{
			MaxTextureSize: 2048, // 9.1 maximum
		},
		depthStates: make(map[depthState]*d3d11.DepthStencilState),
		blendStates: make(map[blendState]*d3d11.BlendState),
	}
	featLvl := dev.GetFeatureLevel()
	if featLvl < d3d11.FEATURE_LEVEL_9_1 {
		d3d11.IUnknownRelease(unsafe.Pointer(dev), dev.Vtbl.Release)
		d3d11.IUnknownRelease(unsafe.Pointer(b.ctx), b.ctx.Vtbl.Release)
		return nil, fmt.Errorf("d3d11: feature level too low: %d", featLvl)
	}
	switch {
	case featLvl >= d3d11.FEATURE_LEVEL_11_0:
		b.caps.MaxTextureSize = 16384
	case featLvl >= d3d11.FEATURE_LEVEL_9_3:
		b.caps.MaxTextureSize = 4096
	}
	if fmt, ok := detectFloatFormat(dev); ok {
		b.floatFormat = fmt
		b.caps.Features |= driver.FeatureFloatRenderTargets
	}
	// Enable depth mask to match OpenGL.
	b.depthState.mask = true
	// Disable backface culling to match OpenGL.
	state, err := dev.CreateRasterizerState(&d3d11.RASTERIZER_DESC{
		CullMode:        d3d11.CULL_NONE,
		FillMode:        d3d11.FILL_SOLID,
		DepthClipEnable: 1,
	})
	if err != nil {
		return nil, err
	}
	defer d3d11.IUnknownRelease(unsafe.Pointer(state), state.Vtbl.Release)
	b.ctx.RSSetState(state)
	return b, nil
}

func (b *Backend) BeginFrame() driver.Framebuffer {
	renderTarget, depthView := b.ctx.OMGetRenderTargets()
	// Assume someone else is holding on to the render targets.
	if renderTarget != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(renderTarget), renderTarget.Vtbl.Release)
	}
	if depthView != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(depthView), depthView.Vtbl.Release)
	}
	return &Framebuffer{ctx: b.ctx, dev: b.dev, renderTarget: renderTarget, depthView: depthView, foreign: true}
}

func (b *Backend) EndFrame() {
}

func (b *Backend) Caps() driver.Caps {
	return b.caps
}

func (b *Backend) NewTimer() driver.Timer {
	panic("timers not supported")
}

func (b *Backend) IsTimeContinuous() bool {
	panic("timers not supported")
}

func (b *Backend) Release() {
	for _, state := range b.depthStates {
		d3d11.IUnknownRelease(unsafe.Pointer(state), state.Vtbl.Release)
	}
	for _, state := range b.blendStates {
		d3d11.IUnknownRelease(unsafe.Pointer(state), state.Vtbl.Release)
	}
	d3d11.IUnknownRelease(unsafe.Pointer(b.ctx), b.ctx.Vtbl.Release)
	*b = Backend{}
}

func (b *Backend) NewTexture(format driver.TextureFormat, width, height int, minFilter, magFilter driver.TextureFilter, bindings driver.BufferBinding) (driver.Texture, error) {
	var d3dfmt uint32
	switch format {
	case driver.TextureFormatFloat:
		d3dfmt = b.floatFormat
	case driver.TextureFormatSRGB:
		d3dfmt = d3d11.DXGI_FORMAT_R8G8B8A8_UNORM_SRGB
	default:
		return nil, fmt.Errorf("unsupported texture format %d", format)
	}
	tex, err := b.dev.CreateTexture2D(&d3d11.TEXTURE2D_DESC{
		Width:     uint32(width),
		Height:    uint32(height),
		MipLevels: 1,
		ArraySize: 1,
		Format:    d3dfmt,
		SampleDesc: d3d11.DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		BindFlags: convBufferBinding(bindings),
	})
	if err != nil {
		return nil, err
	}
	var (
		sampler *d3d11.SamplerState
		resView *d3d11.ShaderResourceView
	)
	if bindings&driver.BufferBindingTexture != 0 {
		var filter uint32
		switch {
		case minFilter == driver.FilterNearest && magFilter == driver.FilterNearest:
			filter = d3d11.FILTER_MIN_MAG_MIP_POINT
		case minFilter == driver.FilterLinear && magFilter == driver.FilterLinear:
			filter = d3d11.FILTER_MIN_MAG_LINEAR_MIP_POINT
		default:
			d3d11.IUnknownRelease(unsafe.Pointer(tex), tex.Vtbl.Release)
			return nil, fmt.Errorf("unsupported texture filter combination %d, %d", minFilter, magFilter)
		}
		var err error
		sampler, err = b.dev.CreateSamplerState(&d3d11.SAMPLER_DESC{
			Filter:        filter,
			AddressU:      d3d11.TEXTURE_ADDRESS_CLAMP,
			AddressV:      d3d11.TEXTURE_ADDRESS_CLAMP,
			AddressW:      d3d11.TEXTURE_ADDRESS_CLAMP,
			MaxAnisotropy: 1,
			MinLOD:        -math.MaxFloat32,
			MaxLOD:        math.MaxFloat32,
		})
		if err != nil {
			d3d11.IUnknownRelease(unsafe.Pointer(tex), tex.Vtbl.Release)
			return nil, err
		}
		resView, err = b.dev.CreateShaderResourceViewTEX2D(
			(*d3d11.Resource)(unsafe.Pointer(tex)),
			&d3d11.SHADER_RESOURCE_VIEW_DESC_TEX2D{
				SHADER_RESOURCE_VIEW_DESC: d3d11.SHADER_RESOURCE_VIEW_DESC{
					Format:        d3dfmt,
					ViewDimension: d3d11.SRV_DIMENSION_TEXTURE2D,
				},
				Texture2D: d3d11.TEX2D_SRV{
					MostDetailedMip: 0,
					MipLevels:       ^uint32(0),
				},
			},
		)
		if err != nil {
			d3d11.IUnknownRelease(unsafe.Pointer(tex), tex.Vtbl.Release)
			d3d11.IUnknownRelease(unsafe.Pointer(sampler), sampler.Vtbl.Release)
			return nil, err
		}
	}
	return &Texture{backend: b, format: d3dfmt, tex: tex, sampler: sampler, resView: resView, bindings: bindings, width: width, height: height}, nil
}

func (b *Backend) NewFramebuffer(tex driver.Texture, depthBits int) (driver.Framebuffer, error) {
	d3dtex := tex.(*Texture)
	if d3dtex.bindings&driver.BufferBindingFramebuffer == 0 {
		return nil, errors.New("the texture was created without BufferBindingFramebuffer binding")
	}
	resource := (*d3d11.Resource)(unsafe.Pointer(d3dtex.tex))
	renderTarget, err := b.dev.CreateRenderTargetView(resource)
	if err != nil {
		return nil, err
	}
	fbo := &Framebuffer{ctx: b.ctx, dev: b.dev, format: d3dtex.format, resource: resource, renderTarget: renderTarget}
	if depthBits > 0 {
		depthView, err := d3d11.CreateDepthView(b.dev, d3dtex.width, d3dtex.height, depthBits)
		if err != nil {
			d3d11.IUnknownRelease(unsafe.Pointer(renderTarget), renderTarget.Vtbl.Release)
			return nil, err
		}
		fbo.depthView = depthView
	}
	return fbo, nil
}

func (b *Backend) NewInputLayout(vertexShader driver.ShaderSources, layout []driver.InputDesc) (driver.InputLayout, error) {
	if len(vertexShader.Inputs) != len(layout) {
		return nil, fmt.Errorf("NewInputLayout: got %d inputs, expected %d", len(layout), len(vertexShader.Inputs))
	}
	descs := make([]d3d11.INPUT_ELEMENT_DESC, len(layout))
	for i, l := range layout {
		inp := vertexShader.Inputs[i]
		cname, err := windows.BytePtrFromString(inp.Semantic)
		if err != nil {
			return nil, err
		}
		var format uint32
		switch l.Type {
		case driver.DataTypeFloat:
			switch l.Size {
			case 1:
				format = d3d11.DXGI_FORMAT_R32_FLOAT
			case 2:
				format = d3d11.DXGI_FORMAT_R32G32_FLOAT
			case 3:
				format = d3d11.DXGI_FORMAT_R32G32B32_FLOAT
			case 4:
				format = d3d11.DXGI_FORMAT_R32G32B32A32_FLOAT
			default:
				panic("unsupported float data size")
			}
		case driver.DataTypeShort:
			switch l.Size {
			case 1:
				format = d3d11.DXGI_FORMAT_R16_SINT
			case 2:
				format = d3d11.DXGI_FORMAT_R16G16_SINT
			default:
				panic("unsupported float data size")
			}
		default:
			panic("unsupported data type")
		}
		descs[i] = d3d11.INPUT_ELEMENT_DESC{
			SemanticName:      cname,
			SemanticIndex:     uint32(inp.SemanticIndex),
			Format:            format,
			AlignedByteOffset: uint32(l.Offset),
		}
	}
	l, err := b.dev.CreateInputLayout(descs, vertexShader.HLSL)
	if err != nil {
		return nil, err
	}
	return &InputLayout{layout: l}, nil
}

func (b *Backend) NewBuffer(typ driver.BufferBinding, size int) (driver.Buffer, error) {
	if typ&driver.BufferBindingUniforms != 0 {
		if typ != driver.BufferBindingUniforms {
			return nil, errors.New("uniform buffers cannot have other bindings")
		}
		if size%16 != 0 {
			return nil, fmt.Errorf("constant buffer size is %d, expected a multiple of 16", size)
		}
	}
	bind := convBufferBinding(typ)
	buf, err := b.dev.CreateBuffer(&d3d11.BUFFER_DESC{
		ByteWidth: uint32(size),
		BindFlags: bind,
	}, nil)
	if err != nil {
		return nil, err
	}
	return &Buffer{backend: b, buf: buf, bind: bind}, nil
}

func (b *Backend) NewImmutableBuffer(typ driver.BufferBinding, data []byte) (driver.Buffer, error) {
	if typ&driver.BufferBindingUniforms != 0 {
		if typ != driver.BufferBindingUniforms {
			return nil, errors.New("uniform buffers cannot have other bindings")
		}
		if len(data)%16 != 0 {
			return nil, fmt.Errorf("constant buffer size is %d, expected a multiple of 16", len(data))
		}
	}
	bind := convBufferBinding(typ)
	buf, err := b.dev.CreateBuffer(&d3d11.BUFFER_DESC{
		ByteWidth: uint32(len(data)),
		Usage:     d3d11.USAGE_IMMUTABLE,
		BindFlags: bind,
	}, data)
	if err != nil {
		return nil, err
	}
	return &Buffer{backend: b, buf: buf, bind: bind, immutable: true}, nil
}

func (b *Backend) NewComputeProgram(shader driver.ShaderSources) (driver.Program, error) {
	panic("not implemented")
}

func (b *Backend) NewProgram(vertexShader, fragmentShader driver.ShaderSources) (driver.Program, error) {
	vs, err := b.dev.CreateVertexShader(vertexShader.HLSL)
	if err != nil {
		return nil, err
	}
	ps, err := b.dev.CreatePixelShader(fragmentShader.HLSL)
	if err != nil {
		return nil, err
	}
	p := &Program{backend: b}
	p.vert.shader = vs
	p.frag.shader = ps
	return p, nil
}

func (b *Backend) Clear(colr, colg, colb, cola float32) {
	b.clearColor = [4]float32{colr, colg, colb, cola}
	b.ctx.ClearRenderTargetView(b.fbo.renderTarget, &b.clearColor)
}

func (b *Backend) ClearDepth(depth float32) {
	if b.fbo.depthView != nil {
		b.ctx.ClearDepthStencilView(b.fbo.depthView, d3d11.CLEAR_DEPTH|d3d11.CLEAR_STENCIL, depth, 0)
	}
}

func (b *Backend) Viewport(x, y, width, height int) {
	b.viewport = d3d11.VIEWPORT{
		TopLeftX: float32(x),
		TopLeftY: float32(y),
		Width:    float32(width),
		Height:   float32(height),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}
	b.ctx.RSSetViewports(&b.viewport)
}

func (b *Backend) DrawArrays(mode driver.DrawMode, off, count int) {
	b.prepareDraw(mode)
	b.ctx.Draw(uint32(count), uint32(off))
}

func (b *Backend) DrawElements(mode driver.DrawMode, off, count int) {
	b.prepareDraw(mode)
	b.ctx.DrawIndexed(uint32(count), uint32(off), 0)
}

func (b *Backend) prepareDraw(mode driver.DrawMode) {
	if p := b.prog; p != nil {
		b.ctx.VSSetShader(p.vert.shader)
		b.ctx.PSSetShader(p.frag.shader)
		if buf := p.vert.uniforms; buf != nil {
			b.ctx.VSSetConstantBuffers(buf.buf)
		}
		if buf := p.frag.uniforms; buf != nil {
			b.ctx.PSSetConstantBuffers(buf.buf)
		}
	}
	var topology uint32
	switch mode {
	case driver.DrawModeTriangles:
		topology = d3d11.PRIMITIVE_TOPOLOGY_TRIANGLELIST
	case driver.DrawModeTriangleStrip:
		topology = d3d11.PRIMITIVE_TOPOLOGY_TRIANGLESTRIP
	default:
		panic("unsupported draw mode")
	}
	b.ctx.IASetPrimitiveTopology(topology)

	depthState, ok := b.depthStates[b.depthState]
	if !ok {
		var desc d3d11.DEPTH_STENCIL_DESC
		if b.depthState.enable {
			desc.DepthEnable = 1
		}
		if b.depthState.mask {
			desc.DepthWriteMask = d3d11.DEPTH_WRITE_MASK_ALL
		}
		switch b.depthState.fn {
		case driver.DepthFuncGreater:
			desc.DepthFunc = d3d11.COMPARISON_GREATER
		case driver.DepthFuncGreaterEqual:
			desc.DepthFunc = d3d11.COMPARISON_GREATER_EQUAL
		default:
			panic("unsupported depth func")
		}
		var err error
		depthState, err = b.dev.CreateDepthStencilState(&desc)
		if err != nil {
			panic(err)
		}
		b.depthStates[b.depthState] = depthState
	}
	b.ctx.OMSetDepthStencilState(depthState, 0)

	blendState, ok := b.blendStates[b.blendState]
	if !ok {
		var desc d3d11.BLEND_DESC
		t0 := &desc.RenderTarget[0]
		t0.RenderTargetWriteMask = d3d11.COLOR_WRITE_ENABLE_ALL
		t0.BlendOp = d3d11.BLEND_OP_ADD
		t0.BlendOpAlpha = d3d11.BLEND_OP_ADD
		if b.blendState.enable {
			t0.BlendEnable = 1
		}
		scol, salpha := toBlendFactor(b.blendState.sfactor)
		dcol, dalpha := toBlendFactor(b.blendState.dfactor)
		t0.SrcBlend = scol
		t0.SrcBlendAlpha = salpha
		t0.DestBlend = dcol
		t0.DestBlendAlpha = dalpha
		var err error
		blendState, err = b.dev.CreateBlendState(&desc)
		if err != nil {
			panic(err)
		}
		b.blendStates[b.blendState] = blendState
	}
	b.ctx.OMSetBlendState(blendState, nil, 0xffffffff)
}

func (b *Backend) DepthFunc(f driver.DepthFunc) {
	b.depthState.fn = f
}

func (b *Backend) SetBlend(enable bool) {
	b.blendState.enable = enable
}

func (b *Backend) SetDepthTest(enable bool) {
	b.depthState.enable = enable
}

func (b *Backend) DepthMask(mask bool) {
	b.depthState.mask = mask
}

func (b *Backend) BlendFunc(sfactor, dfactor driver.BlendFactor) {
	b.blendState.sfactor = sfactor
	b.blendState.dfactor = dfactor
}

func (b *Backend) BindImageTexture(unit int, tex driver.Texture, access driver.AccessBits, f driver.TextureFormat) {
	panic("not implemented")
}

func (b *Backend) MemoryBarrier() {
	panic("not implemented")
}

func (b *Backend) DispatchCompute(x, y, z int) {
	panic("not implemented")
}

func (t *Texture) Upload(offset, size image.Point, pixels []byte) {
	stride := size.X * 4
	dst := &d3d11.BOX{
		Left:   uint32(offset.X),
		Top:    uint32(offset.Y),
		Right:  uint32(offset.X + size.X),
		Bottom: uint32(offset.Y + size.Y),
		Front:  0,
		Back:   1,
	}
	res := (*d3d11.Resource)(unsafe.Pointer(t.tex))
	t.backend.ctx.UpdateSubresource(res, dst, uint32(stride), uint32(len(pixels)), pixels)
}

func (t *Texture) Release() {
	d3d11.IUnknownRelease(unsafe.Pointer(t.tex), t.tex.Vtbl.Release)
	t.tex = nil
	if t.sampler != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.sampler), t.sampler.Vtbl.Release)
		t.sampler = nil
	}
	if t.resView != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.resView), t.resView.Vtbl.Release)
		t.resView = nil
	}
}

func (b *Backend) BindTexture(unit int, tex driver.Texture) {
	t := tex.(*Texture)
	b.ctx.PSSetSamplers(uint32(unit), t.sampler)
	b.ctx.PSSetShaderResources(uint32(unit), t.resView)
}

func (b *Backend) BindProgram(prog driver.Program) {
	b.prog = prog.(*Program)
}

func (p *Program) Release() {
	d3d11.IUnknownRelease(unsafe.Pointer(p.vert.shader), p.vert.shader.Vtbl.Release)
	d3d11.IUnknownRelease(unsafe.Pointer(p.frag.shader), p.frag.shader.Vtbl.Release)
	p.vert.shader = nil
	p.frag.shader = nil
}

func (p *Program) SetStorageBuffer(binding int, buffer driver.Buffer) {
	panic("not implemented")
}

func (p *Program) SetVertexUniforms(buf driver.Buffer) {
	p.vert.uniforms = buf.(*Buffer)
}

func (p *Program) SetFragmentUniforms(buf driver.Buffer) {
	p.frag.uniforms = buf.(*Buffer)
}

func (b *Backend) BindVertexBuffer(buf driver.Buffer, stride, offset int) {
	b.ctx.IASetVertexBuffers(buf.(*Buffer).buf, uint32(stride), uint32(offset))
}

func (b *Backend) BindIndexBuffer(buf driver.Buffer) {
	b.ctx.IASetIndexBuffer(buf.(*Buffer).buf, d3d11.DXGI_FORMAT_R16_UINT, 0)
}

func (b *Buffer) Download(data []byte) error {
	panic("not implemented")
}

func (b *Buffer) Upload(data []byte) {
	b.backend.ctx.UpdateSubresource((*d3d11.Resource)(unsafe.Pointer(b.buf)), nil, 0, 0, data)
}

func (b *Buffer) Release() {
	d3d11.IUnknownRelease(unsafe.Pointer(b.buf), b.buf.Vtbl.Release)
	b.buf = nil
}

func (f *Framebuffer) ReadPixels(src image.Rectangle, pixels []byte) error {
	if f.resource == nil {
		return errors.New("framebuffer does not support ReadPixels")
	}
	w, h := src.Dx(), src.Dy()
	tex, err := f.dev.CreateTexture2D(&d3d11.TEXTURE2D_DESC{
		Width:     uint32(w),
		Height:    uint32(h),
		MipLevels: 1,
		ArraySize: 1,
		Format:    f.format,
		SampleDesc: d3d11.DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		Usage:          d3d11.USAGE_STAGING,
		CPUAccessFlags: d3d11.CPU_ACCESS_READ,
	})
	if err != nil {
		return fmt.Errorf("ReadPixels: %v", err)
	}
	defer d3d11.IUnknownRelease(unsafe.Pointer(tex), tex.Vtbl.Release)
	res := (*d3d11.Resource)(unsafe.Pointer(tex))
	f.ctx.CopySubresourceRegion(
		res,
		0,       // Destination subresource.
		0, 0, 0, // Destination coordinates (x, y, z).
		f.resource,
		0, // Source subresource.
		&d3d11.BOX{
			Left:   uint32(src.Min.X),
			Top:    uint32(src.Min.Y),
			Right:  uint32(src.Max.X),
			Bottom: uint32(src.Max.Y),
			Front:  0,
			Back:   1,
		},
	)
	resMap, err := f.ctx.Map(res, 0, d3d11.MAP_READ, 0)
	if err != nil {
		return fmt.Errorf("ReadPixels: %v", err)
	}
	defer f.ctx.Unmap(res, 0)
	srcPitch := w * 4
	dstPitch := int(resMap.RowPitch)
	mapSize := dstPitch * h
	data := sliceOf(resMap.PData, mapSize)
	width := w * 4
	for r := 0; r < h; r++ {
		pixels := pixels[r*srcPitch:]
		copy(pixels[:width], data[r*dstPitch:])
	}
	return nil
}

func (b *Backend) BindFramebuffer(fbo driver.Framebuffer) {
	b.fbo = fbo.(*Framebuffer)
	b.ctx.OMSetRenderTargets(b.fbo.renderTarget, b.fbo.depthView)
}

func (f *Framebuffer) Invalidate() {
}

func (f *Framebuffer) Release() {
	if f.foreign {
		panic("framebuffer not created by NewFramebuffer")
	}
	if f.renderTarget != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(f.renderTarget), f.renderTarget.Vtbl.Release)
		f.renderTarget = nil
	}
	if f.depthView != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(f.depthView), f.depthView.Vtbl.Release)
		f.depthView = nil
	}
}

func (b *Backend) BindInputLayout(layout driver.InputLayout) {
	b.ctx.IASetInputLayout(layout.(*InputLayout).layout)
}

func (l *InputLayout) Release() {
	d3d11.IUnknownRelease(unsafe.Pointer(l.layout), l.layout.Vtbl.Release)
	l.layout = nil
}

func convBufferBinding(typ driver.BufferBinding) uint32 {
	var bindings uint32
	if typ&driver.BufferBindingVertices != 0 {
		bindings |= d3d11.BIND_VERTEX_BUFFER
	}
	if typ&driver.BufferBindingIndices != 0 {
		bindings |= d3d11.BIND_INDEX_BUFFER
	}
	if typ&driver.BufferBindingUniforms != 0 {
		bindings |= d3d11.BIND_CONSTANT_BUFFER
	}
	if typ&driver.BufferBindingTexture != 0 {
		bindings |= d3d11.BIND_SHADER_RESOURCE
	}
	if typ&driver.BufferBindingFramebuffer != 0 {
		bindings |= d3d11.BIND_RENDER_TARGET
	}
	return bindings
}

func toBlendFactor(f driver.BlendFactor) (uint32, uint32) {
	switch f {
	case driver.BlendFactorOne:
		return d3d11.BLEND_ONE, d3d11.BLEND_ONE
	case driver.BlendFactorOneMinusSrcAlpha:
		return d3d11.BLEND_INV_SRC_ALPHA, d3d11.BLEND_INV_SRC_ALPHA
	case driver.BlendFactorZero:
		return d3d11.BLEND_ZERO, d3d11.BLEND_ZERO
	case driver.BlendFactorDstColor:
		return d3d11.BLEND_DEST_COLOR, d3d11.BLEND_DEST_ALPHA
	default:
		panic("unsupported blend source factor")
	}
}

// sliceOf returns a slice from a (native) pointer.
func sliceOf(ptr uintptr, cap int) []byte {
	var data []byte
	h := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	h.Data = ptr
	h.Cap = cap
	h.Len = cap
	return data
}
