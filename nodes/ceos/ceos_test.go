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

// testEndpoint builds an endpoint with an optional IPv4/IPv6 address (CIDR).
func testEndpoint(name, v4, v6 string) clablinks.Endpoint {
	e := &clablinks.EndpointVeth{
		EndpointGeneric: clablinks.EndpointGeneric{IfaceName: name},
	}
	if v4 != "" {
		e.IPv4 = netip.MustParsePrefix(v4)
	}
	if v6 != "" {
		e.IPv6 = netip.MustParsePrefix(v6)
	}
	return e
}

func newTestCeos(cfg *clabtypes.NodeConfig, eps []clablinks.Endpoint) *ceos {
	n := &ceos{
		DefaultNode: clabnodes.DefaultNode{Cfg: cfg},
	}
	n.OverwriteNode = n
	n.Endpoints = eps
	return n
}

func TestCheckInterfaceName(t *testing.T) {
	tests := map[string]struct {
		networkMode string
		endpoints   []clablinks.Endpoint
		errContains string
	}{
		"data interfaces ok": {
			endpoints: []clablinks.Endpoint{
				testEndpoint("eth1", "", ""),
				testEndpoint("et2", "", ""),
			},
		},
		"eth0 rejected in default network mode": {
			networkMode: "",
			endpoints:   []clablinks.Endpoint{testEndpoint("eth0", "", "")},
			errContains: "not set to none",
		},
		"eth0 allowed with network-mode none": {
			networkMode: "none",
			endpoints:   []clablinks.Endpoint{testEndpoint("eth0", "", "")},
		},
		"eth0 allowed with network-mode none case insensitive": {
			networkMode: "None",
			endpoints:   []clablinks.Endpoint{testEndpoint("eth0", "", "")},
		},
		"eth0 alongside data interface with none": {
			networkMode: "none",
			endpoints: []clablinks.Endpoint{
				testEndpoint("eth0", "", ""),
				testEndpoint("eth1", "", ""),
			},
		},
		"invalid interface name rejected": {
			endpoints:   []clablinks.Endpoint{testEndpoint("foo", "", "")},
			errContains: "required pattern",
		},
		"et0 rejected": {
			endpoints:   []clablinks.Endpoint{testEndpoint("et0", "", "")},
			errContains: "required pattern",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := newTestCeos(
				&clabtypes.NodeConfig{ShortName: "ceos1", NetworkMode: tc.networkMode},
				tc.endpoints,
			)

			err := n.CheckInterfaceName()

			if tc.errContains == "" {
				if err != nil {
					t.Fatalf("got unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("got no error, want error containing %q", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("got error %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

func TestCeosInterfaceWait(t *testing.T) {
	type result struct {
		DataIntfs int
		Eth0Wired bool
	}
	tests := map[string]struct {
		endpoints []clablinks.Endpoint
		want      result
	}{
		"data interfaces only": {
			endpoints: []clablinks.Endpoint{
				testEndpoint("eth1", "", ""),
				testEndpoint("et2", "", ""),
			},
			want: result{DataIntfs: 2, Eth0Wired: false},
		},
		"eth0 only (oob mgmt)": {
			endpoints: []clablinks.Endpoint{testEndpoint("eth0", "", "")},
			want:      result{DataIntfs: 0, Eth0Wired: true},
		},
		"eth0 plus data interfaces": {
			endpoints: []clablinks.Endpoint{
				testEndpoint("eth0", "", ""),
				testEndpoint("eth1", "", ""),
				testEndpoint("eth2", "", ""),
			},
			want: result{DataIntfs: 2, Eth0Wired: true},
		},
		"no links": {
			endpoints: nil,
			want:      result{DataIntfs: 0, Eth0Wired: false},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got result
			got.DataIntfs, got.Eth0Wired = ceosInterfaceWait(tc.endpoints)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ceosInterfaceWait() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPostDeployConfigs(t *testing.T) {
	tests := map[string]struct {
		cfg       *clabtypes.NodeConfig
		endpoints []clablinks.Endpoint
		want      []string
	}{
		"mgmt from runtime with data interface": {
			cfg: &clabtypes.NodeConfig{
				MgmtIntf:             "Management0",
				MgmtIPv4Address:      "172.20.20.2",
				MgmtIPv4PrefixLength: 24,
			},
			endpoints: []clablinks.Endpoint{testEndpoint("eth1", "10.0.0.1/30", "")},
			want: []string{
				"interface Management0",
				"no ip address",
				"ip address 172.20.20.2/24",
				"interface eth1",
				"no switchport",
				"no ip address",
				"no ipv6 address",
				"ip address 10.0.0.1/30",
			},
		},
		"mgmt v4 and v6 from runtime": {
			cfg: &clabtypes.NodeConfig{
				MgmtIntf:             "Management0",
				MgmtIPv4Address:      "172.20.20.2",
				MgmtIPv4PrefixLength: 24,
				MgmtIPv6Address:      "2001:db8::2",
				MgmtIPv6PrefixLength: 64,
			},
			want: []string{
				"interface Management0",
				"no ip address",
				"ip address 172.20.20.2/24",
				"no ipv6 address",
				"ipv6 address 2001:db8::2/64",
			},
		},
		"detached eth0 wired with address sets mgmt": {
			cfg:       &clabtypes.NodeConfig{MgmtIntf: "Management0", NetworkMode: "none"},
			endpoints: []clablinks.Endpoint{testEndpoint("eth0", "192.168.1.10/24", "")},
			want: []string{
				"interface Management0",
				"no ip address",
				"ip address 192.168.1.10/24",
			},
		},
		"detached eth0 wired without address leaves mgmt untouched": {
			cfg:       &clabtypes.NodeConfig{MgmtIntf: "Management0", NetworkMode: "none"},
			endpoints: []clablinks.Endpoint{testEndpoint("eth0", "", "")},
			want:      nil,
		},
		"detached eth0 plus data interface": {
			cfg: &clabtypes.NodeConfig{MgmtIntf: "Management0", NetworkMode: "none"},
			endpoints: []clablinks.Endpoint{
				testEndpoint("eth0", "192.168.1.10/24", ""),
				testEndpoint("et1", "10.0.0.1/30", ""),
			},
			want: []string{
				"interface Management0",
				"no ip address",
				"ip address 192.168.1.10/24",
				"interface et1",
				"no switchport",
				"no ip address",
				"no ipv6 address",
				"ip address 10.0.0.1/30",
			},
		},
		"detached eth0 wired with ipv6 only resets only v6": {
			cfg:       &clabtypes.NodeConfig{MgmtIntf: "Management0", NetworkMode: "none"},
			endpoints: []clablinks.Endpoint{testEndpoint("eth0", "", "2001:db8::2/64")},
			want: []string{
				"interface Management0",
				"no ipv6 address",
				"ipv6 address 2001:db8::2/64",
			},
		},
		"detached eth0 with ipv4 only does not reset v6": {
			cfg:       &clabtypes.NodeConfig{MgmtIntf: "Management0", NetworkMode: "none"},
			endpoints: []clablinks.Endpoint{testEndpoint("eth0", "192.0.2.10/24", "")},
			want: []string{
				"interface Management0",
				"no ip address",
				"ip address 192.0.2.10/24",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := newTestCeos(tc.cfg, tc.endpoints)

			got := n.postDeployConfigs()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("postDeployConfigs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
