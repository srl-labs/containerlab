package core

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/docker/go-connections/nat"
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

func applyMockNodeWithConfig(
	ctrl *gomock.Controller,
	cfg *clabtypes.NodeConfig,
) clabnodes.Node {
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(cfg).AnyTimes()
	return node
}

func applyRuntimeConfigFromDesired(cfg *clabtypes.NodeConfig) clabruntime.GenericContainerConfig {
	entrypoint, _ := splitApplyShellArgs(cfg.Entrypoint)
	cmd, _ := splitApplyShellArgs(cfg.Cmd)

	return clabruntime.GenericContainerConfig{
		Available:     true,
		Image:         cfg.Image,
		User:          cfg.User,
		Entrypoint:    entrypoint,
		Cmd:           cmd,
		Env:           cloneApplyTestStringMap(cfg.Env),
		Labels:        cloneApplyTestStringMap(cfg.Labels),
		Binds:         append([]string(nil), cfg.Binds...),
		ExposedPorts:  applyGenericPortsFromPortSet(cfg.PortSet),
		PortBindings:  applyGenericPortsFromPortMap(cfg.PortBindings),
		NetworkMode:   cfg.NetworkMode,
		PidMode:       cfg.PidMode,
		MacAddress:    cfg.MacAddress,
		Aliases:       append([]string(nil), cfg.Aliases...),
		ExtraHosts:    append([]string(nil), cfg.ExtraHosts...),
		Sysctls:       cloneApplyTestStringMap(cfg.Sysctls),
		DNS:           cloneApplyTestDNS(cfg.DNS),
		CapAdd:        append([]string(nil), cfg.CapAdd...),
		Devices:       applyDesiredDevices(cfg.Devices),
		Tmpfs:         cloneApplyTestStringMap(cfg.Tmpfs),
		ShmSize:       parseApplyByteSize(cfg.ShmSize),
		CPUQuota:      applyDesiredCPUQuota(cfg.CPU),
		CPUPeriod:     applyDesiredCPUPeriod(cfg.CPU),
		CPUSet:        cfg.CPUSet,
		Memory:        parseApplyByteSize(cfg.Memory),
		RestartPolicy: cfg.RestartPolicy,
		Healthcheck:   cloneApplyTestHealthcheck(cfg.Healthcheck),
	}
}

func cloneApplyTestStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneApplyTestDNS(value *clabtypes.DNSConfig) *clabtypes.DNSConfig {
	if value == nil {
		return nil
	}
	return &clabtypes.DNSConfig{
		Servers: append([]string(nil), value.Servers...),
		Options: append([]string(nil), value.Options...),
		Search:  append([]string(nil), value.Search...),
	}
}

func cloneApplyTestHealthcheck(value *clabtypes.HealthcheckConfig) *clabtypes.HealthcheckConfig {
	if value == nil {
		return nil
	}
	return &clabtypes.HealthcheckConfig{
		Test:        append([]string(nil), value.Test...),
		Interval:    value.Interval,
		Timeout:     value.Timeout,
		Retries:     value.Retries,
		StartPeriod: value.StartPeriod,
	}
}

