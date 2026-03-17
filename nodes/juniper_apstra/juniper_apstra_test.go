// Copyright 2024 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package juniper_apstra

import (
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// TestApstraInit verifies that Init() sets the expected fields on the node.
// Apstra has no data-plane interfaces so there is no interface-mapping test
// (contrast with vr_vjunosswitch which tests ge-0/0/X → ethX mapping).
func TestApstraInit(t *testing.T) {
	tests := map[string]struct {
		shortName string
		wantCmd   string
	}{
		"default-node-name": {
			shortName: "apstra",
			wantCmd: "--username admin --password admin " +
				"--hostname apstra --connection-mode tc --trace",
		},
		"custom-node-name": {
			shortName: "apstra-controller",
			wantCmd: "--username admin --password admin " +
				"--hostname apstra-controller --connection-mode tc --trace",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			node := &vrApstra{}

			cfg := &clabtypes.NodeConfig{
				ShortName: tc.shortName,
				LabDir:    t.TempDir(),
				Env:       map[string]string{},
				Mgmt: &clabtypes.MgmtNet{
					IPv4Subnet: "172.20.20.0/24",
					IPv6Subnet: "",
				},
			}

			err := node.Init(cfg, clabnodes.WithMgmtNet(&clabtypes.MgmtNet{
				IPv4Subnet: "172.20.20.0/24",
			}))
			if err != nil {
				t.Fatalf("Init() returned unexpected error: %v", err)
			}

			// Verify the launch.py command line is built correctly.
			if node.Cfg.Cmd != tc.wantCmd {
				t.Errorf("Cfg.Cmd = %q, want %q", node.Cfg.Cmd, tc.wantCmd)
			}

			// Verify VirtRequired is set — Apstra needs KVM.
			if !node.HostRequirements.VirtRequired {
				t.Error("HostRequirements.VirtRequired should be true for Apstra")
			}

			// Verify the /state bind mount is present.
			foundStateBind := false
			for _, bind := range node.Cfg.Binds {
				if len(bind) > 6 && bind[len(bind)-7:] == ":/state" {
					foundStateBind = true
					break
				}
			}
			if !foundStateBind {
				t.Errorf("expected a :/state bind mount in Cfg.Binds, got: %v", node.Cfg.Binds)
			}
		})
	}
}
