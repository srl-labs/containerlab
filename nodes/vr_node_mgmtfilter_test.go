// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import "testing"

func TestFilterMgmtIPConfigLines(t *testing.T) {
	tests := map[string]struct {
		config string
		v4     string
		v6     string
		want   string
	}{
		"ios-xr-leaf": {
			config: "interface MgmtEth0/RP0/CPU0/0\n ipv4 address 172.20.20.2 255.255.255.0\n!\n" +
				"interface Loopback0\n ipv4 address 10.0.0.1 255.255.255.255\n!",
			v4: "172.20.20.2",
			// interface header kept, only the address leaf dropped
			want: "interface MgmtEth0/RP0/CPU0/0\n!\n" +
				"interface Loopback0\n ipv4 address 10.0.0.1 255.255.255.255\n!",
		},
		"ios-leaf": {
			config: "interface GigabitEthernet1\n ip address 172.20.20.2 255.255.255.0\n!",
			v4:     "172.20.20.2",
			want:   "interface GigabitEthernet1\n!",
		},
		"nxos-slash": {
			config: "interface mgmt0\n  ip address 172.20.20.2/24\n",
			v4:     "172.20.20.2",
			want:   "interface mgmt0\n",
		},
		"junos-leaf": {
			config: "interfaces {\n    fxp0 {\n        unit 0 {\n            family inet {\n" +
				"                address 172.20.20.2/24;\n            }\n        }\n    }\n}",
			v4: "172.20.20.2",
			want: "interfaces {\n    fxp0 {\n        unit 0 {\n            family inet {\n" +
				"            }\n        }\n    }\n}",
		},
		"junos-address-block": {
			// address that OPENS a block: the whole balanced block must go,
			// or braces would be left unbalanced (the bug in naive line-delete).
			config: "        family inet {\n            address 172.20.20.2/24 {\n" +
				"                primary;\n                preferred;\n            }\n        }",
			v4:   "172.20.20.2",
			want: "        family inet {\n        }",
		},
		"routeros-setstyle": {
			config: "/ip address\nadd address=172.20.20.2/24 interface=ether1\n" +
				"add address=10.1.1.1/24 interface=ether2",
			v4:   "172.20.20.2",
			want: "/ip address\nadd address=10.1.1.1/24 interface=ether2",
		},
		"boundary-no-false-positive": {
			// stripping .2 must NOT drop .20 or .200
			config: "ip address 172.20.20.2 255.255.255.0\nip address 172.20.20.20 255.255.255.0\n" +
				"ip address 172.20.20.200 255.255.255.0",
			v4:   "172.20.20.2",
			want: "ip address 172.20.20.20 255.255.255.0\nip address 172.20.20.200 255.255.255.0",
		},
		"address-guard-keeps-non-assignment": {
			// the mgmt IP appears on non-address lines: those must be KEPT
			config: "ip address 172.20.20.2 255.255.255.0\nsnmp-server host 172.20.20.2\n" +
				"ntp server 172.20.20.2",
			v4:   "172.20.20.2",
			want: "snmp-server host 172.20.20.2\nntp server 172.20.20.2",
		},
		"description-with-mgmt-ip-and-address-word-kept": {
			// a description carrying both the mgmt IP and the word "address"
			// must NOT be stripped; only the real address line is.
			config: " description \"mgmt address 172.20.20.2 primary\"\n" +
				" ip address 172.20.20.2 255.255.255.0",
			v4:   "172.20.20.2",
			want: " description \"mgmt address 172.20.20.2 primary\"",
		},
		"ipv6": {
			config: " ipv6 address 3fff:172:20:20::2/64\n ipv6 address 2001:db8::1/64",
			v6:     "3fff:172:20:20::2",
			want:   " ipv6 address 2001:db8::1/64",
		},
		"dual-stack": {
			config: " ipv4 address 172.20.20.2 255.255.255.0\n ipv6 address 3fff::2/64\n description keep",
			v4:     "172.20.20.2",
			v6:     "3fff::2",
			want:   " description keep",
		},
		"no-mgmt-ip-set": {
			config: "ip address 172.20.20.2 255.255.255.0",
			want:   "ip address 172.20.20.2 255.255.255.0",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := FilterMgmtIPConfigLines(tc.config, tc.v4, tc.v6)
			if got != tc.want {
				t.Errorf("FilterMgmtIPConfigLines()\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}
