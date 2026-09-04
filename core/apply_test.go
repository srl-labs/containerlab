package core

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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

func (n *applyFakeLinkNode) AdoptEndpoint(e clablinks.Endpoint) error {
	n.endpoints = append(n.endpoints, e)
	e.SetNode(n)
	return nil
}

func (n *applyFakeLinkNode) ReleaseEndpoint(e clablinks.Endpoint) error {
	for i, ep := range n.endpoints {
		if ep == e {
			n.endpoints = append(n.endpoints[:i], n.endpoints[i+1:]...)
			break
		}
	}
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
	linkType        clablinks.LinkType
	endpoints       []clablinks.Endpoint
	deployCalls     int
	postDeployCalls int
}

func (l *applyFakeLink) Deploy(context.Context, clablinks.Endpoint) error {
	l.deployCalls++
	return nil
}

func (l *applyFakeLink) PostDeploy(context.Context) error {
	l.postDeployCalls++
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

func (l *applyFakeLink) GetRuntimeEndpoints() []clablinks.Endpoint {
	return l.endpoints
}

func (*applyFakeLink) GetMTU() int {
	return 0
}

func (*applyFakeLink) GetVars() map[string]any {
	return nil
}

func TestDeployLinksUsesEndpointOwnership(t *testing.T) {
	node := &applyFakeLinkNode{name: "n1"}
	link := &applyFakeLink{linkType: clablinks.LinkTypeDummy}
	link.endpoints = []clablinks.Endpoint{
		clablinks.NewEndpointDummy(clablinks.NewEndpointGeneric(node, "eth1", link)),
	}

	if err := (&CLab{}).DeployLinks(context.Background(), []clablinks.Link{link}); err != nil {
		t.Fatal(err)
	}
	if link.deployCalls != 1 || link.postDeployCalls != 1 {
		t.Fatalf(
			"deploy calls = %d, post-deploy calls = %d",
			link.deployCalls,
			link.postDeployCalls,
		)
	}
}

func TestDeployLinksPostDeploysSelectedNodeWithoutLinks(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().PostDeployEndpoints(gomock.Any()).Return(nil)

	c := &CLab{Nodes: map[string]clabnodes.Node{"n1": node}}
	if err := c.deployLinks(context.Background(), nil, []string{"n1"}); err != nil {
		t.Fatal(err)
	}
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

func TestApplyWritesStateForNoChanges(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	mockNode := clabmocksmocknodes.NewMockNode(ctrl)

	labDir := t.TempDir()
	topoFile := filepath.Join(labDir, "noop.clab.yml")
	if err := os.WriteFile(topoFile, []byte("name: noop\n"), 0o644); err != nil {
		t.Fatalf("failed to write topology file: %v", err)
	}
	topoPaths, err := clabtypes.NewTopoPaths(topoFile, nil)
	if err != nil {
		t.Fatalf("failed to create topo paths: %v", err)
	}
	if err := topoPaths.SetLabDir(labDir); err != nil {
		t.Fatalf("failed to set lab dir: %v", err)
	}

	nodeCfg := &clabtypes.NodeConfig{
		ShortName: "n1",
		LongName:  "clab-noop-n1",
	}
	diff := &clabtypes.TopologyDiff{}

	mockRuntime.EXPECT().
		ListContainers(gomock.Any(), gomock.Any()).
		Return([]clabruntime.GenericContainer{
			{
				Labels: map[string]string{
					clabconstants.NodeName:      "n1",
					clabconstants.NodeMgmtNetBr: "br-test",
				},
			},
		}, nil)
	mockNode.EXPECT().Config().Return(nodeCfg).AnyTimes()
	mockNode.EXPECT().CheckDeploymentConditions(gomock.Any()).Return(nil)
	mockNode.EXPECT().ComputeDiff(gomock.Any(), gomock.Any()).Return(diff)
	mockNode.EXPECT().
		GetReconcilePlan(gomock.Any(), diff).
		Return(&clabnodes.ReconcileResult{Action: clabtypes.TopologyDiffActionNone}, nil)
	mockNode.EXPECT().GetContainerStatus(gomock.Any()).Return(clabruntime.Running).AnyTimes()
	mockNode.EXPECT().ExecFunction(gomock.Any(), gomock.Any()).Return(nil)
	mockNode.EXPECT().
		Reconcile(gomock.Any(), diff).
		Return(&clabnodes.ReconcileResult{Action: clabtypes.TopologyDiffActionNone}, nil)

	topo := clabtypes.NewTopology()
	topo.Nodes["n1"] = &clabtypes.NodeDefinition{Kind: "linux", Image: "alpine:latest"}

	c := &CLab{
		Config: &Config{
			Name:     "noop",
			Mgmt:     &clabtypes.MgmtNet{},
			Topology: topo,
		},
		TopoPaths: topoPaths,
		Nodes: map[string]clabnodes.Node{
			"n1": mockNode,
		},
		Links: map[int]clablinks.Link{},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: mockRuntime,
		},
	}

	result, err := c.Apply(context.Background(), &ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeployedLab {
		t.Fatal("expected no-op apply, got deploy result")
	}
	if _, err := os.Stat(topoPaths.StateFile()); err != nil {
		t.Fatalf("expected no-op apply to write state file: %v", err)
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

func TestApplyPlanLinkNeedsDeployRejectsMismatchedVethPeer(t *testing.T) {
	t.Parallel()

	n1 := &applyFakeLinkNode{name: "n1"}
	n2 := &applyFakeLinkNode{name: "n2"}
	link := clablinks.NewLinkVEth()
	ep1 := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n1, "eth1", link))
	ep2 := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n2, "eth1", link))
	link.Endpoints = []clablinks.Endpoint{ep1, ep2}

	plan := newApplyPlan(nil, nil)
	plan.liveEndpointSet = map[applyEndpointKey]struct{}{
		{node: "n1", iface: "eth1"}: {},
		{node: "n2", iface: "eth1"}: {},
	}
	plan.liveEndpointInfo = map[applyEndpointKey]clablinks.OwnedInterface{
		{node: "n1", iface: "eth1"}: {Name: "eth1", Index: 11, PeerIndex: 21},
		{node: "n2", iface: "eth1"}: {Name: "eth1", Index: 22, PeerIndex: 11},
	}

	if !plan.linkNeedsDeploy(link) {
		t.Fatal("expected link with mismatched live peers to be redeployed")
	}

	plan.liveEndpointInfo[applyEndpointKey{node: "n1", iface: "eth1"}] = clablinks.OwnedInterface{
		Name: "eth1", Index: 11, PeerIndex: 22,
	}
	if plan.linkNeedsDeploy(link) {
		t.Fatal("expected link with matching live peers to be preserved")
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

func TestPlanStoppedNodesSkipsExternallyManagedNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	managedNode := clabmocksmocknodes.NewMockNode(ctrl)
	externalNode := clabmocksmocknodes.NewMockNode(ctrl)
	managedNode.EXPECT().GetContainerStatus(ctx).Return(clabruntime.Stopped)

	c := &CLab{
		Nodes: map[string]clabnodes.Node{
			"managed":  managedNode,
			"external": externalNode,
		},
	}
	plan := newApplyPlan(map[string]*runtimeNodeGroup{
		"managed":  {},
		"external": {external: true},
	}, nil)

	c.planStoppedNodes(ctx, plan)

	if got, want := sortedStringSet(
		plan.startNodeSet,
	), []string{
		"managed",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("nodes planned for start = %v, want %v", got, want)
	}
}

func TestDiscoverLiveApplyEndpointsRejectsStoppedExternalNode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	externalNode := clabmocksmocknodes.NewMockNode(ctrl)
	externalNode.EXPECT().GetContainerStatus(ctx).Return(clabruntime.Stopped)

	c := &CLab{
		Nodes: map[string]clabnodes.Node{"external": externalNode},
	}
	plan := newApplyPlan(map[string]*runtimeNodeGroup{
		"external": {external: true},
	}, nil)

	err := c.discoverLiveApplyEndpoints(ctx, plan)
	if err == nil || !strings.Contains(err.Error(), "start it outside containerlab") {
		t.Fatalf("error = %v, want external lifecycle guidance", err)
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
		t.Fatalf(
			"stale endpoint delete node = %v, want discovered endpoint node",
			plan.staleEndpoints[0].node,
		)
	}
}

func TestApplyNodeLinkApplyMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     clabnodes.LinkApplyMode
		override clabnodes.LinkApplyMode
		want     clabnodes.LinkApplyMode
	}{
		{
			name: "live node",
			mode: clabnodes.LinkApplyModeLive,
			want: clabnodes.LinkApplyModeLive,
		},
		{
			name: "restart node",
			mode: clabnodes.LinkApplyModeRestart,
			want: clabnodes.LinkApplyModeRestart,
		},
		{
			name: "recreate node",
			mode: clabnodes.LinkApplyModeRecreate,
			want: clabnodes.LinkApplyModeRecreate,
		},
		{
			name: "invalid mode defaults to recreate",
			mode: clabnodes.LinkApplyMode("invalid"),
			want: clabnodes.LinkApplyModeRecreate,
		},
		{
			name: "node without mode defaults to recreate",
			want: clabnodes.LinkApplyModeRecreate,
		},
		{
			name:     "override upgrades recreate kind to live",
			mode:     clabnodes.LinkApplyModeRecreate,
			override: clabnodes.LinkApplyModeLive,
			want:     clabnodes.LinkApplyModeLive,
		},
		{
			name:     "override restricts live kind to recreate",
			mode:     clabnodes.LinkApplyModeLive,
			override: clabnodes.LinkApplyModeRecreate,
			want:     clabnodes.LinkApplyModeRecreate,
		},
		{
			name:     "invalid override falls back to kind mode",
			mode:     clabnodes.LinkApplyModeRestart,
			override: clabnodes.LinkApplyMode("hotplug"),
			want:     clabnodes.LinkApplyModeRestart,
		},
		{
			name:     "override on node without declared mode",
			override: clabnodes.LinkApplyModeLive,
			want:     clabnodes.LinkApplyModeLive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockNode := clabmocksmocknodes.NewMockNode(ctrl)
			mockNode.EXPECT().Config().Return(&clabtypes.NodeConfig{
				ShortName:     "n1",
				LinkApplyMode: tt.override,
			}).AnyTimes()
			mockNode.EXPECT().LinkApplyMode(gomock.Any()).Return(tt.mode)

			if got := clabnodes.LinkApplyModeForNode(
				context.Background(),
				mockNode,
			); got != tt.want {
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
					"n1": mockNode,
				},
			}
			mockNode.EXPECT().Config().Return(&clabtypes.NodeConfig{}).AnyTimes()
			mockNode.EXPECT().LinkApplyMode(gomock.Any()).Return(tt.mode)
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

			reason, hasReason := plan.nodeChangeReasons["n1"]
			wantReason := tt.wantRestart || tt.wantRecreated
			if hasReason != wantReason {
				t.Fatalf("change reason recorded = %v, want %v", hasReason, wantReason)
			}
			if hasReason && reason != tt.change {
				t.Fatalf("change reason = %q, want %q", reason, tt.change)
			}
		})
	}
}

