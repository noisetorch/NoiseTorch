// +build darwin,!nucular_shiny nucular_gio

package font

import (
	"crypto/md5"
	"strings"
	"sync"

	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/unit"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type Face struct {
	fnt     *opentype.Font
	shaper  *text.FontRegistry
	size    int
	fsize   fixed.Int26_6
	metrics font.Metrics
}

var fontsMu sync.Mutex
var fontsMap = map[[md5.Size]byte]*opentype.Font{}

func NewFace(ttf []byte, size int) (Face, error) {
	key := md5.Sum(ttf)
	fontsMu.Lock()
	defer fontsMu.Unlock()

	fnt, _ := fontsMap[key]
	if fnt == nil {
		var err error
		fnt, err = opentype.Parse(ttf)
		if err != nil {
			return Face{}, err
		}
	}

	shaper := &text.FontRegistry{}
	shaper.Register(text.Font{}, fnt)

	face := Face{fnt, shaper, size, fixed.I(size), font.Metrics{}}
	metricsTxt, _ := face.shaper.Layout(text.Font{}, fixed.I(size), 1e6, strings.NewReader("metrics"))
	face.metrics.Ascent = metricsTxt[0].Ascent
	face.metrics.Descent = metricsTxt[0].Descent
	face.metrics.Height = face.metrics.Ascent + face.metrics.Descent
	return face, nil
}

func (face Face) Px(v unit.Value) int {
	return face.size
}

func (face Face) Metrics() font.Metrics {
	return face.metrics
}