func (n *applyLiveMockNode) LinkApplyMode() clabnodes.LinkApplyMode {
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
	plan := &applyPlan{
		result:           &ApplyResult{AddedLinks: []string{applyLinkName(link2)}},
		recreatedNodeSet: map[string]struct{}{"n1": {}},
		plannedLinkSet:   map[int]struct{}{1: {}},
		addedLinks:       []clablinks.Link{link2},
	}

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
	if got, want := len(plan.result.AddedLinks), 2; got != want {
		t.Fatalf("reported added links = %d, want %d", got, want)
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

			if got := applyNodeLinkApplyMode(node); got != tt.want {
				t.Fatalf("applyNodeLinkApplyMode() = %v, want %v", got, tt.want)
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
			plan := &applyPlan{
				addedNodeSet:     map[string]struct{}{},
				affectedNodeSet:  map[string]struct{}{},
				recreatedNodeSet: map[string]struct{}{},
			}

			c.planAffectedApplyNode(plan, "n1", tt.change)

			_, affected := plan.affectedNodeSet["n1"]
			if affected != tt.wantRestart {
				t.Fatalf("restart = %v, want %v", affected, tt.wantRestart)
			}

			_, recreated := plan.recreatedNodeSet["n1"]
			if recreated != tt.wantRecreated {
				t.Fatalf("recreated = %v, want %v", recreated, tt.wantRecreated)
			}

			_, deployNode := plan.addedNodeSet["n1"]
			if deployNode != tt.wantRecreated {
				t.Fatalf("deploy node = %v, want %v", deployNode, tt.wantRecreated)
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

func TestCheckApplySupportedRejectsSpecialLifecycleNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *clabtypes.NodeConfig
		want string
	}{
		{
			name: "root namespace",
			cfg: &clabtypes.NodeConfig{
				ShortName:            "n1",
				IsRootNamespaceBased: true,
			},
			want: "n1: root namespace node",
		},
		{
			name: "auto remove",
			cfg: &clabtypes.NodeConfig{
				ShortName:  "n1",
				AutoRemove: true,
			},
			want: "n1: auto-remove node",
		},
		{
			name: "external pre-existing",
			cfg: &clabtypes.NodeConfig{
				ShortName:           "n1",
				SkipUniquenessCheck: true,
			},
			want: "n1: external/pre-existing node",
		},
		{
			name: "shared container namespace",
			cfg: &clabtypes.NodeConfig{
				ShortName:   "n1",
				NetworkMode: "container:n2",
			},
			want: "n1: shared container namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			c := &CLab{
				Nodes: map[string]clabnodes.Node{
					"n1": applyMockNodeWithConfig(ctrl, tt.cfg),
				},
			}

			err := c.checkApplySupported(nil)
			if err == nil {
				t.Fatal("expected unsupported apply error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to contain %q, got: %v", tt.want, err)
			}
			if !strings.Contains(err.Error(), "cannot handle these node lifecycle modes") {
				t.Fatalf("unexpected unsupported error: %v", err)
			}
		})
	}
}

func TestApplyReportsUnsupportedLifecycleWhenUnsupported(t *testing.T) {
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
		"cannot handle these node lifecycle modes",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got: %v", want, err)
		}
	}
}

func TestPlanRecreatedApplyNodesFromComponentDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		components  []*clabtypes.Component
		runtimeNode *applyRuntimeNode
	}{
		{
			name: "distributed layout changed",
			components: []*clabtypes.Component{
				{Slot: "A"},
				{Slot: "1"},
			},
			runtimeNode: &applyRuntimeNode{
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{clabconstants.NodeName: "sros"}},
				},
			},
		},
		{
			name: "distributed component name changed",
			components: []*clabtypes.Component{
				{Slot: "A"},
				{Slot: "1"},
				{Slot: "2"},
			},
			runtimeNode: &applyRuntimeNode{
				distributed: true,
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{clabconstants.NodeName: "sros-a"}},
					{Labels: map[string]string{clabconstants.NodeName: "sros-1"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			cfg := &clabtypes.NodeConfig{
				ShortName:  "sros",
				LongName:   "clab-lab-sros",
				Kind:       "nokia_srsim",
				Components: tt.components,
			}
			c := &CLab{
				Nodes: map[string]clabnodes.Node{
					"sros": applyMockNodeWithConfig(ctrl, cfg),
				},
			}
			plan := &applyPlan{
				currentNodes:     map[string]*applyRuntimeNode{"sros": tt.runtimeNode},
				addedNodeSet:     map[string]struct{}{},
				recreatedNodeSet: map[string]struct{}{},
				affectedNodeSet:  map[string]struct{}{},
			}

			c.planRecreatedApplyNodes(plan)

			if _, exists := plan.recreatedNodeSet["sros"]; !exists {
				t.Fatal("expected sros to be planned for recreate")
			}
			if _, exists := plan.addedNodeSet["sros"]; !exists {
				t.Fatal("expected sros to be treated as a deploy target")
			}
		})
	}
}

