package nvidia_cumulusvx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func testNode(t *testing.T, layout *KindSpecificConfig, labDir string) *nvidiaCumulusVX {
	t.Helper()
	if labDir == "" {
		labDir = filepath.Join(t.TempDir(), "leaf")
	}
	cfg := &clabtypes.NodeConfig{ShortName: "leaf", LabDir: labDir}
	if layout != nil {
		cfg.KindSpecificConfig = layout
	}
	n := new(nvidiaCumulusVX)
	if err := n.Init(cfg, clabnodes.WithMgmtNet(nil)); err != nil {
		t.Fatal(err)
	}
	return n
}

func addInterface(t *testing.T, n *nvidiaCumulusVX, name string) clablinks.Endpoint {
	t.Helper()
	ep := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n, name, nil))
	if err := n.AddEndpoint(ep); err != nil {
		t.Fatal(err)
	}
	return ep
}

func TestPortLayoutValidation(t *testing.T) {
	tests := map[string]struct {
		cfg  KindSpecificConfig
		want string
	}{
		"missing ports": {
			KindSpecificConfig{Breakouts: []Breakout{{Port: "1", Channels: 4}}},
			"port-count must be between",
		},
		"negative ports": {KindSpecificConfig{PortCount: -1}, "port-count must be between"},
		"too many ports": {
			KindSpecificConfig{PortCount: 1000},
			"port-count must be between",
		},
		"missing breakouts": {
			KindSpecificConfig{PortCount: 64},
			"must define at least one breakout",
		},
		"zero parent": {
			KindSpecificConfig{
				PortCount: 64,
				Breakouts: []Breakout{{Port: "0", Channels: 4}},
			},
			"ascending range within 1..64",
		},
		"parent beyond base": {
			KindSpecificConfig{
				PortCount: 64,
				Breakouts: []Breakout{{Port: "65", Channels: 4}},
			},
			"ascending range within 1..64",
		},
		"invalid width": {
			KindSpecificConfig{
				PortCount: 64,
				Breakouts: []Breakout{{Port: "10", Channels: 3}},
			},
			"must have 2, 4, or 8 channels",
		},
		"one lane": {
			KindSpecificConfig{
				PortCount: 64,
				Breakouts: []Breakout{{Port: "10", Channels: 1}},
			},
			"must have 2, 4, or 8 channels",
		},
		"missing channels": {
			KindSpecificConfig{
				PortCount: 64,
				Breakouts: []Breakout{{Port: "10"}},
			},
			"must have 2, 4, or 8 channels",
		},
		"duplicate parent": {
			KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
				{Port: "10", Channels: 4}, {Port: "10", Channels: 4},
			}},
			"port 10 is configured more than once",
		},
		"overlapping ranges": {
			KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
				{Port: "1..10", Channels: 4}, {Port: "10..20", Channels: 2},
			}},
			"port 10 is configured more than once",
		},
		"parent overlaps range": {
			KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
				{Port: "10", Channels: 4}, {Port: "1..20", Channels: 4},
			}},
			"port 10 is configured more than once",
		},
		"indices exceed tc limit": {
			KindSpecificConfig{
				PortCount: 996,
				Breakouts: []Breakout{{Port: "10", Channels: 4}},
			},
			"exceed vrnetlab's",
		},
		"range indices exceed tc limit": {
			KindSpecificConfig{
				PortCount: 125,
				Breakouts: []Breakout{{Port: "1..110", Channels: 8}},
			},
			"exceed vrnetlab's",
		},
	}
	for _, port := range []string{
		"", "-1", "01", "+1", " 1", "1 ", "swp1", "1.2", "1...2", "1..2..3",
		"..10", "1..", "20..1", "0..10", "1..65", "65..66", "1..999999999999999999999999",
		"999999999999999999999999", "1..02", "1..+2", "1.. 2", "1..-2",
	} {
		tests["invalid port "+port] = struct {
			cfg  KindSpecificConfig
			want string
		}{
			KindSpecificConfig{
				PortCount: 64,
				Breakouts: []Breakout{{Port: port, Channels: 4}},
			},
			"ascending range within 1..64",
		}
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := new(nvidiaCumulusVX)
			err := n.Init(&clabtypes.NodeConfig{
				ShortName:          "leaf",
				KindSpecificConfig: &tc.cfg,
			}, clabnodes.WithMgmtNet(nil))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Init error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBreakoutInterfaceMapping(t *testing.T) {
	n := testNode(t, &KindSpecificConfig{
		PortCount: 64,
		Breakouts: []Breakout{
			{Port: "64", Channels: 8}, {Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
		},
	}, "")
	// Connect sparse lanes in a different order from port allocation. Earlier
	// parents and unconnected lanes must still reserve their NIC indices.
	for _, tc := range []struct{ alias, mapped string }{
		{"swp64s7", "eth82"},
		{"swp11s3", "eth74"},
		{"swp10s3", "eth70"},
		{"swp2s1", "eth66"},
		{"swp10s0", "eth67"},
		{"swp2s0", "eth65"},
		{"swp63", "eth63"},
	} {
		ep := addInterface(t, n, tc.alias)
		if ep.GetIfaceName() != tc.mapped || ep.GetIfaceAlias() != tc.alias {
			t.Errorf(
				"%s mapped to %s (alias %s), want %s",
				tc.alias,
				ep.GetIfaceName(),
				ep.GetIfaceAlias(),
				tc.mapped,
			)
		}
	}
	if err := n.CheckInterfaceName(); err != nil {
		t.Fatal(err)
	}
}

func TestBreakoutPortRanges(t *testing.T) {
	for _, tc := range []struct {
		port        string
		ports       int
		channels    int
		first, last int
	}{
		{port: "1..20", ports: 64, channels: 4, first: 1, last: 20},
		{port: "10..10", ports: 64, channels: 2, first: 10, last: 10},
		{port: "1..64", ports: 64, channels: 8, first: 1, last: 64},
		{port: "991", ports: 991, channels: 8, first: 991, last: 991},
	} {
		t.Run(tc.port, func(t *testing.T) {
			n := testNode(t, &KindSpecificConfig{
				PortCount: tc.ports,
				Breakouts: []Breakout{
					{Port: tc.port, Channels: tc.channels},
				},
			}, "")
			if got, want := len(n.portLayout.breakouts), tc.last-tc.first+1; got != want {
				t.Fatalf("breakout count = %d, want %d", got, want)
			}
			for port := tc.first; port <= tc.last; port++ {
				for lane := range tc.channels {
					alias := fmt.Sprintf("swp%ds%d", port, lane)
					got, err := n.CalculateInterfaceIndex(alias)
					want := tc.ports + (port-tc.first)*tc.channels + lane + 1
					if err != nil || got != want {
						t.Fatalf("%s maps to %d, error %v; want %d", alias, got, err, want)
					}
				}
			}
		})
	}
}

func TestInterfaceValidation(t *testing.T) {
	layout := &KindSpecificConfig{
		PortCount: 64,
		Breakouts: []Breakout{{Port: "10", Channels: 4}},
	}
	for _, name := range []string{
		"swp0", "eth0", "swp10", "eth10", "swp65", "eth69",
		"swp10s4", "swp11s0", "swp10s-1", "swp10s", "swp10s0junk",
		"junk-swp1", "junketh1", "swp999999999999999999999999",
		"swp10s999999999999999999999999",
	} {
		t.Run(name, func(t *testing.T) {
			n := testNode(t, layout, "")
			ep := clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n, name, nil))
			err := n.AddEndpoint(ep)
			if err == nil {
				err = n.CheckInterfaceName()
			}
			if err == nil {
				t.Fatalf("accepted invalid interface %q", name)
			}
		})
	}

	t.Run("alias overlaps direct eth name", func(t *testing.T) {
		n := testNode(t, layout, "")
		addInterface(t, n, "swp10s0")
		addInterface(t, n, "eth65")
		if err := n.CheckInterfaceName(); err == nil ||
			!strings.Contains(err.Error(), "overlapping") {
			t.Fatalf("CheckInterfaceName error = %v, want overlap", err)
		}
	})
}

