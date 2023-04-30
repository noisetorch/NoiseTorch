// SPDX-License-Identifier: Unlicense OR MIT

package gpu

// GPU accelerated path drawing using the algorithms from
// Pathfinder (https://github.com/servo/pathfinder).

import (
	"encoding/binary"
	"image"
	"math"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/f32color"
)

type pather struct {
	ctx driver.Device

	viewport image.Point

	stenciler *stenciler
	coverer   *coverer
}

type coverer struct {
	ctx                    driver.Device
	prog                   [3]*program
	texUniforms            *coverTexUniforms
	colUniforms            *coverColUniforms
	linearGradientUniforms *coverLinearGradientUniforms
	layout                 driver.InputLayout
}

type coverTexUniforms struct {
	vert struct {
		coverUniforms
		_ [12]byte // Padding to multiple of 16.
	}
}

type coverColUniforms struct {
	vert struct {
		coverUniforms
		_ [12]byte // Padding to multiple of 16.
	}
	frag struct {
		colorUniforms
	}
}

type coverLinearGradientUniforms struct {
	vert struct {
		coverUniforms
		_ [12]byte // Padding to multiple of 16.
	}
	frag struct {
		gradientUniforms
	}
}

type coverUniforms struct {
	transform        [4]float32
	uvCoverTransform [4]float32
	uvTransformR1    [4]float32
	uvTransformR2    [4]float32
	z                float32
}

type stenciler struct {
	ctx  driver.Device
	prog struct {
		prog     *program
		uniforms *stencilUniforms
		layout   driver.InputLayout
	}
	iprog struct {
		prog     *program
		uniforms *intersectUniforms
		layout   driver.InputLayout
	}
	fbos          fboSet
	intersections fboSet
	indexBuf      driver.Buffer
}

type stencilUniforms struct {
	vert struct {
		transform  [4]float32
		pathOffset [2]float32
		_          [8]byte // Padding to multiple of 16.
	}
}

type intersectUniforms struct {
	vert struct {
		uvTransform    [4]float32
		subUVTransform [4]float32
	}
}

type fboSet struct {
	fbos []stencilFBO
}

type stencilFBO struct {
	size image.Point
	fbo  driver.Framebuffer
	tex  driver.Texture
}

type pathData struct {
	ncurves int
	data    driver.Buffer
}

// vertex data suitable for passing to vertex programs.
type vertex struct {
	// Corner encodes the corner: +0.5 for south, +.25 for east.
	Corner       float32
	MaxY         float32
	FromX, FromY float32
	CtrlX, CtrlY float32
	ToX, ToY     float32
}

func (v vertex) encode(d []byte, maxy uint32) {
	bo := binary.LittleEndian
	bo.PutUint32(d[0:], math.Float32bits(v.Corner))
	bo.PutUint32(d[4:], maxy)
	bo.PutUint32(d[8:], math.Float32bits(v.FromX))
	bo.PutUint32(d[12:], math.Float32bits(v.FromY))
	bo.PutUint32(d[16:], math.Float32bits(v.CtrlX))
	bo.PutUint32(d[20:], math.Float32bits(v.CtrlY))
	bo.PutUint32(d[24:], math.Float32bits(v.ToX))
	bo.PutUint32(d[28:], math.Float32bits(v.ToY))
}

const (
	// Number of path quads per draw batch.
	pathBatchSize = 10000
	// Size of a vertex as sent to gpu
	vertStride = 8 * 4
)

func newPather(ctx driver.Device) *pather {
	return &pather{
		ctx:       ctx,
		stenciler: newStenciler(ctx),
		coverer:   newCoverer(ctx),
	}
}

func newCoverer(ctx driver.Device) *coverer {
	c := &coverer{
		ctx: ctx,
	}
	c.colUniforms = new(coverColUniforms)
	c.texUniforms = new(coverTexUniforms)
	c.linearGradientUniforms = new(coverLinearGradientUniforms)
	prog, layout, err := createColorPrograms(ctx, shader_cover_vert, shader_cover_frag,
		[3]interface{}{&c.colUniforms.vert, &c.linearGradientUniforms.vert, &c.texUniforms.vert},
		[3]interface{}{&c.colUniforms.frag, &c.linearGradientUniforms.frag, nil},
	)
	if err != nil {
		panic(err)
	}
	c.prog = prog
	c.layout = layout
	return c
}

