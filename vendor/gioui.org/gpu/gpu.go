// SPDX-License-Identifier: Unlicense OR MIT

/*
Package gpu implements the rendering of Gio drawing operations. It
is used by package app and package app/headless and is otherwise not
useful except for integrating with external window implementations.
*/
package gpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
	"reflect"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/gpu/backend"
	"gioui.org/internal/f32color"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	gunsafe "gioui.org/internal/unsafe"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
)

type GPU struct {
	cache *resourceCache

	defFBO                                            backend.Framebuffer
	profile                                           string
	timers                                            *timers
	frameStart                                        time.Time
	zopsTimer, stencilTimer, coverTimer, cleanupTimer *timer
	drawOps                                           drawOps
	ctx                                               backend.Device
	renderer                                          *renderer
}

type renderer struct {
	ctx           backend.Device
	blitter       *blitter
	pather        *pather
	packer        packer
	intersections packer
}

type drawOps struct {
	profile    bool
	reader     ops.Reader
	cache      *resourceCache
	vertCache  []byte
	viewport   image.Point
	clearColor f32color.RGBA
	imageOps   []imageOp
	// zimageOps are the rectangle clipped opaque images
	// that can use fast front-to-back rendering with z-test
	// and no blending.
	zimageOps   []imageOp
	pathOps     []*pathOp
	pathOpCache []pathOp
	qs          quadSplitter
	pathCache   *opCache
}

type drawState struct {
	clip  f32.Rectangle
	t     f32.Affine2D
	cpath *pathOp
	rect  bool
	z     int

	matType materialType
	// Current paint.ImageOp
	image imageOpData
	// Current paint.ColorOp, if any.
	color color.RGBA
}

type pathOp struct {
	off f32.Point
	// clip is the union of all
	// later clip rectangles.
	clip      image.Rectangle
	bounds    f32.Rectangle
	pathKey   ops.Key
	path      bool
	pathVerts []byte
	parent    *pathOp
	place     placement
}

type imageOp struct {
	z        float32
	path     *pathOp
	off      f32.Point
	clip     image.Rectangle
	material material
	clipType clipType
	place    placement
}

type material struct {
	material materialType
	opaque   bool
	// For materialTypeColor.
	color f32color.RGBA
	// For materialTypeTexture.
	texture *texture
	uvTrans f32.Affine2D
}

// clipOp is the shadow of clip.Op.
type clipOp struct {
	bounds f32.Rectangle
}

// imageOpData is the shadow of paint.ImageOp.
type imageOpData struct {
	rect   image.Rectangle
	src    *image.RGBA
	handle interface{}
}

func (op *clipOp) decode(data []byte) {
	if opconst.OpType(data[0]) != opconst.TypeClip {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	r := f32.Rectangle{
		Min: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[1:])),
			Y: math.Float32frombits(bo.Uint32(data[5:])),
		},
		Max: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[9:])),
			Y: math.Float32frombits(bo.Uint32(data[13:])),
		},
	}
	*op = clipOp{
		bounds: r,
	}
}

func decodeImageOp(data []byte, refs []interface{}) imageOpData {
	if opconst.OpType(data[0]) != opconst.TypeImage {
		panic("invalid op")
	}
	handle := refs[1]
	if handle == nil {
		return imageOpData{}
	}
	bo := binary.LittleEndian
	return imageOpData{
		rect: image.Rectangle{
			Min: image.Point{
				X: int(bo.Uint32(data[1:])),
				Y: int(bo.Uint32(data[5:])),
			},
			Max: image.Point{
				X: int(bo.Uint32(data[9:])),
				Y: int(bo.Uint32(data[13:])),
			},
		},
		src:    refs[0].(*image.RGBA),
		handle: handle,
	}
}

func decodeColorOp(data []byte) color.RGBA {
	if opconst.OpType(data[0]) != opconst.TypeColor {
		panic("invalid op")
	}
	return color.RGBA{
		R: data[1],
		G: data[2],
		B: data[3],
		A: data[4],
	}
}

func decodePaintOp(data []byte) paint.PaintOp {
	bo := binary.LittleEndian
	if opconst.OpType(data[0]) != opconst.TypePaint {
		panic("invalid op")
	}
	r := f32.Rectangle{
		Min: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[1:])),
			Y: math.Float32frombits(bo.Uint32(data[5:])),
		},
		Max: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[9:])),
			Y: math.Float32frombits(bo.Uint32(data[13:])),
		},
	}
	return paint.PaintOp{
		Rect: r,
	}
}

type clipType uint8

type resource interface {
	release()
}

type texture struct {
	src *image.RGBA
	tex backend.Texture
}

type blitter struct {
	ctx         backend.Device
	viewport    image.Point
	prog        [2]*program
	layout      backend.InputLayout
	colUniforms *blitColUniforms
	texUniforms *blitTexUniforms
	quadVerts   backend.Buffer
}

type blitColUniforms struct {
	vert struct {
		blitUniforms
		_ [12]byte // Padding to a multiple of 16.
	}
	frag struct {
		colorUniforms
	}
}

