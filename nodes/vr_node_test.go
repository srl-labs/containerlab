package nodes

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/types"
)

func TestParseInterfaces(t *testing.T) {
	tests := map[string]struct {
		endpoints           []*links.EndpointVeth
		node                *VRNode
		endpointErrContains string
		checkErrContains    string
		resultEps           []string
	}{
		"basic-parse": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/0",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/2",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/4",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "juniper",
					},
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth3", "eth5",
			},
		},
		"basic-parse-2": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "GigabitEthernet2",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi3",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "GigabitEthernet 5",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "cisco",
					},
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth2", "eth4",
			},
		},
		"original-parse": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
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
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "cisco-original",
					},
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth2", "eth4",
			},
		},
		"overlap": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/0",
					},
				},
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "juniper-overlap",
					},
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
			},
			endpointErrContains: "",
			checkErrContains:    "overlapping interface names",
			resultEps:           []string{},
		},
		"out-of-bounds-index": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi1",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "cisco-oob",
					},
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "0 ! >= 1",
			checkErrContains:    "",
			resultEps:           []string{},
		},
		"regexp-no-match": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
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
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "does not match interface alias regexp",
			checkErrContains:    "",
			resultEps:           []string{},
		},
		"regexp-no-group": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi2",
					},
				},
			},
			node: &VRNode{
				DefaultNode: DefaultNode{
					Cfg: &types.NodeConfig{
						ShortName: "cisco-noregexpgroup",
					},
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "'port' capture group missing",
			checkErrContains:    "",
			resultEps:           []string{},
		},
		"nomatch": {
			endpoints: []*links.EndpointVeth{
				&links.EndpointVeth{
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
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
				InterfaceHelp:   "helpful string :)",
			},
			endpointErrContains: "",
			checkErrContains:    "doesn't match the required interface patterns: helpful string :)",
			resultEps:           []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			foundError := false
			tc.node.OverwriteVRNode = tc.node
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					if tc.endpointErrContains != "" && !(strings.Contains(fmt.Sprint(gotEndpointErr), tc.endpointErrContains)) {
						t.Errorf("got error for endpoint %+v, want %+v", gotEndpointErr, tc.endpointErrContains)
					}
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
