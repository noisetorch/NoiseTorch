// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"

	"gioui.org/internal/f32color"

	"golang.org/x/sys/windows"
)

type _DXGI_SWAP_CHAIN_DESC struct {
	BufferDesc   _DXGI_MODE_DESC
	SampleDesc   _DXGI_SAMPLE_DESC
	BufferUsage  uint32
	BufferCount  uint32
	OutputWindow windows.Handle
	Windowed     uint32
	SwapEffect   uint32
	Flags        uint32
}

type _DXGI_SAMPLE_DESC struct {
	Count   uint32
	Quality uint32
}

type _DXGI_MODE_DESC struct {
	Width            uint32
	Height           uint32
	RefreshRate      _DXGI_RATIONAL
	Format           uint32
	ScanlineOrdering uint32
	Scaling          uint32
}

type _DXGI_RATIONAL struct {
	Numerator   uint32
	Denominator uint32
}

type _D3D11_TEXTURE2D_DESC struct {
	Width          uint32
	Height         uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32
	SampleDesc     _DXGI_SAMPLE_DESC
	Usage          uint32
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

type _D3D11_SAMPLER_DESC struct {
	Filter         uint32
	AddressU       uint32
	AddressV       uint32
	AddressW       uint32
	MipLODBias     float32
	MaxAnisotropy  uint32
	ComparisonFunc uint32
	BorderColor    [4]float32
	MinLOD         float32
	MaxLOD         float32
}

type _D3D11_SHADER_RESOURCE_VIEW_DESC_TEX2D struct {
	_D3D11_SHADER_RESOURCE_VIEW_DESC
	Texture2D _D3D11_TEX2D_SRV
}

type _D3D11_SHADER_RESOURCE_VIEW_DESC struct {
	Format        uint32
	ViewDimension uint32
}

type _D3D11_TEX2D_SRV struct {
	MostDetailedMip uint32
	MipLevels       uint32
}

type _D3D11_INPUT_ELEMENT_DESC struct {
	SemanticName         *byte
	SemanticIndex        uint32
	Format               uint32
	InputSlot            uint32
	AlignedByteOffset    uint32
	InputSlotClass       uint32
	InstanceDataStepRate uint32
}

type _IDXGISwapChain struct {
	vtbl *struct {
		_IUnknownVTbl
		SetPrivateData          uintptr
		SetPrivateDataInterface uintptr
		GetPrivateData          uintptr
		GetParent               uintptr
		GetDevice               uintptr
		Present                 uintptr
		GetBuffer               uintptr
		SetFullscreenState      uintptr
		GetFullscreenState      uintptr
		GetDesc                 uintptr
		ResizeBuffers           uintptr
		ResizeTarget            uintptr
		GetContainingOutput     uintptr
		GetFrameStatistics      uintptr
		GetLastPresentCount     uintptr
	}
}

type _ID3D11Device struct {
	vtbl *struct {
		_IUnknownVTbl
		CreateBuffer                         uintptr
		CreateTexture1D                      uintptr
		CreateTexture2D                      uintptr
		CreateTexture3D                      uintptr
		CreateShaderResourceView             uintptr
		CreateUnorderedAccessView            uintptr
		CreateRenderTargetView               uintptr
		CreateDepthStencilView               uintptr
		CreateInputLayout                    uintptr
		CreateVertexShader                   uintptr
		CreateGeometryShader                 uintptr
		CreateGeometryShaderWithStreamOutput uintptr
		CreatePixelShader                    uintptr
		CreateHullShader                     uintptr
		CreateDomainShader                   uintptr
		CreateComputeShader                  uintptr
		CreateClassLinkage                   uintptr
		CreateBlendState                     uintptr
		CreateDepthStencilState              uintptr
		CreateRasterizerState                uintptr
		CreateSamplerState                   uintptr
		CreateQuery                          uintptr
		CreatePredicate                      uintptr
		CreateCounter                        uintptr
		CreateDeferredContext                uintptr
		OpenSharedResource                   uintptr
		CheckFormatSupport                   uintptr
		CheckMultisampleQualityLevels        uintptr
		CheckCounterInfo                     uintptr
		CheckCounter                         uintptr
		CheckFeatureSupport                  uintptr
		GetPrivateData                       uintptr
		SetPrivateData                       uintptr
		SetPrivateDataInterface              uintptr
		GetFeatureLevel                      uintptr
		GetCreationFlags                     uintptr
		GetDeviceRemovedReason               uintptr
		GetImmediateContext                  uintptr
		SetExceptionMode                     uintptr
		GetExceptionMode                     uintptr
	}
}

type _ID3D11DeviceContext struct {
	vtbl *struct {
		_IUnknownVTbl
		GetDevice                                 uintptr
		GetPrivateData                            uintptr
		SetPrivateData                            uintptr
		SetPrivateDataInterface                   uintptr
		VSSetConstantBuffers                      uintptr
		PSSetShaderResources                      uintptr
		PSSetShader                               uintptr
		PSSetSamplers                             uintptr
		VSSetShader                               uintptr
		DrawIndexed                               uintptr
		Draw                                      uintptr
		Map                                       uintptr
		Unmap                                     uintptr
		PSSetConstantBuffers                      uintptr
		IASetInputLayout                          uintptr
		IASetVertexBuffers                        uintptr
		IASetIndexBuffer                          uintptr
		DrawIndexedInstanced                      uintptr
		DrawInstanced                             uintptr
		GSSetConstantBuffers                      uintptr
		GSSetShader                               uintptr
		IASetPrimitiveTopology                    uintptr
		VSSetShaderResources                      uintptr
		VSSetSamplers                             uintptr
		Begin                                     uintptr
		End                                       uintptr
		GetData                                   uintptr
		SetPredication                            uintptr
		GSSetShaderResources                      uintptr
		GSSetSamplers                             uintptr
		OMSetRenderTargets                        uintptr
		OMSetRenderTargetsAndUnorderedAccessViews uintptr
		OMSetBlendState                           uintptr
		OMSetDepthStencilState                    uintptr
		SOSetTargets                              uintptr
		DrawAuto                                  uintptr
		DrawIndexedInstancedIndirect              uintptr
		DrawInstancedIndirect                     uintptr
		Dispatch                                  uintptr
		DispatchIndirect                          uintptr
		RSSetState                                uintptr
		RSSetViewports                            uintptr
		RSSetScissorRects                         uintptr
		CopySubresourceRegion                     uintptr
		CopyResource                              uintptr
		UpdateSubresource                         uintptr
		CopyStructureCount                        uintptr
		ClearRenderTargetView                     uintptr
		ClearUnorderedAccessViewUint              uintptr
		ClearUnorderedAccessViewFloat             uintptr
		ClearDepthStencilView                     uintptr
		GenerateMips                              uintptr
		SetResourceMinLOD                         uintptr
		GetResourceMinLOD                         uintptr
		ResolveSubresource                        uintptr
		ExecuteCommandList                        uintptr
		HSSetShaderResources                      uintptr
		HSSetShader                               uintptr
		HSSetSamplers                             uintptr
		HSSetConstantBuffers                      uintptr
		DSSetShaderResources                      uintptr
		DSSetShader                               uintptr
		DSSetSamplers                             uintptr
		DSSetConstantBuffers                      uintptr
		CSSetShaderResources                      uintptr
		CSSetUnorderedAccessViews                 uintptr
		CSSetShader                               uintptr
		CSSetSamplers                             uintptr
		CSSetConstantBuffers                      uintptr
		VSGetConstantBuffers                      uintptr
		PSGetShaderResources                      uintptr
		PSGetShader                               uintptr
		PSGetSamplers                             uintptr
		VSGetShader                               uintptr
		PSGetConstantBuffers                      uintptr
		IAGetInputLayout                          uintptr
		IAGetVertexBuffers                        uintptr
		IAGetIndexBuffer                          uintptr
		GSGetConstantBuffers                      uintptr
		GSGetShader                               uintptr
		IAGetPrimitiveTopology                    uintptr
		VSGetShaderResources                      uintptr
		VSGetSamplers                             uintptr
		GetPredication                            uintptr
		GSGetShaderResources                      uintptr
		GSGetSamplers                             uintptr
		OMGetRenderTargets                        uintptr
		OMGetRenderTargetsAndUnorderedAccessViews uintptr
		OMGetBlendState                           uintptr
		OMGetDepthStencilState                    uintptr
		SOGetTargets                              uintptr
		RSGetState                                uintptr
		RSGetViewports                            uintptr
		RSGetScissorRects                         uintptr
		HSGetShaderResources                      uintptr
		HSGetShader                               uintptr
		HSGetSamplers                             uintptr
		HSGetConstantBuffers                      uintptr
		DSGetShaderResources                      uintptr
		DSGetShader                               uintptr
		DSGetSamplers                             uintptr
		DSGetConstantBuffers                      uintptr
		CSGetShaderResources                      uintptr
		CSGetUnorderedAccessViews                 uintptr
		CSGetShader                               uintptr
		CSGetSamplers                             uintptr
		CSGetConstantBuffers                      uintptr
		ClearState                                uintptr
		Flush                                     uintptr
		GetType                                   uintptr
		GetContextFlags                           uintptr
		FinishCommandList                         uintptr
	}
}

type _ID3D11RenderTargetView struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11Resource struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11Texture2D struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11Buffer struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11SamplerState struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11PixelShader struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11ShaderResourceView struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11DepthStencilView struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11BlendState struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11DepthStencilState struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11VertexShader struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11RasterizerState struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _ID3D11InputLayout struct {
	vtbl *struct {
		_IUnknownVTbl
		GetBufferPointer uintptr
		GetBufferSize    uintptr
	}
}

type _D3D11_DEPTH_STENCIL_DESC struct {
	DepthEnable      uint32
	DepthWriteMask   uint32
	DepthFunc        uint32
	StencilEnable    uint32
	StencilReadMask  uint8
	StencilWriteMask uint8
	FrontFace        _D3D11_DEPTH_STENCILOP_DESC
	BackFace         _D3D11_DEPTH_STENCILOP_DESC
}

type _D3D11_DEPTH_STENCILOP_DESC struct {
	StencilFailOp      uint32
	StencilDepthFailOp uint32
	StencilPassOp      uint32
	StencilFunc        uint32
}

type _D3D11_DEPTH_STENCIL_VIEW_DESC_TEX2D struct {
	Format        uint32
	ViewDimension uint32
	Flags         uint32
	Texture2D     _D3D11_TEX2D_DSV
}

type _D3D11_TEX2D_DSV struct {
	MipSlice uint32
}

type _D3D11_BLEND_DESC struct {
	AlphaToCoverageEnable  uint32
	IndependentBlendEnable uint32
	RenderTarget           [8]_D3D11_RENDER_TARGET_BLEND_DESC
}

type _D3D11_RENDER_TARGET_BLEND_DESC struct {
	BlendEnable           uint32
	SrcBlend              uint32
	DestBlend             uint32
	BlendOp               uint32
	SrcBlendAlpha         uint32
	DestBlendAlpha        uint32
	BlendOpAlpha          uint32
	RenderTargetWriteMask uint8
}

type _IDXGIObject struct {
	vtbl *struct {
		_IUnknownVTbl
		SetPrivateData          uintptr
		SetPrivateDataInterface uintptr
		GetPrivateData          uintptr
		GetParent               uintptr
	}
}

type _IDXGIAdapter struct {
	vtbl *struct {
		_IUnknownVTbl
		SetPrivateData          uintptr
		SetPrivateDataInterface uintptr
		GetPrivateData          uintptr
		GetParent               uintptr
		EnumOutputs             uintptr
		GetDesc                 uintptr
		CheckInterfaceSupport   uintptr
		GetDesc1                uintptr
	}
}

type _IDXGIFactory struct {
	vtbl *struct {
		_IUnknownVTbl
		SetPrivateData          uintptr
		SetPrivateDataInterface uintptr
		GetPrivateData          uintptr
		GetParent               uintptr
		EnumAdapters            uintptr
		MakeWindowAssociation   uintptr
		GetWindowAssociation    uintptr
		CreateSwapChain         uintptr
		CreateSoftwareAdapter   uintptr
	}
}

type _IDXGIDevice struct {
	vtbl *struct {
		_IUnknownVTbl
		SetPrivateData          uintptr
		SetPrivateDataInterface uintptr
		GetPrivateData          uintptr
		GetParent               uintptr
		GetAdapter              uintptr
		CreateSurface           uintptr
		QueryResourceResidency  uintptr
		SetGPUThreadPriority    uintptr
		GetGPUThreadPriority    uintptr
	}
}

type _IUnknown struct {
	vtbl *struct {
		_IUnknownVTbl
	}
}

type _IUnknownVTbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type _D3D11_BUFFER_DESC struct {
	ByteWidth           uint32
	Usage               uint32
	BindFlags           uint32
	CPUAccessFlags      uint32
	MiscFlags           uint32
	StructureByteStride uint32
}

type _GUID struct {
	Data1   uint32
	Data2   uint16
	Data3   uint16
	Data4_0 uint8
	Data4_1 uint8
	Data4_2 uint8
	Data4_3 uint8
	Data4_4 uint8
	Data4_5 uint8
	Data4_6 uint8
	Data4_7 uint8
}

type _D3D11_VIEWPORT struct {
	TopLeftX float32
	TopLeftY float32
	Width    float32
	Height   float32
	MinDepth float32
	MaxDepth float32
}

type _D3D11_SUBRESOURCE_DATA struct {
	pSysMem *byte
}

type _D3D11_BOX struct {
	left   uint32
	top    uint32
	front  uint32
	right  uint32
	bottom uint32
	back   uint32
}

type _D3D11_MAPPED_SUBRESOURCE struct {
	pData      uintptr
	RowPitch   uint32
	DepthPitch uint32
}

type ErrorCode struct {
	Name string
	Code uint32
}

type _D3D11_RASTERIZER_DESC struct {
	FillMode              uint32
	CullMode              uint32
	FrontCounterClockwise uint32
	DepthBias             int32
	DepthBiasClamp        float32
	SlopeScaledDepthBias  float32
	DepthClipEnable       uint32
	ScissorEnable         uint32
	MultisampleEnable     uint32
	AntialiasedLineEnable uint32
}

var (
	_IID_ID3D11Texture2D = _GUID{0x6f15aaf2, 0xd208, 0x4e89, 0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}
	_IID_IDXGIDevice     = _GUID{0x54ec77fa, 0x1377, 0x44e6, 0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c}
	_IID_IDXGIFactory    = _GUID{0x7b7166ec, 0x21c7, 0x44ae, 0xb2, 0x1a, 0xc9, 0xae, 0x32, 0x1a, 0xe3, 0x69}
)

var (
	d3d11 = windows.NewLazySystemDLL("d3d11.dll")

	__D3D11CreateDevice             = d3d11.NewProc("D3D11CreateDevice")
	__D3D11CreateDeviceAndSwapChain = d3d11.NewProc("D3D11CreateDeviceAndSwapChain")
)

const (
	_D3D11_SDK_VERSION        = 7
	_D3D_DRIVER_TYPE_HARDWARE = 1

	_DXGI_FORMAT_UNKNOWN             = 0
	_DXGI_FORMAT_R16_FLOAT           = 54
	_DXGI_FORMAT_R32_FLOAT           = 41
	_DXGI_FORMAT_R32G32_FLOAT        = 16
	_DXGI_FORMAT_R32G32B32_FLOAT     = 6
	_DXGI_FORMAT_R32G32B32A32_FLOAT  = 2
	_DXGI_FORMAT_R8G8B8A8_UNORM_SRGB = 29
	_DXGI_FORMAT_R16_SINT            = 59
	_DXGI_FORMAT_R16G16_SINT         = 38
	_DXGI_FORMAT_R16_UINT            = 57
	_DXGI_FORMAT_D24_UNORM_S8_UINT   = 45
	_DXGI_FORMAT_R16G16_FLOAT        = 34
	_DXGI_FORMAT_R16G16B16A16_FLOAT  = 10

	_D3D11_FORMAT_SUPPORT_TEXTURE2D     = 0x20
	_D3D11_FORMAT_SUPPORT_RENDER_TARGET = 0x4000

	_DXGI_USAGE_RENDER_TARGET_OUTPUT = 1 << (1 + 4)

	_D3D11_CPU_ACCESS_READ = 0x20000

	_D3D11_MAP_READ = 1

	_DXGI_SWAP_EFFECT_DISCARD = 0

	_D3D_FEATURE_LEVEL_9_1  = 0x9100
	_D3D_FEATURE_LEVEL_9_3  = 0x9300
	_D3D_FEATURE_LEVEL_11_0 = 0xb000

	_D3D11_USAGE_IMMUTABLE = 1
	_D3D11_USAGE_STAGING   = 3

	_D3D11_BIND_VERTEX_BUFFER   = 0x1
	_D3D11_BIND_INDEX_BUFFER    = 0x2
	_D3D11_BIND_CONSTANT_BUFFER = 0x4
	_D3D11_BIND_SHADER_RESOURCE = 0x8
	_D3D11_BIND_RENDER_TARGET   = 0x20
	_D3D11_BIND_DEPTH_STENCIL   = 0x40

	_D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST  = 4
	_D3D11_PRIMITIVE_TOPOLOGY_TRIANGLESTRIP = 5

	_D3D11_FILTER_MIN_MAG_LINEAR_MIP_POINT = 0x14
	_D3D11_FILTER_MIN_MAG_MIP_POINT        = 0

	_D3D11_TEXTURE_ADDRESS_MIRROR = 2
	_D3D11_TEXTURE_ADDRESS_CLAMP  = 3
	_D3D11_TEXTURE_ADDRESS_WRAP   = 1

	_D3D11_SRV_DIMENSION_TEXTURE2D = 4

	_D3D11_CREATE_DEVICE_DEBUG = 0x2

	_D3D11_FILL_SOLID = 3

	_D3D11_CULL_NONE = 1

	_D3D11_CLEAR_DEPTH   = 0x1
	_D3D11_CLEAR_STENCIL = 0x2

	_D3D11_DSV_DIMENSION_TEXTURE2D = 3

	_D3D11_DEPTH_WRITE_MASK_ALL = 1

	_D3D11_COMPARISON_GREATER       = 5
	_D3D11_COMPARISON_GREATER_EQUAL = 7

	_D3D11_BLEND_OP_ADD        = 1
	_D3D11_BLEND_ONE           = 2
	_D3D11_BLEND_INV_SRC_ALPHA = 6
	_D3D11_BLEND_ZERO          = 1
	_D3D11_BLEND_DEST_COLOR    = 9
	_D3D11_BLEND_DEST_ALPHA    = 7

	_D3D11_COLOR_WRITE_ENABLE_ALL = 1 | 2 | 4 | 8

	DXGI_STATUS_OCCLUDED      = 0x087A0001
	DXGI_ERROR_DEVICE_RESET   = 0x887A0007
	DXGI_ERROR_DEVICE_REMOVED = 0x887A0005
	D3DDDIERR_DEVICEREMOVED   = 1<<31 | 0x876<<16 | 2160
)

func _D3D11CreateDevice(driverType uint32, flags uint32) (*_ID3D11Device, *_ID3D11DeviceContext, uint32, error) {
	var (
		dev     *_ID3D11Device
		ctx     *_ID3D11DeviceContext
		featLvl uint32
	)
	r, _, _ := __D3D11CreateDevice.Call(
		0,                                 // pAdapter
		uintptr(driverType),               // driverType
		0,                                 // Software
		uintptr(flags),                    // Flags
		0,                                 // pFeatureLevels
		0,                                 // FeatureLevels
		_D3D11_SDK_VERSION,                // SDKVersion
		uintptr(unsafe.Pointer(&dev)),     // ppDevice
		uintptr(unsafe.Pointer(&featLvl)), // pFeatureLevel
		uintptr(unsafe.Pointer(&ctx)),     // ppImmediateContext
	)
	if r != 0 {
		return nil, nil, 0, ErrorCode{Name: "D3D11CreateDevice", Code: uint32(r)}
	}
	return dev, ctx, featLvl, nil
}

func _D3D11CreateDeviceAndSwapChain(driverType uint32, flags uint32, swapDesc *_DXGI_SWAP_CHAIN_DESC) (*_ID3D11Device, *_ID3D11DeviceContext, *_IDXGISwapChain, uint32, error) {
	var (
		dev     *_ID3D11Device
		ctx     *_ID3D11DeviceContext
		swchain *_IDXGISwapChain
		featLvl uint32
	)
	r, _, _ := __D3D11CreateDeviceAndSwapChain.Call(
		0,                                 // pAdapter
		uintptr(driverType),               // driverType
		0,                                 // Software
		uintptr(flags),                    // Flags
		0,                                 // pFeatureLevels
		0,                                 // FeatureLevels
		_D3D11_SDK_VERSION,                // SDKVersion
		uintptr(unsafe.Pointer(swapDesc)), // pSwapChainDesc
		uintptr(unsafe.Pointer(&swchain)), // ppSwapChain
		uintptr(unsafe.Pointer(&dev)),     // ppDevice
		uintptr(unsafe.Pointer(&featLvl)), // pFeatureLevel
		uintptr(unsafe.Pointer(&ctx)),     // ppImmediateContext
	)
	if r != 0 {
		return nil, nil, nil, 0, ErrorCode{Name: "D3D11CreateDeviceAndSwapChain", Code: uint32(r)}
	}
	return dev, ctx, swchain, featLvl, nil
}

func (d *_ID3D11Device) CheckFormatSupport(format uint32) (uint32, error) {
	var support uint32
	r, _, _ := syscall.Syscall(
		d.vtbl.CheckFormatSupport,
		3,
		uintptr(unsafe.Pointer(d)),
		uintptr(format),
		uintptr(unsafe.Pointer(&support)),
	)
	if r != 0 {
		return 0, ErrorCode{Name: "ID3D11DeviceCheckFormatSupport", Code: uint32(r)}
	}
	return support, nil
}

func (d *_ID3D11Device) CreateBuffer(desc *_D3D11_BUFFER_DESC, data []byte) (*_ID3D11Buffer, error) {
	var dataDesc *_D3D11_SUBRESOURCE_DATA
	if len(data) > 0 {
		dataDesc = &_D3D11_SUBRESOURCE_DATA{
			pSysMem: &data[0],
		}
	}
	var buf *_ID3D11Buffer
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateBuffer,
		4,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(dataDesc)),
		uintptr(unsafe.Pointer(&buf)),
		0, 0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateBuffer", Code: uint32(r)}
	}
	return buf, nil
}

