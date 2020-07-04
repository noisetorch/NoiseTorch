// SPDX-License-Identifier: Unlicense OR MIT

package gl

type (
	Attrib uint
	Enum   uint
)

const (
	ARRAY_BUFFER                          = 0x8892
	BLEND                                 = 0xbe2
	CLAMP_TO_EDGE                         = 0x812f
	COLOR_ATTACHMENT0                     = 0x8ce0
	COLOR_BUFFER_BIT                      = 0x4000
	COMPILE_STATUS                        = 0x8b81
	DEPTH_BUFFER_BIT                      = 0x100
	DEPTH_ATTACHMENT                      = 0x8d00
	DEPTH_COMPONENT16                     = 0x81a5
	DEPTH_COMPONENT24                     = 0x81A6
	DEPTH_COMPONENT32F                    = 0x8CAC
	DEPTH_TEST                            = 0xb71
	DST_COLOR                             = 0x306
	ELEMENT_ARRAY_BUFFER                  = 0x8893
	EXTENSIONS                            = 0x1f03
	FALSE                                 = 0
	FLOAT                                 = 0x1406
	FRAGMENT_SHADER                       = 0x8b30
	FRAMEBUFFER                           = 0x8d40
	FRAMEBUFFER_ATTACHMENT_COLOR_ENCODING = 0x8210
	FRAMEBUFFER_BINDING                   = 0x8ca6
	FRAMEBUFFER_COMPLETE                  = 0x8cd5
	HALF_FLOAT                            = 0x140b
	HALF_FLOAT_OES                        = 0x8d61
	INFO_LOG_LENGTH                       = 0x8B84
	INVALID_INDEX                         = ^uint(0)
	GREATER                               = 0x204
	GEQUAL                                = 0x206
	LINEAR                                = 0x2601
	LINK_STATUS                           = 0x8b82
	LUMINANCE                             = 0x1909
	MAX_TEXTURE_SIZE                      = 0xd33
	NEAREST                               = 0x2600
	NO_ERROR                              = 0x0
	NUM_EXTENSIONS                        = 0x821D
	ONE                                   = 0x1
	ONE_MINUS_SRC_ALPHA                   = 0x303
	QUERY_RESULT                          = 0x8866
	QUERY_RESULT_AVAILABLE                = 0x8867
	R16F                                  = 0x822d
	R8                                    = 0x8229
	READ_FRAMEBUFFER                      = 0x8ca8
	RED                                   = 0x1903
	RENDERER                              = 0x1F01
	RENDERBUFFER                          = 0x8d41
	RENDERBUFFER_BINDING                  = 0x8ca7
	RENDERBUFFER_HEIGHT                   = 0x8d43
	RENDERBUFFER_WIDTH                    = 0x8d42
	RGB                                   = 0x1907
	RGBA                                  = 0x1908
	RGBA8                                 = 0x8058
	SHORT                                 = 0x1402
	SRGB                                  = 0x8c40
	SRGB_ALPHA_EXT                        = 0x8c42
	SRGB8                                 = 0x8c41
	SRGB8_ALPHA8                          = 0x8c43
	STATIC_DRAW                           = 0x88e4
	TEXTURE_2D                            = 0xde1
	TEXTURE_MAG_FILTER                    = 0x2800
	TEXTURE_MIN_FILTER                    = 0x2801
	TEXTURE_WRAP_S                        = 0x2802
	TEXTURE_WRAP_T                        = 0x2803
	TEXTURE0                              = 0x84c0
	TEXTURE1                              = 0x84c1
	TRIANGLE_STRIP                        = 0x5
	TRIANGLES                             = 0x4
	TRUE                                  = 1
	UNIFORM_BUFFER                        = 0x8A11
	UNPACK_ALIGNMENT                      = 0xcf5
	UNSIGNED_BYTE                         = 0x1401
	UNSIGNED_SHORT                        = 0x1403
	VERSION                               = 0x1f02
	VERTEX_SHADER                         = 0x8b31
	ZERO                                  = 0x0

	// EXT_disjoint_timer_query
	TIME_ELAPSED_EXT = 0x88BF
	GPU_DISJOINT_EXT = 0x8FBB
)

type Functions interface {
	ActiveTexture(texture Enum)
	AttachShader(p Program, s Shader)
	BeginQuery(target Enum, query Query)
	BindAttribLocation(p Program, a Attrib, name string)
	BindBuffer(target Enum, b Buffer)
	BindBufferBase(target Enum, index int, buffer Buffer)
	BindFramebuffer(target Enum, fb Framebuffer)
	BindRenderbuffer(target Enum, fb Renderbuffer)
	BindTexture(target Enum, t Texture)
	BlendEquation(mode Enum)
	BlendFunc(sfactor, dfactor Enum)
	BufferData(target Enum, src []byte, usage Enum)
	CheckFramebufferStatus(target Enum) Enum
	Clear(mask Enum)
	ClearColor(red, green, blue, alpha float32)
	ClearDepthf(d float32)
	CompileShader(s Shader)
	CreateBuffer() Buffer
	CreateFramebuffer() Framebuffer
	CreateProgram() Program
	CreateQuery() Query
	CreateRenderbuffer() Renderbuffer
	CreateShader(ty Enum) Shader
	CreateTexture() Texture
	DeleteBuffer(v Buffer)
	DeleteFramebuffer(v Framebuffer)
	DeleteProgram(p Program)
	DeleteQuery(query Query)
	DeleteRenderbuffer(r Renderbuffer)
	DeleteShader(s Shader)
	DeleteTexture(v Texture)
	DepthFunc(f Enum)
	DepthMask(mask bool)
	DisableVertexAttribArray(a Attrib)
	Disable(cap Enum)
	DrawArrays(mode Enum, first, count int)
	DrawElements(mode Enum, count int, ty Enum, offset int)
	Enable(cap Enum)
	EnableVertexAttribArray(a Attrib)
	EndQuery(target Enum)
	FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int)
	FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer)
	GetBinding(pname Enum) Object
	GetError() Enum
	GetInteger(pname Enum) int
	GetProgrami(p Program, pname Enum) int
	GetProgramInfoLog(p Program) string
	GetQueryObjectuiv(query Query, pname Enum) uint
	GetShaderi(s Shader, pname Enum) int
	GetShaderInfoLog(s Shader) string
	GetString(pname Enum) string
	GetUniformBlockIndex(p Program, name string) uint
	GetUniformLocation(p Program, name string) Uniform
	InvalidateFramebuffer(target, attachment Enum)
	LinkProgram(p Program)
	ReadPixels(x, y, width, height int, format, ty Enum, data []byte)
	RenderbufferStorage(target, internalformat Enum, width, height int)
	ShaderSource(s Shader, src string)
	TexImage2D(target Enum, level int, internalFormat int, width, height int, format, ty Enum, data []byte)
	TexParameteri(target, pname Enum, param int)
	UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint)
	Uniform1f(dst Uniform, v float32)
	Uniform1i(dst Uniform, v int)
	Uniform2f(dst Uniform, v0, v1 float32)
	Uniform3f(dst Uniform, v0, v1, v2 float32)
	Uniform4f(dst Uniform, v0, v1, v2, v3 float32)
	UseProgram(p Program)
	VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int)
	Viewport(x, y, width, height int)
}
