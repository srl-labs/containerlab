package nodes

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/types"
)

func TestVMInterfaceAliases(t *testing.T) { //skipcq: GO-R1005
	tests := map[string]struct {
		endpoints           []*links.EndpointVeth
		node                *VRNode
		endpointErrContains string
		checkErrContains    string
		resultEps           []string
	}{
		"regexp-no-match": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/0",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "cisco-noregexpmatch",
					},
					InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
					InterfaceOffset: 2,
				},
			},
			endpointErrContains: "does not match regexp",
			checkErrContains:    "",
			resultEps:           []string{},
		},
		"nomatch": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "0/0/1",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "juniper-nomatch",
					},
					InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
					InterfaceOffset: 0,
					InterfaceHelp:   "helpful string :)",
				},
			},
			endpointErrContains: "",
			checkErrContains:    "helpful string :)",
			resultEps:           []string{},
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
					if tc.endpointErrContains != "" && !(strings.Contains(fmt.Sprint(gotEndpointErr), tc.endpointErrContains)) {
						t.Errorf("got error for endpoint %+v, want %+v", gotEndpointErr, tc.endpointErrContains)
					}
				}
			}

			if tc.endpointErrContains != "" && !foundError {
				t.Errorf("got no error for endpoints, want %s", tc.endpointErrContains)
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
