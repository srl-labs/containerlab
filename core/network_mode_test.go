package core

import (
	"context"
	"slices"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

type networkModeTestNode struct {
	clabnodes.Node
	cfg *clabtypes.NodeConfig
}

func (n *networkModeTestNode) Config() *clabtypes.NodeConfig { return n.cfg }

func TestPlanApplyCascadesLinkRestart(t *testing.T) {
	t.Parallel()
	c, current := newNetworkModePlanTestLab(t, clabruntime.Running, clabnodes.LinkApplyModeRestart)
	plan, err := c.planApply(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedStringSet(plan.linkRestartNodeSet); !slices.Equal(got, []string{"leaf", "sidecar", "target"}) {
		t.Fatalf("restarted nodes = %v, want target and both dependents", got)
	}
	if len(plan.recreatedNodeSet) != 0 {
		t.Fatalf("restart must preserve container identities: %v", plan.recreatedNodeSet)
	}
}

func TestPlanNetworkModeRestartsForNewDependents(t *testing.T) {
	t.Parallel()
	for _, late := range []bool{false, true} {
		for _, recreated := range []bool{false, true} {
			c := &CLab{Nodes: map[string]clabnodes.Node{
				"target":  &networkModeTestNode{cfg: &clabtypes.NodeConfig{}},
				"sidecar": &networkModeTestNode{cfg: &clabtypes.NodeConfig{NetworkMode: "container:target"}},
			}}
			plan := newApplyPlan(nil, nil)
			if late {
				plan.linkRestartNodeSet["target"] = struct{}{}
			} else {
				plan.restartNodeSet["target"] = struct{}{}
			}
			if recreated {
				plan.recreatedNodeSet["sidecar"] = struct{}{}
			} else {
				plan.addedNodeSet["sidecar"] = struct{}{}
			}
			if err := c.planNetworkModeRestarts(plan); err != nil {
				t.Fatal(err)
			}
			if _, restart := plan.linkRestartNodeSet["sidecar"]; restart != late {
				t.Fatalf("late=%v recreated=%v: sidecar restart=%v", late, recreated, restart)
			}
		}
	}
}

func TestRestartApplyNodesOrdersNamespaceDependencies(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	c := &CLab{Nodes: map[string]clabnodes.Node{}}
	nodeSet := map[string]struct{}{}
	var calls []any
	for _, name := range []string{"target", "sidecar", "leaf"} {
		node := clabmocksmocknodes.NewMockNode(ctrl)
		cfg := &clabtypes.NodeConfig{ShortName: name}
		switch name {
		case "sidecar":
			cfg.NetworkMode = "container:target"
		case "leaf":
			cfg.NetworkMode = "container:sidecar"
		}
		node.EXPECT().Config().Return(cfg).AnyTimes()
		calls = append(calls, node.EXPECT().Stop(gomock.Any()).Return(nil), node.EXPECT().Start(gomock.Any()).Return(nil))
		node.EXPECT().GetContainerStatus(gomock.Any()).Return(clabruntime.Running)
		c.Nodes[name] = node
		nodeSet[name] = struct{}{}
	}
	gomock.InOrder(calls...)
	if err := c.restartApplyNodes(context.Background(), nodeSet); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkModeNodeOrderRejectsCycles(t *testing.T) {
	t.Parallel()
	c := &CLab{Nodes: map[string]clabnodes.Node{
		"a": &networkModeTestNode{cfg: &clabtypes.NodeConfig{NetworkMode: "container:b"}},
		"b": &networkModeTestNode{cfg: &clabtypes.NodeConfig{NetworkMode: "container:a"}},
	}}
	if _, err := c.networkModeNodeOrder([]string{"a", "b"}); err == nil {
		t.Fatal("expected cyclic dependency error")
	}
}

func TestPlanApplyCascadesLinkRecreate(t *testing.T) {
	t.Parallel()

	for _, status := range []clabruntime.ContainerStatus{clabruntime.Running, clabruntime.Stopped} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			c, current := newNetworkModePlanTestLab(t, status, clabnodes.LinkApplyModeRecreate)
			plan, err := c.planApply(context.Background(), current)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"target", "sidecar", "leaf"} {
				if _, ok := plan.recreatedNodeSet[name]; !ok {
					t.Errorf("node %q must be recreated after the target's link change", name)
				}
				if _, ok := plan.startNodeSet[name]; ok {
					t.Errorf("recreated node %q must not also be started", name)
				}
				wantParked := name == "target" || status == clabruntime.Running
				if _, parked := plan.parkedNodeSet[name]; parked != wantParked {
					t.Errorf("node %q parked = %v, want %v", name, parked, wantParked)
				}
			}
		})
	}
}

// newNetworkModePlanTestLab models a new link on a running target and a chain
// of existing namespace-sharing dependents. Interface discovery is mocked so
// the complete planner can run without creating real network namespaces.
func newNetworkModePlanTestLab(
	t *testing.T,
	dependentStatus clabruntime.ContainerStatus,
	mode clabnodes.LinkApplyMode,
) (*CLab, map[string]*runtimeNodeGroup) {
	t.Helper()
	ctrl := gomock.NewController(t)
	paths := &clabtypes.TopoPaths{}
	if err := paths.SetLabDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	c := &CLab{
		Config:    &Config{Topology: clabtypes.NewTopology()},
		TopoPaths: paths,
		Nodes:     map[string]clabnodes.Node{},
		Links:     map[int]clablinks.Link{},
	}
	current := map[string]*runtimeNodeGroup{}
	for name, target := range map[string]string{"target": "", "sidecar": "target", "leaf": "sidecar"} {
		cfg := &clabtypes.NodeConfig{ShortName: name, LongName: "clab-netmode-test-" + name}
		status := clabruntime.Running
		if target != "" {
			cfg.NetworkMode = "container:" + target
			status = dependentStatus
		}
		node := clabmocksmocknodes.NewMockNode(ctrl)
		node.EXPECT().Config().Return(cfg).AnyTimes()
		node.EXPECT().GetShortName().Return(name).AnyTimes()
		node.EXPECT().GetContainerStatus(gomock.Any()).Return(status).AnyTimes()
		node.EXPECT().ExecFunction(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		node.EXPECT().ComputeDiff(gomock.Any(), gomock.Any()).Return(&clabtypes.TopologyDiff{})
		node.EXPECT().GetReconcilePlan(gomock.Any(), gomock.Any()).Return(&clabnodes.ReconcileResult{}, nil)
		if name == "target" {
			node.EXPECT().LinkApplyMode(gomock.Any()).Return(mode)
		}
		c.Nodes[name] = node
		c.Config.Topology.Nodes[name] = &clabtypes.NodeDefinition{Kind: "linux"}
		current[name] = &runtimeNodeGroup{}
	}
	link := &applyFakeLink{linkType: clablinks.LinkTypeDummy}
	link.endpoints = []clablinks.Endpoint{
		clablinks.NewEndpointDummy(clablinks.NewEndpointGeneric(c.Nodes["target"], "eth1", link)),
	}
	c.Links[0] = link
	return c, current
}