func (d *_ID3D11Device) CreateDepthStencilViewTEX2D(res *_ID3D11Resource, desc *_D3D11_DEPTH_STENCIL_VIEW_DESC_TEX2D) (*_ID3D11DepthStencilView, error) {
	var view *_ID3D11DepthStencilView
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateDepthStencilView,
		4,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(res)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&view)),
		0, 0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateDepthStencilView", Code: uint32(r)}
	}
	return view, nil
}

func (d *_ID3D11Device) CreatePixelShader(bytecode []byte) (*_ID3D11PixelShader, error) {
	var shader *_ID3D11PixelShader
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreatePixelShader,
		5,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(&bytecode[0])),
		uintptr(len(bytecode)),
		0, // pClassLinkage
		uintptr(unsafe.Pointer(&shader)),
		0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreatePixelShader", Code: uint32(r)}
	}
	return shader, nil
}

func (d *_ID3D11Device) CreateVertexShader(bytecode []byte) (*_ID3D11VertexShader, error) {
	var shader *_ID3D11VertexShader
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateVertexShader,
		5,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(&bytecode[0])),
		uintptr(len(bytecode)),
		0, // pClassLinkage
		uintptr(unsafe.Pointer(&shader)),
		0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateVertexShader", Code: uint32(r)}
	}
	return shader, nil
}

