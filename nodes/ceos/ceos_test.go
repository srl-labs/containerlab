// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// TestCEOSManagementInterfaceResolution checks that the EOS management interface
// name is derived from the MGMT_INTF env var.
func TestCEOSManagementInterfaceResolution(t *testing.T) {
	tests := map[string]struct {
		env      map[string]string
		wantIntf string
	}{
		"default-is-management0": {
			env:      nil,
			wantIntf: "Management0",
		},
		"explicit-eth0-is-management0": {
			env:      map[string]string{mgmtIntfEnvVar: "eth0"},
			wantIntf: "Management0",
		},
		"ma1-is-management1": {
			env:      map[string]string{mgmtIntfEnvVar: "ma1"},
			wantIntf: "Management1",
		},
		"ma0-is-management0": {
			env:      map[string]string{mgmtIntfEnvVar: "ma0"},
			wantIntf: "Management0",
		},
		"ma5-is-management5": {
			env:      map[string]string{mgmtIntfEnvVar: "ma5"},
			wantIntf: "Management5",
		},
		"eth1-is-management0-legacy": {
			env:      map[string]string{mgmtIntfEnvVar: "eth1"},
			wantIntf: "Management0",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			node := &clabtypes.NodeConfig{
				ShortName: "ceos",
				Env:       tc.env,
			}

			if err := setMgmtInterface(node); err != nil {
				t.Fatalf("setMgmtInterface returned error: %v", err)
			}

			if node.MgmtIntf != tc.wantIntf {
				t.Errorf("got MgmtIntf %q, want %q", node.MgmtIntf, tc.wantIntf)
			}
		})
	}
}

// TestCEOSInitManagementEnv checks that MAPETH0 is dropped when MGMT_INTF is
// remapped and that eth0 is renamed only when attached to the mgmt network.
func TestCEOSInitManagementEnv(t *testing.T) {
	tests := map[string]struct {
		env          map[string]string
		networkMode  string
		wantMapEth0  bool
		wantMgmtIntf string
		wantRename   string // substring expected in the boot cmd, empty if none
	}{
		"default-keeps-mapeth0-no-rename": {
			env:          nil,
			wantMapEth0:  true,
			wantMgmtIntf: "eth0",
			wantRename:   "",
		},
		"eth1-keeps-mapeth0-no-rename-legacy": {
			env:          map[string]string{mgmtIntfEnvVar: "eth1"},
			wantMapEth0:  true,
			wantMgmtIntf: "eth1",
			wantRename:   "", // not maN: passed through unchanged
		},
		"ma1-drops-mapeth0-and-renames": {
			env:          map[string]string{mgmtIntfEnvVar: "ma1"},
			wantMapEth0:  false,
			wantMgmtIntf: "ma1",
			wantRename:   "ip link set eth0 name ma1",
		},
		"ma1-with-network-mode-none-does-not-rename": {
			env:          map[string]string{mgmtIntfEnvVar: "ma1"},
			networkMode:  networkModeNone,
			wantMapEth0:  false,
			wantMgmtIntf: "ma1",
			wantRename:   "", // no eth0; netdev provided by a link
		},
		"ma1-with-network-mode-host-does-not-rename": {
			env:          map[string]string{mgmtIntfEnvVar: "ma1"},
			networkMode:  "host",
			wantMapEth0:  false,
			wantMgmtIntf: "ma1",
			wantRename:   "", // must never rename a shared/host eth0
		},
		"ma1-with-network-mode-container-does-not-rename": {
			env:          map[string]string{mgmtIntfEnvVar: "ma1"},
			networkMode:  "container:other",
			wantMapEth0:  false,
			wantMgmtIntf: "ma1",
			wantRename:   "", // must never rename a peer container's eth0
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := &ceos{}
			cfg := &clabtypes.NodeConfig{
				ShortName:   "ceos",
				LongName:    "clab-test-ceos",
				Env:         tc.env,
				NetworkMode: tc.networkMode,
				Certificate: &clabtypes.CertificateConfig{},
			}

			if err := n.Init(cfg); err != nil {
				t.Fatalf("Init returned error: %v", err)
			}

			if _, ok := n.Cfg.Env[mapEth0EnvVar]; ok != tc.wantMapEth0 {
				t.Errorf("MAPETH0 present=%v, want %v", ok, tc.wantMapEth0)
			}

			if got := n.Cfg.Env[mgmtIntfEnvVar]; got != tc.wantMgmtIntf {
				t.Errorf("MGMT_INTF=%q, want %q", got, tc.wantMgmtIntf)
			}

			switch {
			case tc.wantRename == "" && strings.Contains(n.Cfg.Cmd, "ip link set eth0 name"):
				t.Errorf("boot cmd unexpectedly renames eth0: %q", n.Cfg.Cmd)
			case tc.wantRename != "" && !strings.Contains(n.Cfg.Cmd, tc.wantRename):
				t.Errorf("boot cmd missing %q, got: %q", tc.wantRename, n.Cfg.Cmd)
			}
		})
	}
}

