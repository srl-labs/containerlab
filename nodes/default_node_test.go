package nodes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	mockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestDefaultNodeEndpointOwnership(t *testing.T) {
	d := &DefaultNode{
		Cfg: &clabtypes.NodeConfig{
			ShortName: "node1",
		},
	}

	ep1 := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(d, "eth1", nil))
	ep2 := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(d, "eth2", nil))

	if err := d.AdoptEndpoint(ep1); err != nil {
		t.Fatalf("unexpected adopt error for ep1: %v", err)
	}
	if err := d.AdoptEndpoint(ep2); err != nil {
		t.Fatalf("unexpected adopt error for ep2: %v", err)
	}
	if err := d.ReleaseEndpoint(ep1); err != nil {
		t.Fatalf("unexpected release error for ep1: %v", err)
	}

	got := d.GetEndpoints()
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint after deregister, got %d", len(got))
	}

	if got[0] != ep2 {
		t.Fatalf("expected remaining endpoint to be ep2, got %#v", got[0])
	}
}

func TestDefaultNodeConfigChangesRecreate(t *testing.T) {
	tests := []struct {
		name string
		old  *clabtypes.NodeConfig
		new  *clabtypes.NodeConfig
	}{
		{
			name: "exec",
			old:  &clabtypes.NodeConfig{Exec: []string{"echo old"}},
			new:  &clabtypes.NodeConfig{Exec: []string{"echo new"}},
		},
		{
			name: "environment",
			old:  &clabtypes.NodeConfig{Env: map[string]string{"MODE": "old"}},
			new:  &clabtypes.NodeConfig{Env: map[string]string{"MODE": "new"}},
		},
		{
			name: "components",
			old:  &clabtypes.NodeConfig{},
			new:  &clabtypes.NodeConfig{Components: []*clabtypes.Component{{Slot: "1"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &DefaultNode{Cfg: &clabtypes.NodeConfig{LongName: "clab-lab-n1"}}
			plan := node.ComputeReconcilePlan(node.ComputeDiff(tt.old, tt.new))
			if plan.Action != clabtypes.TopologyDiffActionRecreate {
				t.Fatalf("action = %q, want %q", plan.Action, clabtypes.TopologyDiffActionRecreate)
			}
			if len(plan.Recreated) != 1 || plan.Recreated[0] != "clab-lab-n1" {
				t.Fatalf("recreated nodes = %v, want only clab-lab-n1", plan.Recreated)
			}
		})
	}
}

func TestDefaultNodeComputeDiffDetectsHostnameChange(t *testing.T) {
	d := &DefaultNode{}
	diff := d.ComputeDiff(
		&clabtypes.NodeConfig{ShortName: "node1"},
		&clabtypes.NodeConfig{ShortName: "node1", Hostname: "production-host"},
	)

	if len(diff.Fields) != 1 || diff.Fields[0] != "Hostname" {
		t.Fatalf("ComputeDiff fields = %#v, want [Hostname]", diff.Fields)
	}
	if got := diff.DefaultAction(); got != clabtypes.TopologyDiffActionRecreate {
		t.Fatalf("DefaultAction() = %q, want %q", got, clabtypes.TopologyDiffActionRecreate)
	}
}

func TestDefaultNodeComputeDiffIgnoresEquivalentDefaultHostname(t *testing.T) {
	d := &DefaultNode{}
	diff := d.ComputeDiff(
		&clabtypes.NodeConfig{ShortName: "node1"},
		&clabtypes.NodeConfig{ShortName: "node1", Hostname: "node1"},
	)

	if diff.HasDiff() {
		t.Fatalf("ComputeDiff fields = %#v, want no diff", diff.Fields)
	}
}

func TestDefaultNodeComputeDiffTreatsBindsAsUnordered(t *testing.T) {
	d := &DefaultNode{}
	oldCfg := &clabtypes.NodeConfig{Binds: []string{
		"/host/config:/etc/config:ro",
		"/host/data:/var/lib/data",
	}}

	t.Run("reordered binds", func(t *testing.T) {
		newCfg := &clabtypes.NodeConfig{Binds: []string{
			"/host/data:/var/lib/data",
			"/host/config:/etc/config:ro",
		}}

		if diff := d.ComputeDiff(oldCfg, newCfg); diff.HasDiff() {
			t.Fatalf("ComputeDiff fields = %#v, want no diff", diff.Fields)
		}
	})

	t.Run("changed bind", func(t *testing.T) {
		newCfg := &clabtypes.NodeConfig{Binds: []string{
			"/host/data:/var/lib/data",
			"/host/config:/etc/config",
		}}

		diff := d.ComputeDiff(oldCfg, newCfg)
		if len(diff.Fields) != 1 || diff.Fields[0] != "Binds" {
			t.Fatalf("ComputeDiff fields = %#v, want [Binds]", diff.Fields)
		}
	})
}

func TestDefaultNodeAdoptEndpointRejectsForeignOwner(t *testing.T) {
	d := &DefaultNode{
		Cfg: &clabtypes.NodeConfig{
			ShortName: "node1",
		},
	}
	other := &DefaultNode{
		Cfg: &clabtypes.NodeConfig{
			ShortName: "node2",
		},
	}

	ep := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(other, "eth1", nil))

	if err := d.AdoptEndpoint(ep); err == nil {
		t.Fatalf("expected adopt to reject endpoint owned by another node")
	}
}

func TestDefaultNodeShouldSkipLifecycle(t *testing.T) {
	tests := []struct {
		name string
		cfg  *clabtypes.NodeConfig
		want bool
	}{
		{
			name: "regular node participates",
			cfg: &clabtypes.NodeConfig{
				ShortName: "node1",
			},
			want: false,
		},
		{
			name: "root namespace node skips",
			cfg: &clabtypes.NodeConfig{
				ShortName:            "node1",
				IsRootNamespaceBased: true,
			},
			want: true,
		},
		{
			name: "auto-remove node skips",
			cfg: &clabtypes.NodeConfig{
				ShortName:  "node1",
				AutoRemove: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultNode{Cfg: tt.cfg}
			if got := d.ShouldSkipLifecycle(); got != tt.want {
				t.Fatalf("ShouldSkipLifecycle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultNodeLinkApplyMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  *clabtypes.NodeConfig
		want LinkApplyMode
	}{
		{name: "regular", want: LinkApplyModeRecreate},
		{name: "root namespace", cfg: &clabtypes.NodeConfig{IsRootNamespaceBased: true}, want: LinkApplyModeLive},
		{name: "external", cfg: &clabtypes.NodeConfig{SkipUniquenessCheck: true}, want: LinkApplyModeLive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultNode{Cfg: tt.cfg}
			if got := d.LinkApplyMode(context.Background()); got != tt.want {
				t.Fatalf("LinkApplyMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultNodeImageLinkApplyMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		labels     map[string]string
		inspectErr error
		fallback   LinkApplyMode
		want       LinkApplyMode
	}{
		{
			name:     "boxen image uses live apply",
			labels:   map[string]string{boxenImageVendorLabel: boxenImageVendorValue},
			fallback: LinkApplyModeRecreate,
			want:     LinkApplyModeLive,
		},
		{
			name:     "missing label keeps restart fallback",
			labels:   map[string]string{"other": "label"},
			fallback: LinkApplyModeRestart,
			want:     LinkApplyModeRestart,
		},
		{
			name:       "inspect error keeps recreate fallback",
			inspectErr: errors.New("inspect image"),
			fallback:   LinkApplyModeRecreate,
			want:       LinkApplyModeRecreate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			runtime := mockruntime.NewMockContainerRuntime(ctrl)
			if tt.inspectErr != nil {
				runtime.EXPECT().
					InspectImage(gomock.Any(), "img").
					Return(nil, tt.inspectErr)
			} else {
				runtime.EXPECT().
					InspectImage(gomock.Any(), "img").
					Return(&clabruntime.ImageInspect{
						Config: clabruntime.ImageConfig{Labels: tt.labels},
					}, nil)
			}

			d := &DefaultNode{
				Cfg:     &clabtypes.NodeConfig{Image: "img"},
				Runtime: runtime,
			}

			if got := d.ImageLinkApplyMode(context.Background(), tt.fallback); got != tt.want {
				t.Fatalf("ImageLinkApplyMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultNodeRuntimeEndpointInventory(t *testing.T) {
	d := &DefaultNode{
		Cfg: &clabtypes.NodeConfig{
			ShortName: "node1",
		},
	}

	topologyEp := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(d, "eth1", nil))
	runtimeEp := clablinks.NewRuntimeEndpoint(d, "eth99")

	if err := d.AdoptEndpoint(topologyEp); err != nil {
		t.Fatalf("unexpected adopt error for topology endpoint: %v", err)
	}
	if err := d.AdoptEndpoint(runtimeEp); err != nil {
		t.Fatalf("unexpected adopt error for runtime endpoint: %v", err)
	}

	got := d.GetEndpoints()
	if len(got) != 2 {
		t.Fatalf("expected 2 tracked endpoints, got %d", len(got))
	}

	if got[0] != topologyEp {
		t.Fatalf("expected first endpoint to be topology-backed, got %#v", got[0])
	}

	if got[1] != runtimeEp {
		t.Fatalf("expected second endpoint to be runtime-discovered, got %#v", got[1])
	}

	if got[0].IsRuntimeDiscovered() {
		t.Fatalf("topology endpoint should not be marked runtime-discovered")
	}

	if !got[1].IsRuntimeDiscovered() {
		t.Fatalf("runtime endpoint should be marked runtime-discovered")
	}

	if err := d.ReleaseEndpoint(runtimeEp); err != nil {
		t.Fatalf("unexpected release error for runtime endpoint: %v", err)
	}

	got = d.GetEndpoints()
	if len(got) != 1 {
		t.Fatalf("expected runtime endpoint to be removed, got %d tracked endpoints", len(got))
	}

	if got[0] != topologyEp {
		t.Fatalf("unexpected remaining endpoint %#v", got[0])
	}
}

func TestDefaultNodeRestoreEndpointsMissingParkingNetNSError(t *testing.T) {
	longName := fmt.Sprintf(
		"clab-%s-%d",
		strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-"),
		os.Getpid(),
	)

	d := &DefaultNode{
		Cfg: &clabtypes.NodeConfig{
			ShortName: "node1",
			LongName:  longName,
		},
	}

	// A node stopped outside of containerlab (or with no dataplane links) has no
	// parking netns; restoring should be a no-op rather than an error.
	if err := d.RestoreEndpoints(context.Background()); err != nil {
		t.Fatalf("expected no error when parking netns is missing, got %v", err)
	}
}

func TestDefaultNodeSharedNetNSNeverParksProviderEndpoints(t *testing.T) {
	t.Parallel()

	d := &DefaultNode{Cfg: &clabtypes.NodeConfig{
		ShortName:   "child",
		LongName:    "clab-test-child",
		NetworkMode: "container:provider",
	}}

	if err := d.ParkEndpoints(context.Background()); err != nil {
		t.Fatalf("shared-netns ParkEndpoints returned error: %v", err)
	}
	if err := d.RestoreEndpoints(context.Background()); err != nil {
		t.Fatalf("shared-netns RestoreEndpoints returned error: %v", err)
	}
	if _, err := d.parkingNetNSPath(); err == nil {
		t.Fatal("shared-netns child created a parking namespace")
	}
}

func TestGenerateConfigs(t *testing.T) {
	defCfg := "default config"
	oldCfg := "old config"
	newCfg := "new config"

	tests := map[string]struct {
		cfg        *clabtypes.NodeConfig
		err        error
		preExists  bool
		postExists bool
		template   string
		out        string
	}{
		"suppress-true-first-start": {
			cfg: &clabtypes.NodeConfig{
				SuppressStartupConfig: true,
			},
			err:        nil,
			preExists:  false,
			postExists: false,
			template:   defCfg,
		},
		"suppress-true-existing-lab": {
			cfg: &clabtypes.NodeConfig{
				SuppressStartupConfig: true,
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        oldCfg,
			template:   defCfg,
		},
		"suppress-false-first-start": {
			cfg: &clabtypes.NodeConfig{
				SuppressStartupConfig: false,
			},
			err:        nil,
			preExists:  false,
			postExists: true,
			out:        defCfg,
			template:   defCfg,
		},
		"suppress-false-existing-lab": {
			cfg: &clabtypes.NodeConfig{
				SuppressStartupConfig: false,
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        oldCfg,
			template:   defCfg,
		},
		"startup-config-first-start": {
			cfg: &clabtypes.NodeConfig{
				StartupConfig: "other",
			},
			err:        nil,
			preExists:  false,
			postExists: true,
			out:        newCfg,
			template:   newCfg,
		},
		"startup-config-existing-lab": {
			cfg: &clabtypes.NodeConfig{
				StartupConfig: "other",
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        oldCfg,
			template:   newCfg,
		},
		"enforce-startup-config-first-start": {
			cfg: &clabtypes.NodeConfig{
				StartupConfig:        "other",
				EnforceStartupConfig: true,
			},
			err:        nil,
			preExists:  false,
			postExists: true,
			out:        newCfg,
			template:   newCfg,
		},
		"enforce-startup-config-existing-lab": {
			cfg: &clabtypes.NodeConfig{
				StartupConfig:        "other",
				EnforceStartupConfig: true,
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        newCfg,
			template:   newCfg,
		},
		"enforce-startup-config-no-startup": {
			cfg: &clabtypes.NodeConfig{
				EnforceStartupConfig: true,
			},
			err: ErrNoStartupConfig,
		},
		"enforce-and-suppress-startup-config": {
			cfg: &clabtypes.NodeConfig{
				EnforceStartupConfig:  true,
				SuppressStartupConfig: true,
			},
			err: ErrIncompatibleOptions,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			node := &DefaultNode{Cfg: tc.cfg}
			dstFolder := tt.TempDir()
			dstFile := filepath.Join(dstFolder, "config")

			if tc.preExists {
				err := os.WriteFile(dstFile, []byte(oldCfg), 0o666)
				if err != nil {
					tt.Errorf("Could not write existing config: %v", err)
				}
			}

			err := node.GenerateConfig(dstFile, tc.template)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					tt.Errorf("got: %v, wanted: %v", err, tc.err)
				}
			}
			if tc.postExists {
				cnt, err := os.ReadFile(dstFile)
				if err != nil {
					tt.Fatal(err)
				}
				if string(cnt) != tc.out {
					tt.Errorf("got %v, wanted %v", string(cnt), tc.out)
				}
			} else {
				if _, err := os.Stat(dstFile); err == nil {
					tt.Errorf("config file created")
				}
			}
		})
	}
}

func TestInterfacesAliases(t *testing.T) { // skipcq: GO-R1005
	tests := map[string]struct {
		endpoints           []*clablinks.EndpointVeth
		node                *DefaultNode
		endpointErrContains string
		checkErrContains    string
		resultEps           []string
	}{
		"basic-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ge-0/0/0",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ge-0/0/2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ge-0/0/4",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "juniper",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth0", "eth2", "eth4",
			},
		},
		"parse-offset": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "Gi3",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet 5",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "cisco",
				},
				InterfaceRegexp:  regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset:  2,
				FirstDataIfIndex: 1,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth2", "eth4",
			},
		},
		"skip-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
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
			},
			node: &DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "cisco-original",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth2", "eth4",
			},
		},
		"overlap": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ge-0/0/1",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "juniper-overlap",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
			},
			endpointErrContains: "",
			checkErrContains:    "overlapping interface names",
			resultEps:           []string{},
		},
		"out-of-bounds-index": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "Gi1",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "cisco-oob",
				},
				InterfaceRegexp:  regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset:  2,
				FirstDataIfIndex: 1,
			},
			endpointErrContains: "0 ! >= 1",
			checkErrContains:    "",
			resultEps:           []string{},
		},
		"regexp-no-group": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "Gi2",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "cisco-noregexpgroup",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "'port' capture group missing",
			checkErrContains:    "",
			resultEps:           []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(*testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					if tc.endpointErrContains != "" && !(strings.Contains(
						fmt.Sprint(gotEndpointErr), tc.endpointErrContains)) {
						t.Errorf(
							"got error for endpoint %+v, want %s",
							gotEndpointErr,
							tc.endpointErrContains,
						)
					}
				}
			}

			if tc.endpointErrContains != "" && !foundError {
				t.Errorf("got no error for endpoints, want %s", tc.endpointErrContains)
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					if tc.checkErrContains != "" && !(strings.Contains(
						fmt.Sprint(gotCheckErr), tc.checkErrContains)) {
						t.Errorf(
							"got error for check %+v, want %s",
							gotCheckErr,
							tc.checkErrContains,
						)
					}
				}

				if tc.checkErrContains != "" && !foundError {
					t.Errorf("got no error for check interface, want %s", tc.checkErrContains)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							t.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}

