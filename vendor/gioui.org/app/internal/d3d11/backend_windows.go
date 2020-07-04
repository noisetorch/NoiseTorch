// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"errors"
	"fmt"
	"image"
	"math"
	"unsafe"

	"gioui.org/gpu/backend"
	gunsafe "gioui.org/internal/unsafe"
	"golang.org/x/sys/windows"
)

const debug = false

type Device struct {
	dev         *_ID3D11Device
	ctx         *_ID3D11DeviceContext
	featLvl     uint32
	floatFormat uint32
	depthStates map[depthState]*_ID3D11DepthStencilState
	blendStates map[blendState]*_ID3D11BlendState
}

type Backend struct {
	// Temporary storage to avoid garbage.
	clearColor [4]float32
	viewport   _D3D11_VIEWPORT
	depthState depthState
	blendState blendState
	prog       *Program

	dev  *Device
	caps backend.Caps

	// fbo is the currently bound fbo.
	fbo *Framebuffer
}

type blendState struct {
	enable  bool
	sfactor backend.BlendFactor
	dfactor backend.BlendFactor
}

type depthState struct {
	enable bool
	mask   bool
	fn     backend.DepthFunc
}

type Texture struct {
	backend  *Backend
	format   uint32
	bindings backend.BufferBinding
	tex      *_ID3D11Texture2D
	sampler  *_ID3D11SamplerState
	resView  *_ID3D11ShaderResourceView
	width    int
	height   int
}

type Program struct {
	backend *Backend

	vert struct {
		shader   *_ID3D11VertexShader
		uniforms *Buffer
	}
	frag struct {
		shader   *_ID3D11PixelShader
		uniforms *Buffer
	}
}

type Framebuffer struct {
	dev          *Device
	format       uint32
	resource     *_ID3D11Resource
	renderTarget *_ID3D11RenderTargetView
	depthView    *_ID3D11DepthStencilView
	foreign      bool
}

type Buffer struct {
	backend   *Backend
	bind      uint32
	buf       *_ID3D11Buffer
	immutable bool
}

type InputLayout struct {
	dev    *Device
	layout *_ID3D11InputLayout
}

type SwapChain struct {
	swchain *_IDXGISwapChain
	fbo     *Framebuffer
}

func NewDevice() (*Device, error) {
	var flags uint32
	if debug {
		flags |= _D3D11_CREATE_DEVICE_DEBUG
	}
	d3ddev, d3dctx, featLvl, err := _D3D11CreateDevice(
		_D3D_DRIVER_TYPE_HARDWARE,
		flags,
	)
	if err != nil {
		return nil, fmt.Errorf("NewContext: %v", err)
	}
	dev := &Device{dev: d3ddev, ctx: d3dctx, featLvl: featLvl}
	if featLvl < _D3D_FEATURE_LEVEL_9_1 {
		_IUnknownRelease(unsafe.Pointer(d3ddev), d3ddev.vtbl.Release)
		_IUnknownRelease(unsafe.Pointer(d3dctx), d3dctx.vtbl.Release)
		return nil, fmt.Errorf("d3d11: feature level too low: %d", featLvl)
	}
	floatFormat, ok := detectFloatFormat(d3ddev)
	if !ok {
		_IUnknownRelease(unsafe.Pointer(d3ddev), d3ddev.vtbl.Release)
		_IUnknownRelease(unsafe.Pointer(d3dctx), d3dctx.vtbl.Release)
		return nil, fmt.Errorf("d3d11: no available floating point formats")
	}
	dev.floatFormat = floatFormat
	dev.depthStates = make(map[depthState]*_ID3D11DepthStencilState)
	dev.blendStates = make(map[blendState]*_ID3D11BlendState)
	return dev, nil
}

