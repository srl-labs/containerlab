// Copyright 2025 6WIND S.A.
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sixwind_vsr

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

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	ifWaitScriptContainerPath = "/usr/sbin/if-wait.sh"
	generateable              = true
	generateIfFormat          = "eth%d"
)

var (
	// additional config that clab adds on top of the init config.
	//go:embed 6wind_vsr_default_config.go.tpl
	vsrConfigCmdsTpl string

	kindnames          = []string{"6wind_vsr"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")
	vsrCfgTpl, _       = template.New("clab-vsr-default-config").Funcs(clabutils.CreateFuncs()).
				Parse(vsrConfigCmdsTpl)
	saveCmd = `bash -c "echo 'show config nodefault fullpath' | nc-cli"`
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindnames, func() clabnodes.Node {
		return new(sixwind_vsr)
	}, nrea)
}

type sixwind_vsr struct {
	clabnodes.DefaultNode
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
	// Path of the script to wait for all interfaces to be added in the container
	itfwaitpath        string
	ConsolidatedConfig string
	UserStartupConfig  string
}

func (n *sixwind_vsr) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	// store provided pubkeys
	n.sshPubKeys = params.SSHPubKeys

	// If user-defined startup exists, copy it into the Consolidated config file
	if clabutils.FileExists(n.UserStartupConfig) {
		clabutils.CopyFile(ctx, n.UserStartupConfig, n.ConsolidatedConfig,
			clabconstants.PermissionsFileDefault)
	} else {
		if n.Cfg.StartupConfig != "" {
			// Copy startup-config in the Labdir
			clabutils.CopyFile(ctx, n.Cfg.StartupConfig, n.ConsolidatedConfig,
				clabconstants.PermissionsFileDefault)
		}
		// Consolidate the startup config with the default template
		if err := n.addDefaultConfig(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (n *sixwind_vsr) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	// Default value for 6WIND VSR
	if n.Cfg.ShmSize == "" {
		n.Cfg.ShmSize = "512M"
	}

	// Containers are run in privileged mode so it should not matter now
	// If it changes, add the capabilities to run 6WIND VSR
	n.Cfg.CapAdd = append(n.Cfg.CapAdd,
		"NET_ADMIN",
		"SYS_ADMIN",
		"NET_BROADCAST",
		"NET_RAW",
		"SYS_NICE",
		"IPC_LOCK",
		"SYSLOG",
		"AUDIT_WRITE",
		"SYS_PTRACE",
		"SYS_TIME",
	)

	// Devices to be mapped to the container
	n.Cfg.Devices = append(n.Cfg.Devices,
		"/dev/net/tun",
		"/dev/vhost-net",
		"/dev/ppp",
	)

	// Creating if-wait script in lab dir
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	n.itfwaitpath = path.Join(n.Cfg.LabDir, "if-wait.sh")
	clabutils.CreateFile(n.itfwaitpath, clabutils.IfWaitScript)
	os.Chmod(n.itfwaitpath, clabconstants.PermissionsOpen)

	// Adding if-wait.sh script to the filesystem
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.itfwaitpath, ":", ifWaitScriptContainerPath))

	// Path to the consolidated config file (startup + default will be aggregated)
	n.ConsolidatedConfig = path.Join(n.Cfg.LabDir, "init-config.cli")

	// Path to the user persistent config
	n.UserStartupConfig = path.Join(n.Cfg.LabDir, "user-startup.conf")

	// Mount startup config
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.ConsolidatedConfig, ":/etc/init-config.cli"))

	// We need the interfaces with their correct name before launching the init process
	// prepending original Cmd with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	n.Cfg.Entrypoint = "bash -c '" + ifWaitScriptContainerPath + " ; exec /sbin/init-container.sh'"

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *sixwind_vsr) SaveConfig(ctx context.Context) error {
	cmd, _ := clabexec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if execResult.GetStdErrString() != "" {
		return fmt.Errorf("show config command failed: %s", execResult.GetStdErrString())
	}

	err = os.WriteFile(n.UserStartupConfig, execResult.GetStdOutByteSlice(),
		clabconstants.PermissionsOpen)
	if err != nil {
		return fmt.Errorf("failed to write config by %s path from %s container: %v",
			n.UserStartupConfig, n.Cfg.ShortName, err)
	}
	log.Infof("saved 6WIND VSR configuration from %s node to %s\n", n.Cfg.ShortName, n.UserStartupConfig)

	return nil
}

// CheckInterfaceName allows any interface name for 6wind_vsr nodes, but checks
// if eth0 is only used with network-mode=none.
func (n *sixwind_vsr) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf("eth0 interface name is not allowed for %s node when network mode is not set to none", n.Cfg.ShortName)
		}
	}
	return nil
}

const banner = `#######################################################################
# Welcome to 6WIND Virtual Service Router                             #
#                                                                     #
# Most useful commands at that step:                                  #
#                                                                     #
# edit running          # to edit the running configuration           #
# show interface        # for interface names, state and IP addresses #
# show summary          # for the vRouter state summary               #
# ?                     # for the list of available commands          #
# help <cmd>            # for detailed help on the <cmd> command      #
#                                                                     #
# Feel free to customize this banner using                            #
# cmd banner post-login message                                       #
#######################################################################
`

// 6WIND VSR template data top level data struct.
type vsrTemplateData struct {
	Banner     string
	License    string
	SSHPubKeys []string
}

// addDefaultConfig adds VSR default configuration.
func (n *sixwind_vsr) addDefaultConfig(_ context.Context) error {
	// tplData holds data used in templating of the default config snippet
	tplData := vsrTemplateData{
		Banner:     banner,
		SSHPubKeys: clabutils.MarshalSSHPubKeys(n.sshPubKeys),
	}

	if n.Cfg.License != "" {
		content, err := os.ReadFile(n.Cfg.License)
		if err != nil {
			return err
		}
		tplData.License = strings.TrimSuffix(string(content), "\n")
	}

	buf := new(bytes.Buffer)
	err := vsrCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}

	log.Debugf("Node %q additional config:\n%s", n.Cfg.ShortName, buf.String())

	out, err := os.OpenFile(n.ConsolidatedConfig, os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		clabconstants.PermissionsFileDefault)
	if err != nil {
		log.Errorf("failed to open consolidated config file: %v", err)
	}
	defer out.Close()

	b, err := out.WriteString(buf.String())
	if err != nil {
		log.Errorf("failed to write in the file: %v", err)
	}
	log.Debugf("Wrote %d bytes", b)

	return nil
}
