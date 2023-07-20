package types

import (
	"context"
	"fmt"
)

// LinkHostRaw is the raw (string) representation of a host link as defined in the topology file.
type LinkHostRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

// ToLinkConfig converts the raw link into a LinkConfig.
func (r *LinkHostRaw) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: make([]string, 2),
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "host", r.HostInterface)

	return lc
}

func hostFromLinkConfig(lc LinkConfig, specialEPIndex int) (RawLink, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &LinkHostRaw{
		LinkCommonParams: LinkCommonParams{
			Mtu:    lc.MTU,
			Labels: lc.Labels,
			Vars:   lc.Vars,
		},
		HostInterface: hostIf,
		Endpoint:      NewEndpointRaw(node, nodeIf, ""),
	}
	return result, nil
}

func (r *LinkHostRaw) Resolve() (LinkInterf, error) {
	// TODO: needs implementation
	return nil, nil
}

type LinkHost struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Endpoint         *Endpt `yaml:"endpoint"`
}

func (l *LinkHost) Deploy(ctx context.Context) error {
	// TODO: implementation required
	return nil
}

func (l *LinkHost) GetType() LinkType {
	return LinkTypeHost
}
