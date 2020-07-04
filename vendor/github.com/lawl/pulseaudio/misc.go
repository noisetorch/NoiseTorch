package pulseaudio

import (
	"io"
)

type formatInfo struct {
	Encoding byte
	PropList map[string]string
}

func (i *formatInfo) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r, formatInfoTag, uint8Tag, &i.Encoding, &i.PropList)
}

type cvolume []uint32

func (v *cvolume) ReadFrom(r io.Reader) (int64, error) {
	var n byte
	err := bread(r, cvolumeTag, &n)
	if err != nil {
		return 0, err
	}
	*v = make([]uint32, n)
	return 0, bread(r, []uint32(*v))
}

type channelMap []byte

func (m *channelMap) ReadFrom(r io.Reader) (int64, error) {
	var n byte
	err := bread(r, channelMapTag, &n)
	if err != nil {
		return 0, err
	}
	*m = make([]byte, n)
	_, err = r.Read(*m)
	return 0, err
}

type sampleSpec struct {
	Format   byte
	Channels byte
	Rate     uint32
}

func (s *sampleSpec) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r, sampleSpecTag, &s.Format, &s.Channels, &s.Rate)
}

type profile struct {
	Name, Description string
	Nsinks, Nsources  uint32
	Priority          uint32
	Available         uint32
}

type port struct {
	Card              *Card
	Name, Description string
	Pririty           uint32
	Available         uint32
	Direction         byte
	PropList          map[string]string
	Profiles          []*profile
	LatencyOffset     int64
}

func (p *port) ReadFrom(r io.Reader) (int64, error) {
	err := bread(r,
		stringTag, &p.Name,
		stringTag, &p.Description,
		uint32Tag, &p.Pririty,
		uint32Tag, &p.Available,
		uint8Tag, &p.Direction,
		&p.PropList)
	if err != nil {
		return 0, err
	}
	var portProfileCount uint32
	err = bread(r, uint32Tag, &portProfileCount)
	if err != nil {
		return 0, err
	}
	for j := uint32(0); j < portProfileCount; j++ {
		var profileName string
		err = bread(r, stringTag, &profileName)
		if err != nil {
			return 0, err
		}
		p.Profiles = append(p.Profiles, p.Card.Profiles[profileName])
	}
	return 0, bread(r, int64Tag, &p.LatencyOffset)
}
