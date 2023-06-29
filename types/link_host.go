package types

import "fmt"

type RawHostLink struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (r *RawHostLink) ToLinkConfig() *LinkConfig {
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

// func hostFromLinkConfig(lc *LinkConfig, specialEPIndex int) (*RawHostLink, error) {
// 	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

// 	result := &RawHostLink{
// 		RawLinkType: RawLinkType{
// 			Type:     string(LinkTypeHost),
// 			Labels:   lc.Labels,
// 			Vars:     lc.Vars,
// 			Instance: nil,
// 		},
// 		HostInterface: hostIf,
// 		Node:          node,
// 		NodeInterface: nodeIf,
// 	}
// 	return result, nil
// }