type blitTexUniforms struct {
	vert struct {
		blitUniforms
		_ [12]byte // Padding to a multiple of 16.
	}
}

type uniformBuffer struct {
	buf backend.Buffer
	ptr []byte
}

type program struct {
	prog         backend.Program
	vertUniforms *uniformBuffer
	fragUniforms *uniformBuffer
}

type blitUniforms struct {
	transform     [4]float32
	uvTransformR1 [4]float32
	uvTransformR2 [4]float32
	z             float32
}

type colorUniforms struct {
	color f32color.RGBA
}

type materialType uint8

const (
	clipTypeNone clipType = iota
	clipTypePath
	clipTypeIntersection
)

const (
	materialColor materialType = iota
	materialTexture
)

func New(ctx backend.Device) (*GPU, error) {
	defFBO := ctx.CurrentFramebuffer()
	g := &GPU{
		defFBO: defFBO,
		cache:  newResourceCache(),
	}
	g.drawOps.pathCache = newOpCache()
	if err := g.init(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *GPU) init(ctx backend.Device) error {
	g.ctx = ctx
	g.renderer = newRenderer(ctx)
	return nil
}

func (g *GPU) Release() {
	g.renderer.release()
	g.drawOps.pathCache.release()
	g.cache.release()
	if g.timers != nil {
		g.timers.release()
	}
}

func (g *GPU) Collect(viewport image.Point, frameOps *op.Ops) {
	g.renderer.blitter.viewport = viewport
	g.renderer.pather.viewport = viewport
	g.drawOps.reset(g.cache, viewport)
	g.drawOps.collect(g.cache, frameOps, viewport)
	g.frameStart = time.Now()
	if g.drawOps.profile && g.timers == nil && g.ctx.Caps().Features.Has(backend.FeatureTimers) {
		g.timers = newTimers(g.ctx)
		g.zopsTimer = g.timers.newTimer()
		g.stencilTimer = g.timers.newTimer()
		g.coverTimer = g.timers.newTimer()
		g.cleanupTimer = g.timers.newTimer()
	}
	for _, p := range g.drawOps.pathOps {
		if v, exists := g.drawOps.pathCache.get(p.pathKey); !exists || v.data.data == nil {
			data := buildPath(g.ctx, p.pathVerts)
			g.drawOps.pathCache.put(p.pathKey, opCacheValue{
				data:   data,
				bounds: p.bounds,
			})
		}
		p.pathVerts = nil
	}
}

func (g *GPU) BeginFrame() {
	g.ctx.BeginFrame()
	defer g.ctx.EndFrame()
	viewport := g.renderer.blitter.viewport
	for _, img := range g.drawOps.imageOps {
		expandPathOp(img.path, img.clip)
	}
	if g.drawOps.profile {
		g.zopsTimer.begin()
	}
	g.ctx.BindFramebuffer(g.defFBO)
	g.ctx.DepthFunc(backend.DepthFuncGreater)
	g.ctx.ClearDepth(0.0)
	g.ctx.Clear(g.drawOps.clearColor.Float32())
	g.ctx.Viewport(0, 0, viewport.X, viewport.Y)
	g.renderer.drawZOps(g.drawOps.zimageOps)
	g.zopsTimer.end()
	g.stencilTimer.begin()
	g.ctx.SetBlend(true)
	g.renderer.packStencils(&g.drawOps.pathOps)
	g.renderer.stencilClips(g.drawOps.pathCache, g.drawOps.pathOps)
	g.renderer.packIntersections(g.drawOps.imageOps)
	g.renderer.intersect(g.drawOps.imageOps)
	g.stencilTimer.end()
	g.coverTimer.begin()
	g.ctx.BindFramebuffer(g.defFBO)
	g.ctx.Viewport(0, 0, viewport.X, viewport.Y)
	g.renderer.drawOps(g.drawOps.imageOps)
	g.ctx.SetBlend(false)
	g.renderer.pather.stenciler.invalidateFBO()
	g.coverTimer.end()
	g.ctx.BindFramebuffer(g.defFBO)
}

func (g *GPU) EndFrame() {
	g.cleanupTimer.begin()
	g.cache.frame()
	g.drawOps.pathCache.frame()
	g.cleanupTimer.end()
	if g.drawOps.profile && g.timers.ready() {
		zt, st, covt, cleant := g.zopsTimer.Elapsed, g.stencilTimer.Elapsed, g.coverTimer.Elapsed, g.cleanupTimer.Elapsed
		ft := zt + st + covt + cleant
		q := 100 * time.Microsecond
		zt, st, covt = zt.Round(q), st.Round(q), covt.Round(q)
		frameDur := time.Since(g.frameStart).Round(q)
		ft = ft.Round(q)
		g.profile = fmt.Sprintf("draw:%7s gpu:%7s zt:%7s st:%7s cov:%7s", frameDur, ft, zt, st, covt)
	}
}

func (g *GPU) Profile() string {
	return g.profile
}

func (r *renderer) texHandle(t *texture) backend.Texture {
	if t.tex != nil {
		return t.tex
	}
	tex, err := r.ctx.NewTexture(backend.TextureFormatSRGB, t.src.Bounds().Dx(), t.src.Bounds().Dy(), backend.FilterLinear, backend.FilterLinear, backend.BufferBindingTexture)
	if err != nil {
		panic(err)
	}
	tex.Upload(t.src)
	t.tex = tex
	return t.tex
}

func (t *texture) release() {
	if t.tex != nil {
		t.tex.Release()
	}
}

func newRenderer(ctx backend.Device) *renderer {
	r := &renderer{
		ctx:     ctx,
		blitter: newBlitter(ctx),
		pather:  newPather(ctx),
	}
	r.packer.maxDim = ctx.Caps().MaxTextureSize
	r.intersections.maxDim = r.packer.maxDim
	return r
}

func (r *renderer) release() {
	r.pather.release()
	r.blitter.release()
}

func newBlitter(ctx backend.Device) *blitter {
	quadVerts, err := ctx.NewImmutableBuffer(backend.BufferBindingVertices,
		gunsafe.BytesView([]float32{
			-1, +1, 0, 0,
			+1, +1, 1, 0,
			-1, -1, 0, 1,
			+1, -1, 1, 1,
		}),
	)
	if err != nil {
		panic(err)
	}
	b := &blitter{
		ctx:       ctx,
		quadVerts: quadVerts,
	}
	b.colUniforms = new(blitColUniforms)
	b.texUniforms = new(blitTexUniforms)
	prog, layout, err := createColorPrograms(ctx, shader_blit_vert, shader_blit_frag,
		[2]interface{}{&b.colUniforms.vert, &b.texUniforms.vert}, [2]interface{}{&b.colUniforms.frag, nil})
	if err != nil {
		panic(err)
	}
	b.prog = prog
	b.layout = layout
	return b
}

func (b *blitter) release() {
	b.quadVerts.Release()
	for _, p := range b.prog {
		p.Release()
	}
	b.layout.Release()
}

func createColorPrograms(b backend.Device, vsSrc backend.ShaderSources, fsSrc [2]backend.ShaderSources, vertUniforms, fragUniforms [2]interface{}) ([2]*program, backend.InputLayout, error) {
	var progs [2]*program
	prog, err := b.NewProgram(vsSrc, fsSrc[materialTexture])
	if err != nil {
		return progs, nil, err
	}
	var vertBuffer *uniformBuffer
	if u := vertUniforms[materialTexture]; u != nil {
		vertBuffer = newUniformBuffer(b, u)
		prog.SetVertexUniforms(vertBuffer.buf)
	}
	var fragBuffer *uniformBuffer
	if u := fragUniforms[materialTexture]; u != nil {
		fragBuffer = newUniformBuffer(b, u)
		prog.SetFragmentUniforms(fragBuffer.buf)
	}
	progs[materialTexture] = newProgram(prog, vertBuffer, fragBuffer)
	prog, err = b.NewProgram(vsSrc, fsSrc[materialColor])
	if err != nil {
		progs[materialTexture].Release()
		return progs, nil, err
	}
	if u := vertUniforms[materialColor]; u != nil {
		vertBuffer = newUniformBuffer(b, u)
		prog.SetVertexUniforms(vertBuffer.buf)
	}
	if u := fragUniforms[materialColor]; u != nil {
		fragBuffer = newUniformBuffer(b, u)
		prog.SetFragmentUniforms(fragBuffer.buf)
	}
	progs[materialColor] = newProgram(prog, vertBuffer, fragBuffer)
	layout, err := b.NewInputLayout(vsSrc, []backend.InputDesc{
		{Type: backend.DataTypeFloat, Size: 2, Offset: 0},
		{Type: backend.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		progs[materialTexture].Release()
		progs[materialColor].Release()
		return progs, nil, err
	}
	return progs, layout, nil
}

func (r *renderer) stencilClips(pathCache *opCache, ops []*pathOp) {
	if len(r.packer.sizes) == 0 {
		return
	}
	fbo := -1
	r.pather.begin(r.packer.sizes)
	for _, p := range ops {
		if fbo != p.place.Idx {
			fbo = p.place.Idx
			f := r.pather.stenciler.cover(fbo)
			r.ctx.BindFramebuffer(f.fbo)
			r.ctx.Clear(0.0, 0.0, 0.0, 0.0)
		}
		v, _ := pathCache.get(p.pathKey)
		r.pather.stencilPath(p.clip, p.off, p.place.Pos, v.data)
	}
}

func (r *renderer) intersect(ops []imageOp) {
	if len(r.intersections.sizes) == 0 {
		return
	}
	fbo := -1
	r.pather.stenciler.beginIntersect(r.intersections.sizes)
	r.ctx.BindVertexBuffer(r.blitter.quadVerts, 4*4, 0)
	r.ctx.BindInputLayout(r.pather.stenciler.iprog.layout)
	for _, img := range ops {
		if img.clipType != clipTypeIntersection {
			continue
		}
		if fbo != img.place.Idx {
			fbo = img.place.Idx
			f := r.pather.stenciler.intersections.fbos[fbo]
			r.ctx.BindFramebuffer(f.fbo)
			r.ctx.Clear(1.0, 0.0, 0.0, 0.0)
		}
		r.ctx.Viewport(img.place.Pos.X, img.place.Pos.Y, img.clip.Dx(), img.clip.Dy())
		r.intersectPath(img.path, img.clip)
	}
}

func (r *renderer) intersectPath(p *pathOp, clip image.Rectangle) {
	if p.parent != nil {
		r.intersectPath(p.parent, clip)
	}
	if !p.path {
		return
	}
	uv := image.Rectangle{
		Min: p.place.Pos,
		Max: p.place.Pos.Add(p.clip.Size()),
	}
	o := clip.Min.Sub(p.clip.Min)
	sub := image.Rectangle{
		Min: o,
		Max: o.Add(clip.Size()),
	}
	fbo := r.pather.stenciler.cover(p.place.Idx)
	r.ctx.BindTexture(0, fbo.tex)
	coverScale, coverOff := texSpaceTransform(toRectF(uv), fbo.size)
	subScale, subOff := texSpaceTransform(toRectF(sub), p.clip.Size())
	r.pather.stenciler.iprog.uniforms.vert.uvTransform = [4]float32{coverScale.X, coverScale.Y, coverOff.X, coverOff.Y}
	r.pather.stenciler.iprog.uniforms.vert.subUVTransform = [4]float32{subScale.X, subScale.Y, subOff.X, subOff.Y}
	r.pather.stenciler.iprog.prog.UploadUniforms()
	r.ctx.DrawArrays(backend.DrawModeTriangleStrip, 0, 4)
}

func (r *renderer) packIntersections(ops []imageOp) {
	r.intersections.clear()
	for i, img := range ops {
		var npaths int
		var onePath *pathOp
		for p := img.path; p != nil; p = p.parent {
			if p.path {
				onePath = p
				npaths++
			}
		}
		switch npaths {
		case 0:
		case 1:
			place := onePath.place
			place.Pos = place.Pos.Sub(onePath.clip.Min).Add(img.clip.Min)
			ops[i].place = place
			ops[i].clipType = clipTypePath
		default:
			sz := image.Point{X: img.clip.Dx(), Y: img.clip.Dy()}
			place, ok := r.intersections.add(sz)
			if !ok {
				panic("internal error: if the intersection fit, the intersection should fit as well")
			}
			ops[i].clipType = clipTypeIntersection
			ops[i].place = place
		}
	}
}

func (r *renderer) packStencils(pops *[]*pathOp) {
	r.packer.clear()
	ops := *pops
	// Allocate atlas space for cover textures.
	var i int
	for i < len(ops) {
		p := ops[i]
		if p.clip.Empty() {
			ops[i] = ops[len(ops)-1]
			ops = ops[:len(ops)-1]
			continue
		}
		sz := image.Point{X: p.clip.Dx(), Y: p.clip.Dy()}
		place, ok := r.packer.add(sz)
		if !ok {
			// The clip area is at most the entire screen. Hopefully no
			// screen is larger than GL_MAX_TEXTURE_SIZE.
			panic(fmt.Errorf("clip area %v is larger than maximum texture size %dx%d", p.clip, r.packer.maxDim, r.packer.maxDim))
		}
		p.place = place
		i++
	}
	*pops = ops
}

// intersects intersects clip and b where b is offset by off.
// ceilRect returns a bounding image.Rectangle for a f32.Rectangle.
func boundRectF(r f32.Rectangle) image.Rectangle {
	return image.Rectangle{
		Min: image.Point{
			X: int(floor(r.Min.X)),
			Y: int(floor(r.Min.Y)),
		},
		Max: image.Point{
			X: int(ceil(r.Max.X)),
			Y: int(ceil(r.Max.Y)),
		},
	}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{
			X: float32(r.Min.X),
			Y: float32(r.Min.Y),
		},
		Max: f32.Point{
			X: float32(r.Max.X),
			Y: float32(r.Max.Y),
		},
	}
}

func ceil(v float32) int {
	return int(math.Ceil(float64(v)))
}

func floor(v float32) int {
	return int(math.Floor(float64(v)))
}

func (d *drawOps) reset(cache *resourceCache, viewport image.Point) {
	d.profile = false
	d.clearColor = f32color.RGBA{R: 1.0, G: 1.0, B: 1.0, A: 1.0}
	d.cache = cache
	d.viewport = viewport
	d.imageOps = d.imageOps[:0]
	d.zimageOps = d.zimageOps[:0]
	d.pathOps = d.pathOps[:0]
	d.pathOpCache = d.pathOpCache[:0]
	d.vertCache = d.vertCache[:0]
}

func (d *drawOps) collect(cache *resourceCache, root *op.Ops, viewport image.Point) {
	d.reset(cache, viewport)
	clip := f32.Rectangle{
		Max: f32.Point{X: float32(viewport.X), Y: float32(viewport.Y)},
	}
	d.reader.Reset(root)
	state := drawState{
		clip:  clip,
		rect:  true,
		color: color.RGBA{A: 0xff},
	}
	d.collectOps(&d.reader, state)
}

func (d *drawOps) newPathOp() *pathOp {
	d.pathOpCache = append(d.pathOpCache, pathOp{})
	return &d.pathOpCache[len(d.pathOpCache)-1]
}

func (d *drawOps) addClipPath(state *drawState, aux []byte, auxKey ops.Key, bounds f32.Rectangle, off f32.Point) {
	npath := d.newPathOp()
	*npath = pathOp{
		parent: state.cpath,
		bounds: bounds,
		off:    off,
	}
	state.cpath = npath
	if len(aux) > 0 {
		state.rect = false
		state.cpath.pathKey = auxKey
		state.cpath.path = true
		state.cpath.pathVerts = aux
		d.pathOps = append(d.pathOps, state.cpath)
	}
}

// split a transform into two parts, one which is pur offset and the
// other representing the scaling, shearing and rotation part
func splitTransform(t f32.Affine2D) (srs f32.Affine2D, offset f32.Point) {
	sx, hx, ox, hy, sy, oy := t.Elems()
	offset = f32.Point{X: ox, Y: oy}
	srs = f32.NewAffine2D(sx, hx, 0, hy, sy, 0)
	return
}

func (d *drawOps) collectOps(r *ops.Reader, state drawState) int {
	var aux []byte
	var auxKey ops.Key
loop:
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeProfile:
			d.profile = true
		case opconst.TypeTransform:
			dop := ops.DecodeTransform(encOp.Data)
			state.t = state.t.Mul(dop)
		case opconst.TypeAux:
			aux = encOp.Data[opconst.TypeAuxLen:]
			auxKey = encOp.Key
		case opconst.TypeClip:
			var op clipOp
			op.decode(encOp.Data)
			bounds := op.bounds
			trans, off := splitTransform(state.t)
			if len(aux) > 0 {
				// There is a clipping path, build the gpu data and update the
				// cache key such that it will be equal only if the transform is the
				// same also. Use cached data if we have it.
				auxKey = auxKey.SetTransform(trans)
				if v, ok := d.pathCache.get(auxKey); ok {
					// Since the GPU data exists in the cache aux will not be used.
					// Why is this not used for the offset shapes?
					op.bounds = v.bounds
				} else {
					aux, op.bounds = d.buildVerts(aux, trans)
					// add it to the cache, without GPU data, so the transform can be
					// reused.
					d.pathCache.put(auxKey, opCacheValue{bounds: op.bounds})
				}
			} else {
				aux, op.bounds, _ = d.boundsForTransformedRect(bounds, trans)
				auxKey = encOp.Key
				auxKey.SetTransform(trans)
			}
			state.clip = state.clip.Intersect(op.bounds.Add(off))
			d.addClipPath(&state, aux, auxKey, op.bounds, off)
			aux = nil
			auxKey = ops.Key{}
		case opconst.TypeColor:
			state.matType = materialColor
			state.color = decodeColorOp(encOp.Data)
		case opconst.TypeImage:
			state.matType = materialTexture
			state.image = decodeImageOp(encOp.Data, encOp.Refs)
		case opconst.TypePaint:
			op := decodePaintOp(encOp.Data)
			// Transform (if needed) the painting rectangle and if so generate a clip path,
			// for those cases also compute a partialTrans that maps texture coordinates between
			// the new bounding rectangle and the transformed original paint rectangle.
			trans, off := splitTransform(state.t)
			clipData, bnd, partialTrans := d.boundsForTransformedRect(op.Rect, trans)
			clip := state.clip.Intersect(bnd.Add(off))
			if clip.Empty() {
				continue
			}

			wasrect := state.rect
			if clipData != nil {
				// The paint operation is sheared or rotated, add a clip path representing
				// this transformed rectangle.
				encOp.Key.SetTransform(trans)
				d.addClipPath(&state, clipData, encOp.Key, bnd, off)
			}

			bounds := boundRectF(clip)
			mat := state.materialFor(d.cache, bnd, off, partialTrans, bounds)

			if bounds.Min == (image.Point{}) && bounds.Max == d.viewport && state.rect && mat.opaque && mat.material == materialColor {
				// The image is a uniform opaque color and takes up the whole screen.
				// Scrap images up to and including this image and set clear color.
				d.zimageOps = d.zimageOps[:0]
				d.imageOps = d.imageOps[:0]
				state.z = 0
				d.clearColor = mat.color.Opaque()
				continue
			}
			state.z++
			if state.z != int(uint16(state.z)) {
				// TODO(eliasnaur) gioui.org/issue/127.
				panic("more than 65k paint objects not supported")
			}
			// Assume 16-bit depth buffer.
			const zdepth = 1 << 16
			// Convert z to window-space, assuming depth range [0;1].
			zf := float32(state.z)*2/zdepth - 1.0
			img := imageOp{
				z:        zf,
				path:     state.cpath,
				off:      off,
				clip:     bounds,
				material: mat,
			}

			if state.rect && img.material.opaque {
				d.zimageOps = append(d.zimageOps, img)
			} else {
				d.imageOps = append(d.imageOps, img)
			}
			if clipData != nil {
				// we added a clip path that should not remain
				state.cpath = state.cpath.parent
				state.rect = wasrect
			}
		case opconst.TypePush:
			state.z = d.collectOps(r, state)
		case opconst.TypePop:
			break loop
		}
	}
	return state.z
}

