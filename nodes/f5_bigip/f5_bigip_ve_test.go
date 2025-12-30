package f5_bigip

import (
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestF5BigIPVEInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints   []*clablinks.EndpointVeth
		node        *f5BigIPVE
		resultEps   []string
		errContains string
		wantAddErr  bool
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "1.1"}},
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "1.3"}},
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "1.5"}},
			},
			node: &f5BigIPVE{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg:             &clabtypes.NodeConfig{ShortName: "bigip"},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
						InterfaceHelp:   InterfaceHelp,
					},
				},
			},
			resultEps: []string{"eth1", "eth3", "eth5"},
		},
		"eth-passthrough": {
			endpoints: []*clablinks.EndpointVeth{
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "eth2"}},
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "eth4"}},
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "eth6"}},
			},
			node: &f5BigIPVE{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg:             &clabtypes.NodeConfig{ShortName: "bigip"},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
						InterfaceHelp:   InterfaceHelp,
					},
				},
			},
			resultEps: []string{"eth2", "eth4", "eth6"},
		},
		"invalid-alias-includes-help": {
			endpoints: []*clablinks.EndpointVeth{
				{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "1.a"}},
			},
			node: &f5BigIPVE{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg:             &clabtypes.NodeConfig{ShortName: "bigip"},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
						InterfaceHelp:   InterfaceHelp,
					},
				},
			},
			wantAddErr:  true,
			errContains: InterfaceHelp,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			tc.node.FirstDataIfIndex = 1

			for _, ep := range tc.endpoints {
				err := tc.node.AddEndpoint(ep)
				if tc.wantAddErr {
					if err == nil {
						t.Fatalf("got no error for endpoint %+v, want error", ep)
					}
					if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
						t.Fatalf("got error %q, want it to contain %q", err, tc.errContains)
					}
					return
				}
				if err != nil {
					t.Fatalf("got error for endpoint %+v: %v", ep, err)
				}
			}

			if err := tc.node.CheckInterfaceName(); err != nil {
				t.Fatalf("got error for check interface: %v", err)
			}

			for idx, ep := range tc.node.Endpoints {
				if ep.GetIfaceName() != tc.resultEps[idx] {
					t.Fatalf(
						"got wrong mapped endpoint %q (%q), want %q",
						ep.GetIfaceName(),
						ep.GetIfaceAlias(),
						tc.resultEps[idx],
					)
				}
			}
		})
	}
}
