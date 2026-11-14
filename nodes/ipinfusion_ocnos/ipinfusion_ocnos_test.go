// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ipinfusion_ocnos

import (
	"path/filepath"
	"slices"
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestInitMountsConfigDir(t *testing.T) {
	labDir := t.TempDir()
	n := new(IPInfusionOcNOS)

	err := n.Init(&clabtypes.NodeConfig{
		ShortName:   "r1",
		LabDir:      labDir,
		Credentials: clabtypes.NodeCredentials{Username: "admin", Password: "admin@123"},
	}, clabnodes.WithMgmtNet(&clabtypes.MgmtNet{}))
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// vrnetlab's OcNOS launcher applies /config/startup-config.cfg, so the
	// lab config dir must be mounted there.
	want := filepath.Join(labDir, "config") + ":/config"
	if !slices.Contains(n.Cfg.Binds, want) {
		t.Errorf("Binds = %v, want it to contain %q", n.Cfg.Binds, want)
	}
}