func TestPlanRecreatedApplyNodesFromRuntimeLabelDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cfg           *clabtypes.NodeConfig
		runtimeNode   *applyRuntimeNode
		wantReasonKey string
	}{
		{
			name: "long name drift",
			cfg: &clabtypes.NodeConfig{
				ShortName: "n1",
				LongName:  "clab-lab-n1",
				Kind:      "linux",
			},
			runtimeNode: &applyRuntimeNode{
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{
						clabconstants.LongName:  "clab-old-n1",
						clabconstants.NodeKind:  "linux",
						clabconstants.NodeType:  "",
						clabconstants.NodeGroup: "",
					}},
				},
			},
			wantReasonKey: clabconstants.LongName,
		},
		{
			name: "kind drift",
			cfg: &clabtypes.NodeConfig{
				ShortName: "n1",
				LongName:  "clab-lab-n1",
				Kind:      "linux",
			},
			runtimeNode: &applyRuntimeNode{
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{
						clabconstants.LongName:  "clab-lab-n1",
						clabconstants.NodeKind:  "nokia_srlinux",
						clabconstants.NodeType:  "",
						clabconstants.NodeGroup: "",
					}},
				},
			},
			wantReasonKey: clabconstants.NodeKind,
		},
		{
			name: "type drift",
			cfg: &clabtypes.NodeConfig{
				ShortName: "n1",
				LongName:  "clab-lab-n1",
				Kind:      "linux",
				NodeType:  "new-type",
			},
			runtimeNode: &applyRuntimeNode{
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{
						clabconstants.LongName:  "clab-lab-n1",
						clabconstants.NodeKind:  "linux",
						clabconstants.NodeType:  "old-type",
						clabconstants.NodeGroup: "",
					}},
				},
			},
			wantReasonKey: clabconstants.NodeType,
		},
		{
			name: "group drift",
			cfg: &clabtypes.NodeConfig{
				ShortName: "n1",
				LongName:  "clab-lab-n1",
				Kind:      "linux",
				Group:     "new-group",
			},
			runtimeNode: &applyRuntimeNode{
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{
						clabconstants.LongName:  "clab-lab-n1",
						clabconstants.NodeKind:  "linux",
						clabconstants.NodeType:  "",
						clabconstants.NodeGroup: "old-group",
					}},
				},
			},
			wantReasonKey: clabconstants.NodeGroup,
		},
		{
			name: "distributed root long name drift",
			cfg: &clabtypes.NodeConfig{
				ShortName: "sros",
				LongName:  "clab-lab-sros",
				Kind:      "nokia_srsim",
				Components: []*clabtypes.Component{
					{Slot: "A"},
					{Slot: "1"},
				},
			},
			runtimeNode: &applyRuntimeNode{
				distributed: true,
				containers: []clabruntime.GenericContainer{
					{Labels: map[string]string{
						clabconstants.NodeName:         "sros-a",
						clabconstants.RootNodeLongName: "clab-old-sros",
						clabconstants.NodeKind:         "nokia_srsim",
						clabconstants.NodeType:         "",
						clabconstants.NodeGroup:        "",
					}},
					{Labels: map[string]string{
						clabconstants.NodeName:         "sros-1",
						clabconstants.RootNodeLongName: "clab-old-sros",
						clabconstants.NodeKind:         "nokia_srsim",
						clabconstants.NodeType:         "",
						clabconstants.NodeGroup:        "",
					}},
				},
			},
			wantReasonKey: clabconstants.RootNodeLongName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			c := &CLab{
				Nodes: map[string]clabnodes.Node{
					tt.cfg.ShortName: applyMockNodeWithConfig(ctrl, tt.cfg),
				},
			}
			plan := &applyPlan{
				currentNodes: map[string]*applyRuntimeNode{
					tt.cfg.ShortName: tt.runtimeNode,
				},
				addedNodeSet:     map[string]struct{}{},
				recreatedNodeSet: map[string]struct{}{},
				affectedNodeSet:  map[string]struct{}{},
			}

			reasons := strings.Join(
				applyRecreateReasons(c.Nodes[tt.cfg.ShortName], tt.runtimeNode),
				";",
			)
			if !strings.Contains(reasons, tt.wantReasonKey) {
				t.Fatalf("expected recreate reason to mention %q, got %q", tt.wantReasonKey, reasons)
			}

			c.planRecreatedApplyNodes(plan)

			if _, exists := plan.recreatedNodeSet[tt.cfg.ShortName]; !exists {
				t.Fatalf("expected %s to be planned for recreate", tt.cfg.ShortName)
			}
			if _, exists := plan.addedNodeSet[tt.cfg.ShortName]; !exists {
				t.Fatalf("expected %s to be treated as a deploy target", tt.cfg.ShortName)
			}
		})
	}
}