func detectFloatFormat(dev *_ID3D11Device) (uint32, bool) {
	formats := []uint32{
		_DXGI_FORMAT_R16_FLOAT,
		_DXGI_FORMAT_R32_FLOAT,
		_DXGI_FORMAT_R16G16_FLOAT,
		_DXGI_FORMAT_R32G32_FLOAT,
		// These last two are really wasteful, but c'est la vie.
		_DXGI_FORMAT_R16G16B16A16_FLOAT,
		_DXGI_FORMAT_R32G32B32A32_FLOAT,
	}
	for _, format := range formats {
		need := uint32(_D3D11_FORMAT_SUPPORT_TEXTURE2D | _D3D11_FORMAT_SUPPORT_RENDER_TARGET)
		if support, _ := dev.CheckFormatSupport(format); support&need == need {
			return format, true
		}
	}
	return 0, false
}

func (d *Device) CreateSwapChain(hwnd windows.Handle) (*SwapChain, error) {
	dxgiDev, err := _IUnknownQueryInterface(unsafe.Pointer(d.dev), d.dev.vtbl.QueryInterface, &_IID_IDXGIDevice)
	if err != nil {
		return nil, fmt.Errorf("NewContext: %v", err)
	}
	adapter, err := (*_IDXGIDevice)(unsafe.Pointer(dxgiDev)).GetAdapter()
	_IUnknownRelease(unsafe.Pointer(dxgiDev), dxgiDev.vtbl.Release)
	if err != nil {
		return nil, fmt.Errorf("NewContext: %v", err)
	}
	dxgiFactory, err := (*_IDXGIObject)(unsafe.Pointer(adapter)).GetParent(&_IID_IDXGIFactory)
	_IUnknownRelease(unsafe.Pointer(adapter), adapter.vtbl.Release)
	if err != nil {
		return nil, fmt.Errorf("NewContext: %v", err)
	}
	d3dswchain, err := (*_IDXGIFactory)(unsafe.Pointer(dxgiFactory)).CreateSwapChain(
		(*_IUnknown)(unsafe.Pointer(d.dev)),
		&_DXGI_SWAP_CHAIN_DESC{
			BufferDesc: _DXGI_MODE_DESC{
				Format: _DXGI_FORMAT_R8G8B8A8_UNORM_SRGB,
			},
			SampleDesc: _DXGI_SAMPLE_DESC{
				Count: 1,
			},
			BufferUsage:  _DXGI_USAGE_RENDER_TARGET_OUTPUT,
			BufferCount:  1,
			OutputWindow: hwnd,
			Windowed:     1,
			SwapEffect:   _DXGI_SWAP_EFFECT_DISCARD,
		},
	)
	_IUnknownRelease(unsafe.Pointer(dxgiFactory), dxgiFactory.vtbl.Release)
	if err != nil {
		return nil, fmt.Errorf("NewContext: %v", err)
	}
	return &SwapChain{swchain: d3dswchain, fbo: &Framebuffer{}}, nil
}

func (s *SwapChain) Framebuffer(d *Device) (*Framebuffer, error) {
	if s.fbo.renderTarget != nil {
		return s.fbo, nil
	}
	desc, err := s.swchain.GetDesc()
	if err != nil {
		return nil, err
	}
	backBuffer, err := s.swchain.GetBuffer(0, &_IID_ID3D11Texture2D)
	if err != nil {
		return nil, err
	}
	texture := (*_ID3D11Resource)(unsafe.Pointer(backBuffer))
	renderTarget, err := d.dev.CreateRenderTargetView(texture)
	_IUnknownRelease(unsafe.Pointer(backBuffer), backBuffer.vtbl.Release)
	if err != nil {
		return nil, err
	}
	depthView, err := createDepthView(d.dev, int(desc.BufferDesc.Width), int(desc.BufferDesc.Height), 24)
	if err != nil {
		_IUnknownRelease(unsafe.Pointer(renderTarget), renderTarget.vtbl.Release)
		return nil, err
	}
	s.fbo.renderTarget = renderTarget
	s.fbo.depthView = depthView
	s.fbo.dev = d
	return s.fbo, nil
}