func (d *_ID3D11Device) CreateShaderResourceViewTEX2D(res *_ID3D11Resource, desc *_D3D11_SHADER_RESOURCE_VIEW_DESC_TEX2D) (*_ID3D11ShaderResourceView, error) {
	var resView *_ID3D11ShaderResourceView
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateShaderResourceView,
		4,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(res)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&resView)),
		0, 0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateShaderResourceView", Code: uint32(r)}
	}
	return resView, nil
}

func (d *_ID3D11Device) CreateRasterizerState(desc *_D3D11_RASTERIZER_DESC) (*_ID3D11RasterizerState, error) {
	var state *_ID3D11RasterizerState
	r, _, _ := syscall.Syscall(
		d.vtbl.CreateRasterizerState,
		3,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&state)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateRasterizerState", Code: uint32(r)}
	}
	return state, nil
}

func (d *_ID3D11Device) CreateInputLayout(descs []_D3D11_INPUT_ELEMENT_DESC, bytecode []byte) (*_ID3D11InputLayout, error) {
	var pdesc *_D3D11_INPUT_ELEMENT_DESC
	if len(descs) > 0 {
		pdesc = &descs[0]
	}
	var layout *_ID3D11InputLayout
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateInputLayout,
		6,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(pdesc)),
		uintptr(len(descs)),
		uintptr(unsafe.Pointer(&bytecode[0])),
		uintptr(len(bytecode)),
		uintptr(unsafe.Pointer(&layout)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateInputLayout", Code: uint32(r)}
	}
	return layout, nil
}