func TestPlanRecreatedApplyNodesFromRuntimeConfigDrift(t *testing.T) {
	t.Parallel()

	baseCfg := &clabtypes.NodeConfig{
		ShortName:     "n1",
		LongName:      "clab-lab-n1",
		Kind:          "linux",
		Image:         "alpine:latest",
		User:          "1000",
		Entrypoint:    "/bin/sh -c",
		Cmd:           "sleep 10",
		Env:           map[string]string{"FOO": "bar"},
		Labels:        map[string]string{"custom": "desired"},
		Binds:         []string{"/tmp/src:/data:ro"},
		PortSet:       nat.PortSet{nat.Port("80/tcp"): struct{}{}},
		PortBindings:  nat.PortMap{nat.Port("80/tcp"): []nat.PortBinding{{HostPort: "8080"}}},
		NetworkMode:   "",
		PidMode:       "host",
		MacAddress:    "aa:bb:cc:dd:ee:ff",
		Aliases:       []string{"n1-alias"},
		ExtraHosts:    []string{"peer:192.0.2.1"},
		Sysctls:       map[string]string{"net.ipv6.conf.all.disable_ipv6": "0"},
		DNS:           &clabtypes.DNSConfig{Servers: []string{"1.1.1.1"}},
		CapAdd:        []string{"NET_ADMIN"},
		Devices:       []string{"/dev/net/tun"},
		Tmpfs:         map[string]string{"/run": "rw"},
		ShmSize:       "64M",
		CPU:           0.5,
		CPUSet:        "0",
		Memory:        "128M",
		RestartPolicy: "always",
		Healthcheck: &clabtypes.HealthcheckConfig{
			Test:     []string{"CMD", "true"},
			Interval: 1,
			Timeout:  2,
			Retries:  3,
		},
	}

	tests := []struct {
		name       string
		field      string
		mutate     func(*clabruntime.GenericContainerConfig)
		wantReason string
	}{
		{
			name:  "image",
			field: applyRuntimeConfigFieldImage,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.Image = "busybox:latest"
			},
		},
		{
			name:  "env",
			field: applyRuntimeConfigFieldEnv,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.Env["FOO"] = "old"
			},
		},
		{
			name:  "binds",
			field: applyRuntimeConfigFieldBinds,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.Binds = []string{"/tmp/other:/data:ro"}
			},
		},
		{
			name:  "cmd",
			field: applyRuntimeConfigFieldCmd,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.Cmd = []string{"sleep", "1"}
			},
		},
		{
			name:  "ports",
			field: applyRuntimeConfigFieldPortBindings,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.PortBindings = nil
			},
		},
		{
			name:  "memory",
			field: applyRuntimeConfigFieldMemory,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.Memory = parseApplyByteSize("64M")
			},
		},
		{
			name:  "shm-size",
			field: applyRuntimeConfigFieldShmSize,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.ShmSize = parseApplyByteSize("32M")
			},
		},
		{
			name:  "healthcheck",
			field: applyRuntimeConfigFieldHealthcheck,
			mutate: func(cfg *clabruntime.GenericContainerConfig) {
				cfg.Healthcheck.Timeout = 9
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			runtimeConfig := applyRuntimeConfigFromDesired(baseCfg)
			tt.mutate(&runtimeConfig)
			runtimeNode := &applyRuntimeNode{
				containers: []clabruntime.GenericContainer{
					{
						Names:   []string{baseCfg.LongName},
						ID:      "1234567890ab",
						ShortID: "1234567890ab",
						Labels: map[string]string{
							clabconstants.LongName:  baseCfg.LongName,
							clabconstants.NodeKind:  baseCfg.Kind,
							clabconstants.NodeType:  baseCfg.NodeType,
							clabconstants.NodeGroup: baseCfg.Group,
						},
						Config: runtimeConfig,
					},
				},
			}
			c := &CLab{
				Nodes: map[string]clabnodes.Node{
					baseCfg.ShortName: applyMockNodeWithConfig(ctrl, baseCfg),
				},
			}
			plan := &applyPlan{
				currentNodes: map[string]*applyRuntimeNode{
					baseCfg.ShortName: runtimeNode,
				},
				addedNodeSet:     map[string]struct{}{},
				recreatedNodeSet: map[string]struct{}{},
				affectedNodeSet:  map[string]struct{}{},
			}

			reasons := strings.Join(
				applyRecreateReasons(c.Nodes[baseCfg.ShortName], runtimeNode),
				";",
			)
			if !strings.Contains(reasons, tt.field) {
				t.Fatalf("expected recreate reason to mention %q, got %q", tt.field, reasons)
			}

			c.planRecreatedApplyNodes(plan)

			if _, exists := plan.recreatedNodeSet[baseCfg.ShortName]; !exists {
				t.Fatalf("expected %s to be planned for recreate", baseCfg.ShortName)
			}
			if _, exists := plan.addedNodeSet[baseCfg.ShortName]; !exists {
				t.Fatalf("expected %s to be treated as a deploy target", baseCfg.ShortName)
			}
		})
	}
}