func (d *Device) Release() {
	_IUnknownRelease(unsafe.Pointer(d.ctx), d.ctx.vtbl.Release)
	_IUnknownRelease(unsafe.Pointer(d.dev), d.dev.vtbl.Release)
	d.ctx = nil
	d.dev = nil
	for _, state := range d.depthStates {
		_IUnknownRelease(unsafe.Pointer(state), state.vtbl.Release)
	}
	d.depthStates = nil
	for _, state := range d.blendStates {
		_IUnknownRelease(unsafe.Pointer(state), state.vtbl.Release)
	}
	d.blendStates = nil
}

func (s *SwapChain) Resize() error {
	if s.fbo.renderTarget != nil {
		s.fbo.Release()
	}
	return s.swchain.ResizeBuffers(0, 0, 0, _DXGI_FORMAT_UNKNOWN, 0)
}

func (s *SwapChain) Release() {
	_IUnknownRelease(unsafe.Pointer(s.swchain), s.swchain.vtbl.Release)
}

func (s *SwapChain) Present() error {
	return s.swchain.Present(1, 0)
}

func NewBackend(d *Device) (*Backend, error) {
	caps := backend.Caps{
		MaxTextureSize: 2048, // 9.1 maximum
	}
	switch {
	case d.featLvl >= _D3D_FEATURE_LEVEL_11_0:
		caps.MaxTextureSize = 16384
	case d.featLvl >= _D3D_FEATURE_LEVEL_9_3:
		caps.MaxTextureSize = 4096
	}
	b := &Backend{dev: d, caps: caps}
	// Disable backface culling to match OpenGL.
	state, err := b.dev.dev.CreateRasterizerState(&_D3D11_RASTERIZER_DESC{
		CullMode:        _D3D11_CULL_NONE,
		FillMode:        _D3D11_FILL_SOLID,
		DepthClipEnable: 1,
	})
	// Enable depth mask to match OpenGL.
	b.depthState.mask = true
	if err != nil {
		return nil, err
	}
	b.dev.ctx.RSSetState(state)
	_IUnknownRelease(unsafe.Pointer(state), state.vtbl.Release)
	return b, nil
}

func (b *Backend) BeginFrame() {
}

func (b *Backend) EndFrame() {
}

func (b *Backend) Caps() backend.Caps {
	return b.caps
}

func (b *Backend) NewTimer() backend.Timer {
	panic("timers not supported")
}

func (b *Backend) IsTimeContinuous() bool {
	panic("timers not supported")
}

