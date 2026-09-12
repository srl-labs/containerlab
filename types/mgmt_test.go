package types

import "testing"

func TestMacvlanManagementAcceptsPrivateAddresses(t *testing.T) {
	for _, tc := range []struct {
		name, subnet, gateway, aux string
		ipv6                       bool
	}{
		{name: "RFC1918 10/8", subnet: "10.1.0.0/24", gateway: "10.1.0.1", aux: "10.1.0.2"},
		{name: "RFC1918 172.16/12", subnet: "172.16.1.0/24", gateway: "172.16.1.1", aux: "172.16.1.2"},
		{name: "RFC1918 192.168/16", subnet: "192.168.1.0/24", gateway: "192.168.1.1", aux: "192.168.1.2"},
		{name: "IPv6 ULA", subnet: "fd00:1::/64", gateway: "fd00:1::1", aux: "fd00:1::2", ipv6: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &MgmtNet{Driver: "macvlan", MacvlanParent: "eth0", MacvlanAux: tc.aux}
			if tc.ipv6 {
				m.IPv6Subnet, m.IPv6Gw = tc.subnet, tc.gateway
			} else {
				m.IPv4Subnet, m.IPv4Gw = tc.subnet, tc.gateway
			}
			if err := m.Validate(); err != nil {
				t.Fatalf("private management addresses rejected: %v", err)
			}
			ip, route, err := m.MacvlanHostAddress()
			if err != nil || ip.String() != tc.aux || route.String() != tc.subnet {
				t.Fatalf("MacvlanHostAddress() = %s, %s, %v; want %s, %s", ip, route, err, tc.aux, tc.subnet)
			}
		})
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
		{
			name:    "aux host-only route",
			change:  func(m *MgmtNet) { m.MacvlanAux = "192.0.2.10/32" },
			wantErr: true,
		},
		{name: "auxiliary CIDR", change: func(m *MgmtNet) { m.MacvlanAux = "192.0.2.129/26" }},
		{name: "dual stack", change: func(m *MgmtNet) { m.IPv6Subnet = "2001:db8::/64" }},
		{
			name:   "IPv6 only",
			change: func(m *MgmtNet) { m.IPv4Subnet = ""; m.IPv6Subnet = "2001:db8::/64" },
		},
		{name: "missing driver", change: func(m *MgmtNet) { m.Driver = "" }, wantErr: true},
		{name: "unknown driver", change: func(m *MgmtNet) { m.Driver = "ipvlan" }, wantErr: true},
		{name: "missing parent", change: func(m *MgmtNet) { m.MacvlanParent = "" }, wantErr: true},
		{name: "missing subnet", change: func(m *MgmtNet) { m.IPv4Subnet = "" }, wantErr: true},
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
		{name: "private without aux", change: func(m *MgmtNet) { m.MacvlanMode = "private" }},
		{
			name:    "private with aux",
			change:  func(m *MgmtNet) { m.MacvlanMode = "private"; m.MacvlanAux = "192.0.2.10" },
			wantErr: true,
		},
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
		{name: "invalid aux", change: func(m *MgmtNet) { m.MacvlanAux = "bad" }, wantErr: true},
		{
			name:    "IPv6 aux",
			change:  func(m *MgmtNet) { m.MacvlanAux = "2001:db8::10/64" },
			wantErr: true,
		},
		{
			name:    "aux outside subnet",
			change:  func(m *MgmtNet) { m.MacvlanAux = "198.51.100.10" },
			wantErr: true,
		},
		{
			name:    "aux route too broad",
			change:  func(m *MgmtNet) { m.MacvlanAux = "192.0.2.10/16" },
			wantErr: true,
		},
		{
			name:    "aux implicit gateway",
			change:  func(m *MgmtNet) { m.MacvlanAux = "192.0.2.1" },
			wantErr: true,
		},
		{
			name:    "aux explicit gateway",
			change:  func(m *MgmtNet) { m.IPv4Gw = "192.0.2.10"; m.MacvlanAux = m.IPv4Gw },
			wantErr: true,
		},
		{
			name:    "aux network address",
			change:  func(m *MgmtNet) { m.MacvlanAux = "192.0.2.0" },
			wantErr: true,
		},
		{
			name:    "aux broadcast",
			change:  func(m *MgmtNet) { m.MacvlanAux = "192.0.2.255" },
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

func TestMacvlanHostAddress(t *testing.T) {
	for _, tc := range []struct{ aux, route string }{
		{"192.0.2.129", "192.0.2.0/24"},
		{"192.0.2.129/26", "192.0.2.128/26"},
	} {
		m := &MgmtNet{IPv4Subnet: "192.0.2.0/24", MacvlanAux: tc.aux}
		ip, route, err := m.MacvlanHostAddress()
		if err != nil || ip.String() != "192.0.2.129" || route.String() != tc.route {
			t.Errorf("MacvlanHostAddress(%s) = %s, %s, %v", tc.aux, ip, route, err)
		}
	}
}

func TestMacvlanIPv6HostAddress(t *testing.T) {
	for _, tc := range []struct {
		name, aux, subnet, gw, route string
		wantErr                      bool
	}{
		{name: "plain", aux: "2001:db8::2", subnet: "2001:db8::/64", route: "2001:db8::/64"},
		{name: "narrow route", aux: "2001:db8::8000:2/97", subnet: "2001:db8::/64", route: "2001:db8::8000:0/97"},
		{name: "ULA", aux: "fd00::2", subnet: "fd00::/64", route: "fd00::/64"},
		{name: "last address is not broadcast", aux: "2001:db8::ffff", subnet: "2001:db8::/112", route: "2001:db8::/112"},
		{name: "missing IPv6 subnet", aux: "2001:db8::2", wantErr: true},
		{name: "outside subnet", aux: "2001:db8:1::2", subnet: "2001:db8::/64", wantErr: true},
		{name: "wide route", aux: "2001:db8::2/48", subnet: "2001:db8::/64", wantErr: true},
		{name: "host-only route", aux: "2001:db8::2/128", subnet: "2001:db8::/64", wantErr: true},
		{name: "default gateway", aux: "2001:db8::1", subnet: "2001:db8::/64", wantErr: true},
		{name: "explicit gateway", aux: "2001:db8::2", subnet: "2001:db8::/64", gw: "2001:db8::2", wantErr: true},
		{name: "subnet-router anycast", aux: "2001:db8::", subnet: "2001:db8::/64", wantErr: true},
		{name: "link local", aux: "fe80::2", subnet: "fe80::/64", wantErr: true},
		{name: "scoped", aux: "2001:db8::2%eth0", subnet: "2001:db8::/64", wantErr: true},
		{name: "mapped IPv4", aux: "::ffff:192.0.2.2", subnet: "::ffff:192.0.2.0/120", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &MgmtNet{
				Driver:        "macvlan",
				MacvlanParent: "eth0",
				IPv4Subnet:    "192.0.2.0/24",
				IPv6Subnet:    tc.subnet,
				IPv6Gw:        tc.gw,
				MacvlanAux:    tc.aux,
			}
			if err := m.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() = %v", err)
			}
			ip, route, err := m.MacvlanHostAddress()
			if (err != nil) != tc.wantErr {
				t.Fatalf("MacvlanHostAddress() = %v", err)
			}
			if !tc.wantErr && (!ip.Is6() || route.String() != tc.route) {
				t.Fatalf("address = %s, route = %s", ip, route)
			}
		})
	}
}
