package cisco_viosl2

import (
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestVIOSL2InterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*clablinks.EndpointVeth
		node      *vrViosL2
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "Gi0",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet1",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet 2",
					},
				},
			},
			node: &vrViosL2{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "viosl2",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth2", "eth3",
			},
		},
		"original-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth3",
					},
				},
			},
			node: &vrViosL2{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "viosl2",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth2", "eth3",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			tc.node.FirstDataIfIndex = 1
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					t.Errorf("got error for endpoint %+v", gotEndpointErr)
				}
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					t.Errorf("got error for check %+v", gotCheckErr)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							t.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}