func (b *Backend) NewTexture(format backend.TextureFormat, width, height int, minFilter, magFilter backend.TextureFilter, bindings backend.BufferBinding) (backend.Texture, error) {
	var d3dfmt uint32
	switch format {
	case backend.TextureFormatFloat:
		d3dfmt = b.dev.floatFormat
	case backend.TextureFormatSRGB:
		d3dfmt = _DXGI_FORMAT_R8G8B8A8_UNORM_SRGB
	default:
		return nil, fmt.Errorf("unsupported texture format %d", format)
	}
	tex, err := b.dev.dev.CreateTexture2D(&_D3D11_TEXTURE2D_DESC{
		Width:     uint32(width),
		Height:    uint32(height),
		MipLevels: 1,
		ArraySize: 1,
		Format:    d3dfmt,
		SampleDesc: _DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		BindFlags: convBufferBinding(bindings),
	})
	if err != nil {
		return nil, err
	}
	var (
		sampler *_ID3D11SamplerState
		resView *_ID3D11ShaderResourceView
	)
	if bindings&backend.BufferBindingTexture != 0 {
		var filter uint32
		switch {
		case minFilter == backend.FilterNearest && magFilter == backend.FilterNearest:
			filter = _D3D11_FILTER_MIN_MAG_MIP_POINT
		case minFilter == backend.FilterLinear && magFilter == backend.FilterLinear:
			filter = _D3D11_FILTER_MIN_MAG_LINEAR_MIP_POINT
		default:
			_IUnknownRelease(unsafe.Pointer(tex), tex.vtbl.Release)
			return nil, fmt.Errorf("unsupported texture filter combination %d, %d", minFilter, magFilter)
		}
		var err error
		sampler, err = b.dev.dev.CreateSamplerState(&_D3D11_SAMPLER_DESC{
			Filter:        filter,
			AddressU:      _D3D11_TEXTURE_ADDRESS_CLAMP,
			AddressV:      _D3D11_TEXTURE_ADDRESS_CLAMP,
			AddressW:      _D3D11_TEXTURE_ADDRESS_CLAMP,
			MaxAnisotropy: 1,
			MinLOD:        -math.MaxFloat32,
			MaxLOD:        math.MaxFloat32,
		})
		if err != nil {
			_IUnknownRelease(unsafe.Pointer(tex), tex.vtbl.Release)
			return nil, err
		}
		resView, err = b.dev.dev.CreateShaderResourceViewTEX2D(
			(*_ID3D11Resource)(unsafe.Pointer(tex)),
			&_D3D11_SHADER_RESOURCE_VIEW_DESC_TEX2D{
				_D3D11_SHADER_RESOURCE_VIEW_DESC: _D3D11_SHADER_RESOURCE_VIEW_DESC{
					Format:        d3dfmt,
					ViewDimension: _D3D11_SRV_DIMENSION_TEXTURE2D,
				},
				Texture2D: _D3D11_TEX2D_SRV{
					MostDetailedMip: 0,
					MipLevels:       ^uint32(0),
				},
			},
		)
		if err != nil {
			_IUnknownRelease(unsafe.Pointer(tex), tex.vtbl.Release)
			_IUnknownRelease(unsafe.Pointer(sampler), sampler.vtbl.Release)
			return nil, err
		}
	}
	return &Texture{backend: b, format: d3dfmt, tex: tex, sampler: sampler, resView: resView, bindings: bindings, width: width, height: height}, nil
}

func (b *Backend) CurrentFramebuffer() backend.Framebuffer {
	renderTarget := b.dev.ctx.OMGetRenderTargets()
	if renderTarget != nil {
		// Assume someone else is holding on to it.
		_IUnknownRelease(unsafe.Pointer(renderTarget), renderTarget.vtbl.Release)
	}
	if renderTarget == b.fbo.renderTarget {
		return b.fbo
	}
	return &Framebuffer{dev: b.dev, renderTarget: renderTarget, foreign: true}
}

func (b *Backend) NewFramebuffer(tex backend.Texture, depthBits int) (backend.Framebuffer, error) {
	d3dtex := tex.(*Texture)
	if d3dtex.bindings&backend.BufferBindingFramebuffer == 0 {
		return nil, errors.New("the texture was created without BufferBindingFramebuffer binding")
	}
	resource := (*_ID3D11Resource)(unsafe.Pointer(d3dtex.tex))
	renderTarget, err := b.dev.dev.CreateRenderTargetView(resource)
	if err != nil {
		return nil, err
	}
	fbo := &Framebuffer{dev: b.dev, format: d3dtex.format, resource: resource, renderTarget: renderTarget}
	if depthBits > 0 {
		depthView, err := createDepthView(b.dev.dev, d3dtex.width, d3dtex.height, depthBits)
		if err != nil {
			_IUnknownRelease(unsafe.Pointer(renderTarget), renderTarget.vtbl.Release)
			return nil, err
		}
		fbo.depthView = depthView
	}
	return fbo, nil
}

func createDepthView(d *_ID3D11Device, width, height, depthBits int) (*_ID3D11DepthStencilView, error) {
	depthTex, err := d.CreateTexture2D(&_D3D11_TEXTURE2D_DESC{
		Width:     uint32(width),
		Height:    uint32(height),
		MipLevels: 1,
		ArraySize: 1,
		Format:    _DXGI_FORMAT_D24_UNORM_S8_UINT,
		SampleDesc: _DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		BindFlags: _D3D11_BIND_DEPTH_STENCIL,
	})
	if err != nil {
		return nil, err
	}
	depthView, err := d.CreateDepthStencilViewTEX2D(
		(*_ID3D11Resource)(unsafe.Pointer(depthTex)),
		&_D3D11_DEPTH_STENCIL_VIEW_DESC_TEX2D{
			Format:        _DXGI_FORMAT_D24_UNORM_S8_UINT,
			ViewDimension: _D3D11_DSV_DIMENSION_TEXTURE2D,
		},
	)
	_IUnknownRelease(unsafe.Pointer(depthTex), depthTex.vtbl.Release)
	return depthView, err
}