func expandPathOp(p *pathOp, clip image.Rectangle) {
	for p != nil {
		pclip := p.clip
		if !pclip.Empty() {
			clip = clip.Union(pclip)
		}
		p.clip = clip
		p = p.parent
	}
}

func (d *drawState) materialFor(cache *resourceCache, rect f32.Rectangle, off f32.Point, trans f32.Affine2D, clip image.Rectangle) material {
	var m material
	switch d.matType {
	case materialColor:
		m.material = materialColor
		m.color = f32color.RGBAFromSRGB(d.color)
		m.opaque = m.color.A == 1.0
	case materialTexture:
		m.material = materialTexture
		dr := boundRectF(rect.Add(off))
		sz := d.image.src.Bounds().Size()
		sr := layout.FRect(d.image.rect)
		if dx := float32(dr.Dx()); dx != 0 {
			// Don't clip 1 px width sources.
			if sdx := sr.Dx(); sdx > 1 {
				sr.Min.X += (float32(clip.Min.X-dr.Min.X)*sdx + dx/2) / dx
				sr.Max.X -= (float32(dr.Max.X-clip.Max.X)*sdx + dx/2) / dx
			}
		}
		if dy := float32(dr.Dy()); dy != 0 {
			// Don't clip 1 px height sources.
			if sdy := sr.Dy(); sdy > 1 {
				sr.Min.Y += (float32(clip.Min.Y-dr.Min.Y)*sdy + dy/2) / dy
				sr.Max.Y -= (float32(dr.Max.Y-clip.Max.Y)*sdy + dy/2) / dy
			}
		}
		tex, exists := cache.get(d.image.handle)
		if !exists {
			t := &texture{
				src: d.image.src,
			}
			cache.put(d.image.handle, t)
			tex = t
		}
		m.texture = tex.(*texture)
		uvScale, uvOffset := texSpaceTransform(sr, sz)
		m.uvTrans = trans.Mul(f32.Affine2D{}.Scale(f32.Point{}, uvScale).Offset(uvOffset))
	}
	return m
}

