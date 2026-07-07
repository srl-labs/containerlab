package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
)

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