func TestPlanAffectedApplyNodeIgnoresExternalLifecycleOverride(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	externalNode := clabmocksmocknodes.NewMockNode(ctrl)
	externalNode.EXPECT().Config().Return(&clabtypes.NodeConfig{
		LinkApplyMode: clabnodes.LinkApplyModeRestart,
	}).AnyTimes()
	externalNode.EXPECT().
		LinkApplyMode(gomock.Any()).
		Return(clabnodes.LinkApplyModeLive).
		AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{"external": externalNode},
	}
	plan := newApplyPlan(map[string]*runtimeNodeGroup{
		"external": {external: true},
	}, nil)

	c.planAffectedApplyNode(context.Background(), plan, "external", "added link")

	if len(plan.linkRestartNodeSet) != 0 || len(plan.recreatedNodeSet) != 0 {
		t.Fatalf(
			"external node received lifecycle action: restart=%v recreate=%v",
			plan.linkRestartNodeSet,
			plan.recreatedNodeSet,
		)
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

func TestResolveNodeConfigFromTopologyRuntimeOptions(t *testing.T) {
	registry := clabnodes.NewNodeRegistry()
	attributes := clabnodes.NewNodeRegistryEntryAttributes(nil, nil, nil).
		WithPrivilegedByDefault(false)
	if err := registry.Register([]string{"test"}, nil, attributes); err != nil {
		t.Fatal(err)
	}

	topo := clabtypes.NewTopology()
	topo.Nodes["n1"] = &clabtypes.NodeDefinition{
		Kind:         "test",
		Hostname:     "custom-host",
		CgroupnsMode: "host",
		CgroupParent: "/xform/my-lab/leaves",
		PidMode:      "host",
		Tmpfs:        map[string]string{"/run": "rw,nosuid"},
		SecurityOpts: []string{"seccomp=unconfined"},
	}

	cfg := (&CLab{Reg: registry}).resolveNodeConfigFromTopology(topo, "n1")
	if cfg.Privileged {
		t.Fatal("expected kind default to resolve to unprivileged")
	}
	if cfg.Hostname != "custom-host" {
		t.Fatalf("Hostname = %q, want custom-host", cfg.Hostname)
	}
	if cfg.CgroupnsMode != "host" {
		t.Fatalf("CgroupnsMode = %q, want host", cfg.CgroupnsMode)
	}
	if cfg.CgroupParent != "/xform/my-lab/leaves" {
		t.Fatalf("CgroupParent = %q, want /xform/my-lab/leaves", cfg.CgroupParent)
	}
	if cfg.PidMode != "host" {
		t.Fatalf("PidMode = %q, want host", cfg.PidMode)
	}
	if got := cfg.Tmpfs["/run"]; got != "rw,nosuid" {
		t.Fatalf("Tmpfs[/run] = %q, want rw,nosuid", got)
	}
	if !slices.Equal(cfg.SecurityOpts, []string{"seccomp=unconfined"}) {
		t.Fatalf("SecurityOpts = %v, want seccomp=unconfined", cfg.SecurityOpts)
	}
}

func TestNetworkModeContainerTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		networkMode string
		want        string
	}{
		{networkMode: "", want: ""},
		{networkMode: "host", want: ""},
		{networkMode: "none", want: ""},
		{networkMode: "container:worker1", want: "worker1"},
	}

	for _, tt := range tests {
		t.Run(tt.networkMode, func(t *testing.T) {
			t.Parallel()
			if got := networkModeContainerTarget(tt.networkMode); got != tt.want {
				t.Fatalf(
					"networkModeContainerTarget(%q) = %q, want %q",
					tt.networkMode,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestPlanNetworkModeCascadeForcesRecreate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	target := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	target.EXPECT().Config().Return(&clabtypes.NodeConfig{ShortName: "target"}).AnyTimes()
	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:target",
	}).AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{
			"target":  target,
			"sidecar": sidecar,
		},
	}
	plan := newApplyPlan(nil, nil)
	plan.recreatedNodeSet["target"] = struct{}{}
	plan.restartNodeSet["sidecar"] = struct{}{}

	c.planNetworkModeCascade(plan)

	if _, recreated := plan.recreatedNodeSet["sidecar"]; !recreated {
		t.Fatal("expected sidecar sharing the recreated node's netns to be recreated too")
	}
	if _, restarting := plan.restartNodeSet["sidecar"]; restarting {
		t.Fatal("expected sidecar to be removed from restartNodeSet once forced to recreate")
	}
	if _, hasReason := plan.nodeChangeReasons["sidecar"]; !hasReason {
		t.Fatal("expected a change reason to be recorded for the cascaded recreate")
	}
}

func TestPlanNetworkModeCascadeIsTransitive(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	target := clabmocksmocknodes.NewMockNode(ctrl)
	mid := clabmocksmocknodes.NewMockNode(ctrl)
	leaf := clabmocksmocknodes.NewMockNode(ctrl)
	target.EXPECT().Config().Return(&clabtypes.NodeConfig{ShortName: "target"}).AnyTimes()
	mid.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "mid",
		NetworkMode: "container:target",
	}).AnyTimes()
	leaf.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "leaf",
		NetworkMode: "container:mid",
	}).AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{
			"target": target,
			"mid":    mid,
			"leaf":   leaf,
		},
	}
	plan := newApplyPlan(nil, nil)
	plan.recreatedNodeSet["target"] = struct{}{}

	c.planNetworkModeCascade(plan)

	if _, recreated := plan.recreatedNodeSet["leaf"]; !recreated {
		t.Fatal("expected recreate to cascade transitively through a chain of shared namespaces")
	}
}

