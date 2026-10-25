// Copyright 2026 NetPilot Labs
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package mikrotik_chr

import (
	"bytes"
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

func newInitialized(t *testing.T, cfg *clabtypes.NodeConfig) *mikrotikCHR {
	t.Helper()

	if cfg.Credentials.Username == "" {
		cfg.Credentials = clabtypes.NodeCredentials{Username: "admin", Password: "admin"}
	}

	n := new(mikrotikCHR)
	if err := n.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	return n
}

func TestRegisteredKind(t *testing.T) {
	registry := clabnodes.NewNodeRegistry()
	Register(registry)

	node, err := registry.NewNodeOfKind("mikrotik_chr")
	if err != nil {
		t.Fatalf("failed to create mikrotik_chr node: %v", err)
	}
	if _, ok := node.(*mikrotikCHR); !ok {
		t.Fatalf("mikrotik_chr registered unexpected implementation %T", node)
	}
}

func TestInitDefaults(t *testing.T) {
	labDir := t.TempDir()
	n := newInitialized(t, &clabtypes.NodeConfig{
		ShortName:   "r1",
		LabDir:      labDir,
		Credentials: clabtypes.NodeCredentials{Username: "netops", Password: "pass word#1"},
	})

	if n.Cfg.Entrypoint != "/bin/sh /chr-launch.sh" {
		t.Errorf("Entrypoint = %q, want the launcher", n.Cfg.Entrypoint)
	}
	if n.Cfg.Cmd != "--hostname r1" {
		t.Errorf("Cmd = %q, want only the hostname (credentials travel as env)", n.Cfg.Cmd)
	}
	if n.Cfg.Env["USERNAME"] != "netops" || n.Cfg.Env["PASSWORD"] != "pass word#1" {
		t.Errorf("Env credentials = %q/%q, want netops/%q",
			n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], "pass word#1")
	}

	for _, want := range []string{
		filepath.Join(labDir, "ftpboot") + ":/ftpboot",
		filepath.Join(labDir, "chr-launch.sh") + ":/chr-launch.sh:ro",
	} {
		if !slices.Contains(n.Cfg.Binds, want) {
			t.Errorf("Binds = %v, want it to contain %q", n.Cfg.Binds, want)
		}
	}

	hc := n.Cfg.Healthcheck
	if hc == nil || !slices.Equal(hc.Test, []string{"CMD-SHELL", "test -f /.chr-launch-ready"}) {
		t.Errorf("Healthcheck = %+v, want the launcher's ready file check", hc)
	}

	if n.HostRequirements.VirtRequired {
		t.Error("VirtRequired = true, want false: the image falls back to TCG without KVM")
	}
}

func TestInitKeepsUserSettings(t *testing.T) {
	userHC := &clabtypes.HealthcheckConfig{Test: []string{"CMD", "true"}}
	n := newInitialized(t, &clabtypes.NodeConfig{
		ShortName:   "r1",
		LabDir:      t.TempDir(),
		Entrypoint:  "/start.sh",
		Healthcheck: userHC,
		Env:         map[string]string{"MEM": "1024", "PASSWORD": "override"},
	})

	if n.Cfg.Entrypoint != "/start.sh" {
		t.Errorf("Entrypoint = %q, want the user's", n.Cfg.Entrypoint)
	}
	if n.Cfg.Healthcheck != userHC {
		t.Errorf("Healthcheck = %+v, want the user's", n.Cfg.Healthcheck)
	}
	if n.Cfg.Env["MEM"] != "1024" || n.Cfg.Env["PASSWORD"] != "override" {
		t.Errorf("Env = %v, want the user's MEM and PASSWORD kept", n.Cfg.Env)
	}
}

