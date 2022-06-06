package pulseaudio

import "io"

// Module contains information about a pulseaudio module
type Module struct {
	Index    uint32
	Name     string
	Argument string
	NUsed    uint32
	PropList map[string]string
}

// ReadFrom deserializes a PA module packet
func (s *Module) ReadFrom(r io.Reader) (int64, error) {
	err := bread(r,
		uint32Tag, &s.Index,
		stringTag, &s.Name,
		stringTag, &s.Argument,
		uint32Tag, &s.NUsed,
		&s.PropList)
	if err != nil {
		return 0, err
	}

	return 0, nil
}

// ModuleList queries pulseaudio for a list of loaded modules and returns an array
func (c *Client) ModuleList() ([]Module, error) {
	b, err := c.request(commandGetModuleInfoList)
	if err != nil {
		return nil, err
	}
	var modules []Module
	for b.Len() > 0 {
		var module Module
		err = bread(b, &module)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	return modules, nil
}

// UnloadModule requests pulseaudio to unload the module with the specified index.
// The index can be found e.g. with ModuleList()
func (c *Client) UnloadModule(index uint32) error {
	_, err := c.request(commandUnloadModule,
		uint32Tag, index)
	return err
}

// LoadModule requests pulseaudio to load the module with the specified name and argument string.
// More information on how to supply these can be found in the pulseaudio documentation:
// https://www.freedesktop.org/wiki/Software/PulseAudio/Documentation/User/Modules/#loadablemodules
// e.g. LoadModule("module-alsa-sink", "sink_name=headphones sink_properties=device.description=Headphones")
// would be equivalent to the pulse config directive: load-module module-alsa-sink sink_name=headphones sink_properties=device.description=Headphones
// Returns the index of the loaded module or an error
func (c *Client) LoadModule(name string, argument string) (index uint32, err error) {
	var idx uint32
	r, err := c.request(commandLoadModule,
		stringTag, []byte(name), byte(0), stringTag, []byte(argument), byte(0))

	if err != nil {
		return 0, err
	}

	err = bread(r, uint32Tag, &idx)
	return idx, err
}
