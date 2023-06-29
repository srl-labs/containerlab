package types

import "fmt"

type RawVEthLink struct {
	LinkCommonParams `yaml:",inline"`
	Endpoints        []*EndpointRaw `yaml:"endpoints"`
}

func (r *RawVEthLink) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: []string{},
	}
	for _, e := range r.Endpoints {
		lc.Endpoints = append(lc.Endpoints, fmt.Sprintf("%s:%s", e.Node, e.Iface))
	}
	return lc
}

// func vEthFromLinkConfig(lc *LinkConfig) (*RawVEthLink, error) {
// 	nodeA, nodeAIf, nodeB, nodeBIf := extractHostNodeInterfaceData(lc, 0)

// 	result := &RawVEthLink{
// 		RawLinkType: RawLinkType{
// 			Type:     string(LinkTypeVEth),
// 			Labels:   lc.Labels,
// 			Vars:     lc.Vars,
// 			Instance: nil,
// 		},
// 		Mtu: lc.MTU,
// 		Endpoints: []*EndpointRaw{
// 			{
// 				Node:  nodeA,
// 				Iface: nodeAIf,
// 			},
// 			{
// 				Node:  nodeB,
// 				Iface: nodeBIf,
// 			},
// 		},
// 	}
// 	return result, nil
// }
