// +build !darwin,!nucular_gio nucular_shiny

package font

import (
	"crypto/md5"
	"sync"

	"golang.org/x/image/font"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

type Face struct {
	face font.Face
}

var fontsMu sync.Mutex
var fontsMap = map[[md5.Size]byte]*truetype.Font{}

// NewFace returns a new face by parsing the ttf font.
func NewFace(ttf []byte, size int) (Face, error) {
	key := md5.Sum(ttf)
	fontsMu.Lock()
	defer fontsMu.Unlock()

	fnt, _ := fontsMap[key]
	if fnt == nil {
		var err error
		fnt, err = freetype.ParseFont(ttf)
		if err != nil {
			return Face{}, err
		}
	}

	return Face{truetype.NewFace(fnt, &truetype.Options{Size: float64(size), Hinting: font.HintingFull, DPI: 72})}, nil
}

func (face Face) Metrics() font.Metrics {
	return face.face.Metrics()
}
