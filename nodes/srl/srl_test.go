package srl

import (
	"fmt"
	"strings"
	"testing"

	containerlablinks "github.com/srl-labs/containerlab/links"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
)

func TestSRLInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints        []*containerlablinks.EndpointVeth
		node             *srl
		checkErrContains string
		resultEps        []string
	}{
		"alias-parse": {
			endpoints: []*containerlablinks.EndpointVeth{
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "ethernet-1/1",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "ethernet-3/2",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "ethernet-2/3/4",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "ethernet-1/5",
					},
				},
			},
			node: &srl{
				DefaultNode: containerlabnodes.DefaultNode{
					Cfg: &containerlabtypes.NodeConfig{
						ShortName: "srl",
					},
					InterfaceRegexp: InterfaceRegexp,
				},
			},
			checkErrContains: "",
			resultEps: []string{
				"e1-1", "e3-2", "e2-3-4", "e1-5",
			},
		},
		"original-parse": {
			endpoints: []*containerlablinks.EndpointVeth{
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "e1-2",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "e1-2-4",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "e3-6",
					},
				},
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "mgmt0",
					},
				},
			},
			node: &srl{
				DefaultNode: containerlabnodes.DefaultNode{
					Cfg: &containerlabtypes.NodeConfig{
						ShortName:   "srl",
						NetworkMode: "none",
					},
					InterfaceRegexp: InterfaceRegexp,
				},
			},
			checkErrContains: "",
			resultEps: []string{
				"e1-2", "e1-2-4", "e3-6", "mgmt0",
			},
		},
		"parse-fail": {
			endpoints: []*containerlablinks.EndpointVeth{
				{
					EndpointGeneric: containerlablinks.EndpointGeneric{
						IfaceName: "eth0",
					},
				},
			},
			node: &srl{
				DefaultNode: containerlabnodes.DefaultNode{
					Cfg: &containerlabtypes.NodeConfig{
						ShortName: "srl",
					},
					InterfaceRegexp: InterfaceRegexp,
					InterfaceHelp:   InterfaceHelp,
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
					if tc.checkErrContains != "" && !(strings.Contains(
						fmt.Sprint(gotCheckErr), tc.checkErrContains)) {
						t.Errorf("got error for check %+v, want %+v", gotCheckErr, tc.checkErrContains)
					}
				}

				if tc.checkErrContains != "" && !foundError {
					t.Errorf("got no error for check interface, want %s", tc.checkErrContains)
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