func TestInterfacesWithoutPortLayout(t *testing.T) {
	n := testNode(t, nil, "")
	for _, name := range []string{"swp1", "swp64", "eth3"} {
		addInterface(t, n, name)
	}
	if err := n.CheckInterfaceName(); err != nil {
		t.Fatal(err)
	}
	_, err := n.GetMappedInterfaceName("swp1s0")
	if err == nil || !strings.Contains(err.Error(), "requires a breakout") {
		t.Fatalf("breakout without layout: %v", err)
	}
}

func TestPreDeployPortsConfig(t *testing.T) {
	n := testNode(t, &KindSpecificConfig{
		PortCount: 6, Breakouts: []Breakout{
			{Port: "4..5", Channels: 2}, {Port: "1", Channels: 4},
		},
	}, "")
	params := &clabnodes.PreDeployParams{}
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(n.Cfg.LabDir, configDirName, portsConfigName)
	checkFile := func(want string) {
		t.Helper()
		got, err := os.ReadFile(filename)
		if err != nil || string(got) != want {
			t.Fatalf("ports.conf = %q, error %v; want %q", got, err, want)
		}
	}
	checkFile(portsConfigHeader + "1=4x\n2=1x\n3=1x\n4=2x\n5=2x\n6=1x\n")
	if !slices.Contains(n.Cfg.Binds, filepath.Join(n.Cfg.LabDir, configDirName)+":/config") {
		t.Fatal("generated directory is not mounted at /config")
	}
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatalf("repeated predeploy: %v", err)
	}

	// Topology changes regenerate ports.conf, independent of startup-config flags.
	n = testNode(
		t,
		&KindSpecificConfig{
			PortCount: 6,
			Breakouts: []Breakout{{Port: "2", Channels: 8}},
		},
		n.Cfg.LabDir,
	)
	n.Cfg.SuppressStartupConfig = true
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	checkFile(portsConfigHeader + "1=1x\n2=8x\n3=1x\n4=1x\n5=1x\n6=1x\n")

	n = testNode(t, nil, n.Cfg.LabDir)
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	checkFile(portsConfigHeader + "1=1x\n2=8x\n3=1x\n4=1x\n5=1x\n6=1x\n")
}