func (d *_ID3D11Device) CreateSamplerState(desc *_D3D11_SAMPLER_DESC) (*_ID3D11SamplerState, error) {
	var sampler *_ID3D11SamplerState
	r, _, _ := syscall.Syscall(
		d.vtbl.CreateSamplerState,
		3,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&sampler)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateSamplerState", Code: uint32(r)}
	}
	return sampler, nil
}

func (d *_ID3D11Device) CreateTexture2D(desc *_D3D11_TEXTURE2D_DESC) (*_ID3D11Texture2D, error) {
	var tex *_ID3D11Texture2D
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateTexture2D,
		4,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		0, // pInitialData
		uintptr(unsafe.Pointer(&tex)),
		0, 0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11CreateTexture2D", Code: uint32(r)}
	}
	return tex, nil
}

func (d *_ID3D11Device) CreateRenderTargetView(res *_ID3D11Resource) (*_ID3D11RenderTargetView, error) {
	var target *_ID3D11RenderTargetView
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateRenderTargetView,
		4,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(res)),
		0, // pDesc
		uintptr(unsafe.Pointer(&target)),
		0, 0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateRenderTargetView", Code: uint32(r)}
	}
	return target, nil
}

func (d *_ID3D11Device) CreateBlendState(desc *_D3D11_BLEND_DESC) (*_ID3D11BlendState, error) {
	var state *_ID3D11BlendState
	r, _, _ := syscall.Syscall(
		d.vtbl.CreateBlendState,
		3,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&state)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateBlendState", Code: uint32(r)}
	}
	return state, nil
}

