// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math/bits"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/f32color"
	"gioui.org/internal/ops"
	"gioui.org/internal/scene"
	"gioui.org/layout"
	"gioui.org/op"
)

type compute struct {
	ctx driver.Device
	enc encoder

	drawOps       drawOps
	texOps        []textureOp
	cache         *resourceCache
	maxTextureDim int

	programs struct {
		elements   driver.Program
		tileAlloc  driver.Program
		pathCoarse driver.Program
		backdrop   driver.Program
		binning    driver.Program
		coarse     driver.Program
		kernel4    driver.Program
	}
	buffers struct {
		config driver.Buffer
		scene  sizedBuffer
		state  sizedBuffer
		memory sizedBuffer
	}
	output struct {
		size image.Point
		// image is the output texture. Note that it is in RGBA format,
		// but contains data in sRGB. See blitOutput for more detail.
		image    driver.Texture
		blitProg driver.Program
	}
	// images contains ImageOp images packed into a texture atlas.
	images struct {
		packer packer
		// positions maps imageOpData.handles to positions inside tex.
		positions map[interface{}]image.Point
		tex       driver.Texture
	}
	// materials contains the pre-processed materials (transformed images for
	// now, gradients etc. later) packed in a texture atlas. The atlas is used
	// as source in kernel4.
	materials struct {
		// offsets maps texture ops to the offsets to put in their FillImage commands.
		offsets map[textureKey]image.Point

		prog   driver.Program
		layout driver.InputLayout

		packer packer

		tex   driver.Texture
		fbo   driver.Framebuffer
		quads []materialVertex

		bufSize int
		buffer  driver.Buffer
	}
	timers struct {
		profile         string
		t               *timers
		elements        *timer
		tileAlloc       *timer
		pathCoarse      *timer
		backdropBinning *timer
		coarse          *timer
		kernel4         *timer
	}

	// The following fields hold scratch space to avoid garbage.
	zeroSlice []byte
	memHeader *memoryHeader
	conf      *config
}

// materialVertex describes a vertex of a quad used to render a transformed
// material.
type materialVertex struct {
	posX, posY float32
	u, v       float32
}

// textureKey identifies textureOp.
type textureKey struct {
	handle    interface{}
	transform f32.Affine2D
}

// textureOp represents an imageOp that requires texture space.
type textureOp struct {
	// sceneIdx is the index in the scene that contains the fill image command
	// that corresponds to the operation.
	sceneIdx int
	key      textureKey
	img      imageOpData

	// pos is the position of the untransformed image in the images texture.
	pos image.Point
}

type encoder struct {
	scene    []scene.Command
	npath    int
	npathseg int
	ntrans   int
}

type encodeState struct {
	trans f32.Affine2D
	clip  f32.Rectangle
}

type sizedBuffer struct {
	size   int
	buffer driver.Buffer
}

// config matches Config in setup.h
type config struct {
	n_elements      uint32 // paths
	n_pathseg       uint32
	width_in_tiles  uint32
	height_in_tiles uint32
	tile_alloc      memAlloc
	bin_alloc       memAlloc
	ptcl_alloc      memAlloc
	pathseg_alloc   memAlloc
	anno_alloc      memAlloc
	trans_alloc     memAlloc
}

// memAlloc matches Alloc in mem.h
type memAlloc struct {
	offset uint32
	//size   uint32
}

// memoryHeader matches the header of Memory in mem.h.
type memoryHeader struct {
	mem_offset uint32
	mem_error  uint32
}

// GPU structure sizes and constants.
const (
	tileWidthPx       = 32
	tileHeightPx      = 32
	ptclInitialAlloc  = 1024
	kernel4OutputUnit = 2
	kernel4AtlasUnit  = 3

	pathSize    = 12
	binSize     = 8
	pathsegSize = 52
	annoSize    = 32
	transSize   = 24
	stateSize   = 60
	stateStride = 4 + 2*stateSize
)

// mem.h constants.
const (
	memNoError      = 0 // NO_ERROR
	memMallocFailed = 1 // ERR_MALLOC_FAILED
)