func TestPortsConfigWithoutLayout(t *testing.T) {
	n := testNode(t, nil, "")
	params := &clabnodes.PreDeployParams{}
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(n.Cfg.LabDir, configDirName, portsConfigName)
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("ports.conf should not be created without a layout: %v", err)
	}
	const manual = "1=4x\n64=1x\n"
	if err := os.WriteFile(filename, []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filename)
	if err != nil || string(got) != manual {
		t.Fatalf("manual file changed: %q, %v", got, err)
	}
}

func TestPreDeployPortsConfigErrors(t *testing.T) {
	layout := &KindSpecificConfig{
		PortCount: 6,
		Breakouts: []Breakout{{Port: "1", Channels: 4}},
	}

	t.Run("missing startup config preserves ports config", func(t *testing.T) {
		n := testNode(t, layout, "")
		filename := filepath.Join(n.Cfg.LabDir, configDirName, portsConfigName)
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		const previous = "1=2x\n2=1x\n"
		if err := os.WriteFile(filename, []byte(previous), 0o644); err != nil {
			t.Fatal(err)
		}
		n.Cfg.StartupConfig = filepath.Join(t.TempDir(), "missing.cfg")

		err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{})
		var pathErr *os.PathError
		if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathErr) ||
			pathErr.Path != n.Cfg.StartupConfig {
			t.Fatalf(
				"PreDeploy error = %v, want missing startup config %q",
				err,
				n.Cfg.StartupConfig,
			)
		}
		got, err := os.ReadFile(filename)
		if err != nil || string(got) != previous {
			t.Fatalf("ports.conf changed after startup config failure: %q, %v", got, err)
		}
	})

	t.Run("ports config write failure", func(t *testing.T) {
		n := testNode(t, layout, "")
		filename := filepath.Join(n.Cfg.LabDir, configDirName, portsConfigName)
		// A directory at the output path rejects writes even when tests run as root.
		if err := os.MkdirAll(filename, 0o755); err != nil {
			t.Fatal(err)
		}

		err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{})
		var pathErr *os.PathError
		if err == nil || !strings.Contains(err.Error(), "writing ports.conf") ||
			!errors.As(err, &pathErr) || pathErr.Path != filename {
			t.Fatalf("PreDeploy error = %v, want wrapped write error for %q", err, filename)
		}
	})
}