func (b *Backend) NewInputLayout(vertexShader backend.ShaderSources, layout []backend.InputDesc) (backend.InputLayout, error) {
	if len(vertexShader.Inputs) != len(layout) {
		return nil, fmt.Errorf("NewInputLayout: got %d inputs, expected %d", len(layout), len(vertexShader.Inputs))
	}
	descs := make([]_D3D11_INPUT_ELEMENT_DESC, len(layout))
	for i, l := range layout {
		inp := vertexShader.Inputs[i]
		cname, err := windows.BytePtrFromString(inp.Semantic)
		if err != nil {
			return nil, err
		}
		var format uint32
		switch l.Type {
		case backend.DataTypeFloat:
			switch l.Size {
			case 1:
				format = _DXGI_FORMAT_R32_FLOAT
			case 2:
				format = _DXGI_FORMAT_R32G32_FLOAT
			case 3:
				format = _DXGI_FORMAT_R32G32B32_FLOAT
			case 4:
				format = _DXGI_FORMAT_R32G32B32A32_FLOAT
			default:
				panic("unsupported float data size")
			}
		case backend.DataTypeShort:
			switch l.Size {
			case 1:
				format = _DXGI_FORMAT_R16_SINT
			case 2:
				format = _DXGI_FORMAT_R16G16_SINT
			default:
				panic("unsupported float data size")
			}
		default:
			panic("unsupported data type")
		}
		descs[i] = _D3D11_INPUT_ELEMENT_DESC{
			SemanticName:      cname,
			SemanticIndex:     uint32(inp.SemanticIndex),
			Format:            format,
			AlignedByteOffset: uint32(l.Offset),
		}
	}
	l, err := b.dev.dev.CreateInputLayout(descs, vertexShader.HLSL)
	if err != nil {
		return nil, err
	}
	return &InputLayout{dev: b.dev, layout: l}, nil
}

func (b *Backend) NewBuffer(typ backend.BufferBinding, size int) (backend.Buffer, error) {
	if typ&backend.BufferBindingUniforms != 0 {
		if typ != backend.BufferBindingUniforms {
			return nil, errors.New("uniform buffers cannot have other bindings")
		}
		if size%16 != 0 {
			return nil, fmt.Errorf("constant buffer size is %d, expected a multiple of 16", size)
		}
	}
	bind := convBufferBinding(typ)
	buf, err := b.dev.dev.CreateBuffer(&_D3D11_BUFFER_DESC{
		ByteWidth: uint32(size),
		BindFlags: bind,
	}, nil)
	if err != nil {
		return nil, err
	}
	return &Buffer{backend: b, buf: buf, bind: bind}, nil
}

func (b *Backend) NewImmutableBuffer(typ backend.BufferBinding, data []byte) (backend.Buffer, error) {
	if typ&backend.BufferBindingUniforms != 0 {
		if typ != backend.BufferBindingUniforms {
			return nil, errors.New("uniform buffers cannot have other bindings")
		}
		if len(data)%16 != 0 {
			return nil, fmt.Errorf("constant buffer size is %d, expected a multiple of 16", len(data))
		}
	}
	bind := convBufferBinding(typ)
	buf, err := b.dev.dev.CreateBuffer(&_D3D11_BUFFER_DESC{
		ByteWidth: uint32(len(data)),
		Usage:     _D3D11_USAGE_IMMUTABLE,
		BindFlags: bind,
	}, data)
	if err != nil {
		return nil, err
	}
	return &Buffer{backend: b, buf: buf, bind: bind, immutable: true}, nil
}

