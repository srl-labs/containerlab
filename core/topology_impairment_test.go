package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestExpandLinkImpairmentsAutoCreatesBridgeForSrsimRegularLink(t *testing.T) {
	c := newTestCLabWithRegularVethLink("nokia_srsim", "linux")

	if err := c.expandLinkImpairments(); err != nil {
		t.Fatalf("expandLinkImpairments() error = %v", err)
	}

	if len(c.Config.Topology.Links) != 2 {
		t.Fatalf("links = %d, want 2", len(c.Config.Topology.Links))
	}
	if _, exists := c.Config.Topology.Nodes["impairment-bridge-01"]; !exists {
		t.Fatalf("generated impairment bridge node missing for SR-SIM regular link")
	}
}

func TestExpandLinkImpairmentsKeepsRegularLinkDirectWithoutBridgeRequirement(t *testing.T) {
	c := newTestCLabWithRegularVethLink("linux", "linux")

	if err := c.expandLinkImpairments(); err != nil {
		t.Fatalf("expandLinkImpairments() error = %v", err)
	}

	if len(c.Config.Topology.Links) != 1 {
		t.Fatalf("links = %d, want 1", len(c.Config.Topology.Links))
	}
	if _, exists := c.Config.Topology.Nodes["impairment-bridge-01"]; exists {
		t.Fatalf("generated impairment bridge node exists for direct linux-to-linux link")
	}
}

func TestNewImpairmentBridgeNodeDefinition(t *testing.T) {
	got := newImpairmentBridgeNodeDefinition(&clablinks.ImpairmentBridgeNode{
		Name:          "link-0102--01",
		Image:         clablinks.DefaultImpairmentBridgeImage,
		OriginalNodes: []string{"pe01", "pe02"},
	})

	want := &clabtypes.NodeDefinition{
		Kind:   "linux",
		Image:  clablinks.DefaultImpairmentBridgeImage,
		CapAdd: []string{"NET_ADMIN"},
		Exec:   []string{impairmentBridgeExec},
		Labels: map[string]string{
			clabconstants.GeneratedNode:           "true",
			clabconstants.GeneratedNodeRole:       "impairment-bridge",
			clabconstants.ImpairmentOriginalNodes: "pe01,pe02",
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("generated bridge node definition mismatch (-want +got):\n%s", diff)
	}
}

func newTestCLabWithRegularVethLink(nodeAKind, nodeBKind string) *CLab {
	registry := clabnodes.NewNodeRegistry()
	_ = registry.Register(
		[]string{"linux"},
		nil,
		clabnodes.NewNodeRegistryEntryAttributes(nil, nil, nil),
	)
	_ = registry.Register(
		[]string{"nokia_srsim"},
		nil,
		clabnodes.NewNodeRegistryEntryAttributes(nil, nil, nil).WithLinkImpairmentBridgeRequired(),
	)

	return &CLab{
		Reg: registry,
		Config: &Config{
			Topology: &clabtypes.Topology{
				Nodes: map[string]*clabtypes.NodeDefinition{
					"pe01": {Kind: nodeAKind},
					"pe02": {Kind: nodeBKind},
				},
				Links: []*clablinks.LinkDefinition{
					{
						Type: string(clablinks.LinkTypeBrief),
						Link: &clablinks.LinkVEthRaw{
							Endpoints: []*clablinks.EndpointRaw{
								{Node: "pe01", Iface: "eth1"},
								{Node: "pe02", Iface: "eth1"},
							},
						},
					},
				},
			},
		},
	}
}

func TestExpandNodeFilterForGeneratedNodes(t *testing.T) {
	c := &CLab{
		Config: &Config{
			Topology: &clabtypes.Topology{
				Nodes: map[string]*clabtypes.NodeDefinition{
					"pe01": {Kind: "nokia_srsim"},
					"pe02": {Kind: "nokia_srsim"},
					"link-0102--01": {
						Kind: "linux",
						Labels: map[string]string{
							clabconstants.GeneratedNodeRole:       "impairment-bridge",
							clabconstants.ImpairmentOriginalNodes: "pe01,pe02",
						},
					},
				},
			},
		},
	}

	got := c.expandNodeFilterForGeneratedNodes([]string{"pe01", "pe02"})
	want := []string{"pe01", "pe02", "link-0102--01"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("expanded node filter mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandNodeFilterForGeneratedNodesRequiresBothOriginalEndpoints(t *testing.T) {
	c := &CLab{
		Config: &Config{
			Topology: &clabtypes.Topology{
				Nodes: map[string]*clabtypes.NodeDefinition{
					"pe01": {Kind: "nokia_srsim"},
					"pe02": {Kind: "nokia_srsim"},
					"link-0102--01": {
						Kind: "linux",
						Labels: map[string]string{
							clabconstants.GeneratedNodeRole:       "impairment-bridge",
							clabconstants.ImpairmentOriginalNodes: "pe01,pe02",
						},
					},
				},
			},
		},
	}

	got := c.expandNodeFilterForGeneratedNodes([]string{"pe01"})
	want := []string{"pe01"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("expanded node filter mismatch (-want +got):\n%s", diff)
	}
}

func TestFilterClabNodesKeepsGeneratedImpairmentBridge(t *testing.T) {
	c := &CLab{
		Config: &Config{
			Topology: &clabtypes.Topology{
				Nodes: map[string]*clabtypes.NodeDefinition{
					"pe01": {Kind: "nokia_srsim"},
					"pe02": {Kind: "nokia_srsim"},
					"pe03": {Kind: "nokia_srsim"},
					"link-0102--01": {
						Kind: "linux",
						Labels: map[string]string{
							clabconstants.GeneratedNodeRole:       "impairment-bridge",
							clabconstants.ImpairmentOriginalNodes: "pe01,pe02",
						},
					},
				},
			},
		},
	}

	if err := c.filterClabNodes([]string{"pe01", "pe02"}); err != nil {
		t.Fatalf("filterClabNodes() error = %v", err)
	}

	if _, ok := c.Config.Topology.Nodes["link-0102--01"]; !ok {
		t.Fatalf("generated impairment bridge node was filtered out")
	}
	if _, ok := c.Config.Topology.Nodes["pe03"]; ok {
		t.Fatalf("unselected node pe03 was not filtered out")
	}

	wantNodeFilter := []string{"pe01", "pe02", "link-0102--01"}
	if diff := cmp.Diff(wantNodeFilter, c.nodeFilter); diff != "" {
		t.Fatalf("node filter mismatch (-want +got):\n%s", diff)
	}
}
