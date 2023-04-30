// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
)

// packer packs a set of many smaller rectangles into
// much fewer larger atlases.
type packer struct {
	maxDim int
	spaces []image.Rectangle

	sizes []image.Point
	pos   image.Point
}

type placement struct {
	Idx int
	Pos image.Point
}

// add adds the given rectangle to the atlases and
// return the allocated position.
func (p *packer) add(s image.Point) (placement, bool) {
	if place, ok := p.tryAdd(s); ok {
		return place, true
	}
	p.newPage()
	return p.tryAdd(s)
}

func (p *packer) clear() {
	p.sizes = p.sizes[:0]
	p.spaces = p.spaces[:0]
}

func (p *packer) newPage() {
	p.pos = image.Point{}
	p.sizes = append(p.sizes, image.Point{})
	p.spaces = p.spaces[:0]
	p.spaces = append(p.spaces, image.Rectangle{
		Max: image.Point{X: p.maxDim, Y: p.maxDim},
	})
}

func (p *packer) tryAdd(s image.Point) (placement, bool) {
	// Go backwards to prioritize smaller spaces first.
	for i := len(p.spaces) - 1; i >= 0; i-- {
		space := p.spaces[i]
		rightSpace := space.Dx() - s.X
		bottomSpace := space.Dy() - s.Y
		if rightSpace >= 0 && bottomSpace >= 0 {
			// Remove space.
			p.spaces[i] = p.spaces[len(p.spaces)-1]
			p.spaces = p.spaces[:len(p.spaces)-1]
			// Put s in the top left corner and add the (at most)
			// two smaller spaces.
			pos := space.Min
			if bottomSpace > 0 {
				p.spaces = append(p.spaces, image.Rectangle{
					Min: image.Point{X: pos.X, Y: pos.Y + s.Y},
					Max: image.Point{X: space.Max.X, Y: space.Max.Y},
				})
			}
			if rightSpace > 0 {
				p.spaces = append(p.spaces, image.Rectangle{
					Min: image.Point{X: pos.X + s.X, Y: pos.Y},
					Max: image.Point{X: space.Max.X, Y: pos.Y + s.Y},
				})
			}
			idx := len(p.sizes) - 1
			size := &p.sizes[idx]
			if x := pos.X + s.X; x > size.X {
				size.X = x
			}
			if y := pos.Y + s.Y; y > size.Y {
				size.Y = y
			}
			return placement{Idx: idx, Pos: pos}, true
		}
	}
	return placement{}, false
}
