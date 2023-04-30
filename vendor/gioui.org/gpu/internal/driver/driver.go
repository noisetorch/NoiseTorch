// SPDX-License-Identifier: Unlicense OR MIT

package driver

import (
	"errors"
	"image"
	"time"
)

// Device represents the abstraction of underlying GPU
// APIs such as OpenGL, Direct3D useful for rendering Gio
// operations.
type Device interface {
	BeginFrame() Framebuffer
	EndFrame()
	Caps() Caps
	NewTimer() Timer
	// IsContinuousTime reports whether all timer measurements
	// are valid at the point of call.
	IsTimeContinuous() bool
	NewTexture(format TextureFormat, width, height int, minFilter, magFilter TextureFilter, bindings BufferBinding) (Texture, error)
	NewFramebuffer(tex Texture, depthBits int) (Framebuffer, error)
	NewImmutableBuffer(typ BufferBinding, data []byte) (Buffer, error)
	NewBuffer(typ BufferBinding, size int) (Buffer, error)
	NewComputeProgram(shader ShaderSources) (Program, error)
	NewProgram(vertexShader, fragmentShader ShaderSources) (Program, error)
	NewInputLayout(vertexShader ShaderSources, layout []InputDesc) (InputLayout, error)

	DepthFunc(f DepthFunc)
	ClearDepth(d float32)
	Clear(r, g, b, a float32)
	Viewport(x, y, width, height int)
	DrawArrays(mode DrawMode, off, count int)
	DrawElements(mode DrawMode, off, count int)
	SetBlend(enable bool)
	SetDepthTest(enable bool)
	DepthMask(mask bool)
	BlendFunc(sfactor, dfactor BlendFactor)

	BindInputLayout(i InputLayout)
	BindProgram(p Program)
	BindFramebuffer(f Framebuffer)
	BindTexture(unit int, t Texture)
	BindVertexBuffer(b Buffer, stride, offset int)
	BindIndexBuffer(b Buffer)
	BindImageTexture(unit int, texture Texture, access AccessBits, format TextureFormat)

	MemoryBarrier()
	DispatchCompute(x, y, z int)

	Release()
}

type ShaderSources struct {
	Name           string
	GLSL100ES      string
	GLSL300ES      string
	GLSL310ES      string
	GLSL130        string
	GLSL150        string
	HLSL           []byte
	Uniforms       UniformsReflection
	Inputs         []InputLocation
	Textures       []TextureBinding
	StorageBuffers []StorageBufferBinding
}

type StorageBufferBinding struct {
	Binding   int
	BlockSize int
}

type UniformsReflection struct {
	Blocks    []UniformBlock
	Locations []UniformLocation
	Size      int
}

type TextureBinding struct {
	Name    string
	Binding int
}

type UniformBlock struct {
	Name    string
	Binding int
}

type UniformLocation struct {
	Name   string
	Type   DataType
	Size   int
	Offset int
}

type InputLocation struct {
	// For GLSL.
	Name     string
	Location int
	// For HLSL.
	Semantic      string
	SemanticIndex int

	Type DataType
	Size int
}

// InputDesc describes a vertex attribute as laid out in a Buffer.
type InputDesc struct {
	Type DataType
	Size int

	Offset int
}

// InputLayout is the driver specific representation of the mapping
// between Buffers and shader attributes.
type InputLayout interface {
	Release()
}

type AccessBits uint8

type BlendFactor uint8

type DrawMode uint8

type TextureFilter uint8
type TextureFormat uint8

type BufferBinding uint8

type DataType uint8

type DepthFunc uint8

type Features uint

type Caps struct {
	// BottomLeftOrigin is true if the driver has the origin in the lower left
	// corner. The OpenGL driver returns true.
	BottomLeftOrigin bool
	Features         Features
	MaxTextureSize   int
}

type Program interface {
	Release()
	SetStorageBuffer(binding int, buf Buffer)
	SetVertexUniforms(buf Buffer)
	SetFragmentUniforms(buf Buffer)
}

type Buffer interface {
	Release()
	Upload(data []byte)
	Download(data []byte) error
}

type Framebuffer interface {
	Invalidate()
	Release()
	ReadPixels(src image.Rectangle, pixels []byte) error
}

type Timer interface {
	Begin()
	End()
	Duration() (time.Duration, bool)
	Release()
}

type Texture interface {
	Upload(offset, size image.Point, pixels []byte)
	Release()
}

const (
	DepthFuncGreater DepthFunc = iota
	DepthFuncGreaterEqual
)

const (
	DataTypeFloat DataType = iota
	DataTypeInt
	DataTypeShort
)

const (
	BufferBindingIndices BufferBinding = 1 << iota
	BufferBindingVertices
	BufferBindingUniforms
	BufferBindingTexture
	BufferBindingFramebuffer
	BufferBindingShaderStorage
)

const (
	TextureFormatSRGB TextureFormat = iota
	TextureFormatFloat
	TextureFormatRGBA8
)

const (
	AccessRead AccessBits = 1 + iota
	AccessWrite
)

const (
	FilterNearest TextureFilter = iota
	FilterLinear
)

const (
	FeatureTimers Features = 1 << iota
	FeatureFloatRenderTargets
	FeatureCompute
)

const (
	DrawModeTriangleStrip DrawMode = iota
	DrawModeTriangles
)

const (
	BlendFactorOne BlendFactor = iota
	BlendFactorOneMinusSrcAlpha
	BlendFactorZero
	BlendFactorDstColor
)

var ErrContentLost = errors.New("buffer content lost")

func (f Features) Has(feats Features) bool {
	return f&feats == feats
}

func DownloadImage(d Device, f Framebuffer, r image.Rectangle) (*image.RGBA, error) {
	img := image.NewRGBA(r)
	if err := f.ReadPixels(r, img.Pix); err != nil {
		return nil, err
	}
	if d.Caps().BottomLeftOrigin {
		// OpenGL origin is in the lower-left corner. Flip the image to
		// match.
		flipImageY(r.Dx()*4, r.Dy(), img.Pix)
	}
	return img, nil
}

func flipImageY(stride, height int, pixels []byte) {
	// Flip image in y-direction. OpenGL's origin is in the lower
	// left corner.
	row := make([]uint8, stride)
	for y := 0; y < height/2; y++ {
		y1 := height - y - 1
		dest := y1 * stride
		src := y * stride
		copy(row, pixels[dest:])
		copy(pixels[dest:], pixels[src:src+len(row)])
		copy(pixels[src:], row)
	}
}

func UploadImage(t Texture, offset image.Point, img *image.RGBA) {
	var pixels []byte
	size := img.Bounds().Size()
	if img.Stride != size.X*4 {
		panic("unsupported stride")
	}
	start := img.PixOffset(0, 0)
	end := img.PixOffset(size.X, size.Y-1)
	pixels = img.Pix[start:end]
	t.Upload(offset, size, pixels)
}
