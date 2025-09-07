// Copyright 2025 Pim van Pelt <pim@ipng.ch>
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package fdio_vpp

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	targetAuthzKeysPath = "/root/.ssh/authorized_keys"

	ifWaitScriptDstPath  = "/usr/sbin/if-wait.sh"
	vppStartupCfgDstPath = "/etc/vpp/startup.conf"

	vppCfgDstPath = "/etc/vpp/vppcfg.yaml"
)

var (
	kindNames          = []string{"fdio_vpp"}
	defaultCredentials = clabnodes.NewCredentials("root", "vpp")
	saveCmd            = `bash -c "echo TODO(pim): Not implemented yet - needs vppcfg in the Docker container"`

	// vppStartupConfigTpl is the template for the vpp startup config itself
	// (plugins, page sizes, etc)
	//go:embed vpp_startup_config.go.tpl
	vppStartupConfigTpl string
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(fdio_vpp)
	}, nrea)
}

type fdio_vpp struct {
	clabnodes.DefaultNode
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
	// Path of the script to wait for all interfaces to be added in the container
	ifWaitSrcPath string
	// Path to the vpp startup config file
	vppStartupCfgSrcPath string
	// Path to vpp config file
	vppCfgSrcPath string
}

func (n *fdio_vpp) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	// Containers are run in privileged mode so it should not matter now
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
	)

	// Adding if-wait.sh script mount
	n.ifWaitSrcPath = path.Join(n.Cfg.LabDir, "if-wait.sh")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.ifWaitSrcPath, ":", ifWaitScriptDstPath))

	// Path to the VPP startup config file, used to start the dataplane
	n.vppStartupCfgSrcPath = path.Join(n.Cfg.LabDir, "vpp-startup.conf")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.vppStartupCfgSrcPath, ":", vppStartupCfgDstPath))

	// Path to the VPP config file that configures the vpp interfaces/etc
	n.vppCfgSrcPath = path.Join(n.Cfg.LabDir, "vppcfg.yaml")
	n.Cfg.ResStartupConfig = n.vppCfgSrcPath
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.Cfg.ResStartupConfig, ":", vppCfgDstPath))

	// We need the interfaces with their correct name before launching the init process
	// prepending original CMD with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	n.Cfg.Entrypoint = "bash -c '" + ifWaitScriptDstPath + " ; exec /sbin/init-container.sh'"

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *fdio_vpp) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	nodeCfg := n.Config()

	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	clabutils.CreateFile(n.ifWaitSrcPath, clabutils.IfWaitScript)
	os.Chmod(n.ifWaitSrcPath, clabconstants.PermissionsOpen)

	// record pubkeys extracted by clab
	// with the vpp struct
	n.sshPubKeys = params.SSHPubKeys

	// handle vpp config provided in yaml file
	// and mounted to /etc/vpp/vppcfg.yaml
	var vppCfgTpl string

	// use the startup config file provided by a user
	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		vppCfgTpl = string(c)
	}

	err := n.GenerateConfig(n.Cfg.ResStartupConfig, vppCfgTpl)
	if err != nil {
		return err
	}

	// template the vpp dataplane config (aka vpp startup config)
	err = n.GenerateConfig(n.vppStartupCfgSrcPath, vppStartupConfigTpl)
	if err != nil {
		return err
	}

	return nil
}

func (n *fdio_vpp) SaveConfig(ctx context.Context) error {
	cmd, _ := clabexec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if execResult.GetStdErrString() != "" {
		return fmt.Errorf("show config command failed: %s", execResult.GetStdErrString())
	}

	log.Infof("Saved VPP configuration from %s node\n", n.Cfg.ShortName)

	return nil
}

// CheckInterfaceName allows any interface name for vpp nodes, but checks
// if eth0 is only used with network-mode=none.
func (n *fdio_vpp) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf("eth0 interface name is not allowed for %s node when network mode is not set to none", n.Cfg.ShortName)
		}
	}
	return nil
}

func (n *fdio_vpp) PostDeploy(ctx context.Context, params *clabnodes.PostDeployParams) error {
	// add public keys extracted by containerlab from the host
	// to the vpp's root linux user authorized keys
	// to enable passwordless ssh
	keys := strings.Join(clabutils.MarshalSSHPubKeys(n.sshPubKeys), "\n")
	execCmd := clabexec.NewExecCmdFromSlice([]string{
		"bash", "-c",
		fmt.Sprintf("echo '%s' > %s", keys, targetAuthzKeysPath),
	})
	_, err := n.RunExec(ctx, execCmd)

	return err
}
