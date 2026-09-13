package core

import (
	"slices"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestLifecycleStartNodesIncludesDependenciesInOrder(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	c := CLab{
		Nodes: getNodeMap(mockCtrl),
	}

	nodes, deps, err := c.lifecycleStartNodes([]string{"node5"})
	if err != nil {
		t.Fatalf("lifecycleStartNodes() error = %v", err)
	}

	got := make([]string, 0, len(nodes))
	for _, n := range nodes {
		got = append(got, n.Config().ShortName)
	}

	want := []string{"node1", "node2", "node3", "node4", "node5"}
	if !slices.Equal(got, want) {
		t.Fatalf("lifecycleStartNodes() order = %v, want %v", got, want)
	}

	if len(deps["node5"]) != 2 {
		t.Fatalf("node5 dependencies = %d, want 2", len(deps["node5"]))
	}
}

func TestLifecycleStartNodesIncludesContainerNetworkModeDependency(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	c := CLab{
		Nodes: getNodeMap(mockCtrl),
	}

	_, deps, err := c.lifecycleStartNodes([]string{"node3"})
	if err != nil {
		t.Fatalf("lifecycleStartNodes() error = %v", err)
	}

	var found bool
	for _, dep := range deps["node3"] {
		if dep.Node == "node2" && dep.Stage == clabtypes.WaitForCreate {
			found = true
		}
	}

	if !found {
		t.Fatalf("node3 dependencies = %#v, want network-mode dependency on node2 create", deps["node3"])
	}
}

func TestLifecycleStartNodesDetectsCycles(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	nodes := getNodeMap(mockCtrl)
	nodes["node1"].Config().Stages.Create.WaitFor = clabtypes.WaitForList{
		&clabtypes.WaitFor{
			Node:  "node2",
			Stage: clabtypes.WaitForCreate,
		},
	}

	c := CLab{
		Nodes: nodes,
	}

	_, _, err := c.lifecycleStartNodes([]string{"node2"})
	if err == nil {
		t.Fatal("lifecycleStartNodes() error = nil, want cycle error")
	}
}