func (r *renderer) drawZOps(ops []imageOp) {
	r.ctx.SetDepthTest(true)
	r.ctx.BindVertexBuffer(r.blitter.quadVerts, 4*4, 0)
	r.ctx.BindInputLayout(r.blitter.layout)
	// Render front to back.
	for i := len(ops) - 1; i >= 0; i-- {
		img := ops[i]
		m := img.material
		switch m.material {
		case materialTexture:
			r.ctx.BindTexture(0, r.texHandle(m.texture))
		}
		drc := img.clip
		scale, off := clipSpaceTransform(drc, r.blitter.viewport)
		r.blitter.blit(img.z, m.material, m.color, scale, off, m.uvTrans)
	}
	r.ctx.SetDepthTest(false)
}

func (r *renderer) drawOps(ops []imageOp) {
	r.ctx.SetDepthTest(true)
	r.ctx.DepthMask(false)
	r.ctx.BlendFunc(backend.BlendFactorOne, backend.BlendFactorOneMinusSrcAlpha)
	r.ctx.BindVertexBuffer(r.blitter.quadVerts, 4*4, 0)
	r.ctx.BindInputLayout(r.pather.coverer.layout)
	var coverTex backend.Texture
	for _, img := range ops {
		m := img.material
		switch m.material {
		case materialTexture:
			r.ctx.BindTexture(0, r.texHandle(m.texture))
		}
		drc := img.clip

		scale, off := clipSpaceTransform(drc, r.blitter.viewport)
		var fbo stencilFBO
		switch img.clipType {
		case clipTypeNone:
			r.blitter.blit(img.z, m.material, m.color, scale, off, m.uvTrans)
			continue
		case clipTypePath:
			fbo = r.pather.stenciler.cover(img.place.Idx)
		case clipTypeIntersection:
			fbo = r.pather.stenciler.intersections.fbos[img.place.Idx]
		}
		if coverTex != fbo.tex {
			coverTex = fbo.tex
			r.ctx.BindTexture(1, coverTex)
		}
		uv := image.Rectangle{
			Min: img.place.Pos,
			Max: img.place.Pos.Add(drc.Size()),
		}
		coverScale, coverOff := texSpaceTransform(toRectF(uv), fbo.size)
		r.pather.cover(img.z, m.material, m.color, scale, off, m.uvTrans, coverScale, coverOff)
	}
	r.ctx.DepthMask(true)
	r.ctx.SetDepthTest(false)
}

