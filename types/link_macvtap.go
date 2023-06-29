package types

import "fmt"

type RawMacVTapLink struct {
	rawMacVXType `yaml:",inline"`
}

func (r *RawMacVTapLink) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: make([]string, 2),
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Node, r.NodeInterface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "macvtap", r.HostInterface)

	return lc
}
