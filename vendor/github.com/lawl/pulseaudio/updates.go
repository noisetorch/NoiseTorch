package pulseaudio

// Updates returns a channel with PulseAudio updates.
func (c *Client) Updates() (updates <-chan struct{}, err error) {
	const subscriptionMaskAll = 0x02ff
	_, err = c.request(commandSubscribe, uint32Tag, uint32(subscriptionMaskAll))
	if err != nil {
		return nil, err
	}
	return c.updates, nil
}
