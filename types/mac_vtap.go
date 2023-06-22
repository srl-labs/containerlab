package types

import "context"

type RawMacVTapLink struct {
	rawMacVXType `yaml:",inline"`
}

func (r *RawMacVTapLink) Resolve(res NodeResolver) (Link, error) {
	mvxt, err := r.rawMacVXType.UnRaw(res)
	if err != nil {
		return nil, err
	}
	return &MacVTapLink{
		macVXType: *mvxt,
	}, nil
}

func macVTapFromLinkConfig(lc LinkConfig, specialEPIndex int) (*RawMacVTapLink, error) {
	macvx, err := macVXTypeFromLinkConfig(lc, specialEPIndex)
	if err != nil {
		return nil, err
	}

	return &RawMacVTapLink{*macvx}, nil
}

type MacVTapLink struct {
	macVXType
}

func (l *MacVTapLink) GetType() (LinkType, error) {
	return LinkTypeMacVTap, nil
}

func (m *MacVTapLink) Deploy(ctx context.Context) error {
	return m.macVXType.Deploy(LinkTypeMacVTap)
}

func (m *MacVTapLink) Remove(_ context.Context) error {
	return m.macVXType.Remove(LinkTypeMacVLan)
}
