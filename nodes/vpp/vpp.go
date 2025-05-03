// Copyright 2025 Pim van Pelt <pim@ipng.ch>
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vpp

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"

	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	ifWaitScriptContainerPath = "/usr/sbin/if-wait.sh"
	generateable              = true
	generateIfFormat          = "eth%d"
)

var (
	kindNames          = []string{"vpp"}
	defaultCredentials = nodes.NewCredentials("root", "vpp")
	saveCmd            = `bash -c "echo TODO(pim): Not implemented yet - needs vppcfg in the Docker container"`
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindNames, func() nodes.Node {
		return new(vpp)
	}, nrea)
}

type vpp struct {
	nodes.DefaultNode
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
	// Path of the script to wait for all interfaces to be added in the container
	itfWaitPath string
}

func (n *vpp) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	n.Cfg = cfg

	// Containers are run in priviledge mode so it should not matter now
	// If it changes, add the capabilities to run VPP
	n.Cfg.CapAdd = append(n.Cfg.CapAdd,
		"NET_ADMIN",
		"SYS_ADMIN",
		"SYS_NICE",
		"IPC_LOCK",
		"SYSLOG",
		"SYS_PTRACE",
	)

	// Devices to be mapped to the container
	n.Cfg.Devices = append(n.Cfg.Devices,
		"/dev/net/tun",
		"/dev/vhost-net",
		"/dev/vfio/vfio",
	)

	// Creating if-wait script in lab dir
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	n.itfWaitPath = path.Join(n.Cfg.LabDir, "if-wait.sh")
	utils.CreateFile(n.itfWaitPath, utils.IfWaitScript)
	os.Chmod(n.itfWaitPath, 0777)

	// Adding if-wait.sh script to the filesystem
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.itfWaitPath, ":", ifWaitScriptContainerPath))

	// We need the interfaces with their correct name before launching the init process
	// prepending original CMD with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	n.Cfg.Entrypoint = "bash -c '" + ifWaitScriptContainerPath + " ; exec /sbin/init-container.sh'"

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *vpp) SaveConfig(ctx context.Context) error {
	cmd, _ := exec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("show config command failed: %s", execResult.GetStdErrString())
	}

	log.Infof("Saved VPP configuration from %s node\n", n.Cfg.ShortName)

	return nil
}

// CheckInterfaceName allows any interface name for vpp nodes, but checks
// if eth0 is only used with network-mode=none.
func (n *vpp) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf("eth0 interface name is not allowed for %s node when network mode is not set to none", n.Cfg.ShortName)
		}
	}
	return nil
}
