// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_freebsd

import (
	"context"
	"fmt"
	"path"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"freebsd"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")
	saveCmd            = "sh -c \"/backup.sh -u $USERNAME -p $PASSWORD backup\""
	InterfaceRegexp    = regexp.MustCompile(`vtnet(?P<port>\d+)`)
	InterfaceOffset    = 1
	InterfaceHelp      = "vtnetX (where X >= 1) or ethX (where X >= 1)"
)

const (
	configDirName = "config"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(vrFreeBSD)
	}, defaultCredentials)
}

type vrFreeBSD struct {
	nodes.VRNode
}

func (n *vrFreeBSD) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *nodes.NewVRNode(n)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = utils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support config backup functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"))

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

func (n *vrFreeBSD) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return err
}

func (n *vrFreeBSD) SaveConfig(ctx context.Context) error {
	cmd, _ := exec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", n.Cfg.ShortName, err)
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("%s errors: %s", n.Cfg.ShortName, execResult.GetStdErrString())
	}

	confPath := n.Cfg.LabDir + "/" + configDirName
	log.Infof("saved /etc backup from %s node to %s\n", n.Cfg.ShortName, confPath)

	return nil
}
