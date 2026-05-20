package core

import (
	"context"
	"strings"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
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

func TestCheckApplySupportedRejectsNonVethLinks(t *testing.T) {
	t.Parallel()

	c := &CLab{
		Links: map[int]clablinks.Link{
			0: &applyFakeLink{linkType: clablinks.LinkTypeHost},
		},
	}

	err := c.checkApplySupported(nil)
	if err == nil {
		t.Fatal("expected unsupported link type error")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("unexpected error: %v", err)
	}
}