func (b *blitter) blit(z float32, mat materialType, col f32color.RGBA, scale, off f32.Point, uvTrans f32.Affine2D) {
	p := b.prog[mat]
	b.ctx.BindProgram(p.prog)
	var uniforms *blitUniforms
	switch mat {
	case materialColor:
		b.colUniforms.frag.color = col
		uniforms = &b.colUniforms.vert.blitUniforms
	case materialTexture:
		t1, t2, t3, t4, t5, t6 := uvTrans.Elems()
		b.texUniforms.vert.blitUniforms.uvTransformR1 = [4]float32{t1, t2, t3, 0}
		b.texUniforms.vert.blitUniforms.uvTransformR2 = [4]float32{t4, t5, t6, 0}
		uniforms = &b.texUniforms.vert.blitUniforms
	}
	uniforms.z = z
	uniforms.transform = [4]float32{scale.X, scale.Y, off.X, off.Y}
	p.UploadUniforms()
	b.ctx.DrawArrays(backend.DrawModeTriangleStrip, 0, 4)
}

// newUniformBuffer creates a new GPU uniform buffer backed by the
// structure uniformBlock points to.
func newUniformBuffer(b backend.Device, uniformBlock interface{}) *uniformBuffer {
	ref := reflect.ValueOf(uniformBlock)
	// Determine the size of the uniforms structure, *uniforms.
	size := ref.Elem().Type().Size()
	// Map the uniforms structure as a byte slice.
	ptr := (*[1 << 30]byte)(unsafe.Pointer(ref.Pointer()))[:size:size]
	ubuf, err := b.NewBuffer(backend.BufferBindingUniforms, len(ptr))
	if err != nil {
		panic(err)
	}
	return &uniformBuffer{buf: ubuf, ptr: ptr}
}

