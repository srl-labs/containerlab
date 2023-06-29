package types

import "fmt"

type RawMgmtNetLink struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (r *RawMgmtNetLink) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: make([]string, 2),
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "mgmt-net", r.HostInterface)

	return lc
}

// func mgmtNetFromLinkConfig(lc *LinkConfig, specialEPIndex int) (*RawMgmtNetLink, error) {
// 	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

// 	result := &RawMgmtNetLink{
// 		RawLinkType:   RawLinkType{Type: string(LinkTypeMgmtNet), Labels: lc.Labels, Vars: lc.Vars, Instance: nil},
// 		HostInterface: hostIf,
// 		Endpoint: &EndpointRaw{
// 			Node:  node,
// 			Iface: nodeIf,
// 			Mac:   "",
// 		},
// 	}
// 	return result, nil
// }
