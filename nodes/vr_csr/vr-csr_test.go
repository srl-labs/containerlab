package vr_csr

import (
	"testing"

	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

func TestCSRInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*links.EndpointVeth
		node      *vrCsr
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi2",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "GigabitEthernet4",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "GigabitEthernet 6",
					},
				},
			},
			node: &vrCsr{
				VRNode: nodes.VRNode{
					DefaultNode: nodes.DefaultNode{
						Cfg: &types.NodeConfig{
							ShortName: "csr",
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
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth6",
					},
				},
			},
			node: &vrCsr{
				VRNode: nodes.VRNode{
					DefaultNode: nodes.DefaultNode{
						Cfg: &types.NodeConfig{
							ShortName: "csr",
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
							t.Errorf("got wrong mapped endpoint %q (%q), want %q", ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}