func (d *_ID3D11Device) CreateDepthStencilState(desc *_D3D11_DEPTH_STENCIL_DESC) (*_ID3D11DepthStencilState, error) {
	var state *_ID3D11DepthStencilState
	r, _, _ := syscall.Syscall(
		d.vtbl.CreateDepthStencilState,
		3,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&state)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "ID3D11DeviceCreateDepthStencilState", Code: uint32(r)}
	}
	return state, nil
}

func (s *_IDXGISwapChain) GetDesc() (_DXGI_SWAP_CHAIN_DESC, error) {
	var desc _DXGI_SWAP_CHAIN_DESC
	r, _, _ := syscall.Syscall(
		s.vtbl.GetDesc,
		2,
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(&desc)),
		0,
	)
	if r != 0 {
		return _DXGI_SWAP_CHAIN_DESC{}, ErrorCode{Name: "IDXGISwapChainGetDesc", Code: uint32(r)}
	}
	return desc, nil
}

func (s *_IDXGISwapChain) ResizeBuffers(buffers, width, height, newFormat, flags uint32) error {
	r, _, _ := syscall.Syscall6(
		s.vtbl.ResizeBuffers,
		6,
		uintptr(unsafe.Pointer(s)),
		uintptr(buffers),
		uintptr(width),
		uintptr(height),
		uintptr(newFormat),
		uintptr(flags),
	)
	if r != 0 {
		return ErrorCode{Name: "IDXGISwapChainResizeBuffers", Code: uint32(r)}
	}
	return nil
}

