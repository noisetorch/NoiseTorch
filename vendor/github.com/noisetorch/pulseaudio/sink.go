package pulseaudio

import "io"

// Sink contains information about a sink in pulseaudio
type Sink struct {
	Index              uint32
	Name               string
	Description        string
	SampleSpec         sampleSpec
	ChannelMap         channelMap
	ModuleIndex        uint32
	Cvolume            cvolume
	Muted              bool
	MonitorSourceIndex uint32
	MonitorSourceName  string
	Latency            uint64
	Driver             string
	Flags              uint32
	PropList           map[string]string
	RequestedLatency   uint64
	BaseVolume         uint32
	SinkState          uint32
	NVolumeSteps       uint32
	CardIndex          uint32
	Ports              []sinkPort
	ActivePortName     string
	Formats            []formatInfo
}

// ReadFrom deserializes a sink packet from pulseaudio
func (s *Sink) ReadFrom(r io.Reader) (int64, error) {
	var portCount uint32
	err := bread(r,
		uint32Tag, &s.Index,
		stringTag, &s.Name,
		stringTag, &s.Description,
		&s.SampleSpec,
		&s.ChannelMap,
		uint32Tag, &s.ModuleIndex,
		&s.Cvolume,
		&s.Muted,
		uint32Tag, &s.MonitorSourceIndex,
		stringTag, &s.MonitorSourceName,
		usecTag, &s.Latency,
		stringTag, &s.Driver,
		uint32Tag, &s.Flags,
		&s.PropList,
		usecTag, &s.RequestedLatency,
		volumeTag, &s.BaseVolume,
		uint32Tag, &s.SinkState,
		uint32Tag, &s.NVolumeSteps,
		uint32Tag, &s.CardIndex,
		uint32Tag, &portCount)
	if err != nil {
		return 0, err
	}
	s.Ports = make([]sinkPort, portCount)
	for i := uint32(0); i < portCount; i++ {
		err = bread(r, &s.Ports[i])
		if err != nil {
			return 0, err
		}
	}
	if portCount == 0 {
		err = bread(r, stringNullTag)
		if err != nil {
			return 0, err
		}
	} else {
		err = bread(r, stringTag, &s.ActivePortName)
		if err != nil {
			return 0, err
		}
	}

	var formatCount uint8
	err = bread(r,
		uint8Tag, &formatCount)
	if err != nil {
		return 0, err
	}
	s.Formats = make([]formatInfo, formatCount)
	for i := uint8(0); i < formatCount; i++ {
		err = bread(r, &s.Formats[i])
		if err != nil {
			return 0, err
		}
	}
	return 0, nil
}

// Sinks queries PulseAudio for a list of sinks and returns an array
func (c *Client) Sinks() ([]Sink, error) {
	b, err := c.request(commandGetSinkInfoList)
	if err != nil {
		return nil, err
	}
	var sinks []Sink
	for b.Len() > 0 {
		var sink Sink
		err = bread(b, &sink)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, sink)
	}
	return sinks, nil
}

type sinkPort struct {
	Name, Description string
	Pririty           uint32
	Available         uint32
}

func (p *sinkPort) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r,
		stringTag, &p.Name,
		stringTag, &p.Description,
		uint32Tag, &p.Pririty,
		uint32Tag, &p.Available)
}

func (c *Client) setDefaultSink(sinkName string) error {
	_, err := c.request(commandSetDefaultSink,
		stringTag, []byte(sinkName), byte(0))
	return err
}