func (b *Backend) NewProgram(vertexShader, fragmentShader backend.ShaderSources) (backend.Program, error) {
	vs, err := b.dev.dev.CreateVertexShader(vertexShader.HLSL)
	if err != nil {
		return nil, err
	}
	ps, err := b.dev.dev.CreatePixelShader(fragmentShader.HLSL)
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
	b.dev.ctx.ClearRenderTargetView(b.fbo.renderTarget, &b.clearColor)
}

func (b *Backend) ClearDepth(depth float32) {
	if b.fbo.depthView != nil {
		b.dev.ctx.ClearDepthStencilView(b.fbo.depthView, _D3D11_CLEAR_DEPTH|_D3D11_CLEAR_STENCIL, depth, 0)
	}
}

func (b *Backend) Viewport(x, y, width, height int) {
	b.viewport = _D3D11_VIEWPORT{
		TopLeftX: float32(x),
		TopLeftY: float32(y),
		Width:    float32(width),
		Height:   float32(height),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}
	b.dev.ctx.RSSetViewports(&b.viewport)
}

func (b *Backend) DrawArrays(mode backend.DrawMode, off, count int) {
	b.prepareDraw(mode)
	b.dev.ctx.Draw(uint32(count), uint32(off))
}

func (b *Backend) DrawElements(mode backend.DrawMode, off, count int) {
	b.prepareDraw(mode)
	b.dev.ctx.DrawIndexed(uint32(count), uint32(off), 0)
}

func (b *Backend) prepareDraw(mode backend.DrawMode) {
	if p := b.prog; p != nil {
		b.dev.ctx.VSSetShader(p.vert.shader)
		b.dev.ctx.PSSetShader(p.frag.shader)
		if buf := p.vert.uniforms; buf != nil {
			b.dev.ctx.VSSetConstantBuffers(buf.buf)
		}
		if buf := p.frag.uniforms; buf != nil {
			b.dev.ctx.PSSetConstantBuffers(buf.buf)
		}
	}
	var topology uint32
	switch mode {
	case backend.DrawModeTriangles:
		topology = _D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST
	case backend.DrawModeTriangleStrip:
		topology = _D3D11_PRIMITIVE_TOPOLOGY_TRIANGLESTRIP
	default:
		panic("unsupported draw mode")
	}
	b.dev.ctx.IASetPrimitiveTopology(topology)

	depthState, ok := b.dev.depthStates[b.depthState]
	if !ok {
		var desc _D3D11_DEPTH_STENCIL_DESC
		if b.depthState.enable {
			desc.DepthEnable = 1
		}
		if b.depthState.mask {
			desc.DepthWriteMask = _D3D11_DEPTH_WRITE_MASK_ALL
		}
		switch b.depthState.fn {
		case backend.DepthFuncGreater:
			desc.DepthFunc = _D3D11_COMPARISON_GREATER
		case backend.DepthFuncGreaterEqual:
			desc.DepthFunc = _D3D11_COMPARISON_GREATER_EQUAL
		default:
			panic("unsupported depth func")
		}
		var err error
		depthState, err = b.dev.dev.CreateDepthStencilState(&desc)
		if err != nil {
			panic(err)
		}
		b.dev.depthStates[b.depthState] = depthState
	}
	b.dev.ctx.OMSetDepthStencilState(depthState, 0)

	blendState, ok := b.dev.blendStates[b.blendState]
	if !ok {
		var desc _D3D11_BLEND_DESC
		t0 := &desc.RenderTarget[0]
		t0.RenderTargetWriteMask = _D3D11_COLOR_WRITE_ENABLE_ALL
		t0.BlendOp = _D3D11_BLEND_OP_ADD
		t0.BlendOpAlpha = _D3D11_BLEND_OP_ADD
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
		blendState, err = b.dev.dev.CreateBlendState(&desc)
		if err != nil {
			panic(err)
		}
		b.dev.blendStates[b.blendState] = blendState
	}
	b.dev.ctx.OMSetBlendState(blendState, nil, 0xffffffff)
}