func (s *_IDXGISwapChain) Present(SyncInterval int, Flags uint32) error {
	r, _, _ := syscall.Syscall(
		s.vtbl.Present,
		3,
		uintptr(unsafe.Pointer(s)),
		uintptr(SyncInterval),
		uintptr(Flags),
	)
	if r != 0 {
		return ErrorCode{Name: "IDXGISwapChainPresent", Code: uint32(r)}
	}
	return nil
}

func (s *_IDXGISwapChain) GetBuffer(index int, riid *_GUID) (*_IUnknown, error) {
	var buf *_IUnknown
	r, _, _ := syscall.Syscall6(
		s.vtbl.GetBuffer,
		4,
		uintptr(unsafe.Pointer(s)),
		uintptr(index),
		uintptr(unsafe.Pointer(riid)),
		uintptr(unsafe.Pointer(&buf)),
		0,
		0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "IDXGISwapChainGetBuffer", Code: uint32(r)}
	}
	return buf, nil
}

func (c *_ID3D11DeviceContext) Unmap(resource *_ID3D11Resource, subResource uint32) {
	syscall.Syscall(
		c.vtbl.Unmap,
		3,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(resource)),
		uintptr(subResource),
	)
}

func (c *_ID3D11DeviceContext) Map(resource *_ID3D11Resource, subResource, mapType, mapFlags uint32) (_D3D11_MAPPED_SUBRESOURCE, error) {
	var resMap _D3D11_MAPPED_SUBRESOURCE
	r, _, _ := syscall.Syscall6(
		c.vtbl.Map,
		6,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(resource)),
		uintptr(subResource),
		uintptr(mapType),
		uintptr(mapFlags),
		uintptr(unsafe.Pointer(&resMap)),
	)
	if r != 0 {
		return resMap, ErrorCode{Name: "ID3D11DeviceContextMap", Code: uint32(r)}
	}
	return resMap, nil
}

