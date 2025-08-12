package vr_n9kv

import (
	"testing"

	containerlablinks "github.com/srl-labs/containerlab/links"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
)

func TestN9kvInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*containerlablinks.EndpointVeth
		node      *vrN9kv
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*containerlablinks.EndpointVeth{
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "Ethernet1/1",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "Et1/3",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "Ethernet 1/5",
					},
				},
			},
			node: &vrN9kv{
				VRNode: containerlabnodes.VRNode{
					DefaultNode: containerlabnodes.DefaultNode{
						Cfg: &containerlabtypes.NodeConfig{
							ShortName: "n9kv",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth3", "eth5",
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
			node: &vrN9kv{
				VRNode: containerlabnodes.VRNode{
					DefaultNode: containerlabnodes.DefaultNode{
						Cfg: &containerlabtypes.NodeConfig{
							ShortName: "n9kv",
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