// TestCEOSCheckInterfaceName checks data interface validation and that a maN
// mgmt interface is only accepted with network-mode none and a matching MGMT_INTF.
func TestCEOSCheckInterfaceName(t *testing.T) {
	tests := map[string]struct {
		ifaceNames  []string
		networkMode string
		env         map[string]string
		wantErr     bool
	}{
		"data-eth-ok": {
			ifaceNames: []string{"eth1", "eth2"},
			wantErr:    false,
		},
		"data-et-ok": {
			ifaceNames: []string{"et1", "et10"},
			wantErr:    false,
		},
		"eth0-rejected": {
			ifaceNames: []string{"eth0"},
			wantErr:    true,
		},
		"ma1-requires-network-mode-none": {
			ifaceNames: []string{"ma1"},
			env:        map[string]string{mgmtIntfEnvVar: "ma1"},
			wantErr:    true,
		},
		"ma1-ok-with-network-mode-none-and-matching-env": {
			ifaceNames:  []string{"ma1"},
			networkMode: networkModeNone,
			env:         map[string]string{mgmtIntfEnvVar: "ma1"},
			wantErr:     false,
		},
		"ma0-ok-with-matching-env": {
			ifaceNames:  []string{"ma0"},
			networkMode: networkModeNone,
			env:         map[string]string{mgmtIntfEnvVar: "ma0"},
			wantErr:     false,
		},
		"ma1-and-data-ok": {
			ifaceNames:  []string{"ma1", "eth1"},
			networkMode: networkModeNone,
			env:         map[string]string{mgmtIntfEnvVar: "ma1"},
			wantErr:     false,
		},
		"ma1-mismatched-env-rejected": {
			ifaceNames:  []string{"ma1"},
			networkMode: networkModeNone,
			env:         map[string]string{mgmtIntfEnvVar: "ma2"},
			wantErr:     true,
		},
		"ma1-without-mgmt-intf-env-rejected": {
			ifaceNames:  []string{"ma1"},
			networkMode: networkModeNone,
			wantErr:     true,
		},
		"unknown-name-rejected": {
			ifaceNames: []string{"foo0"},
			wantErr:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := &ceos{
				DefaultNode: clabnodes.DefaultNode{
					Cfg: &clabtypes.NodeConfig{
						ShortName:   "ceos",
						NetworkMode: tc.networkMode,
						Env:         tc.env,
					},
				},
			}
			n.OverwriteNode = n

			for _, ifName := range tc.ifaceNames {
				ep := &clablinks.EndpointVeth{
					EndpointGeneric: clablinks.EndpointGeneric{IfaceName: ifName},
				}
				if err := n.AddEndpoint(ep); err != nil {
					t.Fatalf("AddEndpoint(%q) returned error: %v", ifName, err)
				}
			}

			err := n.CheckInterfaceName()
			if tc.wantErr && err == nil {
				t.Errorf("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestCEOSPostDeployConfig checks the EOS configuration generated after deploy
// for the legacy, remapped (runtime mgmt network) and manually wired management
// cases, including the no-op case where nothing should be configured.
func TestCEOSPostDeployConfig(t *testing.T) {
	endpoint := func(name, v4, v6 string) *clablinks.EndpointVeth {
		ep := &clablinks.EndpointVeth{
			EndpointGeneric: clablinks.EndpointGeneric{IfaceName: name},
		}
		if v4 != "" {
			ep.IPv4 = netip.MustParsePrefix(v4)
		}
		if v6 != "" {
			ep.IPv6 = netip.MustParsePrefix(v6)
		}
		return ep
	}

	tests := map[string]struct {
		cfg       *clabtypes.NodeConfig
		endpoints []*clablinks.EndpointVeth
		want      []string
	}{
		"legacy-mgmt-ipv4": {
			cfg: &clabtypes.NodeConfig{
				ShortName:            "ceos",
				MgmtIntf:             "Management0",
				MgmtIPv4Address:      "172.20.20.2",
				MgmtIPv4PrefixLength: 24,
			},
			want: []string{
				"interface Management0",
				"no ip address",
				"no ipv6 address",
				"ip address 172.20.20.2/24",
			},
		},
		"legacy-mgmt-dualstack-with-data": {
			cfg: &clabtypes.NodeConfig{
				ShortName:            "ceos",
				MgmtIntf:             "Management0",
				MgmtIPv4Address:      "172.20.20.2",
				MgmtIPv4PrefixLength: 24,
				MgmtIPv6Address:      "2001:db8::2",
				MgmtIPv6PrefixLength: 64,
			},
			endpoints: []*clablinks.EndpointVeth{endpoint("eth1", "10.0.0.1/30", "")},
			want: []string{
				"interface Management0",
				"no ip address",
				"no ipv6 address",
				"ip address 172.20.20.2/24",
				"ipv6 address 2001:db8::2/64",
				"interface eth1",
				"no switchport",
				"no ip address",
				"no ipv6 address",
				"ip address 10.0.0.1/30",
			},
		},
		"remapped-runtime-network-with-data": {
			cfg: &clabtypes.NodeConfig{
				ShortName:            "ceos",
				Env:                  map[string]string{mgmtIntfEnvVar: "ma1"},
				MgmtIntf:             "Management1",
				MgmtIPv4Address:      "172.20.20.2",
				MgmtIPv4PrefixLength: 24,
			},
			endpoints: []*clablinks.EndpointVeth{endpoint("eth1", "10.0.0.1/30", "")},
			want: []string{
				"interface Management1",
				"no ip address",
				"no ipv6 address",
				"ip address 172.20.20.2/24",
				"interface eth1",
				"no switchport",
				"no ip address",
				"no ipv6 address",
				"ip address 10.0.0.1/30",
			},
		},
		"manually-wired-uses-endpoint-address": {
			cfg: &clabtypes.NodeConfig{
				ShortName:   "ceos",
				Env:         map[string]string{mgmtIntfEnvVar: "ma1"},
				NetworkMode: networkModeNone,
				MgmtIntf:    "Management1",
			},
			endpoints: []*clablinks.EndpointVeth{endpoint("ma1", "172.31.0.1/24", "")},
			want: []string{
				"interface Management1",
				"no ip address",
				"no ipv6 address",
				"ip address 172.31.0.1/24",
			},
		},
		"manually-wired-no-address-is-noop": {
			cfg: &clabtypes.NodeConfig{
				ShortName:   "ceos",
				Env:         map[string]string{mgmtIntfEnvVar: "ma1"},
				NetworkMode: networkModeNone,
				MgmtIntf:    "Management1",
			},
			endpoints: []*clablinks.EndpointVeth{endpoint("ma1", "", "")},
			want:      nil,
		},
		"manually-wired-no-mgmt-address-only-data": {
			cfg: &clabtypes.NodeConfig{
				ShortName:   "ceos",
				Env:         map[string]string{mgmtIntfEnvVar: "ma1"},
				NetworkMode: networkModeNone,
				MgmtIntf:    "Management1",
			},
			endpoints: []*clablinks.EndpointVeth{
				endpoint("ma1", "", ""),
				endpoint("eth1", "10.0.0.1/30", ""),
			},
			want: []string{
				"interface eth1",
				"no switchport",
				"no ip address",
				"no ipv6 address",
				"ip address 10.0.0.1/30",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := &ceos{
				DefaultNode: clabnodes.DefaultNode{Cfg: tc.cfg},
			}
			n.OverwriteNode = n

			for _, ep := range tc.endpoints {
				if err := n.AddEndpoint(ep); err != nil {
					t.Fatalf("AddEndpoint(%q) returned error: %v", ep.GetIfaceName(), err)
				}
			}

			got := n.postDeployConfig()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("postDeployConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