func (c *_ID3D11DeviceContext) CopySubresourceRegion(dst *_ID3D11Resource, dstSubresource, dstX, dstY, dstZ uint32, src *_ID3D11Resource, srcSubresource uint32, srcBox *_D3D11_BOX) {
	syscall.Syscall9(
		c.vtbl.CopySubresourceRegion,
		9,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(dstSubresource),
		uintptr(dstX),
		uintptr(dstY),
		uintptr(dstZ),
		uintptr(unsafe.Pointer(src)),
		uintptr(srcSubresource),
		uintptr(unsafe.Pointer(srcBox)),
	)
}

func (c *_ID3D11DeviceContext) ClearDepthStencilView(target *_ID3D11DepthStencilView, flags uint32, depth float32, stencil uint8) {
	syscall.Syscall6(
		c.vtbl.ClearDepthStencilView,
		5,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(target)),
		uintptr(flags),
		uintptr(math.Float32bits(depth)),
		uintptr(stencil),
		0,
	)
}

func (c *_ID3D11DeviceContext) ClearRenderTargetView(target *_ID3D11RenderTargetView, color *[4]float32) {
	syscall.Syscall(
		c.vtbl.ClearRenderTargetView,
		3,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(color)),
	)
}

func (c *_ID3D11DeviceContext) RSSetViewports(viewport *_D3D11_VIEWPORT) {
	syscall.Syscall(
		c.vtbl.RSSetViewports,
		3,
		uintptr(unsafe.Pointer(c)),
		1, // NumViewports
		uintptr(unsafe.Pointer(viewport)),
	)
}