func TestPreDeployWritesLauncherAndStartupConfig(t *testing.T) {
	labDir := t.TempDir()
	startup := filepath.Join(t.TempDir(), "r1.rsc")
	rsc := "/ip address add address=10.0.0.1/30 interface=ether2\n"
	if err := os.WriteFile(startup, []byte(rsc), 0o600); err != nil {
		t.Fatal(err)
	}

	n := newInitialized(t, &clabtypes.NodeConfig{
		ShortName:     "r1",
		LongName:      "clab-test-r1",
		LabDir:        labDir,
		StartupConfig: startup,
	})

	if err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{}); err != nil {
		t.Fatalf("PreDeploy failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(labDir, "chr-launch.sh"))
	if err != nil {
		t.Fatalf("launcher not written: %v", err)
	}
	if !bytes.Equal(got, launcherScript) {
		t.Error("written launcher differs from the embedded chr-launch.sh")
	}
	if !bytes.HasPrefix(got, []byte("#!/bin/sh\n")) {
		t.Error("embedded launcher does not start with #!/bin/sh")
	}

	cfg, err := os.ReadFile(filepath.Join(labDir, "ftpboot", "config.auto.rsc"))
	if err != nil {
		t.Fatalf("startup config not written: %v", err)
	}
	if string(cfg) != rsc {
		t.Errorf("startup config = %q, want %q", cfg, rsc)
	}
}

func TestInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		ifaces  []string
		want    []string
		wantErr bool
	}{
		"routeros-names": {
			ifaces: []string{"ether2", "ether5", "ether13"},
			want:   []string{"eth1", "eth4", "eth12"},
		},
		"eth-names":   {ifaces: []string{"eth1", "eth3"}, want: []string{"eth1", "eth3"}},
		"mgmt-ether1": {ifaces: []string{"ether1"}, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := newInitialized(t, &clabtypes.NodeConfig{ShortName: "r1", LabDir: t.TempDir()})

			var err error
			for _, ifName := range tc.ifaces {
				ep := &clablinks.EndpointVeth{
					EndpointGeneric: clablinks.EndpointGeneric{IfaceName: ifName},
				}
				if err = n.AddEndpoint(ep); err != nil {
					break
				}
			}
			if err == nil {
				err = n.CheckInterfaceName()
			}

			if tc.wantErr {
				if err == nil {
					t.Fatalf("interfaces %v: want an error, got none", tc.ifaces)
				}
				return
			}
			if err != nil {
				t.Fatalf("interfaces %v: %v", tc.ifaces, err)
			}
			for i, ep := range n.Endpoints {
				if ep.GetIfaceName() != tc.want[i] {
					t.Errorf(
						"%q mapped to %q, want %q",
						tc.ifaces[i],
						ep.GetIfaceName(),
						tc.want[i],
					)
				}
			}
		})
	}
}

// exportedConfig is a verbatim RouterOS 7.24.5 /export of a node this kind deployed
// (transparent management, IPv4 + IPv6, a lab default route in main).
const exportedConfig = `# 2026-10-16 21:56:34 by RouterOS 7.24.5
# system id = db75dropC8O
#
/interface bridge
add name=lo0
/interface ethernet
set [ find default-name=ether1 ] disable-running-check=no
set [ find default-name=ether2 ] disable-running-check=no
/routing table
add comment=clab-mgmt fib name=mgmt
/ip address
add address=172.20.20.4/24 comment=clab-mgmt interface=ether1 network=\
    172.20.20.0
add address=10.0.0.2/30 interface=ether2 network=10.0.0.0
add address=2.2.2.2 interface=lo0 network=2.2.2.2
/ip dhcp-client
add disabled=yes interface=ether1 name=client1
/ip route
add comment=clab-mgmt dst-address=172.20.20.0/24 gateway=ether1 \
    routing-table=mgmt
add comment=clab-mgmt dst-address=0.0.0.0/0 gateway=172.20.20.1 \
    routing-table=mgmt
add dst-address=0.0.0.0/0 gateway=10.0.0.1
/ipv6 route
add comment=clab-mgmt dst-address=3fff:172:20:20::4/64 gateway=ether1 \
    routing-table=mgmt
add comment=clab-mgmt dst-address=::/0 gateway=3fff:172:20:20::1%ether1 \
    routing-table=mgmt
/ipv6 address
add address=3fff:172:20:20::4 advertise=no comment=clab-mgmt interface=ether1
/ipv6 nd
add interface=ether1 ra-lifetime=none
/routing rule
add action=lookup-only-in-table comment=clab-mgmt src-address=172.20.20.4/32 \
    table=mgmt
add action=lookup-only-in-table comment=clab-mgmt src-address=\
    3fff:172:20:20::4/128 table=mgmt
/system identity
set name=r2`

