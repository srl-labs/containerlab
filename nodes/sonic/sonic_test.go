// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sonic

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWireInterfaceCmds(t *testing.T) {
	tests := map[string]struct {
		ifNames []string
		want    []string
	}{
		"no interfaces": {
			ifNames: nil,
			want:    []string{},
		},
		"skips the management interface": {
			ifNames: []string{"eth0", "eth1"},
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
			},
		},
		"two interfaces": {
			ifNames: []string{"eth1", "eth2"},
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
				"ip link set arp off dev eth2",
				"sysctl -w net.ipv6.conf.eth2.disable_ipv6=1",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := wireInterfaceCmds(tc.ifNames)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("wireInterfaceCmds() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