func newStenciler(ctx driver.Device) *stenciler {
	// Allocate a suitably large index buffer for drawing paths.
	indices := make([]uint16, pathBatchSize*6)
	for i := 0; i < pathBatchSize; i++ {
		i := uint16(i)
		indices[i*6+0] = i*4 + 0
		indices[i*6+1] = i*4 + 1
		indices[i*6+2] = i*4 + 2
		indices[i*6+3] = i*4 + 2
		indices[i*6+4] = i*4 + 1
		indices[i*6+5] = i*4 + 3
	}
	indexBuf, err := ctx.NewImmutableBuffer(driver.BufferBindingIndices, byteslice.Slice(indices))
	if err != nil {
		panic(err)
	}
	progLayout, err := ctx.NewInputLayout(shader_stencil_vert, []driver.InputDesc{
		{Type: driver.DataTypeFloat, Size: 1, Offset: int(unsafe.Offsetof((*(*vertex)(nil)).Corner))},
		{Type: driver.DataTypeFloat, Size: 1, Offset: int(unsafe.Offsetof((*(*vertex)(nil)).MaxY))},
		{Type: driver.DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*vertex)(nil)).FromX))},
		{Type: driver.DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*vertex)(nil)).CtrlX))},
		{Type: driver.DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*vertex)(nil)).ToX))},
	})
	if err != nil {
		panic(err)
	}
	iprogLayout, err := ctx.NewInputLayout(shader_intersect_vert, []driver.InputDesc{
		{Type: driver.DataTypeFloat, Size: 2, Offset: 0},
		{Type: driver.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		panic(err)
	}
	st := &stenciler{
		ctx:      ctx,
		indexBuf: indexBuf,
	}
	prog, err := ctx.NewProgram(shader_stencil_vert, shader_stencil_frag)
	if err != nil {
		panic(err)
	}
	st.prog.uniforms = new(stencilUniforms)
	vertUniforms := newUniformBuffer(ctx, &st.prog.uniforms.vert)
	st.prog.prog = newProgram(prog, vertUniforms, nil)
	st.prog.layout = progLayout
	iprog, err := ctx.NewProgram(shader_intersect_vert, shader_intersect_frag)
	if err != nil {
		panic(err)
	}
	st.iprog.uniforms = new(intersectUniforms)
	vertUniforms = newUniformBuffer(ctx, &st.iprog.uniforms.vert)
	st.iprog.prog = newProgram(iprog, vertUniforms, nil)
	st.iprog.layout = iprogLayout
	return st
}

func (s *fboSet) resize(ctx driver.Device, sizes []image.Point) {
	// Add fbos.
	for i := len(s.fbos); i < len(sizes); i++ {
		s.fbos = append(s.fbos, stencilFBO{})
	}
	// Resize fbos.
	for i, sz := range sizes {
		f := &s.fbos[i]
		// Resizing or recreating FBOs can introduce rendering stalls.
		// Avoid if the space waste is not too high.
		resize := sz.X > f.size.X || sz.Y > f.size.Y
		waste := float32(sz.X*sz.Y) / float32(f.size.X*f.size.Y)
		resize = resize || waste > 1.2
		if resize {
			if f.fbo != nil {
				f.fbo.Release()
				f.tex.Release()
			}
			tex, err := ctx.NewTexture(driver.TextureFormatFloat, sz.X, sz.Y, driver.FilterNearest, driver.FilterNearest,
				driver.BufferBindingTexture|driver.BufferBindingFramebuffer)
			if err != nil {
				panic(err)
			}
			fbo, err := ctx.NewFramebuffer(tex, 0)
			if err != nil {
				panic(err)
			}
			f.size = sz
			f.tex = tex
			f.fbo = fbo
		}
	}
	// Delete extra fbos.
	s.delete(ctx, len(sizes))
}

func (s *fboSet) invalidate(ctx driver.Device) {
	for _, f := range s.fbos {
		f.fbo.Invalidate()
	}
}

func (s *fboSet) delete(ctx driver.Device, idx int) {
	for i := idx; i < len(s.fbos); i++ {
		f := s.fbos[i]
		f.fbo.Release()
		f.tex.Release()
	}
	s.fbos = s.fbos[:idx]
}

func (s *stenciler) release() {
	s.fbos.delete(s.ctx, 0)
	s.prog.layout.Release()
	s.prog.prog.Release()
	s.iprog.layout.Release()
	s.iprog.prog.Release()
	s.indexBuf.Release()
}

func (p *pather) release() {
	p.stenciler.release()
	p.coverer.release()
}

func (c *coverer) release() {
	for _, p := range c.prog {
		p.Release()
	}
	c.layout.Release()
}

func buildPath(ctx driver.Device, p []byte) pathData {
	buf, err := ctx.NewImmutableBuffer(driver.BufferBindingVertices, p)
	if err != nil {
		panic(err)
	}
	return pathData{
		ncurves: len(p) / vertStride,
		data:    buf,
	}
}

func (p pathData) release() {
	p.data.Release()
}

func (p *pather) begin(sizes []image.Point) {
	p.stenciler.begin(sizes)
}

func (p *pather) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data pathData) {
	p.stenciler.stencilPath(bounds, offset, uv, data)
}

