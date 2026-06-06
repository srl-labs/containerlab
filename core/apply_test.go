package core

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
)

type applyFakeLinkNode struct {
	name      string
	endpoints []clablinks.Endpoint
}

type applyLiveMockNode struct {
	*clabmocksmocknodes.MockNode
	linkApplyMode clabnodes.LinkApplyMode
}

func (n *applyLiveMockNode) LinkApplyMode(context.Context) clabnodes.LinkApplyMode {
	return n.linkApplyMode
}

func (n *applyFakeLinkNode) AddLinkToContainer(
	_ context.Context,
	_ netlink.Link,
	_ func(ns.NetNS) error,
) error {
	return nil
}

func (n *applyFakeLinkNode) AddEndpoint(e clablinks.Endpoint) error {
	n.endpoints = append(n.endpoints, e)
	return nil
}

func (*applyFakeLinkNode) GetLinkEndpointType() clablinks.LinkEndpointType {
	return clablinks.LinkEndpointTypeVeth
}

func (n *applyFakeLinkNode) GetShortName() string {
	return n.name
}

func (n *applyFakeLinkNode) GetEndpoints() []clablinks.Endpoint {
	return n.endpoints
}

func (*applyFakeLinkNode) ExecFunction(context.Context, func(ns.NetNS) error) error {
	return nil
}

func (*applyFakeLinkNode) GetState() clabnodesstate.NodeState {
	return clabnodesstate.Deployed
}

func (*applyFakeLinkNode) Delete(context.Context) error {
	return nil
}

type applyFakeLink struct {
	linkType  clablinks.LinkType
	endpoints []clablinks.Endpoint
}

func (*applyFakeLink) Deploy(context.Context, clablinks.Endpoint) error {
	return nil
}

func (*applyFakeLink) Remove(context.Context) error {
	return nil
}

func (l *applyFakeLink) GetType() clablinks.LinkType {
	return l.linkType
}

func (l *applyFakeLink) GetEndpoints() []clablinks.Endpoint {
	return l.endpoints
}

func (*applyFakeLink) GetMTU() int {
	return 0
}

func (*applyFakeLink) GetVars() map[string]any {
	return nil
}

