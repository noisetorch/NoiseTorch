package pulseaudio

import "io"

// Server contains information about the pulseaudio server
type Server struct {
	PackageName    string
	PackageVersion string
	User           string
	Hostname       string
	SampleSpec     sampleSpec
	DefaultSink    string
	DefaultSource  string
	Cookie         uint32
	ChannelMap     channelMap
}

// ReadFrom deserializes a pulseaudio server info packet
func (s *Server) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r,
		stringTag, &s.PackageName,
		stringTag, &s.PackageVersion,
		stringTag, &s.User,
		stringTag, &s.Hostname,
		&s.SampleSpec,
		stringTag, &s.DefaultSink,
		stringTag, &s.DefaultSource,
		uint32Tag, &s.Cookie,
		&s.ChannelMap)
}

// ServerInfo queries the pulseaudio server for its information
func (c *Client) ServerInfo() (*Server, error) {
	r, err := c.request(commandGetServerInfo)
	if err != nil {
		return nil, err
	}
	var s Server
	err = bread(r, &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
