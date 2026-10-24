// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package unmanaged_switch

import (
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestInitDefaults(t *testing.T) {
	n := new(unmanagedSwitch)
	if err := n.Init(&clabtypes.NodeConfig{ShortName: "sw1"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := new(unmanagedSwitch).Init(
		&clabtypes.NodeConfig{ShortName: "sw2", NetworkMode: "none"}); err != nil {
		t.Errorf("Init rejected network-mode none: %v", err)
	}

	if n.Cfg.Image != defaultImage || n.Cfg.Cmd != defaultCmd || n.Cfg.NetworkMode != "none" {
		t.Errorf("image, cmd, network-mode = %q, %q, %q, want %q, %q, %q",
			n.Cfg.Image, n.Cfg.Cmd, n.Cfg.NetworkMode, defaultImage, defaultCmd, "none")
	}

	if got := n.Cfg.Sysctls["net.ipv6.conf.default.disable_ipv6"]; got != "1" {
		t.Errorf("sysctl net.ipv6.conf.default.disable_ipv6 = %q, want %q", got, "1")
	}
}

func TestInitRejectsNetworkMode(t *testing.T) {
	for _, nm := range []string{"host", "bridge", "container:r1"} {
		n := new(unmanagedSwitch)
		if err := n.Init(&clabtypes.NodeConfig{ShortName: "sw1", NetworkMode: nm}); err == nil {
			t.Errorf("Init accepted network-mode %q", nm)
		}
	}
}

func TestCheckInterfaceName(t *testing.T) {
	tests := map[string]bool{
		"eth1":                  true,
		"eth0":                  false,
		"a-very-long-port-name": true,
	}
	for iface, valid := range tests {
		n := new(unmanagedSwitch)
		if err := n.Init(&clabtypes.NodeConfig{ShortName: "sw1"}); err != nil {
			t.Fatalf("Init: %v", err)
		}
		n.Endpoints = append(n.Endpoints,
			clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n, iface, nil)))

		if err := n.CheckInterfaceName(); (err == nil) != valid {
			t.Errorf("CheckInterfaceName() for %q = %v, want valid %v", iface, err, valid)
		}
	}
}
