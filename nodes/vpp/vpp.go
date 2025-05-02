// Copyright 2025 Pim van Pelt <pim@ipng.ch>
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vpp

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

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
	vppStartupConfigCmdsTpl string
	vppBootstrapCmdsTpl     string

	kindnames              = []string{"vpp"}
	defaultCredentials     = nodes.NewCredentials("admin", "admin")
	vppStartupConfigTpl, _ = template.New("clab-startup.conf").Funcs(utils.CreateFuncs()).
				Parse(vppStartupConfigCmdsTpl)
	vppBootstrapTpl, _ = template.New("bootstrap.vpp").Funcs(utils.CreateFuncs()).
				Parse(vppBootstrapCmdsTpl)
	saveCmd = `bash -c "echo TODO(pim): Not implemented yet - needs vppcfg in the Docker container"`
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindnames, func() nodes.Node {
		return new(vpp)
	}, nrea)
}

type vpp struct {
	nodes.DefaultNode
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
	// Path of the script to wait for all interfaces to be added in the container
	itfwaitpath   string
	StartupConfig string
	Bootstrap     string
}

func (n *vpp) PreDeploy(ctx context.Context, params *nodes.PreDeployParams) error {
	// store provided pubkeys
	n.sshPubKeys = params.SSHPubKeys

	// Generate the VPP startup config using a template
	if err := n.addStartupConfig(ctx); err != nil {
		return err
	}

	return nil
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
	n.itfwaitpath = path.Join(n.Cfg.LabDir, "if-wait.sh")
	utils.CreateFile(n.itfwaitpath, utils.IfWaitScript)
	os.Chmod(n.itfwaitpath, 0777)

	// Adding if-wait.sh script to the filesystem
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.itfwaitpath, ":", ifWaitScriptContainerPath))

	// Path to the VPP startup config file, used to start the dataplane
	n.StartupConfig = path.Join(n.Cfg.LabDir, "clab-startup.conf")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.StartupConfig, ":/etc/vpp/clab-startup.conf"))

	// Path to the VPP CLI bootstrap file, used to program the dataplane
	n.Bootstrap = path.Join(n.Cfg.LabDir, "clab.vpp")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.Bootstrap, ":/etc/vpp/clab.vpp"))

	// We need the interfaces with their correct name before launching the init process
	// prepending original Cmd with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	n.Cfg.Entrypoint = "bash -c '" + ifWaitScriptContainerPath + " ; exec /usr/bin/vpp -c /etc/vpp/clab-startup.conf'"

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

	log.Infof("saved VPP configuration from %s node\n", n.Cfg.ShortName)

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

const banner = `#######################################################################
# Welcome to FD.io's Vector Packet Processor                          #
#                                                                     #
# Most useful commands at that step:                                  #
#                                                                     #
# show interface        # for interface names, packet and state       #
# show runtime          # for the VPP runtime statistics              #
# ?                     # for the list of available commands          #
#                                                                     #
# Feel free to customize this banner using                            #
# cmd banner post-login message                                       #
#######################################################################`

// VPP template data top level data struct.
type vppTemplateData struct {
	Banner     string
	SSHPubKeys []string
}

// addDefaultConfig adds VSR default configuration.
func (n *vpp) addStartupConfig(ctx context.Context) error {
	buf := new(bytes.Buffer)
	err := vppStartupConfigTpl.Execute(buf, n)
	if err != nil {
		return err
	}

	log.Debugf("Node %q additional config:\n%s", n.Cfg.ShortName, buf.String())

	out, err := os.OpenFile(n.StartupConfig, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("failed to open VPP startup config file: %v", err)
	}
	defer out.Close()

	b, err := out.WriteString(buf.String())
	if err != nil {
		log.Errorf("failed to write in the file: %v", err)
	}
	log.Debugf("Wrote %d bytes", b)

	return nil
}