func TestFilterLauncherConfig(t *testing.T) {
	got := filterLauncherConfig(exportedConfig)

	if strings.Contains(got, "clab-mgmt") {
		t.Errorf("filtered config still contains a clab-mgmt object:\n%s", got)
	}
	for _, gone := range []string{
		"add disabled=yes interface=ether1 name=client1",
		"add interface=ether1 ra-lifetime=none",
		"172.20.20.4",
		"3fff:172:20:20::4",
	} {
		if strings.Contains(got, gone) {
			t.Errorf("filtered config still contains launcher line %q:\n%s", gone, got)
		}
	}

	for _, kept := range []string{
		"add address=10.0.0.2/30 interface=ether2 network=10.0.0.0",
		"add address=2.2.2.2 interface=lo0 network=2.2.2.2",
		"add dst-address=0.0.0.0/0 gateway=10.0.0.1",
		"add name=lo0",
		"set name=r2",
		"/routing table",
	} {
		if !strings.Contains(got, kept) {
			t.Errorf("filtered config lost user line %q:\n%s", kept, got)
		}
	}
}

func TestFilterLauncherConfigContinuationLines(t *testing.T) {
	in := "/ip route\n" +
		"add comment=clab-mgmt dst-address=0.0.0.0/0 \\\n" +
		"    gateway=198.18.255.2 routing-table=mgmt\n" +
		"add dst-address=10.9.0.0/16 gateway=10.0.0.2 \\\n" +
		"    routing-table=main\n"

	got := filterLauncherConfig(in)

	if strings.Contains(got, "198.18.255.2") {
		t.Errorf("wrapped launcher route kept:\n%s", got)
	}
	if !strings.Contains(
		got,
		"add dst-address=10.9.0.0/16 gateway=10.0.0.2 \\\n    routing-table=main",
	) {
		t.Errorf("wrapped user route lost or reflowed:\n%s", got)
	}
}

func TestFilterLauncherConfigEscapedBackslash(t *testing.T) {
	// A value ending in a literal backslash is exported as `\\`: that line is complete and
	// must not swallow the launcher line after it.
	in := "/ip address\n" +
		"add address=10.9.9.1/24 interface=ether2 network=10.9.9.0 comment=path\\\\\n" +
		"add address=172.20.20.3/24 comment=clab-mgmt interface=ether1 network=\\\n" +
		"    172.20.20.0\n"

	got := filterLauncherConfig(in)

	if !strings.Contains(
		got,
		"add address=10.9.9.1/24 interface=ether2 network=10.9.9.0 comment=path",
	) {
		t.Errorf("user address ending in an escaped backslash was dropped:\n%s", got)
	}
	if strings.Contains(got, "clab-mgmt") {
		t.Errorf("launcher address kept:\n%s", got)
	}
}

func TestContinues(t *testing.T) {
	for line, want := range map[string]bool{
		`add a=b`:     false,
		`add a=b \`:   true,
		`add a=b\\`:   false,
		`add a=b\\ \`: true,
		`add a=b\\\`:  true,
		`add a=b\\\\`: false,
	} {
		if got := continues(line); got != want {
			t.Errorf("continues(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestHealthcheckStartPeriod(t *testing.T) {
	tests := map[string]struct {
		env  map[string]string
		want int
	}{
		"defaults": {env: nil, want: 60 + 3*300 + 90},
		"overrides": {
			env:  map[string]string{"CHR_INTF_WAIT": "10", "CHR_BOOT_WAIT": "100"},
			want: 10 + 3*100 + 90,
		},
		"invalid": {
			env:  map[string]string{"CHR_INTF_WAIT": "x", "CHR_BOOT_WAIT": "-5"},
			want: 60 + 3*300 + 90,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := newInitialized(
				t,
				&clabtypes.NodeConfig{ShortName: "r1", LabDir: t.TempDir(), Env: tc.env},
			)
			if got := n.Cfg.Healthcheck.StartPeriod; got != tc.want {
				t.Errorf("StartPeriod = %d, want %d", got, tc.want)
			}
		})
	}
}
