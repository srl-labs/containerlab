// Copyright 2024 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package juniper_apstra

import (
	"fmt"
	"path"
	"os"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"juniper_apstra"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")
)

const (
	// Apstra does not expose a Scrapli/NAPALM platform — it is managed
	// via its REST API and Web UI, not via CLI.
	scrapliPlatformName = ""
	NapalmPlatformName  = ""

	// Apstra has no data-plane interfaces so topology generation is
	// not applicable.
	generateable     = false
	generateIfFormat = ""

	// stateDirName is the directory inside the node's lab dir that is
	// bind-mounted to /state inside the container.  launch.py writes
	// the persistent QEMU overlay there so VM state survives clab destroy.
	stateDirName = "state"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	// Apstra has no topology-generation support and no Scrapli/NAPALM
	// platform, so generateNodeAttributes and platformAttrs are minimal.
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		nil, // no platform attributes — Apstra is REST/Web UI managed
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(vrApstra)
	}, nrea)
}

type vrApstra struct {
	clabnodes.VRNode
}

func (n *vrApstra) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode — this sets up LabDir, ConfigDirName, and all base
	// node infrastructure including pre-creation of the lab directory.
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// Apstra is a KVM VM — hardware virtualisation is required.
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// env vars are forwarded to launch.py arguments in the vrnetlab container.
	defEnv := map[string]string{
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// Mount the state directory to /state inside the container so that
	// launch.py can write the persistent QEMU overlay there.
	// n.Cfg.LabDir is pre-created by Containerlab before Init() runs,
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, stateDirName), ":/state"),
	)
	
	// Pre-create the state subdirectory so the bind mount succeeds.
    // Containerlab creates LabDir automatically but not subdirectories.
    stateDir := path.Join(n.Cfg.LabDir, stateDirName)
    if err := os.MkdirAll(stateDir, 0755); err != nil {
        return fmt.Errorf("failed to create state directory %s: %w", stateDir, err)
    }

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	// Build the launch.py command line.
	// --hostname is accepted by launch.py (added for generic_vm compatibility)
	// but not used by Apstra internally.
	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
	)

	// Apstra has no data-plane interfaces — leave InterfaceRegexp,
	// InterfaceOffset, and InterfaceHelp at their zero values.

	return nil
}