func (s *stenciler) beginIntersect(sizes []image.Point) {
	s.ctx.BlendFunc(driver.BlendFactorDstColor, driver.BlendFactorZero)
	// 8 bit coverage is enough, but OpenGL ES only supports single channel
	// floating point formats. Replace with GL_RGB+GL_UNSIGNED_BYTE if
	// no floating point support is available.
	s.intersections.resize(s.ctx, sizes)
	s.ctx.BindProgram(s.iprog.prog.prog)
}

func (s *stenciler) invalidateFBO() {
	s.intersections.invalidate(s.ctx)
	s.fbos.invalidate(s.ctx)
}

func (s *stenciler) cover(idx int) stencilFBO {
	return s.fbos.fbos[idx]
}

func (s *stenciler) begin(sizes []image.Point) {
	s.ctx.BlendFunc(driver.BlendFactorOne, driver.BlendFactorOne)
	s.fbos.resize(s.ctx, sizes)
	s.ctx.BindProgram(s.prog.prog.prog)
	s.ctx.BindInputLayout(s.prog.layout)
	s.ctx.BindIndexBuffer(s.indexBuf)
}

func (s *stenciler) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data pathData) {
	s.ctx.Viewport(uv.X, uv.Y, bounds.Dx(), bounds.Dy())
	// Transform UI coordinates to OpenGL coordinates.
	texSize := f32.Point{X: float32(bounds.Dx()), Y: float32(bounds.Dy())}
	scale := f32.Point{X: 2 / texSize.X, Y: 2 / texSize.Y}
	orig := f32.Point{X: -1 - float32(bounds.Min.X)*2/texSize.X, Y: -1 - float32(bounds.Min.Y)*2/texSize.Y}
	s.prog.uniforms.vert.transform = [4]float32{scale.X, scale.Y, orig.X, orig.Y}
	s.prog.uniforms.vert.pathOffset = [2]float32{offset.X, offset.Y}
	s.prog.prog.UploadUniforms()
	// Draw in batches that fit in uint16 indices.
	start := 0
	nquads := data.ncurves / 4
	for start < nquads {
		batch := nquads - start
		if max := pathBatchSize; batch > max {
			batch = max
		}
		off := vertStride * start * 4
		s.ctx.BindVertexBuffer(data.data, vertStride, off)
		s.ctx.DrawElements(driver.DrawModeTriangles, 0, batch*6)
		start += batch
	}
}

func (p *pather) cover(z float32, mat materialType, col f32color.RGBA, col1, col2 f32color.RGBA, scale, off f32.Point, uvTrans f32.Affine2D, coverScale, coverOff f32.Point) {
	p.coverer.cover(z, mat, col, col1, col2, scale, off, uvTrans, coverScale, coverOff)
}

func (c *coverer) cover(z float32, mat materialType, col f32color.RGBA, col1, col2 f32color.RGBA, scale, off f32.Point, uvTrans f32.Affine2D, coverScale, coverOff f32.Point) {
	p := c.prog[mat]
	c.ctx.BindProgram(p.prog)
	var uniforms *coverUniforms
	switch mat {
	case materialColor:
		c.colUniforms.frag.color = col
		uniforms = &c.colUniforms.vert.coverUniforms
	case materialLinearGradient:
		c.linearGradientUniforms.frag.color1 = col1
		c.linearGradientUniforms.frag.color2 = col2

		t1, t2, t3, t4, t5, t6 := uvTrans.Elems()
		c.linearGradientUniforms.vert.uvTransformR1 = [4]float32{t1, t2, t3, 0}
		c.linearGradientUniforms.vert.uvTransformR2 = [4]float32{t4, t5, t6, 0}
		uniforms = &c.linearGradientUniforms.vert.coverUniforms
	case materialTexture:
		t1, t2, t3, t4, t5, t6 := uvTrans.Elems()
		c.texUniforms.vert.uvTransformR1 = [4]float32{t1, t2, t3, 0}
		c.texUniforms.vert.uvTransformR2 = [4]float32{t4, t5, t6, 0}
		uniforms = &c.texUniforms.vert.coverUniforms
	}
	uniforms.z = z
	uniforms.transform = [4]float32{scale.X, scale.Y, off.X, off.Y}
	uniforms.uvCoverTransform = [4]float32{coverScale.X, coverScale.Y, coverOff.X, coverOff.Y}
	p.UploadUniforms()
	c.ctx.DrawArrays(driver.DrawModeTriangleStrip, 0, 4)
}

func init() {
	// Check that struct vertex has the expected size and
	// that it contains no padding.
	if unsafe.Sizeof(*(*vertex)(nil)) != vertStride {
		panic("unexpected struct size")
	}
}
