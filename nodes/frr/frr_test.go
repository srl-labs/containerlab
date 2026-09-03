// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// newTestNode returns an frr node rooted at a temporary lab directory.
func newTestNode(t *testing.T, cfg *clabtypes.NodeConfig) *frr {
	t.Helper()

	cfg.LabDir = filepath.Join(t.TempDir(), cfg.ShortName)
	if cfg.Sysctls == nil {
		cfg.Sysctls = map[string]string{}
	}

	n := new(frr)
	if err := n.Init(cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return n
}

func readConfigFile(t *testing.T, n *frr, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(n.Cfg.LabDir, cfgDir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return string(b)
}

// Init must mount all three config files, since the image ships neither
// frr.conf nor vtysh.conf and vtysh will not start without them.
func TestInitBindsAllThreeConfigFiles(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	for _, want := range []string{
		"/etc/frr/frr.conf",
		"/etc/frr/daemons",
		"/etc/frr/vtysh.conf",
	} {
		found := false

		for _, b := range n.Cfg.Binds {
			if strings.HasSuffix(b, ":"+want) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("no bind mount for %s, got %v", want, n.Cfg.Binds)
		}
	}
}

func TestInitEnablesForwarding(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	for k, want := range map[string]string{
		"net.ipv4.ip_forward":          "1",
		"net.ipv6.conf.all.forwarding": "1",
	} {
		if got := n.Cfg.Sysctls[k]; got != want {
			t.Errorf("sysctl %s = %q, want %q", k, got, want)
		}
	}
}

func TestCreateFRRFilesWithoutStartupConfig(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	if err := n.createFRRFiles(); err != nil {
		t.Fatalf("createFRRFiles: %v", err)
	}

	got := readConfigFile(t, n, vtyshConfFile)
	if !strings.Contains(got, "service integrated-vtysh-config") {
		t.Errorf("vtysh.conf = %q, want integrated config enabled", got)
	}

	// The bundled default, since no startup-config was given.
	got = readConfigFile(t, n, frrConfFile)
	if !strings.Contains(got, "frr defaults traditional") {
		t.Errorf("frr.conf = %q, want the bundled default", got)
	}

	// No daemon list means every daemon runs.
	daemons := readConfigFile(t, n, daemonsFile)
	for _, d := range configurableDaemons {
		if !strings.Contains(daemons, d+"=yes") {
			t.Errorf("daemon %s is not enabled by default", d)
		}
	}
}

func TestCreateFRRFilesUsesStartupConfig(t *testing.T) {
	startup := filepath.Join(t.TempDir(), "router1.conf")

	want := "router ospf\n network 10.0.0.0/8 area 0\n"
	if err := os.WriteFile(startup, []byte(want), 0o644); err != nil {
		t.Fatalf("writing startup config: %v", err)
	}

	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName:     "router1",
		StartupConfig: startup,
	})

	if err := n.createFRRFiles(); err != nil {
		t.Fatalf("createFRRFiles: %v", err)
	}

	if got := readConfigFile(t, n, frrConfFile); got != want {
		t.Errorf("frr.conf = %q, want %q", got, want)
	}
}

// The daemon list set in the topology must reach the rendered daemons file.
func TestCreateFRRFilesHonoursExtras(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName: "router1",
		Extras: &clabtypes.Extras{
			FRR: &clabtypes.FRRExtras{Daemons: []string{"ospfd", "bfdd"}},
		},
	})

	if err := n.createFRRFiles(); err != nil {
		t.Fatalf("createFRRFiles: %v", err)
	}

	daemons := readConfigFile(t, n, daemonsFile)

	for _, d := range []string{"ospfd", "bfdd"} {
		if !strings.Contains(daemons, d+"=yes") {
			t.Errorf("daemon %s should be enabled", d)
		}
	}

	for _, d := range []string{"bgpd", "isisd", "pimd"} {
		if !strings.Contains(daemons, d+"=no") {
			t.Errorf("daemon %s should be disabled", d)
		}
	}
}

// An unusable daemon name must fail the deploy rather than silently produce a
// node with the wrong daemons running.
func TestCreateFRRFilesRejectsUnknownDaemon(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName: "router1",
		Extras: &clabtypes.Extras{
			FRR: &clabtypes.FRRExtras{Daemons: []string{"bogusd"}},
		},
	})

	err := n.createFRRFiles()
	if err == nil {
		t.Fatal("expected an error for an unknown daemon, got none")
	}

	for _, want := range []string{"bogusd", "router1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestRegisterKindNames(t *testing.T) {
	r := clabnodes.NewNodeRegistry()
	Register(r)

	for _, name := range []string{"frr", "frrouting"} {
		if _, err := r.NewNodeOfKind(name); err != nil {
			t.Errorf("kind %q is not registered: %v", name, err)
		}
	}
}
