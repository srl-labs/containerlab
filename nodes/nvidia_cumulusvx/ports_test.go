package nvidia_cumulusvx

import (
	"context"
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

func testNode(t *testing.T, layout *clabtypes.CumulusVXExtras, labDir string) *nvidiaCumulusVX {
	t.Helper()
	if labDir == "" {
		labDir = filepath.Join(t.TempDir(), "leaf")
	}
	cfg := &clabtypes.NodeConfig{ShortName: "leaf", LabDir: labDir}
	if layout != nil {
		cfg.Extras = &clabtypes.Extras{CumulusVX: layout}
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
		cfg  clabtypes.CumulusVXExtras
		want string
	}{
		"missing ports":  {clabtypes.CumulusVXExtras{}, "ports must be between"},
		"negative ports": {clabtypes.CumulusVXExtras{Ports: -1}, "ports must be between"},
		"too many ports": {
			clabtypes.CumulusVXExtras{Ports: 1000},
			"ports must be between",
		},
		"missing breakouts": {
			clabtypes.CumulusVXExtras{Ports: 64},
			"must define at least one breakout",
		},
		"zero parent": {
			clabtypes.CumulusVXExtras{
				Ports:     64,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "0", Channels: 4}},
			},
			"ascending range within 1..64",
		},
		"parent beyond base": {
			clabtypes.CumulusVXExtras{
				Ports:     64,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "65", Channels: 4}},
			},
			"ascending range within 1..64",
		},
		"invalid width": {
			clabtypes.CumulusVXExtras{
				Ports:     64,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "10", Channels: 3}},
			},
			"must have 2, 4, or 8 channels",
		},
		"one lane": {
			clabtypes.CumulusVXExtras{
				Ports:     64,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "10", Channels: 1}},
			},
			"must have 2, 4, or 8 channels",
		},
		"missing channels": {
			clabtypes.CumulusVXExtras{
				Ports:     64,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "10"}},
			},
			"must have 2, 4, or 8 channels",
		},
		"duplicate parent": {
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
				{Port: "10", Channels: 4}, {Port: "10", Channels: 4},
			}},
			"port 10 is configured more than once",
		},
		"overlapping ranges": {
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
				{Port: "1..10", Channels: 4}, {Port: "10..20", Channels: 2},
			}},
			"port 10 is configured more than once",
		},
		"parent overlaps range": {
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
				{Port: "10", Channels: 4}, {Port: "1..20", Channels: 4},
			}},
			"port 10 is configured more than once",
		},
		"indices exceed tc limit": {
			clabtypes.CumulusVXExtras{
				Ports:     996,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "10", Channels: 4}},
			},
			"exceed vrnetlab's",
		},
		"range indices exceed tc limit": {
			clabtypes.CumulusVXExtras{
				Ports:     125,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: "1..110", Channels: 8}},
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
			cfg  clabtypes.CumulusVXExtras
			want string
		}{
			clabtypes.CumulusVXExtras{
				Ports:     64,
				Breakouts: []clabtypes.CumulusVXBreakout{{Port: port, Channels: 4}},
			},
			"ascending range within 1..64",
		}
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := new(nvidiaCumulusVX)
			err := n.Init(&clabtypes.NodeConfig{
				ShortName: "leaf",
				Extras:    &clabtypes.Extras{CumulusVX: &tc.cfg},
			}, clabnodes.WithMgmtNet(nil))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Init error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBreakoutInterfaceMapping(t *testing.T) {
	n := testNode(t, &clabtypes.CumulusVXExtras{
		Ports: 64,
		Breakouts: []clabtypes.CumulusVXBreakout{
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
			n := testNode(t, &clabtypes.CumulusVXExtras{
				Ports: tc.ports,
				Breakouts: []clabtypes.CumulusVXBreakout{
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
	layout := &clabtypes.CumulusVXExtras{
		Ports:     64,
		Breakouts: []clabtypes.CumulusVXBreakout{{Port: "10", Channels: 4}},
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
	n := testNode(t, &clabtypes.CumulusVXExtras{
		Ports: 6, Breakouts: []clabtypes.CumulusVXBreakout{
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
		&clabtypes.CumulusVXExtras{
			Ports:     6,
			Breakouts: []clabtypes.CumulusVXBreakout{{Port: "2", Channels: 8}},
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

func TestPortLayoutDiff(t *testing.T) {
	base := &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
		{Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
	}}
	n := testNode(t, base, "")
	for _, tc := range []struct {
		name     string
		layout   *clabtypes.CumulusVXExtras
		wantDiff bool
	}{
		{"unchanged", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
		}}, false},
		{"reordered", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "2", Channels: 2}, {Port: "10..11", Channels: 4},
		}}, false},
		{"expanded range", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "11", Channels: 4}, {Port: "2..2", Channels: 2}, {Port: "10", Channels: 4},
		}}, false},
		{"width changed", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "10..11", Channels: 8}, {Port: "2", Channels: 2},
		}}, true},
		{"range changed", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "10..12", Channels: 4}, {Port: "2", Channels: 2},
		}}, true},
		{"base changed", &clabtypes.CumulusVXExtras{Ports: 32, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "10..11", Channels: 4}, {Port: "2", Channels: 2},
		}}, true},
		{"invalid layout", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: []clabtypes.CumulusVXBreakout{
			{Port: "10..11", Channels: 4}, {Port: "11", Channels: 2},
		}}, true},
		{"removed", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldCfg := &clabtypes.NodeConfig{Extras: &clabtypes.Extras{CumulusVX: base}}
			newCfg := &clabtypes.NodeConfig{Extras: &clabtypes.Extras{CumulusVX: tc.layout}}
			diff := n.ComputeDiff(oldCfg, newCfg)
			if diff.HasDiff() != tc.wantDiff {
				t.Fatalf("diff = %+v, want changed=%v", diff, tc.wantDiff)
			}
			if tc.wantDiff && diff.DefaultAction() != clabtypes.TopologyDiffActionRecreate {
				t.Fatalf("action = %s, want recreate", diff.DefaultAction())
			}
			_, err := n.GetReconcilePlan(context.Background(), diff)
			if tc.wantDiff {
				if err == nil || !strings.Contains(err.Error(), "--reconfigure") {
					t.Fatalf("layout change should require a fresh guest disk: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
