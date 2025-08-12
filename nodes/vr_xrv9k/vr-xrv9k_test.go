package vr_xrv9k

import (
	"testing"

	containerlablinks "github.com/srl-labs/containerlab/links"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
)

func TestXRV9KInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*containerlablinks.EndpointVeth
		node      *vrXRV9K
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*containerlablinks.EndpointVeth{
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet0/0/0/0",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "Gi0/0/0/1",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet 0/0/0/2",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "TenGigabitEthernet0/0/0/3",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "TenGigE0/0/0/4",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "Te0/0/0/5",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "TenGigabitEthernet 0/0/0/6",
					},
				},
			},
			node: &vrXRV9K{
				VRNode: containerlabnodes.VRNode{
					DefaultNode: containerlabnodes.DefaultNode{
						Cfg: &containerlabtypes.NodeConfig{
							ShortName: "xrv9k",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth2", "eth3", "eth4", "eth5", "eth6", "eth7",
			},
		},
		"original-parse": {
			endpoints: []*containerlablinks.EndpointVeth{
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "eth6",
					},
				},
			},
			node: &vrXRV9K{
				VRNode: containerlabnodes.VRNode{
					DefaultNode: containerlabnodes.DefaultNode{
						Cfg: &containerlabtypes.NodeConfig{
							ShortName: "xrv9k",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth2", "eth4", "eth6",
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