func (u *uniformBuffer) Upload() {
	u.buf.Upload(u.ptr)
}

func (u *uniformBuffer) Release() {
	u.buf.Release()
	u.buf = nil
}

func newProgram(prog backend.Program, vertUniforms, fragUniforms *uniformBuffer) *program {
	if vertUniforms != nil {
		prog.SetVertexUniforms(vertUniforms.buf)
	}
	if fragUniforms != nil {
		prog.SetFragmentUniforms(fragUniforms.buf)
	}
	return &program{prog: prog, vertUniforms: vertUniforms, fragUniforms: fragUniforms}
}

func (p *program) UploadUniforms() {
	if p.vertUniforms != nil {
		p.vertUniforms.Upload()
	}
	if p.fragUniforms != nil {
		p.fragUniforms.Upload()
	}
}

func (p *program) Release() {
	p.prog.Release()
	p.prog = nil
	if p.vertUniforms != nil {
		p.vertUniforms.Release()
		p.vertUniforms = nil
	}
	if p.fragUniforms != nil {
		p.fragUniforms.Release()
		p.fragUniforms = nil
	}
}

// texSpaceTransform return the scale and offset that transforms the given subimage
// into quad texture coordinates.
func texSpaceTransform(r f32.Rectangle, bounds image.Point) (f32.Point, f32.Point) {
	size := f32.Point{X: float32(bounds.X), Y: float32(bounds.Y)}
	scale := f32.Point{X: r.Dx() / size.X, Y: r.Dy() / size.Y}
	offset := f32.Point{X: r.Min.X / size.X, Y: r.Min.Y / size.Y}
	return scale, offset
}

