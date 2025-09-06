// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
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
    nokia_srlinux:
      vars:
        ansible_network_os: nokia.srlinux.srlinux
        # default connection type for nodes of this kind
        # feel free to override this in your inventory
        ansible_connection: ansible.netcommon.httpapi
        ansible_user: admin
        ansible_password: NokiaSrl1!
      hosts:
        clab-topo1-node1:
          ansible_host: 172.100.100.11
        clab-topo1-node2:
          ansible_host: 172.100.100.12`,
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
    nokia_srlinux:
      vars:
        ansible_network_os: nokia.srlinux.srlinux
        # default connection type for nodes of this kind
        # feel free to override this in your inventory
        ansible_connection: ansible.netcommon.httpapi
        ansible_user: admin
        ansible_password: NokiaSrl1!
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
          ansible_host: 172.100.100.11`,
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

			if diff := cmp.Diff(tc.want, s.String()); diff != "" {
				t.Errorf("failed at '%s', diff: (-want +got)\n%s", name, diff)
			}
		})
	}
}

func TestGeneratNornirSimpleInventory(t *testing.T) {
	tests := map[string]struct {
		got                              string
		want                             string
		clab_nornir_platform_name_schema string // environment variable feature flag to direct platform name
	}{
		"case1-default-platform": {
			got:                              "test_data/topo1.yml",
			clab_nornir_platform_name_schema: "",
			want: `---
node1:
    username: admin
    password: NokiaSrl1!
    platform: nokia_srlinux
    hostname: 172.100.100.11
node2:
    username: admin
    password: NokiaSrl1!
    platform: nokia_srlinux
    hostname: 172.100.100.12`,
		},
		"case2-scrapi-platform": {
			got:                              "test_data/topo12.yml",
			clab_nornir_platform_name_schema: "scrapi",
			want: `---
node1:
    username: admin
    password: admin
    platform: arista_eos
    hostname:
node2:
    username: admin
    password: admin
    platform: arista_eos
    hostname:
node3:
    username: admin
    password: admin
    platform: arista_eos
    hostname:
node4:
    username:
    password:
    platform: linux
    hostname: `,
		},
		"case3-nornir-platform": {
			got:                              "test_data/topo12.yml",
			clab_nornir_platform_name_schema: "napalm",
			want: `---
node1:
    username: admin
    password: admin
    platform: eos
    hostname:
node2:
    username: admin
    password: admin
    platform: eos
    hostname:
node3:
    username: admin
    password: admin
    platform: eos
    hostname:
node4:
    username:
    password:
    platform: linux
    hostname: `,
		},
		"case4-default-groups-platform": {
			got:                              "test_data/topo8_nornir_groups.yml",
			clab_nornir_platform_name_schema: "",
			want: `---
node4:
    username:
    password:
    platform: linux
    hostname: 172.100.100.14
node1:
    username: admin
    password: NokiaSrl1!
    platform: nokia_srlinux
    hostname: 172.100.100.11
    groups:
      - spine
node2:
    username: admin
    password: NokiaSrl1!
    platform: nokia_srlinux
    hostname: 172.100.100.12
    groups:
      - extra_group
      - second_extra_group
node3:
    username: admin
    password: NokiaSrl1!
    platform: nokia_srlinux
    hostname: 172.100.100.13
    groups:
      - extra_group`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set the environment variable
			t.Setenv("CLAB_NORNIR_PLATFORM_NAME_SCHEMA", tc.clab_nornir_platform_name_schema)

			opts := []ClabOption{
				WithTopoPath(tc.got, ""),
			}

			c, err := NewContainerLab(opts...)
			if err != nil {
				t.Fatal(err)
			}

			var s strings.Builder

			err = c.generateNornirSimpleInventory(&s)
			if err != nil {
				t.Fatal(err)
			}

			// compare "real" objects so we dont have to faff w/ whitespace or having editors
			// add/remove trailing whitespace in the want strings in the test table
			var gotData, wantData map[string]any

			err = yaml.Unmarshal([]byte(s.String()), &gotData)
			if err != nil {
				t.Fatalf("failed to unmarshal got YAML: %v", err)
			}

			err = yaml.Unmarshal([]byte(tc.want), &wantData)
			if err != nil {
				t.Fatalf("failed to unmarshal want YAML: %v", err)
			}

			if diff := cmp.Diff(wantData, gotData); diff != "" {
				t.Errorf("failed at '%s', diff: (-want +got)\n%s", name, diff)
			}
		})
	}
}