type defaultNodeContainerNameOverride struct {
	DefaultNode
	containerName string
}

func (n *defaultNodeContainerNameOverride) GetContainerName() string {
	return n.containerName
}

func TestDefaultNodeNetnsStatusUsesOverwriteContainerName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(context.Context, *defaultNodeContainerNameOverride) error
	}{
		"add-link": {
			run: func(ctx context.Context, node *defaultNodeContainerNameOverride) error {
				return node.AddLinkToContainer(ctx, nil, nil)
			},
		},
		"exec-function": {
			run: func(ctx context.Context, node *defaultNodeContainerNameOverride) error {
				return node.ExecFunction(ctx, nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctrl := gomock.NewController(t)
			rt := mockruntime.NewMockContainerRuntime(ctrl)
			node := &defaultNodeContainerNameOverride{
				containerName: "clab-srsim-sros-a",
			}
			node.DefaultNode = *NewDefaultNode(node)
			node.Cfg = &clabtypes.NodeConfig{
				ShortName: "sros",
				LongName:  "clab-srsim-sros",
			}
			node.Runtime = rt

			rt.EXPECT().
				GetContainerStatus(ctx, "clab-srsim-sros-a").
				Return(clabruntime.NotFound)

			err := tc.run(ctx, node)
			if err == nil {
				t.Fatal("got nil error, want namespace availability error")
			}
			if !strings.Contains(err.Error(), "status=NotFound") {
				t.Fatalf("got error %q, want status=NotFound", err)
			}
		})
	}
}

func TestDefaultNodeGetContainerStatusUsesOverwriteContainerName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	rt := mockruntime.NewMockContainerRuntime(ctrl)
	node := &defaultNodeContainerNameOverride{
		containerName: "clab-srsim-sros-a",
	}
	node.DefaultNode = *NewDefaultNode(node)
	node.Runtime = rt

	rt.EXPECT().
		GetContainerStatus(ctx, "clab-srsim-sros-a").
		Return(clabruntime.Running)

	if got := node.GetContainerStatus(ctx); got != clabruntime.Running {
		t.Fatalf("got %q, want %q", got, clabruntime.Running)
	}
}