func (b *Backend) DepthFunc(f backend.DepthFunc) {
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

func (b *Backend) BlendFunc(sfactor, dfactor backend.BlendFactor) {
	b.blendState.sfactor = sfactor
	b.blendState.dfactor = dfactor
}

func (t *Texture) Upload(img *image.RGBA) {
	b := img.Bounds()
	w := b.Dx()
	if img.Stride != w*4 {
		panic("unsupported stride")
	}
	start := (b.Min.X + b.Min.Y*w) * 4
	end := (b.Max.X + (b.Max.Y-1)*w) * 4
	pixels := img.Pix[start:end]
	res := (*_ID3D11Resource)(unsafe.Pointer(t.tex))
	t.backend.dev.ctx.UpdateSubresource(res, uint32(img.Stride), uint32(len(pixels)), pixels)
}

func (t *Texture) Release() {
	_IUnknownRelease(unsafe.Pointer(t.tex), t.tex.vtbl.Release)
	t.tex = nil
	if t.sampler != nil {
		_IUnknownRelease(unsafe.Pointer(t.sampler), t.sampler.vtbl.Release)
		t.sampler = nil
	}
	if t.resView != nil {
		_IUnknownRelease(unsafe.Pointer(t.resView), t.resView.vtbl.Release)
		t.resView = nil
	}
}

func (b *Backend) BindTexture(unit int, tex backend.Texture) {
	t := tex.(*Texture)
	b.dev.ctx.PSSetSamplers(uint32(unit), t.sampler)
	b.dev.ctx.PSSetShaderResources(uint32(unit), t.resView)
}

func (b *Backend) BindProgram(prog backend.Program) {
	b.prog = prog.(*Program)
}

func (p *Program) Release() {
	_IUnknownRelease(unsafe.Pointer(p.vert.shader), p.vert.shader.vtbl.Release)
	_IUnknownRelease(unsafe.Pointer(p.frag.shader), p.frag.shader.vtbl.Release)
	p.vert.shader = nil
	p.frag.shader = nil
}

func (p *Program) SetVertexUniforms(buf backend.Buffer) {
	p.vert.uniforms = buf.(*Buffer)
}

func (p *Program) SetFragmentUniforms(buf backend.Buffer) {
	p.frag.uniforms = buf.(*Buffer)
}

func (b *Backend) BindVertexBuffer(buf backend.Buffer, stride, offset int) {
	b.dev.ctx.IASetVertexBuffers(buf.(*Buffer).buf, uint32(stride), uint32(offset))
}

func (b *Backend) BindIndexBuffer(buf backend.Buffer) {
	b.dev.ctx.IASetIndexBuffer(buf.(*Buffer).buf, _DXGI_FORMAT_R16_UINT, 0)
}

func (b *Buffer) Upload(data []byte) {
	b.backend.dev.ctx.UpdateSubresource((*_ID3D11Resource)(unsafe.Pointer(b.buf)), 0, 0, data)
}

func (b *Buffer) Release() {
	_IUnknownRelease(unsafe.Pointer(b.buf), b.buf.vtbl.Release)
	b.buf = nil
}

func (f *Framebuffer) ReadPixels(src image.Rectangle, pixels []byte) error {
	if f.resource == nil {
		return errors.New("framebuffer does not support ReadPixels")
	}
	w, h := src.Dx(), src.Dy()
	tex, err := f.dev.dev.CreateTexture2D(&_D3D11_TEXTURE2D_DESC{
		Width:     uint32(w),
		Height:    uint32(h),
		MipLevels: 1,
		ArraySize: 1,
		Format:    f.format,
		SampleDesc: _DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		Usage:          _D3D11_USAGE_STAGING,
		CPUAccessFlags: _D3D11_CPU_ACCESS_READ,
	})
	if err != nil {
		return fmt.Errorf("ReadPixels: %v", err)
	}
	defer _IUnknownRelease(unsafe.Pointer(tex), tex.vtbl.Release)
	res := (*_ID3D11Resource)(unsafe.Pointer(tex))
	f.dev.ctx.CopySubresourceRegion(
		res,
		0,       // Destination subresource.
		0, 0, 0, // Destination coordinates (x, y, z).
		f.resource,
		0, // Source subresource.
		&_D3D11_BOX{
			left:   uint32(src.Min.X),
			top:    uint32(src.Min.Y),
			right:  uint32(src.Max.X),
			bottom: uint32(src.Max.Y),
			front:  0,
			back:   1,
		},
	)
	resMap, err := f.dev.ctx.Map(res, 0, _D3D11_MAP_READ, 0)
	if err != nil {
		return fmt.Errorf("ReadPixels: %v", err)
	}
	defer f.dev.ctx.Unmap(res, 0)
	srcPitch := w * 4
	dstPitch := int(resMap.RowPitch)
	mapSize := dstPitch * h
	data := gunsafe.SliceOf(resMap.pData)[:mapSize:mapSize]
	width := w * 4
	for r := 0; r < h; r++ {
		pixels := pixels[r*srcPitch:]
		copy(pixels[:width], data[r*dstPitch:])
	}
	return nil
}

func (b *Backend) BindFramebuffer(fbo backend.Framebuffer) {
	b.fbo = fbo.(*Framebuffer)
	b.dev.ctx.OMSetRenderTargets(b.fbo.renderTarget, b.fbo.depthView)
}

func (f *Framebuffer) Invalidate() {
}

func (f *Framebuffer) Release() {
	if f.foreign {
		panic("cannot release Framebuffer from CurrentFramebuffer")
	}
	if f.renderTarget != nil {
		_IUnknownRelease(unsafe.Pointer(f.renderTarget), f.renderTarget.vtbl.Release)
		f.renderTarget = nil
	}
	if f.depthView != nil {
		_IUnknownRelease(unsafe.Pointer(f.depthView), f.depthView.vtbl.Release)
		f.depthView = nil
	}
}

func (b *Backend) BindInputLayout(layout backend.InputLayout) {
	b.dev.ctx.IASetInputLayout(layout.(*InputLayout).layout)
}

func (l *InputLayout) Release() {
	_IUnknownRelease(unsafe.Pointer(l.layout), l.layout.vtbl.Release)
	l.layout = nil
}

func convBufferBinding(typ backend.BufferBinding) uint32 {
	var bindings uint32
	if typ&backend.BufferBindingVertices != 0 {
		bindings |= _D3D11_BIND_VERTEX_BUFFER
	}
	if typ&backend.BufferBindingIndices != 0 {
		bindings |= _D3D11_BIND_INDEX_BUFFER
	}
	if typ&backend.BufferBindingUniforms != 0 {
		bindings |= _D3D11_BIND_CONSTANT_BUFFER
	}
	if typ&backend.BufferBindingTexture != 0 {
		bindings |= _D3D11_BIND_SHADER_RESOURCE
	}
	if typ&backend.BufferBindingFramebuffer != 0 {
		bindings |= _D3D11_BIND_RENDER_TARGET
	}
	return bindings
}

func toBlendFactor(f backend.BlendFactor) (uint32, uint32) {
	switch f {
	case backend.BlendFactorOne:
		return _D3D11_BLEND_ONE, _D3D11_BLEND_ONE
	case backend.BlendFactorOneMinusSrcAlpha:
		return _D3D11_BLEND_INV_SRC_ALPHA, _D3D11_BLEND_INV_SRC_ALPHA
	case backend.BlendFactorZero:
		return _D3D11_BLEND_ZERO, _D3D11_BLEND_ZERO
	case backend.BlendFactorDstColor:
		return _D3D11_BLEND_DEST_COLOR, _D3D11_BLEND_DEST_ALPHA
	default:
		panic("unsupported blend source factor")
	}
}