func (c *_ID3D11DeviceContext) VSSetShader(s *_ID3D11VertexShader) {
	syscall.Syscall6(
		c.vtbl.VSSetShader,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)),
		0, // ppClassInstances
		0, // NumClassInstances
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) VSSetConstantBuffers(b *_ID3D11Buffer) {
	syscall.Syscall6(
		c.vtbl.VSSetConstantBuffers,
		4,
		uintptr(unsafe.Pointer(c)),
		0, // StartSlot
		1, // NumBuffers
		uintptr(unsafe.Pointer(&b)),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) PSSetConstantBuffers(b *_ID3D11Buffer) {
	syscall.Syscall6(
		c.vtbl.PSSetConstantBuffers,
		4,
		uintptr(unsafe.Pointer(c)),
		0, // StartSlot
		1, // NumBuffers
		uintptr(unsafe.Pointer(&b)),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) PSSetShaderResources(startSlot uint32, s *_ID3D11ShaderResourceView) {
	syscall.Syscall6(
		c.vtbl.PSSetShaderResources,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(startSlot),
		1, // NumViews
		uintptr(unsafe.Pointer(&s)),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) PSSetSamplers(startSlot uint32, s *_ID3D11SamplerState) {
	syscall.Syscall6(
		c.vtbl.PSSetSamplers,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(startSlot),
		1, // NumSamplers
		uintptr(unsafe.Pointer(&s)),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) PSSetShader(s *_ID3D11PixelShader) {
	syscall.Syscall6(
		c.vtbl.PSSetShader,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)),
		0, // ppClassInstances
		0, // NumClassInstances
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) UpdateSubresource(res *_ID3D11Resource, rowPitch, depthPitch uint32, data []byte) {
	syscall.Syscall9(
		c.vtbl.UpdateSubresource,
		7,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(res)),
		0, // DstSubresource
		0, // pDstBox
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(rowPitch),
		uintptr(depthPitch),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) RSSetState(state *_ID3D11RasterizerState) {
	syscall.Syscall(
		c.vtbl.RSSetState,
		2,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(state)),
		0,
	)
}

func (c *_ID3D11DeviceContext) IASetInputLayout(layout *_ID3D11InputLayout) {
	syscall.Syscall(
		c.vtbl.IASetInputLayout,
		2,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(layout)),
		0,
	)
}

func (c *_ID3D11DeviceContext) IASetIndexBuffer(buf *_ID3D11Buffer, format, offset uint32) {
	syscall.Syscall6(
		c.vtbl.IASetIndexBuffer,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(buf)),
		uintptr(format),
		uintptr(offset),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) IASetVertexBuffers(buf *_ID3D11Buffer, stride, offset uint32) {
	syscall.Syscall6(
		c.vtbl.IASetVertexBuffers,
		6,
		uintptr(unsafe.Pointer(c)),
		0, // StartSlot
		1, // NumBuffers,
		uintptr(unsafe.Pointer(&buf)),
		uintptr(unsafe.Pointer(&stride)),
		uintptr(unsafe.Pointer(&offset)),
	)
}

func (c *_ID3D11DeviceContext) IASetPrimitiveTopology(mode uint32) {
	syscall.Syscall(
		c.vtbl.IASetPrimitiveTopology,
		2,
		uintptr(unsafe.Pointer(c)),
		uintptr(mode),
		0,
	)
}

func (c *_ID3D11DeviceContext) OMGetRenderTargets() *_ID3D11RenderTargetView {
	var target *_ID3D11RenderTargetView
	syscall.Syscall6(
		c.vtbl.OMGetRenderTargets,
		4,
		uintptr(unsafe.Pointer(c)),
		1, // NumViews
		uintptr(unsafe.Pointer(&target)),
		0, // pDepthStencilView
		0, 0,
	)
	return target
}

func (c *_ID3D11DeviceContext) OMSetRenderTargets(target *_ID3D11RenderTargetView, depthStencil *_ID3D11DepthStencilView) {
	syscall.Syscall6(
		c.vtbl.OMSetRenderTargets,
		4,
		uintptr(unsafe.Pointer(c)),
		1, // NumViews
		uintptr(unsafe.Pointer(&target)),
		uintptr(unsafe.Pointer(depthStencil)),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) Draw(count, start uint32) {
	syscall.Syscall(
		c.vtbl.Draw,
		3,
		uintptr(unsafe.Pointer(c)),
		uintptr(count),
		uintptr(start),
	)
}

func (c *_ID3D11DeviceContext) DrawIndexed(count, start uint32, base int32) {
	syscall.Syscall6(
		c.vtbl.DrawIndexed,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(count),
		uintptr(start),
		uintptr(base),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) OMSetBlendState(state *_ID3D11BlendState, factor *f32color.RGBA, sampleMask uint32) {
	syscall.Syscall6(
		c.vtbl.OMSetBlendState,
		4,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(state)),
		uintptr(unsafe.Pointer(factor)),
		uintptr(sampleMask),
		0, 0,
	)
}

func (c *_ID3D11DeviceContext) OMSetDepthStencilState(state *_ID3D11DepthStencilState, stencilRef uint32) {
	syscall.Syscall(
		c.vtbl.OMSetDepthStencilState,
		3,
		uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(state)),
		uintptr(stencilRef),
	)
}

func (d *_IDXGIObject) GetParent(guid *_GUID) (*_IDXGIObject, error) {
	var parent *_IDXGIObject
	r, _, _ := syscall.Syscall(
		d.vtbl.GetParent,
		3,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(guid)),
		uintptr(unsafe.Pointer(&parent)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "IDXGIObjectGetParent", Code: uint32(r)}
	}
	return parent, nil
}

func (d *_IDXGIFactory) CreateSwapChain(device *_IUnknown, desc *_DXGI_SWAP_CHAIN_DESC) (*_IDXGISwapChain, error) {
	var swchain *_IDXGISwapChain
	r, _, _ := syscall.Syscall6(
		d.vtbl.CreateSwapChain,
		4,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(unsafe.Pointer(&swchain)),
		0, 0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "IDXGIFactory", Code: uint32(r)}
	}
	return swchain, nil
}

func (d *_IDXGIDevice) GetAdapter() (*_IDXGIAdapter, error) {
	var adapter *_IDXGIAdapter
	r, _, _ := syscall.Syscall(
		d.vtbl.GetAdapter,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(&adapter)),
		0,
	)
	if r != 0 {
		return nil, ErrorCode{Name: "IDXGIDeviceGetAdapter", Code: uint32(r)}
	}
	return adapter, nil
}

func _IUnknownQueryInterface(obj unsafe.Pointer, queryInterfaceMethod uintptr, guid *_GUID) (*_IUnknown, error) {
	var ref *_IUnknown
	r, _, _ := syscall.Syscall(
		queryInterfaceMethod,
		3,
		uintptr(obj),
		uintptr(unsafe.Pointer(guid)),
		uintptr(unsafe.Pointer(&ref)),
	)
	if r != 0 {
		return nil, ErrorCode{Name: "IUnknownQueryInterface", Code: uint32(r)}
	}
	return ref, nil
}

func _IUnknownRelease(obj unsafe.Pointer, releaseMethod uintptr) {
	syscall.Syscall(
		releaseMethod,
		1,
		uintptr(obj),
		0,
		0,
	)
}

func (e ErrorCode) Error() string {
	return fmt.Sprintf("%s: %#x", e.Name, e.Code)
}