func TestPortLayoutDiffWithoutConfig(t *testing.T) {
	n := testNode(t, nil, "")
	withLayout := &clabtypes.NodeConfig{KindSpecificConfig: &KindSpecificConfig{
		PortCount: 6,
		Breakouts: []Breakout{{Port: "1", Channels: 4}},
	}}
	for _, tc := range []struct {
		name           string
		oldCfg, newCfg *clabtypes.NodeConfig
		wantFields     []string
	}{
		{name: "both configs missing"},
		{name: "old config missing", newCfg: withLayout},
		{name: "new config missing", oldCfg: withLayout},
		{
			name:   "both layouts missing",
			oldCfg: &clabtypes.NodeConfig{},
			newCfg: &clabtypes.NodeConfig{},
		},
		{
			name:   "empty kind-specific config",
			oldCfg: &clabtypes.NodeConfig{},
			newCfg: &clabtypes.NodeConfig{KindSpecificConfig: &KindSpecificConfig{}},
		},
		{
			name:       "image change without layout",
			oldCfg:     &clabtypes.NodeConfig{Image: "cumulus:old"},
			newCfg:     &clabtypes.NodeConfig{Image: "cumulus:new"},
			wantFields: []string{"Image"},
		},
		{
			name:       "layout added",
			oldCfg:     &clabtypes.NodeConfig{},
			newCfg:     withLayout,
			wantFields: []string{portLayoutDiffField},
		},
		{
			name: "undecodable old config",
			oldCfg: &clabtypes.NodeConfig{
				KindSpecificConfig: clabnodes.InvalidKindSpecificConfig{Err: "unknown key"},
			},
			newCfg:     &clabtypes.NodeConfig{},
			wantFields: []string{kindConfigDiffField},
		},
		{
			name: "undecodable old config with new layout",
			oldCfg: &clabtypes.NodeConfig{
				KindSpecificConfig: clabnodes.InvalidKindSpecificConfig{Err: "unknown key"},
			},
			newCfg:     withLayout,
			wantFields: []string{kindConfigDiffField},
		},
		{
			name: "old config with invalid breakout channels",
			oldCfg: &clabtypes.NodeConfig{
				KindSpecificConfig: &KindSpecificConfig{
					PortCount: 4,
					Breakouts: []Breakout{{Port: "1", Channels: 3}},
				},
			},
			newCfg:     withLayout,
			wantFields: []string{kindConfigDiffField},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diff := n.ComputeDiff(tc.oldCfg, tc.newCfg)
			if !slices.Equal(diff.Fields, tc.wantFields) {
				t.Fatalf("diff fields = %v, want %v", diff.Fields, tc.wantFields)
			}
		})
	}
}

func TestPortLayoutDiff(t *testing.T) {
	base := &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
		{Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
	}}
	n := testNode(t, base, "")
	for _, tc := range []struct {
		name     string
		layout   *KindSpecificConfig
		wantDiff bool
		// wantPlanErr expects GetReconcilePlan to demand a fresh guest disk.
		// Layout-invalid configs keep the generic kind config drift field and
		// reconcile with the default action instead.
		wantPlanErr bool
	}{
		{"unchanged", &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
			{Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
		}}, false, false},
		{"reordered", &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
			{Port: "2", Channels: 2}, {Port: "10..11", Channels: 4},
		}}, false, false},
		{"expanded range", &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
			{Port: "11", Channels: 4}, {Port: "2..2", Channels: 2}, {Port: "10", Channels: 4},
		}}, false, false},
		{"width changed", &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
			{Port: "10..11", Channels: 8}, {Port: "2", Channels: 2},
		}}, true, true},
		{"range changed", &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
			{Port: "10..12", Channels: 4}, {Port: "2", Channels: 2},
		}}, true, true},
		{"base changed", &KindSpecificConfig{PortCount: 32, Breakouts: []Breakout{
			{Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
		}}, true, true},
		{"invalid layout", &KindSpecificConfig{PortCount: 64, Breakouts: []Breakout{
			{Port: "10..11", Channels: 4}, {Port: "11", Channels: 2},
		}}, true, false},
		{"removed", nil, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldCfg := &clabtypes.NodeConfig{KindSpecificConfig: base}
			newCfg := &clabtypes.NodeConfig{}
			if tc.layout != nil {
				newCfg.KindSpecificConfig = tc.layout
			}
			diff := n.ComputeDiff(oldCfg, newCfg)
			if diff.HasDiff() != tc.wantDiff {
				t.Fatalf("diff = %+v, want changed=%v", diff, tc.wantDiff)
			}
			if tc.wantDiff && diff.DefaultAction() != clabtypes.TopologyDiffActionRecreate {
				t.Fatalf("action = %s, want recreate", diff.DefaultAction())
			}
			_, err := n.GetReconcilePlan(context.Background(), diff)
			if tc.wantPlanErr {
				if err == nil || !strings.Contains(err.Error(), "--reconfigure") {
					t.Fatalf("layout change should require a fresh guest disk: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
