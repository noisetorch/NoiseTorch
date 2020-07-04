// +build darwin

#import <Metal/Metal.h>
#include <stdlib.h>
#include "mtl.h"

struct Device CreateSystemDefaultDevice() {
	id<MTLDevice> device = MTLCreateSystemDefaultDevice();
	if (!device) {
		struct Device d;
		d.Device = NULL;
		return d;
	}

	struct Device d;
	d.Device = device;
	d.Headless = device.headless;
	d.LowPower = device.lowPower;
	d.Removable = device.removable;
	d.RegistryID = device.registryID;
	d.Name = device.name.UTF8String;
	return d;
}

// Caller must call free(d.devices).
struct Devices CopyAllDevices() {
	NSArray<id<MTLDevice>> * devices = MTLCopyAllDevices();

	struct Devices d;
	d.Devices = malloc(devices.count * sizeof(struct Device));
	for (int i = 0; i < devices.count; i++) {
		d.Devices[i].Device = devices[i];
		d.Devices[i].Headless = devices[i].headless;
		d.Devices[i].LowPower = devices[i].lowPower;
		d.Devices[i].Removable = devices[i].removable;
		d.Devices[i].RegistryID = devices[i].registryID;
		d.Devices[i].Name = devices[i].name.UTF8String;
	}
	d.Length = devices.count;
	return d;
}

BOOL Device_SupportsFeatureSet(void * device, uint16_t featureSet) {
	return [(id<MTLDevice>)device supportsFeatureSet:featureSet];
}

void * Device_MakeCommandQueue(void * device) {
	return [(id<MTLDevice>)device newCommandQueue];
}

struct Library Device_MakeLibrary(void * device, const char * source, size_t sourceLength) {
	NSError * error;
	id<MTLLibrary> library = [(id<MTLDevice>)device
		newLibraryWithSource:[[NSString alloc] initWithBytes:source length:sourceLength encoding:NSUTF8StringEncoding]
		options:NULL // TODO.
		error:&error];

	struct Library l;
	l.Library = library;
	if (!library) {
		l.Error = error.localizedDescription.UTF8String;
	}
	return l;
}

struct RenderPipelineState Device_MakeRenderPipelineState(void * device, struct RenderPipelineDescriptor descriptor) {
	MTLRenderPipelineDescriptor * renderPipelineDescriptor = [[MTLRenderPipelineDescriptor alloc] init];
	renderPipelineDescriptor.vertexFunction = descriptor.VertexFunction;
	renderPipelineDescriptor.fragmentFunction = descriptor.FragmentFunction;
	renderPipelineDescriptor.colorAttachments[0].pixelFormat = descriptor.ColorAttachment0PixelFormat;

	NSError * error;
	id<MTLRenderPipelineState> renderPipelineState = [(id<MTLDevice>)device newRenderPipelineStateWithDescriptor:renderPipelineDescriptor
	                                                                                                       error:&error];

	struct RenderPipelineState rps;
	rps.RenderPipelineState = renderPipelineState;
	if (!renderPipelineState) {
		rps.Error = error.localizedDescription.UTF8String;
	}
	return rps;
}

void * Device_MakeBuffer(void * device, const void * bytes, size_t length, uint16_t options) {
	return [(id<MTLDevice>)device newBufferWithBytes:(const void *)bytes
	                                          length:(NSUInteger)length
	                                         options:(MTLResourceOptions)options];
}

void * Device_MakeTexture(void * device, struct TextureDescriptor descriptor) {
	MTLTextureDescriptor * textureDescriptor = [[MTLTextureDescriptor alloc] init];
	textureDescriptor.pixelFormat = descriptor.PixelFormat;
	textureDescriptor.width = descriptor.Width;
	textureDescriptor.height = descriptor.Height;
	textureDescriptor.storageMode = descriptor.StorageMode;
	return [(id<MTLDevice>)device newTextureWithDescriptor:textureDescriptor];
}

void * CommandQueue_MakeCommandBuffer(void * commandQueue) {
	return [(id<MTLCommandQueue>)commandQueue commandBuffer];
}

void CommandBuffer_PresentDrawable(void * commandBuffer, void * drawable) {
	[(id<MTLCommandBuffer>)commandBuffer presentDrawable:(id<MTLDrawable>)drawable];
}

void CommandBuffer_Commit(void * commandBuffer) {
	[(id<MTLCommandBuffer>)commandBuffer commit];
}

void CommandBuffer_WaitUntilCompleted(void * commandBuffer) {
	[(id<MTLCommandBuffer>)commandBuffer waitUntilCompleted];
}