// clipSpaceTransform returns the scale and offset that transforms the given
// rectangle from a viewport into OpenGL clip space.
func clipSpaceTransform(r image.Rectangle, viewport image.Point) (f32.Point, f32.Point) {
	// First, transform UI coordinates to OpenGL coordinates:
	//
	//	[(-1, +1) (+1, +1)]
	//	[(-1, -1) (+1, -1)]
	//
	x, y := float32(r.Min.X), float32(r.Min.Y)
	w, h := float32(r.Dx()), float32(r.Dy())
	vx, vy := 2/float32(viewport.X), 2/float32(viewport.Y)
	x = x*vx - 1
	y = 1 - y*vy
	w *= vx
	h *= vy

	// Then, compute the transformation from the fullscreen quad to
	// the rectangle at (x, y) and dimensions (w, h).
	scale := f32.Point{X: w * .5, Y: h * .5}
	offset := f32.Point{X: x + w*.5, Y: y - h*.5}

	return scale, offset
}

// Fill in maximal Y coordinates of the NW and NE corners.
func fillMaxY(verts []byte) {
	contour := 0
	bo := binary.LittleEndian
	for len(verts) > 0 {
		maxy := float32(math.Inf(-1))
		i := 0
		for ; i+vertStride*4 <= len(verts); i += vertStride * 4 {
			vert := verts[i : i+vertStride]
			// MaxY contains the integer contour index.
			pathContour := int(bo.Uint32(vert[int(unsafe.Offsetof(((*vertex)(nil)).MaxY)):]))
			if contour != pathContour {
				contour = pathContour
				break
			}
			fromy := math.Float32frombits(bo.Uint32(vert[int(unsafe.Offsetof(((*vertex)(nil)).FromY)):]))
			ctrly := math.Float32frombits(bo.Uint32(vert[int(unsafe.Offsetof(((*vertex)(nil)).CtrlY)):]))
			toy := math.Float32frombits(bo.Uint32(vert[int(unsafe.Offsetof(((*vertex)(nil)).ToY)):]))
			if fromy > maxy {
				maxy = fromy
			}
			if ctrly > maxy {
				maxy = ctrly
			}
			if toy > maxy {
				maxy = toy
			}
		}
		fillContourMaxY(maxy, verts[:i])
		verts = verts[i:]
	}
}