func TestPlanNetworkModeCascadeIgnoresAddedNodes(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	target := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	target.EXPECT().Config().Return(&clabtypes.NodeConfig{ShortName: "target"}).AnyTimes()
	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:target",
	}).AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{
			"target":  target,
			"sidecar": sidecar,
		},
	}
	plan := newApplyPlan(nil, nil)
	plan.recreatedNodeSet["target"] = struct{}{}
	plan.addedNodeSet["sidecar"] = struct{}{}

	c.planNetworkModeCascade(plan)

	if _, recreated := plan.recreatedNodeSet["sidecar"]; recreated {
		t.Fatal("a newly added sidecar should deploy fresh, not be marked recreated")
	}
}

func TestCheckApplyNetworkModeTargetsRejectsDeletedTarget(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:worker1",
	}).AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{"sidecar": sidecar},
	}
	plan := newApplyPlan(nil, nil)
	plan.deletedNodeSet["worker1"] = struct{}{}

	err := c.checkApplyNetworkModeTargets(plan)
	if err == nil {
		t.Fatal("expected error when a network-mode target is deleted but the dependent stays")
	}
	if !strings.Contains(err.Error(), "sidecar") || !strings.Contains(err.Error(), "worker1") {
		t.Fatalf("error = %v, want it to name both nodes", err)
	}
}