void * CommandBuffer_MakeRenderCommandEncoder(void * commandBuffer, struct RenderPassDescriptor descriptor) {
	MTLRenderPassDescriptor * renderPassDescriptor = [[MTLRenderPassDescriptor alloc] init];
	renderPassDescriptor.colorAttachments[0].loadAction = descriptor.ColorAttachment0LoadAction;
	renderPassDescriptor.colorAttachments[0].storeAction = descriptor.ColorAttachment0StoreAction;
	renderPassDescriptor.colorAttachments[0].clearColor = MTLClearColorMake(
		descriptor.ColorAttachment0ClearColor.Red,
		descriptor.ColorAttachment0ClearColor.Green,
		descriptor.ColorAttachment0ClearColor.Blue,
		descriptor.ColorAttachment0ClearColor.Alpha
	);
	renderPassDescriptor.colorAttachments[0].texture = (id<MTLTexture>)descriptor.ColorAttachment0Texture;
	return [(id<MTLCommandBuffer>)commandBuffer renderCommandEncoderWithDescriptor:renderPassDescriptor];
}

void * CommandBuffer_MakeBlitCommandEncoder(void * commandBuffer) {
	return [(id<MTLCommandBuffer>)commandBuffer blitCommandEncoder];
}

void CommandEncoder_EndEncoding(void * commandEncoder) {
	[(id<MTLCommandEncoder>)commandEncoder endEncoding];
}

void RenderCommandEncoder_SetRenderPipelineState(void * renderCommandEncoder, void * renderPipelineState) {
	[(id<MTLRenderCommandEncoder>)renderCommandEncoder setRenderPipelineState:(id<MTLRenderPipelineState>)renderPipelineState];
}

void RenderCommandEncoder_SetVertexBuffer(void * renderCommandEncoder, void * buffer, uint_t offset, uint_t index) {
	[(id<MTLRenderCommandEncoder>)renderCommandEncoder setVertexBuffer:(id<MTLBuffer>)buffer
	                                                            offset:(NSUInteger)offset
	                                                           atIndex:(NSUInteger)index];
}

void RenderCommandEncoder_SetVertexBytes(void * renderCommandEncoder, const void * bytes, size_t length, uint_t index) {
	[(id<MTLRenderCommandEncoder>)renderCommandEncoder setVertexBytes:bytes
	                                                           length:(NSUInteger)length
	                                                          atIndex:(NSUInteger)index];
}

void RenderCommandEncoder_DrawPrimitives(void * renderCommandEncoder, uint8_t primitiveType, uint_t vertexStart, uint_t vertexCount) {
	[(id<MTLRenderCommandEncoder>)renderCommandEncoder drawPrimitives:(MTLPrimitiveType)primitiveType
	                                                      vertexStart:(NSUInteger)vertexStart
	                                                      vertexCount:(NSUInteger)vertexCount];
}

void BlitCommandEncoder_CopyFromTexture(void * blitCommandEncoder,
	void * srcTexture, uint_t srcSlice, uint_t srcLevel, struct Origin srcOrigin, struct Size srcSize,
	void * dstTexture, uint_t dstSlice, uint_t dstLevel, struct Origin dstOrigin) {
	[(id<MTLBlitCommandEncoder>)blitCommandEncoder copyFromTexture:(id<MTLTexture>)srcTexture
	                                                   sourceSlice:(NSUInteger)srcSlice
	                                                   sourceLevel:(NSUInteger)srcLevel
	                                                  sourceOrigin:(MTLOrigin){srcOrigin.X, srcOrigin.Y, srcOrigin.Z}
	                                                    sourceSize:(MTLSize){srcSize.Width, srcSize.Height, srcSize.Depth}
	                                                     toTexture:(id<MTLTexture>)dstTexture
	                                              destinationSlice:(NSUInteger)dstSlice
	                                              destinationLevel:(NSUInteger)dstLevel
	                                             destinationOrigin:(MTLOrigin){dstOrigin.X, dstOrigin.Y, dstOrigin.Z}];
}

void BlitCommandEncoder_Synchronize(void * blitCommandEncoder, void * resource) {
	[(id<MTLBlitCommandEncoder>)blitCommandEncoder synchronizeResource:(id<MTLResource>)resource];
}

void * Library_MakeFunction(void * library, const char * name) {
	return [(id<MTLLibrary>)library newFunctionWithName:[NSString stringWithUTF8String:name]];
}

void Texture_ReplaceRegion(void * texture, struct Region region, uint_t level, void * pixelBytes, size_t bytesPerRow) {
	[(id<MTLTexture>)texture replaceRegion:(MTLRegion){{region.Origin.X, region.Origin.Y, region.Origin.Z}, {region.Size.Width, region.Size.Height, region.Size.Depth}}
	                           mipmapLevel:(NSUInteger)level
	                             withBytes:(void *)pixelBytes
	                           bytesPerRow:(NSUInteger)bytesPerRow];
}

void Texture_GetBytes(void * texture, void * pixelBytes, size_t bytesPerRow, struct Region region, uint_t level) {
	[(id<MTLTexture>)texture getBytes:(void *)pixelBytes
	                      bytesPerRow:(NSUInteger)bytesPerRow
	                       fromRegion:(MTLRegion){{region.Origin.X, region.Origin.Y, region.Origin.Z}, {region.Size.Width, region.Size.Height, region.Size.Depth}}
	                      mipmapLevel:(NSUInteger)level];
}
