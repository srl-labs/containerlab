package types

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestNodeConfigManagementIPAMEligible(t *testing.T) {
	for _, tc := range []struct {
		name string
		node NodeConfig
		want bool
	}{
		{name: "default", want: true},
		{name: "host network", node: NodeConfig{NetworkMode: "host"}},
		{name: "no network", node: NodeConfig{NetworkMode: "none"}},
		{name: "shared namespace", node: NodeConfig{NetworkMode: "container:peer"}},
		{name: "root namespace", node: NodeConfig{IsRootNamespaceBased: true}},
		{name: "external container", node: NodeConfig{SkipUniquenessCheck: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.node.ManagementIPAMEligible(); got != tc.want {
				t.Fatalf("ManagementIPAMEligible() = %t; want %t", got, tc.want)
			}
		})
	}
}

func TestMgmtDriverValidation(t *testing.T) {
	for _, tc := range []struct {
		driver MgmtDriver
		valid  bool
	}{
		{valid: true},
		{driver: MgmtDriverBridge, valid: true},
		{driver: MgmtDriverMacvlan, valid: true},
		{driver: "ipvlan"},
	} {
		if got := tc.driver.IsValid(); got != tc.valid {
			t.Fatalf("MgmtDriver(%q).IsValid() = %v, want %v", tc.driver, got, tc.valid)
		}
	}
}

func TestMacvlanManagementValidation(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*MgmtNet)
		wantErr bool
	}{
		{name: "bridge mode default"},
		{
			name:   "auxiliary connectivity disabled",
			change: func(m *MgmtNet) { m.MacvlanAux = new(false) },
		},
		{
			name:    "long parent",
			change:  func(m *MgmtNet) { m.MacvlanParent = "0123456789012345" },
			wantErr: true,
		},
		{
			name:    "parent whitespace",
			change:  func(m *MgmtNet) { m.MacvlanParent = "eth 0" },
			wantErr: true,
		},
		{
			name:    "broadcast gateway",
			change:  func(m *MgmtNet) { m.IPv4Gw = "192.0.2.255" },
			wantErr: true,
		},
		{name: "dual stack", change: func(m *MgmtNet) { m.IPv6Subnet = "2001:db8::/64" }},
		{
			name:   "IPv6 only",
			change: func(m *MgmtNet) { m.IPv4Subnet = ""; m.IPv6Subnet = "2001:db8::/64" },
		},
		{name: "missing driver", change: func(m *MgmtNet) { m.Driver = "" }, wantErr: true},
		{name: "unknown driver", change: func(m *MgmtNet) { m.Driver = "ipvlan" }, wantErr: true},
		{name: "missing parent", change: func(m *MgmtNet) { m.MacvlanParent = "" }, wantErr: true},
		{name: "infer subnet", change: func(m *MgmtNet) { m.IPv4Subnet = "" }},
		{name: "auto subnet", change: func(m *MgmtNet) { m.IPv4Subnet = "auto" }, wantErr: true},
		{
			name:    "wrong subnet family",
			change:  func(m *MgmtNet) { m.IPv4Subnet = "2001:db8::/64" },
			wantErr: true,
		},
		{name: "auto IPv6", change: func(m *MgmtNet) { m.IPv6Subnet = "auto" }, wantErr: true},
		{name: "bridge setting", change: func(m *MgmtNet) { m.Bridge = "br0" }, wantErr: true},
		{name: "MTU setting", change: func(m *MgmtNet) { m.MTU = 1500 }, wantErr: true},
		{
			name:    "external access disabled",
			change:  func(m *MgmtNet) { m.ExternalAccess = new(false) },
			wantErr: true,
		},
		{
			name:    "invalid mode",
			change:  func(m *MgmtNet) { m.MacvlanMode = "invalid" },
			wantErr: true,
		},
		{
			name:   "private mode",
			change: func(m *MgmtNet) { m.MacvlanMode = "private" },
		},
		{name: "vepa mode", change: func(m *MgmtNet) { m.MacvlanMode = "vepa" }},
		{name: "passthru mode", change: func(m *MgmtNet) { m.MacvlanMode = "passthru" }},
		{
			name:    "parent override",
			change:  func(m *MgmtNet) { m.DriverOpts = map[string]string{"parent": "eth1"} },
			wantErr: true,
		},
		{
			name:    "mode override",
			change:  func(m *MgmtNet) { m.DriverOpts = map[string]string{"macvlan_mode": "private"} },
			wantErr: true,
		},
		{
			name:   "matching options",
			change: func(m *MgmtNet) { m.DriverOpts = map[string]string{"parent": "eth0", "macvlan_mode": "bridge"} },
		},
		{
			name:    "invalid gateway",
			change:  func(m *MgmtNet) { m.IPv4Gw = "198.51.100.1" },
			wantErr: true,
		},
		{
			name:    "invalid pool",
			change:  func(m *MgmtNet) { m.IPv4Range = "192.0.0.0/16" },
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &MgmtNet{Driver: "macvlan", MacvlanParent: "eth0", IPv4Subnet: "192.0.2.0/24"}
			if tc.change != nil {
				tc.change(m)
			}
			if err := m.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestMacvlanAuxEnabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode string
		aux  *bool
		want bool
	}{
		{name: "bridge default", want: true},
		{name: "bridge enabled", aux: new(true), want: true},
		{name: "bridge disabled", aux: new(false)},
		{name: "private", mode: "private"},
		{name: "vepa enabled", mode: "vepa", aux: new(true)},
		{name: "passthru", mode: "passthru"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &MgmtNet{MacvlanMode: tc.mode, MacvlanAux: tc.aux}
			if got := m.MacvlanAuxEnabled(); got != tc.want {
				t.Fatalf("MacvlanAuxEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDADValidation(t *testing.T) {
	for _, dad := range []*bool{nil, new(true), new(false)} {
		m := &MgmtNet{IPAM: MgmtIPAM{DAD: dad}}
		if err := m.Validate(); err != nil {
			t.Fatal(err)
		}
		if m.IPAM.DADEnabled() != (dad == nil || *dad) {
			t.Fatal("incorrect DAD default")
		}
	}
	if (&MgmtNet{IPAM: MgmtIPAM{Provider: IPAMProvider("index")}}).Validate() == nil {
		t.Fatal("accepted invalid provider")
	}
}

func TestManagementIPAMYAML(t *testing.T) {
	var m MgmtNet
	if err := yaml.UnmarshalStrict(
		[]byte("ipam:\n  provider: runtime\n  dad: true\n"),
		&m,
	); err != nil {
		t.Fatal(err)
	}
	if m.IPAM.Provider != IPAMProviderRuntime || !m.IPAM.DADEnabled() {
		t.Fatalf("incorrect IPAM config: %+v", m.IPAM)
	}
	if !m.MacvlanAuxEnabled() {
		t.Fatal("macvlan auxiliary connectivity is not enabled by default")
	}
	if err := yaml.UnmarshalStrict([]byte("macvlan-aux: false\n"), &m); err != nil ||
		m.MacvlanAuxEnabled() {
		t.Fatalf("boolean macvlan-aux was not accepted: %+v, %v", m, err)
	}
	if err := yaml.UnmarshalStrict([]byte("macvlan-aux: 192.0.2.2\n"), &m); err == nil {
		t.Fatal("address-valued macvlan-aux was accepted")
	}
}
