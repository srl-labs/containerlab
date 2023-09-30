// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateAnsibleInventory(t *testing.T) {
	tests := map[string]struct {
		got  string
		want string
	}{
		"case1": {
			got: "test_data/topo1.yml",
			want: `all:
  vars:
    # The generated inventory is assumed to be used from the clab host.
    # Hence no http proxy should be used. Therefore we make sure the http
    # module does not attempt using any global http proxy.
    ansible_httpapi_use_proxy: false
  children:
    srl:
      hosts:
        clab-topo1-node1:
          ansible_host: 172.100.100.11
        clab-topo1-node2:
          ansible_host: 172.100.100.12
`,
		},
		"case2": {
			got: "test_data/topo8_ansible_groups.yml",
			want: `all:
  vars:
    # The generated inventory is assumed to be used from the clab host.
    # Hence no http proxy should be used. Therefore we make sure the http
    # module does not attempt using any global http proxy.
    ansible_httpapi_use_proxy: false
  children:
    linux:
      hosts:
        clab-topo8_ansible_groups-node4:
    srl:
      hosts:
        clab-topo8_ansible_groups-node1:
          ansible_host: 172.100.100.11
        clab-topo8_ansible_groups-node2:
          ansible_host: 172.100.100.12
        clab-topo8_ansible_groups-node3:
          ansible_host: 172.100.100.13
    extra_group:
      hosts:
        clab-topo8_ansible_groups-node2:
          ansible_host: 172.100.100.12
        clab-topo8_ansible_groups-node3:
          ansible_host: 172.100.100.13
    spine:
      hosts:
        clab-topo8_ansible_groups-node1:
          ansible_host: 172.100.100.11
`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoPath(tc.got, ""),
			}
			c, err := NewContainerLab(opts...)
			if err != nil {
				t.Fatal(err)
			}

			var s strings.Builder
			err = c.generateAnsibleInventory(&s)
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(s.String(), tc.want) {
				t.Errorf("failed at '%s', expected\n%v, got\n%+v", name, tc.want, s.String())
			}
		})
	}
}
