package pulseaudio

type Card struct {
	Index         uint32
	Name          string
	Module        uint32
	Driver        string
	Profiles      map[string]*profile
	ActiveProfile *profile
	PropList      map[string]string
	Ports         []port
}

func (c *Client) Cards() ([]Card, error) {
	b, err := c.request(commandGetCardInfoList)
	if err != nil {
		return nil, err
	}
	var cards []Card
	for b.Len() > 0 {
		var card Card
		var profileCount uint32
		err := bread(b,
			uint32Tag, &card.Index,
			stringTag, &card.Name,
			uint32Tag, &card.Module,
			stringTag, &card.Driver,
			uint32Tag, &profileCount)
		if err != nil {
			return nil, err
		}
		card.Profiles = make(map[string]*profile)
		for i := uint32(0); i < profileCount; i++ {
			var profile profile
			err = bread(b,
				stringTag, &profile.Name,
				stringTag, &profile.Description,
				uint32Tag, &profile.Nsinks,
				uint32Tag, &profile.Nsources,
				uint32Tag, &profile.Priority,
				uint32Tag, &profile.Available)
			if err != nil {
				return nil, err
			}
			card.Profiles[profile.Name] = &profile
		}
		var portCount uint32
		var activeProfileName string
		err = bread(b,
			stringTag, &activeProfileName,
			&card.PropList,
			uint32Tag, &portCount)
		if err != nil {
			return nil, err
		}
		card.ActiveProfile = card.Profiles[activeProfileName]
		card.Ports = make([]port, portCount)
		for i := uint32(0); i < portCount; i++ {
			card.Ports[i].Card = &card
			err = bread(b, &card.Ports[i])
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (c *Client) SetCardProfile(cardIndex uint32, profileName string) error {
	_, err := c.request(commandSetCardProfile,
		uint32Tag, cardIndex,
		stringNullTag,
		stringTag, []byte(profileName), byte(0))
	return err
}
