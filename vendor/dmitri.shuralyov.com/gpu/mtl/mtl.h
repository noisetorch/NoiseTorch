// +build darwin

typedef signed char BOOL;
typedef unsigned long uint_t;
typedef unsigned char uint8_t;
typedef unsigned short uint16_t;
typedef unsigned long long uint64_t;

struct Device {
	void *       Device;
	BOOL         Headless;
	BOOL         LowPower;
	BOOL         Removable;
	uint64_t     RegistryID;
	const char * Name;
};

struct Devices {
	struct Device * Devices;
	int             Length;
};

struct Library {
	void *       Library;
	const char * Error;
};

struct RenderPipelineDescriptor {
	void *   VertexFunction;
	void *   FragmentFunction;
	uint16_t ColorAttachment0PixelFormat;
};

struct RenderPipelineState {
	void *       RenderPipelineState;
	const char * Error;
};

struct ClearColor {
	double Red;
	double Green;
	double Blue;
	double Alpha;
};

struct RenderPassDescriptor {
	uint8_t           ColorAttachment0LoadAction;
	uint8_t           ColorAttachment0StoreAction;
	struct ClearColor ColorAttachment0ClearColor;
	void *            ColorAttachment0Texture;
};

struct TextureDescriptor {
	uint16_t PixelFormat;
	uint_t   Width;
	uint_t   Height;
	uint8_t  StorageMode;
};

struct Origin {
	uint_t X;
	uint_t Y;
	uint_t Z;
};

struct Size {
	uint_t Width;
	uint_t Height;
	uint_t Depth;
};

struct Region {
	struct Origin Origin;
	struct Size   Size;
};

struct Device CreateSystemDefaultDevice();
struct Devices CopyAllDevices();

BOOL                       Device_SupportsFeatureSet(void * device, uint16_t featureSet);
void *                     Device_MakeCommandQueue(void * device);
struct Library             Device_MakeLibrary(void * device, const char * source, size_t sourceLength);
struct RenderPipelineState Device_MakeRenderPipelineState(void * device, struct RenderPipelineDescriptor descriptor);
void *                     Device_MakeBuffer(void * device, const void * bytes, size_t length, uint16_t options);
void *                     Device_MakeTexture(void * device, struct TextureDescriptor descriptor);

void * CommandQueue_MakeCommandBuffer(void * commandQueue);

void   CommandBuffer_PresentDrawable(void * commandBuffer, void * drawable);
void   CommandBuffer_Commit(void * commandBuffer);
void   CommandBuffer_WaitUntilCompleted(void * commandBuffer);
void * CommandBuffer_MakeRenderCommandEncoder(void * commandBuffer, struct RenderPassDescriptor descriptor);
void * CommandBuffer_MakeBlitCommandEncoder(void * commandBuffer);

void CommandEncoder_EndEncoding(void * commandEncoder);

void RenderCommandEncoder_SetRenderPipelineState(void * renderCommandEncoder, void * renderPipelineState);
void RenderCommandEncoder_SetVertexBuffer(void * renderCommandEncoder, void * buffer, uint_t offset, uint_t index);
void RenderCommandEncoder_SetVertexBytes(void * renderCommandEncoder, const void * bytes, size_t length, uint_t index);
void RenderCommandEncoder_DrawPrimitives(void * renderCommandEncoder, uint8_t primitiveType, uint_t vertexStart, uint_t vertexCount);

void BlitCommandEncoder_CopyFromTexture(void * blitCommandEncoder,
	void * srcTexture, uint_t srcSlice, uint_t srcLevel, struct Origin srcOrigin, struct Size srcSize,
	void * dstTexture, uint_t dstSlice, uint_t dstLevel, struct Origin dstOrigin);
void BlitCommandEncoder_Synchronize(void * blitCommandEncoder, void * resource);

void * Library_MakeFunction(void * library, const char * name);

void Texture_ReplaceRegion(void * texture, struct Region region, uint_t level, void * pixelBytes, size_t bytesPerRow);
void Texture_GetBytes(void * texture, void * pixelBytes, size_t bytesPerRow, struct Region region, uint_t level);
