package types

type RawMacVLanLink struct {
	rawMacVXType `yaml:",inline"`
}

func (r *RawMacVLanLink) UnRaw(res NodeResolver) (Link, error) {
	mvxt, err := r.rawMacVXType.UnRaw(res)
	if err != nil {
		return nil, err
	}
	return &MacVLanLink{
		macVXType: *mvxt,
	}, nil
}

func macVlanFromLinkConfig(lc LinkConfig, specialEPIndex int) (*RawMacVLanLink, error) {
	macvx, err := macVXTypeFromLinkConfig(lc, specialEPIndex)
	if err != nil {
		return nil, err
	}

	return &RawMacVLanLink{*macvx}, nil
}

type MacVLanLink struct {
	macVXType
}

func (l *MacVLanLink) GetType() (LinkType, error) {
	return LinkTypeMacVLan, nil
}

func (m *MacVLanLink) Deploy() error {
	return m.macVXType.Deploy(LinkTypeMacVLan)
}

func (m *MacVLanLink) Remove() error {
	return m.macVXType.Remove(LinkTypeMacVLan)
}
