// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_openbsd

import (
	"context"
	"fmt"
	"path"
	"regexp"

	"github.com/charmbracelet/log"
	containerlabexec "github.com/srl-labs/containerlab/exec"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"openbsd"}
	defaultCredentials = containerlabnodes.NewCredentials("admin", "admin")
	saveCmd            = "sh -c \"/backup.sh -u $USERNAME -p $PASSWORD backup\""

	InterfaceRegexp = regexp.MustCompile(`vio(?P<port>\d+)`)
	InterfaceOffset = 1
	InterfaceHelp   = "vioX (where X >= 1) or ethX (where X >= 1)"
)

const (
	configDirName = "config"

	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	generateNodeAttributes := containerlabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindNames, func() containerlabnodes.Node {
		return new(vrOpenBSD)
	}, nrea)
}

type vrOpenBSD struct {
	containerlabnodes.VRNode
}

func (n *vrOpenBSD) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *containerlabnodes.NewVRNode(n, defaultCredentials, "")
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    containerlabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = containerlabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support config backup functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"))

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *vrOpenBSD) PreDeploy(_ context.Context, params *containerlabnodes.PreDeployParams) error {
	containerlabutils.CreateDirectory(n.Cfg.LabDir, 0o777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return err
}

func (n *vrOpenBSD) SaveConfig(ctx context.Context) error {
	cmd, _ := containerlabexec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", n.Cfg.ShortName, err)
	}

	if execResult.GetStdErrString() != "" {
		return fmt.Errorf("%s errors: %s", n.Cfg.ShortName, execResult.GetStdErrString())
	}

	confPath := n.Cfg.LabDir + "/" + configDirName
	log.Infof("saved /etc backup from %s node to %s\n", n.Cfg.ShortName, confPath)

	return nil
}
