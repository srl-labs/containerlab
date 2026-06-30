package ciena_saos10

import (
	"context"
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestSaosInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*clablinks.EndpointVeth
		node      *cienaSaos10
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "1",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "3",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "5",
					},
				},
			},
			node: &cienaSaos10{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "saos",
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
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth6",
					},
				},
			},
			node: &cienaSaos10{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "saos",
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
					tt.Errorf("got error for endpoint %+v", gotEndpointErr)
				}
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					tt.Errorf("got error for check %+v", gotCheckErr)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							tt.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}

// TestSaosVariantSet ensures the lookup set is derived from the full list and
// includes the hyphenated platform variants from the SAOS image variant_map.
func TestSaosVariantSet(t *testing.T) {
	if len(supportedVariants) != len(supportedVariantList) {
		t.Fatalf("variant set/list size mismatch: set=%d list=%d (duplicate in list?)",
			len(supportedVariants), len(supportedVariantList))
	}
	for _, v := range []string{"3949", "5131-910", "5164-902", "5166-903", "5171-920"} {
		if _, ok := supportedVariants[v]; !ok {
			t.Errorf("expected variant %q to be supported", v)
		}
	}
}

// TestSaosVariantValidation ensures Init rejects a missing or unsupported type.
// Both checks return before any management-network access, so a minimal
// NodeConfig is sufficient. The happy path is not exercised here because it
// proceeds to read the management network, which is populated via NodeOptions
// during a real deploy.
func TestSaosVariantValidation(t *testing.T) {
	tests := map[string]struct {
		nodeType string
	}{
		"missing-type":     {nodeType: ""},
		"unsupported-type": {nodeType: "9999"},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			n := new(cienaSaos10)
			err := n.Init(&clabtypes.NodeConfig{
				ShortName: "saos",
				NodeType:  tc.nodeType,
			})

			if err == nil {
				tt.Fatalf("expected error for type %q, got nil", tc.nodeType)
			}
			if !strings.Contains(err.Error(), "type") {
				tt.Errorf("expected a type-validation error, got %q", err.Error())
			}
		})
	}
}

// TestSaosPartialConfigOnly ensures a non-.partial startup-config is rejected by
// PreDeploy before any deployment work happens.
func TestSaosPartialConfigOnly(t *testing.T) {
	tests := map[string]struct {
		startupConfig string
		wantError     bool
	}{
		"non-partial-rejected": {startupConfig: "configuration.xml", wantError: true},
		"partial-accepted":     {startupConfig: "configuration.xml.partial", wantError: false},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			n := &cienaSaos10{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName:     "saos",
							StartupConfig: tc.startupConfig,
						},
					},
				},
			}

			err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{})

			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "partial") {
					tt.Fatalf("expected partial-config error, got %v", err)
				}
				return
			}

			// A partial config passes our gate; it sets the env var and then
			// delegates to VRNode.PreDeploy. We only assert our gate did not
			// reject it.
			if err != nil && strings.Contains(err.Error(), "only supports partial") {
				tt.Errorf("partial config %q was rejected: %q", tc.startupConfig, err.Error())
			}
			if got := n.Cfg.Env["SAOS_STARTUP_CONFIG_PATH"]; got != "/config/configuration.xml.partial" {
				tt.Errorf("SAOS_STARTUP_CONFIG_PATH = %q, want /config/configuration.xml.partial", got)
			}
		})
	}
}