func TestCheckApplyNetworkModeTargetsAllowsUnrelatedDeletion(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:worker1",
	}).AnyTimes()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{"sidecar": sidecar},
	}
	plan := newApplyPlan(nil, nil)
	plan.deletedNodeSet["unrelated"] = struct{}{}

	if err := c.checkApplyNetworkModeTargets(plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanNodeReconciliationRejectsExternalRecreate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockNode := clabmocksmocknodes.NewMockNode(ctrl)
	diff := &clabtypes.TopologyDiff{Fields: []string{"Env"}}

	mockNode.EXPECT().ComputeDiff(gomock.Any(), gomock.Any()).Return(diff)
	mockNode.EXPECT().
		GetReconcilePlan(gomock.Any(), diff).
		Return(&clabnodes.ReconcileResult{Action: clabtypes.TopologyDiffActionRecreate}, nil)

	topo := clabtypes.NewTopology()
	topo.Nodes["n1"] = &clabtypes.NodeDefinition{Kind: "ext-container"}
	c := &CLab{
		Config: &Config{Topology: topo},
		Nodes:  map[string]clabnodes.Node{"n1": mockNode},
	}
	plan := newApplyPlan(
		map[string]*runtimeNodeGroup{"n1": {external: true}},
		nil,
	)

	err := c.planNodeReconciliation(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "externally managed") {
		t.Fatalf("error = %v, want externally managed node error", err)
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
	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)

	c := &CLab{
		Config: &Config{Name: "lab"},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: mockRuntime,
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

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

func TestRuntimeNodeGroupsIncludesPreExistingNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(&clabtypes.NodeConfig{SkipUniquenessCheck: true})
	node.EXPECT().GetContainerStatus(ctx).Return(clabruntime.Running)
	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(nil, nil)

	c := &CLab{
		Config: &Config{Name: "lab"},
		Nodes:  map[string]clabnodes.Node{"external": node},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: rt,
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

	groups, err := c.runtimeNodeGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	group := groups["external"]
	if group == nil || !group.external || len(group.containers) != 0 {
		t.Fatalf("unexpected external runtime group: %#v", group)
	}
}

func TestRuntimeNodeGroupsClassifiesRootNamespaceResourcesSeparately(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	node := clabmocksmocknodes.NewMockNode(ctrl)

	node.EXPECT().Config().Return(&clabtypes.NodeConfig{
		IsRootNamespaceBased: true,
	})
	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(nil, nil)

	c := &CLab{
		Config: &Config{Name: "lab"},
		Nodes:  map[string]clabnodes.Node{"bridge": node},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: rt,
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

	groups, err := c.runtimeNodeGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	group := groups["bridge"]
	if group == nil || group.external || !group.rootNamespaceBased {
		t.Fatalf("unexpected root namespace runtime group: %#v", group)
	}
}

func TestNeedsInitialDeploy(t *testing.T) {
	t.Parallel()

	topoFile := filepath.Join(t.TempDir(), "lab.clab.yml")
	if err := os.WriteFile(topoFile, []byte("name: lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	topoPaths, err := clabtypes.NewTopoPaths(topoFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := topoPaths.SetLabDirByPrefix("lab"); err != nil {
		t.Fatal(err)
	}
	c := &CLab{TopoPaths: topoPaths}

	initial, err := c.needsInitialDeploy(map[string]*runtimeNodeGroup{
		"external": {external: true},
	})
	if err != nil || !initial {
		t.Fatalf("external nodes without state: initial = %v, error = %v", initial, err)
	}

	if err := os.MkdirAll(topoPaths.TopologyLabDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(topoPaths.StateFile(), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	initial, err = c.needsInitialDeploy(map[string]*runtimeNodeGroup{
		"external": {external: true},
	})
	if err != nil || initial {
		t.Fatalf("external nodes with state: initial = %v, error = %v", initial, err)
	}
}

func TestNeedsInitialDeployIgnoresRootNamespaceResources(t *testing.T) {
	t.Parallel()

	topoFile := filepath.Join(t.TempDir(), "lab.clab.yml")
	if err := os.WriteFile(topoFile, []byte("name: lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	topoPaths, err := clabtypes.NewTopoPaths(topoFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := topoPaths.SetLabDirByPrefix("lab"); err != nil {
		t.Fatal(err)
	}
	c := &CLab{TopoPaths: topoPaths}

	initial, err := c.needsInitialDeploy(map[string]*runtimeNodeGroup{
		"bridge": {rootNamespaceBased: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !initial {
		t.Fatal("expected root-namespace-only state without a state file to deploy initially")
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