func TestApplyRuntimeConfigDriftSkipsMatchingRuntimeConfig(t *testing.T) {
	t.Parallel()

	cfg := &clabtypes.NodeConfig{
		ShortName: "n1",
		LongName:  "clab-lab-n1",
		Kind:      "linux",
		Image:     "alpine:latest",
		Env:       map[string]string{"FOO": "bar"},
		Labels:    map[string]string{"custom": "desired"},
		Binds:     []string{"/tmp/src:/data:ro"},
		Sysctls:   map[string]string{"net.ipv6.conf.all.disable_ipv6": "0"},
	}

	runtimeConfig := applyRuntimeConfigFromDesired(cfg)
	runtimeConfig.Env[clabconstants.ClabEnvIntfs] = "2"
	runtimeConfig.Env["CLAB_LABEL_CLAB_NODE_NAME"] = cfg.ShortName
	runtimeConfig.Env["RESTORE_SNAPSHOT"] = "1"
	runtimeConfig.Env["no_proxy"] = "localhost,n1,n2,added-node"
	runtimeConfig.Env["NO_PROXY"] = "localhost,n1,n2,added-node"
	runtimeConfig.Env["PATH"] = "/usr/bin"
	runtimeConfig.Labels[clabconstants.GitHash] = "old"
	runtimeConfig.Labels["image-label"] = "runtime-only"
	runtimeConfig.Binds = append(runtimeConfig.Binds,
		"/tmp/clab/n1/yum.repo:/etc/yum.repos.d/srlinux.repo:ro",
		"/tmp/clab/n1/apt.list:/etc/apt/sources.list.d/srlinux.list:ro",
	)
	runtimeConfig.ShmSize = parseApplyByteSize("64M")

	reasons := applyRuntimeConfigDrifts(cfg, clabruntime.GenericContainer{
		Names:   []string{cfg.LongName},
		ID:      "1234567890ab",
		ShortID: "1234567890ab",
		Config:  runtimeConfig,
	})
	if len(reasons) != 0 {
		t.Fatalf("expected matching runtime config, got recreate reasons %v", reasons)
	}
}

func TestApplyRuntimeConfigDriftSkipsUncomparableRuntimeFields(t *testing.T) {
	t.Parallel()

	cfg := &clabtypes.NodeConfig{
		ShortName: "n1",
		LongName:  "clab-lab-n1",
		Kind:      "linux",
		Sysctls:   map[string]string{"net.ipv6.conf.all.disable_ipv6": "0"},
	}
	runtimeConfig := applyRuntimeConfigFromDesired(cfg)
	runtimeConfig.Sysctls = nil
	runtimeConfig.UncomparableFields = map[string]struct{}{
		applyRuntimeConfigFieldSysctls: {},
	}

	reasons := applyRuntimeConfigDrifts(cfg, clabruntime.GenericContainer{Config: runtimeConfig})
	if len(reasons) != 0 {
		t.Fatalf("expected uncomparable runtime field to be skipped, got %v", reasons)
	}
}

func TestPlanRecreatedApplyNodesSkipsMatchingRuntime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	cfg := &clabtypes.NodeConfig{
		ShortName: "n1",
		LongName:  "clab-lab-n1",
		Kind:      "linux",
	}
	runtimeNode := &applyRuntimeNode{
		containers: []clabruntime.GenericContainer{
			{Labels: map[string]string{
				clabconstants.LongName:  "clab-lab-n1",
				clabconstants.NodeKind:  "linux",
				clabconstants.NodeType:  "",
				clabconstants.NodeGroup: "",
			}},
		},
	}
	c := &CLab{
		Nodes: map[string]clabnodes.Node{
			"n1": applyMockNodeWithConfig(ctrl, cfg),
		},
	}
	plan := &applyPlan{
		currentNodes:     map[string]*applyRuntimeNode{"n1": runtimeNode},
		addedNodeSet:     map[string]struct{}{},
		recreatedNodeSet: map[string]struct{}{},
		affectedNodeSet:  map[string]struct{}{},
	}

	c.planRecreatedApplyNodes(plan)

	if len(plan.recreatedNodeSet) != 0 {
		t.Fatalf("expected no recreated nodes, got %v", sortedStringSet(plan.recreatedNodeSet))
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
