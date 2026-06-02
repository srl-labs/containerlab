package core

import (
	"context"
	"sort"
	"strings"
	"testing"

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
	supportsLiveApply bool
}

func (n *applyLiveMockNode) SupportsLiveLinkApply() bool {
	return n.supportsLiveApply
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
		name      string
		live      map[applyEndpointKey]struct{}
		addedNode map[string]struct{}
		want      bool
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &applyPlan{
				addedNodeSet:    tt.addedNode,
				liveEndpointSet: tt.live,
			}
			if plan.addedNodeSet == nil {
				plan.addedNodeSet = map[string]struct{}{}
			}

			if got := plan.linkNeedsDeploy(link); got != tt.want {
				t.Fatalf("linkNeedsDeploy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyNodeRequiresRestart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		exec         []string
		hasLiveApply bool
		supportsLive bool
		want         bool
	}{
		{
			name:         "live capable node without exec",
			hasLiveApply: true,
			supportsLive: true,
			want:         false,
		},
		{
			name:         "live capable node with exec",
			exec:         []string{"ip addr add 192.0.2.1/30 dev eth1"},
			hasLiveApply: true,
			supportsLive: true,
			want:         true,
		},
		{
			name:         "live incapable node",
			hasLiveApply: true,
			supportsLive: false,
			want:         true,
		},
		{
			name: "node without live capability",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockNode := clabmocksmocknodes.NewMockNode(ctrl)
			mockNode.EXPECT().
				Config().
				Return(&clabtypes.NodeConfig{ShortName: "n1", Exec: tt.exec}).
				AnyTimes()

			var node clabnodes.Node = mockNode
			if tt.hasLiveApply {
				node = &applyLiveMockNode{
					MockNode:          mockNode,
					supportsLiveApply: tt.supportsLive,
				}
			}

			if got := applyNodeRequiresRestart(node); got != tt.want {
				t.Fatalf("applyNodeRequiresRestart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanAffectedApplyNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		change       string
		exec         []string
		supportsLive bool
		wantAffected bool
	}{
		{
			name:         "added link on live capable node skips restart",
			change:       "added link",
			supportsLive: true,
			wantAffected: false,
		},
		{
			name:         "deleted endpoint on live capable node skips restart",
			change:       "deleted endpoint",
			supportsLive: true,
			wantAffected: false,
		},
		{
			name:         "added link on vm node restarts",
			change:       "added link",
			supportsLive: false,
			wantAffected: true,
		},
		{
			name:         "deleted endpoint on exec node restarts",
			change:       "deleted endpoint",
			exec:         []string{"ip addr add 192.0.2.1/30 dev eth1"},
			supportsLive: true,
			wantAffected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockNode := clabmocksmocknodes.NewMockNode(ctrl)
			mockNode.EXPECT().
				Config().
				Return(&clabtypes.NodeConfig{ShortName: "n1", Exec: tt.exec}).
				AnyTimes()

			c := &CLab{
				Nodes: map[string]clabnodes.Node{
					"n1": &applyLiveMockNode{
						MockNode:          mockNode,
						supportsLiveApply: tt.supportsLive,
					},
				},
			}
			plan := &applyPlan{affectedNodeSet: map[string]struct{}{}}

			c.planAffectedApplyNode(plan, "n1", tt.change)

			_, affected := plan.affectedNodeSet["n1"]
			if affected != tt.wantAffected {
				t.Fatalf("affected = %v, want %v", affected, tt.wantAffected)
			}
		})
	}
}

func TestCheckApplySupportedAllowsAllLinkTypes(t *testing.T) {
	t.Parallel()

	c := &CLab{
		Links: map[int]clablinks.Link{
			0: &applyFakeLink{linkType: clablinks.LinkTypeHost},
		},
	}

	if err := c.checkApplySupported(nil); err != nil {
		t.Fatalf("expected non-veth link type to be supported, got: %v", err)
	}
}

func TestApplyReportsReconfigureRequiredWhenUnsupported(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	c, err := NewContainerLab(WithTopoPath("test_data/topo1.yml", nil))
	if err != nil {
		t.Fatal(err)
	}

	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	c.Runtimes[clabruntimedocker.RuntimeName] = mockRuntime
	c.globalRuntimeName = clabruntimedocker.RuntimeName

	nodeCfg := c.Nodes["node1"].Config()
	nodeCfg.IsRootNamespaceBased = true

	mockRuntime.EXPECT().
		ListContainers(gomock.Any(), gomock.Any()).
		Return([]clabruntime.GenericContainer{
			{
				Names: []string{nodeCfg.LongName},
				Labels: map[string]string{
					clabconstants.NodeName:      nodeCfg.ShortName,
					clabconstants.LongName:      nodeCfg.LongName,
					clabconstants.NodeKind:      nodeCfg.Kind,
					clabconstants.NodeType:      nodeCfg.NodeType,
					clabconstants.NodeGroup:     nodeCfg.Group,
					clabconstants.NodeMgmtNetBr: "br-test",
				},
			},
		}, nil)

	_, err = c.Apply(context.Background(), &ApplyOptions{dryRun: true})
	if err == nil {
		t.Fatal("expected unsupported apply error")
	}

	for _, want := range []string{
		"apply unsupported for",
		"node1: root namespace node",
		"deploy --reconfigure",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got: %v", want, err)
		}
	}
}

func TestRuntimeNodeContainersGroupsDistributedComponents(t *testing.T) {
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

	currentNodes, err := c.runtimeNodeContainers(context.Background())
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

func TestSetApplyMgmtBridgeFromRuntime(t *testing.T) {
	t.Parallel()

	c := &CLab{
		Config: &Config{Mgmt: &clabtypes.MgmtNet{}},
	}

	err := c.setApplyMgmtBridgeFromRuntime(map[string]*applyRuntimeNode{
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

func TestApplyContainerSortsSrosComponentsInDeploymentOrder(t *testing.T) {
	t.Parallel()

	containers := []clabruntime.GenericContainer{
		{Labels: map[string]string{clabconstants.NodeName: "sros-a"}},
		{Labels: map[string]string{clabconstants.NodeName: "sros-b"}},
		{Labels: map[string]string{clabconstants.NodeName: "sros-1"}},
	}

	sort.Slice(containers, func(i, j int) bool {
		return applyContainerLess(containers[i], containers[j])
	})

	got := []string{
		applyContainerNodeName(containers[0]),
		applyContainerNodeName(containers[1]),
		applyContainerNodeName(containers[2]),
	}
	want := []string{"sros-1", "sros-b", "sros-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected component order %v, want %v", got, want)
	}
}
