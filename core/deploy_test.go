// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"strings"
	"testing"
)

func TestCheckReconcileDeployOptionsRejectsManagementNetworkOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		option ClabOption
	}{
		{name: "network name", option: WithManagementNetworkName("other")},
		{name: "IPv4 subnet", option: WithManagementIpv4Subnet("10.0.0.0/24")},
		{name: "IPv6 subnet", option: WithManagementIpv6Subnet("2001:db8::/64")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := NewContainerLab(tt.option)
			if err != nil {
				t.Fatal(err)
			}
			c.Config.Name = "lab"

			err = c.checkReconcileDeployOptions(&DeployOptions{})
			if err == nil || !strings.Contains(err.Error(), "management network overrides") {
				t.Fatalf("expected management network override error, got %v", err)
			}
		})
	}
}

func TestCheckReconcileDeployOptionsAllowsTopologyManagementNetwork(t *testing.T) {
	t.Parallel()

	c, err := NewContainerLab()
	if err != nil {
		t.Fatal(err)
	}
	c.Config.Name = "lab"
	c.Config.Mgmt.Network = "from-topology"
	c.Config.Mgmt.IPv4Subnet = "10.0.0.0/24"
	c.Config.Mgmt.IPv6Subnet = "2001:db8::/64"

	if err := c.checkReconcileDeployOptions(&DeployOptions{}); err != nil {
		t.Fatalf("topology management settings must not be treated as overrides: %v", err)
	}
}
