package nvidia_cumulusvx

import (
	"context"
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
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{0: 4}},
			"outside the base port range",
		},
		"parent beyond base": {
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{65: 4}},
			"outside the base port range",
		},
		"invalid width": {
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{10: 3}},
			"must have 2, 4, or 8 lanes",
		},
		"one lane": {
			clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{10: 1}},
			"must have 2, 4, or 8 lanes",
		},
		"indices exceed tc limit": {
			clabtypes.CumulusVXExtras{Ports: 996, Breakouts: map[int]int{10: 4}},
			"exceed vrnetlab's",
		},
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
		Ports:     64,
		Breakouts: map[int]int{64: 8, 10: 4, 2: 2},
	}, "")
	// Connect sparse lanes in a different order from port allocation. Earlier
	// parents and unconnected lanes must still reserve their NIC indices.
	for _, tc := range []struct{ alias, mapped string }{
		{"swp64s7", "eth78"},
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

func TestInterfaceValidation(t *testing.T) {
	layout := &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{10: 4}}
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
		Ports: 6, Breakouts: map[int]int{4: 2, 1: 4},
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
	checkFile(portsConfigHeader + "1=4x\n2=1x\n3=1x\n4=2x\n5=1x\n6=1x\n")
	if !slices.Contains(n.Cfg.Binds, filepath.Join(n.Cfg.LabDir, configDirName)+":/config") {
		t.Fatal("generated directory is not mounted at /config")
	}
	if err := n.PreDeploy(context.Background(), params); err != nil {
		t.Fatalf("repeated predeploy: %v", err)
	}

	// Topology changes regenerate ports.conf, independent of startup-config flags.
	n = testNode(
		t,
		&clabtypes.CumulusVXExtras{Ports: 6, Breakouts: map[int]int{2: 8}},
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
	base := &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{10: 4}}
	n := testNode(t, base, "")
	for _, tc := range []struct {
		name     string
		layout   *clabtypes.CumulusVXExtras
		wantDiff bool
	}{
		{"unchanged", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{10: 4}}, false},
		{"width changed", &clabtypes.CumulusVXExtras{Ports: 64, Breakouts: map[int]int{10: 8}}, true},
		{"base changed", &clabtypes.CumulusVXExtras{Ports: 32, Breakouts: map[int]int{10: 4}}, true},
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