func TestApplyRequiresTopologyFile(t *testing.T) {
	t.Parallel()

	c, err := NewContainerLab(WithLabNameOnly("lab"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Apply(context.Background(), &ApplyOptions{})
	if err == nil {
		t.Fatal("expected apply to fail without a topology file")
	}
	if !strings.Contains(err.Error(), "requires a topology file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyDryRunPlansDeployWhenLabMissing(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	c, err := NewContainerLab(WithTopoPath("test_data/topo1.yml", nil))
	if err != nil {
		t.Fatal(err)
	}

	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	c.Runtimes[clabruntimedocker.RuntimeName] = mockRuntime
	c.globalRuntimeName = clabruntimedocker.RuntimeName

	mockRuntime.EXPECT().
		ListContainers(gomock.Any(), gomock.Any()).
		Return(nil, nil)

	result, err := c.Apply(context.Background(), &ApplyOptions{dryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DeployedLab {
		t.Fatal("expected apply dry-run to plan lab deployment")
	}
	if result.LabName != c.Config.Name {
		t.Fatalf("planned lab name %q, want %q", result.LabName, c.Config.Name)
	}
}

func TestApplyPlanLinkNeedsDeploy(t *testing.T) {
	t.Parallel()

	n1 := &applyFakeLinkNode{name: "n1"}
	n2 := &applyFakeLinkNode{name: "n2"}
	link := clablinks.NewLinkVEth()
	ep1 := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n1, "eth1", link))
	ep2 := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n2, "eth1", link))
	link.Endpoints = []clablinks.Endpoint{ep1, ep2}

	tests := []struct {
		name       string
		live       map[applyEndpointKey]struct{}
		addedNode  map[string]struct{}
		parkedNode map[string]struct{}
		startNode  map[string]struct{}
		want       bool
	}{
		{
			name: "all endpoints live",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
				{node: "n2", iface: "eth1"}: {},
			},
			want: false,
		},
		{
			name: "one endpoint missing",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
			},
			want: true,
		},
		{
			name: "endpoint belongs to added node",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
				{node: "n2", iface: "eth1"}: {},
			},
			addedNode: map[string]struct{}{"n2": {}},
			want:      true,
		},
		{
			name: "parked recreated node preserves live link",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
				{node: "n2", iface: "eth1"}: {},
			},
			// n2 is recreated but parked, so its existing link is preserved, not redeployed.
			parkedNode: map[string]struct{}{"n2": {}},
			want:       false,
		},
		{
			name: "parked node still deploys a new (non-live) link",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
			},
			parkedNode: map[string]struct{}{"n2": {}},
			want:       true,
		},
		{
			name: "stopped node with preserved endpoint keeps existing link",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
				{node: "n2", iface: "eth1"}: {},
			},
			startNode: map[string]struct{}{"n2": {}},
			want:      false,
		},
		{
			name: "stopped node with missing endpoint deploys link",
			live: map[applyEndpointKey]struct{}{
				{node: "n1", iface: "eth1"}: {},
			},
			startNode: map[string]struct{}{"n2": {}},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := newApplyPlan(nil, nil)
			if tt.addedNode != nil {
				plan.addedNodeSet = tt.addedNode
			}
			if tt.parkedNode != nil {
				plan.parkedNodeSet = tt.parkedNode
			}
			if tt.startNode != nil {
				plan.startNodeSet = tt.startNode
			}
			if tt.live != nil {
				plan.liveEndpointSet = tt.live
			}

			if got := plan.linkNeedsDeploy(link); got != tt.want {
				t.Fatalf("linkNeedsDeploy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanRecreatedNodeLinksDeploysAllTouchingLinks(t *testing.T) {
	t.Parallel()

	n1 := &applyFakeLinkNode{name: "n1"}
	n2 := &applyFakeLinkNode{name: "n2"}
	n3 := &applyFakeLinkNode{name: "n3"}

	link1 := clablinks.NewLinkVEth()
	link1.Endpoints = []clablinks.Endpoint{
		clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n1, "eth1", link1)),
		clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n2, "eth1", link1)),
	}
	link2 := clablinks.NewLinkVEth()
	link2.Endpoints = []clablinks.Endpoint{
		clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n3, "eth1", link2)),
		clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n1, "eth2", link2)),
	}

	c := &CLab{
		Links: map[int]clablinks.Link{
			0: link1,
			1: link2,
		},
	}
	plan := newApplyPlan(nil, nil)
	plan.recreatedNodeSet = map[string]struct{}{"n1": {}}
	plan.plannedLinkSet = map[int]struct{}{1: {}}
	plan.addedLinks = []clablinks.Link{link2}

	c.planRecreatedNodeLinks(plan)

	if got, want := len(plan.addedLinks), 2; got != want {
		t.Fatalf("planned links = %d, want %d", got, want)
	}
	if plan.addedLinks[0] != link2 {
		t.Fatalf("expected pre-planned link to stay first")
	}
	if plan.addedLinks[1] != link1 {
		t.Fatalf("expected unchanged recreated-node link to be planned")
	}
	result := applyResultFromPlan(plan)
	if got, want := len(result.AddedLinks), 2; got != want {
		t.Fatalf("reported added links = %d, want %d", got, want)
	}
}

func TestPlanDeletedEndpointsUsesDiscoveredEndpointNode(t *testing.T) {
	t.Parallel()

	parkingNode := &applyFakeLinkNode{name: "parking-n1"}
	plan := newApplyPlan(nil, nil)
	plan.liveEndpointSet = map[applyEndpointKey]struct{}{
		{node: "n1", iface: "eth1"}: {},
	}
	plan.endpointNodes = map[string]clablinks.Node{
		"n1": parkingNode,
	}

	c := &CLab{}
	c.planDeletedEndpoints(context.Background(), plan)

	if len(plan.staleEndpoints) != 1 {
		t.Fatalf("stale endpoints = %d, want 1", len(plan.staleEndpoints))
	}
	if plan.staleEndpoints[0].node != parkingNode {
		t.Fatalf("stale endpoint delete node = %v, want discovered endpoint node", plan.staleEndpoints[0].node)
	}
}

func TestApplyNodeLinkApplyMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hasMode bool
		mode    clabnodes.LinkApplyMode
		want    clabnodes.LinkApplyMode
	}{
		{
			name:    "live node",
			hasMode: true,
			mode:    clabnodes.LinkApplyModeLive,
			want:    clabnodes.LinkApplyModeLive,
		},
		{
			name:    "restart node",
			hasMode: true,
			mode:    clabnodes.LinkApplyModeRestart,
			want:    clabnodes.LinkApplyModeRestart,
		},
		{
			name:    "recreate node",
			hasMode: true,
			mode:    clabnodes.LinkApplyModeRecreate,
			want:    clabnodes.LinkApplyModeRecreate,
		},
		{
			name:    "invalid mode defaults to recreate",
			hasMode: true,
			mode:    clabnodes.LinkApplyMode("invalid"),
			want:    clabnodes.LinkApplyModeRecreate,
		},
		{
			name: "node without mode defaults to recreate",
			want: clabnodes.LinkApplyModeRecreate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockNode := clabmocksmocknodes.NewMockNode(ctrl)

			var node clabnodes.Node = mockNode
			if tt.hasMode {
				node = &applyLiveMockNode{
					MockNode:      mockNode,
					linkApplyMode: tt.mode,
				}
			}

			if got := clabnodes.LinkApplyModeForNode(context.Background(), node); got != tt.want {
				t.Fatalf("LinkApplyModeForNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanAffectedApplyNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		change        string
		mode          clabnodes.LinkApplyMode
		wantRestart   bool
		wantRecreated bool
	}{
		{
			name:   "added link on live capable node skips lifecycle",
			change: "added link",
			mode:   clabnodes.LinkApplyModeLive,
		},
		{
			name:   "deleted endpoint on live capable node skips lifecycle",
			change: "deleted endpoint",
			mode:   clabnodes.LinkApplyModeLive,
		},
		{
			name:        "added link on restart node restarts",
			change:      "added link",
			mode:        clabnodes.LinkApplyModeRestart,
			wantRestart: true,
		},
		{
			name:          "deleted endpoint on default node recreates",
			change:        "deleted endpoint",
			mode:          clabnodes.LinkApplyModeRecreate,
			wantRecreated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockNode := clabmocksmocknodes.NewMockNode(ctrl)

			c := &CLab{
				Nodes: map[string]clabnodes.Node{
					"n1": &applyLiveMockNode{
						MockNode:      mockNode,
						linkApplyMode: tt.mode,
					},
				},
			}
			mockNode.EXPECT().Config().Return(&clabtypes.NodeConfig{}).AnyTimes()
			plan := newApplyPlan(nil, nil)

			c.planAffectedApplyNode(context.Background(), plan, "n1", tt.change)

			_, restart := plan.linkRestartNodeSet["n1"]
			if restart != tt.wantRestart {
				t.Fatalf("restart = %v, want %v", restart, tt.wantRestart)
			}

			_, recreated := plan.recreatedNodeSet["n1"]
			if recreated != tt.wantRecreated {
				t.Fatalf("recreated = %v, want %v", recreated, tt.wantRecreated)
			}

			deployNode := strings.Join(plan.deployNodeNames(), ",") == "n1"
			if deployNode != tt.wantRecreated {
				t.Fatalf("deploy node = %v, want %v", deployNode, tt.wantRecreated)
			}
		})
	}
}

func TestPlanNodeReconciliationKeepsRecreateAction(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockNode := clabmocksmocknodes.NewMockNode(ctrl)
	diff := &clabtypes.TopologyDiff{}

	mockNode.EXPECT().ComputeDiff(gomock.Any(), gomock.Any()).Return(diff)
	mockNode.EXPECT().
		GetReconcilePlan(gomock.Any(), diff).
		Return(&clabnodes.ReconcileResult{Action: clabtypes.TopologyDiffActionRecreate}, nil)

	topo := clabtypes.NewTopology()
	topo.Nodes["n1"] = &clabtypes.NodeDefinition{Kind: "linux", Image: "img"}

	c := &CLab{
		Config: &Config{
			Topology: topo,
		},
		Nodes: map[string]clabnodes.Node{"n1": mockNode},
	}
	plan := newApplyPlan(map[string]*runtimeNodeGroup{"n1": {}}, nil)

	if err := c.planNodeReconciliation(context.Background(), plan); err != nil {
		t.Fatal(err)
	}

	if _, recreated := plan.recreatedNodeSet["n1"]; !recreated {
		t.Fatal("expected node reconciliation recreate action to be preserved")
	}
	if _, added := plan.addedNodeSet["n1"]; added {
		t.Fatal("did not expect recreated node to be reported as added")
	}
	if got := strings.Join(plan.deployNodeNames(), ","); got != "n1" {
		t.Fatalf("deploy nodes = %q, want n1", got)
	}
}

func TestRestartApplyNodesRestartsLinkAffectedNodes(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockNode := clabmocksmocknodes.NewMockNode(ctrl)

	mockNode.EXPECT().Stop(gomock.Any()).Return(nil)
	mockNode.EXPECT().Start(gomock.Any()).Return(nil)
	mockNode.EXPECT().GetContainerStatus(gomock.Any()).Return(clabruntime.Running)
	mockNode.EXPECT().Config().Return(&clabtypes.NodeConfig{ShortName: "n1"}).AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{
			"n1": mockNode,
		},
		timeout: time.Second,
	}

	if err := c.restartApplyNodes(context.Background(), map[string]struct{}{"n1": {}}); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeNodeGroupsDistributedComponents(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	c := &CLab{
		Config: &Config{Name: "lab"},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: clabmocksmockruntime.NewMockContainerRuntime(ctrl),
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

	mockRuntime := c.Runtimes[clabruntimedocker.RuntimeName].(*clabmocksmockruntime.MockContainerRuntime)
	mockRuntime.EXPECT().
		ListContainers(gomock.Any(), gomock.Any()).
		Return([]clabruntime.GenericContainer{
			{
				Names: []string{"clab-lab-sros-a"},
				Labels: map[string]string{
					clabconstants.NodeName:     "sros-a",
					clabconstants.RootNodeName: "sros",
				},
			},
			{
				Names: []string{"clab-lab-sros-1"},
				Labels: map[string]string{
					clabconstants.NodeName:     "sros-1",
					clabconstants.RootNodeName: "sros",
				},
			},
		}, nil)

	currentNodes, err := c.runtimeNodeGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group := currentNodes["sros"]
	if group == nil {
		t.Fatal("expected distributed components to be grouped under root node name")
	}
	if !group.distributed {
		t.Fatal("expected distributed group marker")
	}
	if got := len(group.containers); got != 2 {
		t.Fatalf("expected 2 component containers, got %d", got)
	}
}

func TestSetMgmtBridgeFromRuntime(t *testing.T) {
	t.Parallel()

	c := &CLab{
		Config: &Config{Mgmt: &clabtypes.MgmtNet{}},
	}

	err := c.setMgmtBridgeFromRuntime(map[string]*runtimeNodeGroup{
		"l1": {
			containers: []clabruntime.GenericContainer{
				{
					Labels: map[string]string{
						clabconstants.NodeMgmtNetBr: "br-test",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if c.Config.Mgmt.Bridge != "br-test" {
		t.Fatalf("expected bridge from runtime labels, got %q", c.Config.Mgmt.Bridge)
	}
}

func TestRuntimeContainerSortsSrosComponentsInDeploymentOrder(t *testing.T) {
	t.Parallel()

	containers := []clabruntime.GenericContainer{
		{Labels: map[string]string{clabconstants.NodeName: "sros-a"}},
		{Labels: map[string]string{clabconstants.NodeName: "sros-b"}},
		{Labels: map[string]string{clabconstants.NodeName: "sros-1"}},
	}

	sort.Slice(containers, func(i, j int) bool {
		return runtimeContainerLess(containers[i], containers[j])
	})

	got := []string{
		runtimeContainerNodeName(containers[0]),
		runtimeContainerNodeName(containers[1]),
		runtimeContainerNodeName(containers[2]),
	}
	want := []string{"sros-1", "sros-b", "sros-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected component order %v, want %v", got, want)
	}
}