func fillContourMaxY(maxy float32, verts []byte) {
	bo := binary.LittleEndian
	for i := 0; i < len(verts); i += vertStride {
		off := int(unsafe.Offsetof(((*vertex)(nil)).MaxY))
		bo.PutUint32(verts[i+off:], math.Float32bits(maxy))
	}
}

func (d *drawOps) writeVertCache(n int) []byte {
	d.vertCache = append(d.vertCache, make([]byte, n)...)
	return d.vertCache[len(d.vertCache)-n:]
}

// transform, split paths as needed, calculate maxY, bounds and create GPU vertices.
func (d *drawOps) buildVerts(aux []byte, tr f32.Affine2D) (verts []byte, bounds f32.Rectangle) {
	inf := float32(math.Inf(+1))
	d.qs.bounds = f32.Rectangle{
		Min: f32.Point{X: inf, Y: inf},
		Max: f32.Point{X: -inf, Y: -inf},
	}
	d.qs.d = d
	bo := binary.LittleEndian
	startLength := len(d.vertCache)
	for qi := 0; len(aux) >= (ops.QuadSize + 4); qi++ {
		d.qs.contour = bo.Uint32(aux)
		quad := ops.DecodeQuad(aux[4:])
		quad = quad.Transform(tr)

		d.qs.splitAndEncode(quad)

		aux = aux[ops.QuadSize+4:]
	}

	fillMaxY(d.vertCache[startLength:])
	return d.vertCache[startLength:], d.qs.bounds
}

// create GPU vertices for transformed r, find the bounds and establish texture transform.
func (d *drawOps) boundsForTransformedRect(r f32.Rectangle, tr f32.Affine2D) (aux []byte, bnd f32.Rectangle, ptr f32.Affine2D) {
	if isPureOffset(tr) {
		// fast-path to allow blitting of pure rectangles
		_, _, ox, _, _, oy := tr.Elems()
		off := f32.Pt(ox, oy)
		bnd.Min = r.Min.Add(off)
		bnd.Max = r.Max.Add(off)
		return
	}

	// transform all corners, find new bounds
	corners := [4]f32.Point{
		tr.Transform(r.Min), tr.Transform(f32.Pt(r.Max.X, r.Min.Y)),
		tr.Transform(r.Max), tr.Transform(f32.Pt(r.Min.X, r.Max.Y)),
	}
	bnd.Min = f32.Pt(math.MaxFloat32, math.MaxFloat32)
	bnd.Max = f32.Pt(-math.MaxFloat32, -math.MaxFloat32)
	for _, c := range corners {
		if c.X < bnd.Min.X {
			bnd.Min.X = c.X
		}
		if c.Y < bnd.Min.Y {
			bnd.Min.Y = c.Y
		}
		if c.X > bnd.Max.X {
			bnd.Max.X = c.X
		}
		if c.Y > bnd.Max.Y {
			bnd.Max.Y = c.Y
		}
	}

	// build the GPU vertices
	l := len(d.vertCache)
	d.vertCache = append(d.vertCache, make([]byte, vertStride*4*4)...)
	aux = d.vertCache[l:]
	encodeQuadTo(aux, 0, corners[0], corners[0].Add(corners[1]).Mul(0.5), corners[1])
	encodeQuadTo(aux[vertStride*4:], 0, corners[1], corners[1].Add(corners[2]).Mul(0.5), corners[2])
	encodeQuadTo(aux[vertStride*4*2:], 0, corners[2], corners[2].Add(corners[3]).Mul(0.5), corners[3])
	encodeQuadTo(aux[vertStride*4*3:], 0, corners[3], corners[3].Add(corners[0]).Mul(0.5), corners[0])
	fillMaxY(aux)

	// establish the transform mapping from bounds rectangle to transformed corners
	var P1, P2, P3 f32.Point
	P1.X = (corners[1].X - bnd.Min.X) / (bnd.Max.X - bnd.Min.X)
	P1.Y = (corners[1].Y - bnd.Min.Y) / (bnd.Max.Y - bnd.Min.Y)
	P2.X = (corners[2].X - bnd.Min.X) / (bnd.Max.X - bnd.Min.X)
	P2.Y = (corners[2].Y - bnd.Min.Y) / (bnd.Max.Y - bnd.Min.Y)
	P3.X = (corners[3].X - bnd.Min.X) / (bnd.Max.X - bnd.Min.X)
	P3.Y = (corners[3].Y - bnd.Min.Y) / (bnd.Max.Y - bnd.Min.Y)
	sx, sy := P2.X-P3.X, P2.Y-P3.Y
	ptr = f32.NewAffine2D(sx, P2.X-P1.X, P1.X-sx, sy, P2.Y-P1.Y, P1.Y-sy).Invert()

	return
}

func isPureOffset(t f32.Affine2D) bool {
	a, b, _, d, e, _ := t.Elems()
	return a == 1 && b == 0 && d == 0 && e == 1
}