func newCompute(ctx driver.Device) (*compute, error) {
	maxDim := ctx.Caps().MaxTextureSize
	// Large atlas textures cause artifacts due to precision loss in
	// shaders.
	if cap := 8192; maxDim > cap {
		maxDim = cap
	}
	g := &compute{
		ctx:           ctx,
		cache:         newResourceCache(),
		maxTextureDim: maxDim,
		conf:          new(config),
		memHeader:     new(memoryHeader),
	}

	blitProg, err := ctx.NewProgram(shader_copy_vert, shader_copy_frag)
	if err != nil {
		g.Release()
		return nil, err
	}
	g.output.blitProg = blitProg

	materialProg, err := ctx.NewProgram(shader_material_vert, shader_material_frag)
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.prog = materialProg
	progLayout, err := ctx.NewInputLayout(shader_material_vert, []driver.InputDesc{
		{Type: driver.DataTypeFloat, Size: 2, Offset: 0},
		{Type: driver.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.layout = progLayout

	g.drawOps.pathCache = newOpCache()
	g.drawOps.compute = true

	buf, err := ctx.NewBuffer(driver.BufferBindingShaderStorage, int(unsafe.Sizeof(config{})))
	if err != nil {
		g.Release()
		return nil, err
	}
	g.buffers.config = buf

	shaders := []struct {
		prog *driver.Program
		src  driver.ShaderSources
	}{
		{&g.programs.elements, shader_elements_comp},
		{&g.programs.tileAlloc, shader_tile_alloc_comp},
		{&g.programs.pathCoarse, shader_path_coarse_comp},
		{&g.programs.backdrop, shader_backdrop_comp},
		{&g.programs.binning, shader_binning_comp},
		{&g.programs.coarse, shader_coarse_comp},
		{&g.programs.kernel4, shader_kernel4_comp},
	}
	for _, shader := range shaders {
		p, err := ctx.NewComputeProgram(shader.src)
		if err != nil {
			g.Release()
			return nil, err
		}
		*shader.prog = p
	}
	return g, nil
}

func (g *compute) Collect(viewport image.Point, ops *op.Ops) {
	g.drawOps.reset(g.cache, viewport)
	g.drawOps.collect(g.ctx, g.cache, ops, viewport)
	for _, img := range g.drawOps.allImageOps {
		expandPathOp(img.path, img.clip)
	}
	if g.drawOps.profile && g.timers.t == nil && g.ctx.Caps().Features.Has(driver.FeatureTimers) {
		t := &g.timers
		t.t = newTimers(g.ctx)
		t.elements = g.timers.t.newTimer()
		t.tileAlloc = g.timers.t.newTimer()
		t.pathCoarse = g.timers.t.newTimer()
		t.backdropBinning = g.timers.t.newTimer()
		t.coarse = g.timers.t.newTimer()
		t.kernel4 = g.timers.t.newTimer()
	}
}

func (g *compute) Clear(col color.NRGBA) {
	g.drawOps.clear = true
	g.drawOps.clearColor = f32color.LinearFromSRGB(col)
}

func (g *compute) Frame() error {
	viewport := g.drawOps.viewport
	tileDims := image.Point{
		X: (viewport.X + tileWidthPx - 1) / tileWidthPx,
		Y: (viewport.Y + tileHeightPx - 1) / tileHeightPx,
	}

	defFBO := g.ctx.BeginFrame()
	defer g.ctx.EndFrame()

	if err := g.encode(viewport); err != nil {
		return err
	}
	if err := g.uploadImages(); err != nil {
		return err
	}
	if err := g.renderMaterials(); err != nil {
		return err
	}
	if err := g.render(tileDims); err != nil {
		return err
	}
	g.ctx.BindFramebuffer(defFBO)
	g.blitOutput(viewport)
	g.cache.frame()
	g.drawOps.pathCache.frame()
	t := &g.timers
	if g.drawOps.profile && t.t.ready() {
		et, tat, pct, bbt := t.elements.Elapsed, t.tileAlloc.Elapsed, t.pathCoarse.Elapsed, t.backdropBinning.Elapsed
		ct, k4t := t.coarse.Elapsed, t.kernel4.Elapsed
		ft := et + tat + pct + bbt + ct + k4t
		q := 100 * time.Microsecond
		ft = ft.Round(q)
		et, tat, pct, bbt = et.Round(q), tat.Round(q), pct.Round(q), bbt.Round(q)
		ct, k4t = ct.Round(q), k4t.Round(q)
		t.profile = fmt.Sprintf("ft:%7s et:%7s tat:%7s pct:%7s bbt:%7s ct:%7s k4t:%7s", ft, et, tat, pct, bbt, ct, k4t)
	}
	g.drawOps.clear = false
	return nil
}

func (g *compute) Profile() string {
	return g.timers.profile
}

// blitOutput copies the compute render output to the output FBO. We need to
// copy because compute shaders can only write to textures, not FBOs. Compute
// shader can only write to RGBA textures, but since we actually render in sRGB
// format we can't use glBlitFramebuffer, because it does sRGB conversion.
func (g *compute) blitOutput(viewport image.Point) {
	if !g.drawOps.clear {
		g.ctx.BlendFunc(driver.BlendFactorOne, driver.BlendFactorOneMinusSrcAlpha)
		g.ctx.SetBlend(true)
		defer g.ctx.SetBlend(false)
	}
	g.ctx.Viewport(0, 0, viewport.X, viewport.Y)
	g.ctx.BindTexture(0, g.output.image)
	g.ctx.BindProgram(g.output.blitProg)
	g.ctx.DrawArrays(driver.DrawModeTriangleStrip, 0, 4)
}

func (g *compute) encode(viewport image.Point) error {
	g.texOps = g.texOps[:0]
	g.enc.reset()

	// Flip Y-axis.
	flipY := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(1, -1)).Offset(f32.Pt(0, float32(viewport.Y)))
	g.enc.transform(flipY)
	if g.drawOps.clear {
		g.enc.rect(f32.Rectangle{Max: layout.FPt(viewport)})
		g.enc.fillColor(f32color.NRGBAToRGBA(g.drawOps.clearColor.SRGB()))
	}
	return g.encodeOps(flipY, viewport, g.drawOps.allImageOps)
}

func (g *compute) renderMaterials() error {
	m := &g.materials
	m.quads = m.quads[:0]
	resize := false
	reclaimed := false
restart:
	for {
		for _, op := range g.texOps {
			if off, exists := m.offsets[op.key]; exists {
				g.enc.setFillImageOffset(op.sceneIdx, off)
				continue
			}
			quad, bounds := g.materialQuad(op.key.transform, op.img, op.pos)

			// A material is clipped to avoid drawing outside its bounds inside the atlas. However,
			// imprecision in the clipping may cause a single pixel overflow. Be safe.
			size := bounds.Size().Add(image.Pt(1, 1))
			place, fits := m.packer.tryAdd(size)
			if !fits {
				m.offsets = nil
				m.quads = m.quads[:0]
				m.packer.clear()
				if !reclaimed {
					// Some images may no longer be in use, try again
					// after clearing existing maps.
					reclaimed = true
				} else {
					m.packer.maxDim += 256
					resize = true
					if m.packer.maxDim > g.maxTextureDim {
						return errors.New("compute: no space left in material atlas")
					}
				}
				m.packer.newPage()
				continue restart
			}
			// Position quad to match place.
			offset := place.Pos.Sub(bounds.Min)
			offsetf := layout.FPt(offset)
			for i := range quad {
				quad[i].posX += offsetf.X
				quad[i].posY += offsetf.Y
			}
			// Draw quad as two triangles.
			m.quads = append(m.quads, quad[0], quad[1], quad[3], quad[3], quad[1], quad[2])
			if m.offsets == nil {
				m.offsets = make(map[textureKey]image.Point)
			}
			m.offsets[op.key] = offset
			g.enc.setFillImageOffset(op.sceneIdx, offset)
		}
		break
	}
	if len(m.quads) == 0 {
		return nil
	}
	texSize := m.packer.maxDim
	if resize {
		if m.fbo != nil {
			m.fbo.Release()
			m.fbo = nil
		}
		if m.tex != nil {
			m.tex.Release()
			m.tex = nil
		}
		handle, err := g.ctx.NewTexture(driver.TextureFormatRGBA8, texSize, texSize,
			driver.FilterNearest, driver.FilterNearest,
			driver.BufferBindingShaderStorage|driver.BufferBindingFramebuffer)
		if err != nil {
			return fmt.Errorf("compute: failed to create material atlas: %v", err)
		}
		m.tex = handle
		fbo, err := g.ctx.NewFramebuffer(handle, 0)
		if err != nil {
			return fmt.Errorf("compute: failed to create material framebuffer: %v", err)
		}
		m.fbo = fbo
	}
	// TODO: move to shaders.
	// Transform to clip space: [-1, -1] - [1, 1].
	clip := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(2/float32(texSize), 2/float32(texSize))).Offset(f32.Pt(-1, -1))
	for i, v := range m.quads {
		p := clip.Transform(f32.Pt(v.posX, v.posY))
		m.quads[i].posX = p.X
		m.quads[i].posY = p.Y
	}
	vertexData := byteslice.Slice(m.quads)
	if len(vertexData) > m.bufSize {
		if m.buffer != nil {
			m.buffer.Release()
			m.buffer = nil
		}
		n := pow2Ceil(len(vertexData))
		buf, err := g.ctx.NewBuffer(driver.BufferBindingVertices, n)
		if err != nil {
			return err
		}
		m.bufSize = n
		m.buffer = buf
	}
	m.buffer.Upload(vertexData)
	g.ctx.BindTexture(0, g.images.tex)
	g.ctx.BindFramebuffer(m.fbo)
	g.ctx.Viewport(0, 0, texSize, texSize)
	if reclaimed {
		g.ctx.Clear(0, 0, 0, 0)
	}
	g.ctx.BindProgram(m.prog)
	g.ctx.BindVertexBuffer(m.buffer, int(unsafe.Sizeof(m.quads[0])), 0)
	g.ctx.BindInputLayout(m.layout)
	g.ctx.DrawArrays(driver.DrawModeTriangles, 0, len(m.quads))
	return nil
}

func (g *compute) uploadImages() error {
	// padding is the number of pixels added to the right and below
	// images, to avoid atlas filtering artifacts.
	const padding = 1

	a := &g.images
	var uploads map[interface{}]*image.RGBA
	resize := false
	reclaimed := false
restart:
	for {
		for i, op := range g.texOps {
			if pos, exists := a.positions[op.img.handle]; exists {
				g.texOps[i].pos = pos
				continue
			}
			size := op.img.src.Bounds().Size().Add(image.Pt(padding, padding))
			place, fits := a.packer.tryAdd(size)
			if !fits {
				a.positions = nil
				uploads = nil
				a.packer.clear()
				if !reclaimed {
					// Some images may no longer be in use, try again
					// after clearing existing maps.
					reclaimed = true
				} else {
					a.packer.maxDim += 256
					resize = true
					if a.packer.maxDim > g.maxTextureDim {
						return errors.New("compute: no space left in image atlas")
					}
				}
				a.packer.newPage()
				continue restart
			}
			if a.positions == nil {
				a.positions = make(map[interface{}]image.Point)
			}
			a.positions[op.img.handle] = place.Pos
			g.texOps[i].pos = place.Pos
			if uploads == nil {
				uploads = make(map[interface{}]*image.RGBA)
			}
			uploads[op.img.handle] = op.img.src
		}
		break
	}
	if len(uploads) == 0 {
		return nil
	}
	if resize {
		if a.tex != nil {
			a.tex.Release()
			a.tex = nil
		}
		sz := a.packer.maxDim
		handle, err := g.ctx.NewTexture(driver.TextureFormatSRGB, sz, sz, driver.FilterLinear, driver.FilterLinear, driver.BufferBindingTexture)
		if err != nil {
			return fmt.Errorf("compute: failed to create image atlas: %v", err)
		}
		a.tex = handle
	}
	for h, img := range uploads {
		pos, ok := a.positions[h]
		if !ok {
			panic("compute: internal error: image not placed")
		}
		size := img.Bounds().Size()
		driver.UploadImage(a.tex, pos, img)
		rightPadding := image.Pt(padding, size.Y)
		a.tex.Upload(image.Pt(pos.X+size.X, pos.Y), rightPadding, g.zeros(rightPadding.X*rightPadding.Y*4))
		bottomPadding := image.Pt(size.X, padding)
		a.tex.Upload(image.Pt(pos.X, pos.Y+size.Y), bottomPadding, g.zeros(bottomPadding.X*bottomPadding.Y*4))
	}
	return nil
}

func pow2Ceil(v int) int {
	exp := bits.Len(uint(v))
	if bits.OnesCount(uint(v)) == 1 {
		exp--
	}
	return 1 << exp
}

// materialQuad constructs a quad that represents the transformed image. It returns the quad
// and its bounds.
func (g *compute) materialQuad(M f32.Affine2D, img imageOpData, uvPos image.Point) ([4]materialVertex, image.Rectangle) {
	imgSize := layout.FPt(img.src.Bounds().Size())
	sx, hx, ox, hy, sy, oy := M.Elems()
	transOff := f32.Pt(ox, oy)
	// The 4 corners of the image rectangle transformed by M, excluding its offset, are:
	//
	// q0: M * (0, 0)   q3: M * (w, 0)
	// q1: M * (0, h)   q2: M * (w, h)
	//
	// Note that q0 = M*0 = 0, q2 = q1 + q3.
	q0 := f32.Pt(0, 0)
	q1 := f32.Pt(hx*imgSize.Y, sy*imgSize.Y)
	q3 := f32.Pt(sx*imgSize.X, hy*imgSize.X)
	q2 := q1.Add(q3)
	q0 = q0.Add(transOff)
	q1 = q1.Add(transOff)
	q2 = q2.Add(transOff)
	q3 = q3.Add(transOff)

	boundsf := f32.Rectangle{
		Min: min(min(q0, q1), min(q2, q3)),
		Max: max(max(q0, q1), max(q2, q3)),
	}

	bounds := boundRectF(boundsf)
	uvPosf := layout.FPt(uvPos)
	atlasScale := 1 / float32(g.images.packer.maxDim)
	uvBounds := f32.Rectangle{
		Min: uvPosf.Mul(atlasScale),
		Max: uvPosf.Add(imgSize).Mul(atlasScale),
	}
	quad := [4]materialVertex{
		{posX: q0.X, posY: q0.Y, u: uvBounds.Min.X, v: uvBounds.Min.Y},
		{posX: q1.X, posY: q1.Y, u: uvBounds.Min.X, v: uvBounds.Max.Y},
		{posX: q2.X, posY: q2.Y, u: uvBounds.Max.X, v: uvBounds.Max.Y},
		{posX: q3.X, posY: q3.Y, u: uvBounds.Max.X, v: uvBounds.Min.Y},
	}
	return quad, bounds
}

func max(p1, p2 f32.Point) f32.Point {
	p := p1
	if p2.X > p.X {
		p.X = p2.X
	}
	if p2.Y > p.Y {
		p.Y = p2.Y
	}
	return p
}

func min(p1, p2 f32.Point) f32.Point {
	p := p1
	if p2.X < p.X {
		p.X = p2.X
	}
	if p2.Y < p.Y {
		p.Y = p2.Y
	}
	return p
}

func (g *compute) encodeOps(trans f32.Affine2D, viewport image.Point, ops []imageOp) error {
	for _, op := range ops {
		bounds := layout.FRect(op.clip)
		// clip is the union of all drawing affected by the clipping
		// operation. TODO: tighten.
		clip := f32.Rect(0, 0, float32(viewport.X), float32(viewport.Y))
		nclips := g.encodeClipStack(clip, bounds, op.path, false)
		m := op.material
		switch m.material {
		case materialTexture:
			t := trans.Mul(m.trans)
			g.texOps = append(g.texOps, textureOp{
				sceneIdx: len(g.enc.scene),
				img:      m.data,
				key: textureKey{
					transform: t,
					handle:    m.data.handle,
				},
			})
			// Add fill command, its offset is resolved and filled in renderMaterials.
			g.enc.fillImage(0)
		case materialColor:
			g.enc.fillColor(f32color.NRGBAToRGBA(op.material.color.SRGB()))
		case materialLinearGradient:
			// TODO: implement.
			g.enc.fillColor(f32color.NRGBAToRGBA(op.material.color1.SRGB()))
		default:
			panic("not implemented")
		}
		if op.path != nil && op.path.path {
			g.enc.fillMode(scene.FillModeNonzero)
			g.enc.transform(op.path.trans.Invert())
		}
		// Pop the clip stack.
		for i := 0; i < nclips; i++ {
			g.enc.endClip(clip)
		}
	}
	return nil
}

// encodeClips encodes a stack of clip paths and return the stack depth.
func (g *compute) encodeClipStack(clip, bounds f32.Rectangle, p *pathOp, begin bool) int {
	nclips := 0
	if p != nil && p.parent != nil {
		nclips += g.encodeClipStack(clip, bounds, p.parent, true)
		nclips += 1
	}
	isStroke := p.stroke.Width > 0
	if p != nil && p.path {
		if isStroke {
			g.enc.fillMode(scene.FillModeStroke)
			g.enc.lineWidth(p.stroke.Width)
		}
		pathData, _ := g.drawOps.pathCache.get(p.pathKey)
		g.enc.transform(p.trans)
		g.enc.append(pathData.computePath)
	} else {
		g.enc.rect(bounds)
	}
	if begin {
		g.enc.beginClip(clip)
		if isStroke {
			g.enc.fillMode(scene.FillModeNonzero)
		}
		if p != nil && p.path {
			g.enc.transform(p.trans.Invert())
		}
	}
	return nclips
}

func encodePath(verts []byte) encoder {
	var enc encoder
	for len(verts) >= scene.CommandSize+4 {
		cmd := ops.DecodeCommand(verts[4:])
		enc.scene = append(enc.scene, cmd)
		enc.npathseg++
		verts = verts[scene.CommandSize+4:]
	}
	return enc
}

func (g *compute) render(tileDims image.Point) error {
	const (
		// wgSize is the largest and most common workgroup size.
		wgSize = 128
		// PARTITION_SIZE from elements.comp
		partitionSize = 32 * 4
	)
	widthInBins := (tileDims.X + 15) / 16
	heightInBins := (tileDims.Y + 7) / 8
	if widthInBins*heightInBins > wgSize {
		return fmt.Errorf("gpu: output too large (%dx%d)", tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx)
	}

	// Pad scene with zeroes to avoid reading garbage in elements.comp.
	scenePadding := partitionSize - len(g.enc.scene)%partitionSize
	g.enc.scene = append(g.enc.scene, make([]scene.Command, scenePadding)...)

	realloced := false
	scene := byteslice.Slice(g.enc.scene)
	if s := len(scene); s > g.buffers.scene.size {
		realloced = true
		paddedCap := s * 11 / 10
		if err := g.buffers.scene.ensureCapacity(g.ctx, paddedCap); err != nil {
			return err
		}
	}
	g.buffers.scene.buffer.Upload(scene)

	w, h := tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx
	if g.output.size.X != w || g.output.size.Y != h {
		if err := g.resizeOutput(image.Pt(w, h)); err != nil {
			return err
		}
	}
	g.ctx.BindImageTexture(kernel4OutputUnit, g.output.image, driver.AccessWrite, driver.TextureFormatRGBA8)
	if t := g.materials.tex; t != nil {
		g.ctx.BindImageTexture(kernel4AtlasUnit, t, driver.AccessRead, driver.TextureFormatRGBA8)
	}

	// alloc is the number of allocated bytes for static buffers.
	var alloc uint32
	round := func(v, quantum int) int {
		return (v + quantum - 1) &^ (quantum - 1)
	}
	malloc := func(size int) memAlloc {
		size = round(size, 4)
		offset := alloc
		alloc += uint32(size)
		return memAlloc{offset /*, uint32(size)*/}
	}

	*g.conf = config{
		n_elements:      uint32(g.enc.npath),
		n_pathseg:       uint32(g.enc.npathseg),
		width_in_tiles:  uint32(tileDims.X),
		height_in_tiles: uint32(tileDims.Y),
		tile_alloc:      malloc(g.enc.npath * pathSize),
		bin_alloc:       malloc(round(g.enc.npath, wgSize) * binSize),
		ptcl_alloc:      malloc(tileDims.X * tileDims.Y * ptclInitialAlloc),
		pathseg_alloc:   malloc(g.enc.npathseg * pathsegSize),
		anno_alloc:      malloc(g.enc.npath * annoSize),
		trans_alloc:     malloc(g.enc.ntrans * transSize),
	}

	numPartitions := (g.enc.numElements() + 127) / 128
	// clearSize is the atomic partition counter plus flag and 2 states per partition.
	clearSize := 4 + numPartitions*stateStride
	if clearSize > g.buffers.state.size {
		realloced = true
		paddedCap := clearSize * 11 / 10
		if err := g.buffers.state.ensureCapacity(g.ctx, paddedCap); err != nil {
			return err
		}
	}

	g.buffers.config.Upload(byteslice.Struct(g.conf))

	minSize := int(unsafe.Sizeof(memoryHeader{})) + int(alloc)
	if minSize > g.buffers.memory.size {
		realloced = true
		// Add space for dynamic GPU allocations.
		const sizeBump = 4 * 1024 * 1024
		minSize += sizeBump
		if err := g.buffers.memory.ensureCapacity(g.ctx, minSize); err != nil {
			return err
		}
	}
	for {
		*g.memHeader = memoryHeader{
			mem_offset: alloc,
		}
		g.buffers.memory.buffer.Upload(byteslice.Struct(g.memHeader))
		g.buffers.state.buffer.Upload(g.zeros(clearSize))

		if realloced {
			realloced = false
			g.bindBuffers()
		}
		t := &g.timers
		g.ctx.MemoryBarrier()
		t.elements.begin()
		g.ctx.BindProgram(g.programs.elements)
		g.ctx.DispatchCompute(numPartitions, 1, 1)
		g.ctx.MemoryBarrier()
		t.elements.end()
		t.tileAlloc.begin()
		g.ctx.BindProgram(g.programs.tileAlloc)
		g.ctx.DispatchCompute((g.enc.npath+wgSize-1)/wgSize, 1, 1)
		g.ctx.MemoryBarrier()
		t.tileAlloc.end()
		t.pathCoarse.begin()
		g.ctx.BindProgram(g.programs.pathCoarse)
		g.ctx.DispatchCompute((g.enc.npathseg+31)/32, 1, 1)
		g.ctx.MemoryBarrier()
		t.pathCoarse.end()
		t.backdropBinning.begin()
		g.ctx.BindProgram(g.programs.backdrop)
		g.ctx.DispatchCompute((g.enc.npath+wgSize-1)/wgSize, 1, 1)
		// No barrier needed between backdrop and binning.
		g.ctx.BindProgram(g.programs.binning)
		g.ctx.DispatchCompute((g.enc.npath+wgSize-1)/wgSize, 1, 1)
		g.ctx.MemoryBarrier()
		t.backdropBinning.end()
		t.coarse.begin()
		g.ctx.BindProgram(g.programs.coarse)
		g.ctx.DispatchCompute(widthInBins, heightInBins, 1)
		g.ctx.MemoryBarrier()
		t.coarse.end()
		t.kernel4.begin()
		g.ctx.BindProgram(g.programs.kernel4)
		g.ctx.DispatchCompute(tileDims.X, tileDims.Y, 1)
		g.ctx.MemoryBarrier()
		t.kernel4.end()

		if err := g.buffers.memory.buffer.Download(byteslice.Struct(g.memHeader)); err != nil {
			if err == driver.ErrContentLost {
				continue
			}
			return err
		}
		switch errCode := g.memHeader.mem_error; errCode {
		case memNoError:
			return nil
		case memMallocFailed:
			// Resize memory and try again.
			realloced = true
			sz := g.buffers.memory.size * 15 / 10
			if err := g.buffers.memory.ensureCapacity(g.ctx, sz); err != nil {
				return err
			}
			continue
		default:
			return fmt.Errorf("compute: shader program failed with error %d", errCode)
		}
	}
}

// zeros returns a byte slice with size bytes of zeros.
func (g *compute) zeros(size int) []byte {
	if cap(g.zeroSlice) < size {
		g.zeroSlice = append(g.zeroSlice, make([]byte, size)...)
	}
	return g.zeroSlice[:size]
}

func (g *compute) resizeOutput(size image.Point) error {
	if g.output.image != nil {
		g.output.image.Release()
		g.output.image = nil
	}
	img, err := g.ctx.NewTexture(driver.TextureFormatRGBA8, size.X, size.Y,
		driver.FilterNearest,
		driver.FilterNearest,
		driver.BufferBindingShaderStorage|driver.BufferBindingTexture)
	if err != nil {
		return err
	}
	g.output.image = img
	g.output.size = size
	return nil
}

func (g *compute) Release() {
	if g.drawOps.pathCache != nil {
		g.drawOps.pathCache.release()
	}
	if g.cache != nil {
		g.cache.release()
	}
	progs := []driver.Program{
		g.programs.elements,
		g.programs.tileAlloc,
		g.programs.pathCoarse,
		g.programs.backdrop,
		g.programs.binning,
		g.programs.coarse,
		g.programs.kernel4,
	}
	if p := g.output.blitProg; p != nil {
		p.Release()
	}
	for _, p := range progs {
		if p != nil {
			p.Release()
		}
	}
	g.buffers.scene.release()
	g.buffers.state.release()
	g.buffers.memory.release()
	if b := g.buffers.config; b != nil {
		b.Release()
	}
	if g.output.image != nil {
		g.output.image.Release()
	}
	if g.images.tex != nil {
		g.images.tex.Release()
	}
	if g.materials.layout != nil {
		g.materials.layout.Release()
	}
	if g.materials.prog != nil {
		g.materials.prog.Release()
	}
	if g.materials.fbo != nil {
		g.materials.fbo.Release()
	}
	if g.materials.tex != nil {
		g.materials.tex.Release()
	}
	if g.materials.buffer != nil {
		g.materials.buffer.Release()
	}
	if g.timers.t != nil {
		g.timers.t.release()
	}

	*g = compute{}
}

func (g *compute) bindBuffers() {
	bindStorageBuffers(g.programs.elements, g.buffers.memory.buffer, g.buffers.config, g.buffers.scene.buffer, g.buffers.state.buffer)
	bindStorageBuffers(g.programs.tileAlloc, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.pathCoarse, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.backdrop, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.binning, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.coarse, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.kernel4, g.buffers.memory.buffer, g.buffers.config)
}

func (b *sizedBuffer) release() {
	if b.buffer == nil {
		return
	}
	b.buffer.Release()
	*b = sizedBuffer{}
}

func (b *sizedBuffer) ensureCapacity(ctx driver.Device, size int) error {
	if b.size >= size {
		return nil
	}
	if b.buffer != nil {
		b.release()
	}
	buf, err := ctx.NewBuffer(driver.BufferBindingShaderStorage, size)
	if err != nil {
		return err
	}
	b.buffer = buf
	b.size = size
	return nil
}

func bindStorageBuffers(prog driver.Program, buffers ...driver.Buffer) {
	for i, buf := range buffers {
		prog.SetStorageBuffer(i, buf)
	}
}

var bo = binary.LittleEndian

func (e *encoder) reset() {
	e.scene = e.scene[:0]
	e.npath = 0
	e.npathseg = 0
	e.ntrans = 0
}

func (e *encoder) numElements() int {
	return len(e.scene)
}

func (e *encoder) append(e2 encoder) {
	e.scene = append(e.scene, e2.scene...)
	e.npath += e2.npath
	e.npathseg += e2.npathseg
	e.ntrans += e2.ntrans
}

func (e *encoder) transform(m f32.Affine2D) {
	e.scene = append(e.scene, scene.Transform(m))
	e.ntrans++
}

func (e *encoder) lineWidth(width float32) {
	e.scene = append(e.scene, scene.SetLineWidth(width))
}

func (e *encoder) fillMode(mode scene.FillMode) {
	e.scene = append(e.scene, scene.SetFillMode(mode))
}

func (e *encoder) beginClip(bbox f32.Rectangle) {
	e.scene = append(e.scene, scene.BeginClip(bbox))
	e.npath++
}

func (e *encoder) endClip(bbox f32.Rectangle) {
	e.scene = append(e.scene, scene.EndClip(bbox))
	e.npath++
}

func (e *encoder) rect(r f32.Rectangle) {
	// Rectangle corners, clock-wise.
	c0, c1, c2, c3 := r.Min, f32.Pt(r.Min.X, r.Max.Y), r.Max, f32.Pt(r.Max.X, r.Min.Y)
	e.line(c0, c1)
	e.line(c1, c2)
	e.line(c2, c3)
	e.line(c3, c0)
}

func (e *encoder) fillColor(col color.RGBA) {
	e.scene = append(e.scene, scene.FillColor(col))
	e.npath++
}

func (e *encoder) setFillImageOffset(index int, offset image.Point) {
	x := int16(offset.X)
	y := int16(offset.Y)
	e.scene[index][2] = uint32(uint16(x)) | uint32(uint16(y))<<16
}

func (e *encoder) fillImage(index int) {
	e.scene = append(e.scene, scene.FillImage(index))
	e.npath++
}

func (e *encoder) line(start, end f32.Point) {
	e.scene = append(e.scene, scene.Line(start, end))
	e.npathseg++
}

func (e *encoder) quad(start, ctrl, end f32.Point) {
	e.scene = append(e.scene, scene.Quad(start, ctrl, end))
	e.npathseg++
}
