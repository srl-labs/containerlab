package srl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

func TestSRLInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints        []*links.EndpointVeth
		node             *srl
		checkErrContains string
		resultEps        []string
	}{
		"alias-parse": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ethernet1-1",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ethernet1-1-3",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ethernet1-5",
					},
				},
			},
			node: &srl{
				DefaultNode: nodes.DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "srl",
					},
					InterfaceRegexp:       InterfaceRegexp,
					InterfaceMappedPrefix: InterfaceMappedPrefix,
					InterfaceOffset:       InterfaceOffset,
				},
			},
			checkErrContains: "",
			resultEps: []string{
				"e1-1", "e1-3", "e1-5", "mgmt0",
			},
		},
		"original-parse": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "e1-2",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "e1-1-4",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "e1-6",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "mgmt0",
					},
				},
			},
			node: &srl{
				DefaultNode: nodes.DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName:   "srl",
						NetworkMode: "none",
					},
					InterfaceRegexp:       InterfaceRegexp,
					InterfaceMappedPrefix: InterfaceMappedPrefix,
					InterfaceOffset:       InterfaceOffset,
				},
			},
			checkErrContains: "",
			resultEps: []string{
				"e1-2", "e1-1-4", "e1-6", "mgmt0",
			},
		},
		"parse-fail": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth0",
					},
				},
			},
			node: &srl{
				DefaultNode: nodes.DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "srl",
					},
					InterfaceRegexp:       InterfaceRegexp,
					InterfaceMappedPrefix: InterfaceMappedPrefix,
					InterfaceOffset:       InterfaceOffset,
					InterfaceHelp:         InterfaceHelp,
				},
			},
			checkErrContains: InterfaceHelp,
			resultEps:        []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
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
					if tc.checkErrContains != "" && !(strings.Contains(fmt.Sprint(gotCheckErr), tc.checkErrContains)) {
						t.Errorf("got error for check %+v, want %+v", gotCheckErr, tc.checkErrContains)
					}
				}

				if tc.checkErrContains != "" && !foundError {
					t.Errorf("got no error for check interface, want %s", tc.checkErrContains)
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
