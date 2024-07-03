// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause
// The author of this code is Adam Kulagowski (adam.kulagowski@codilime.com), CodiLime (codilime.com).

package sonic_vm

import (
	"context"
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"sonic-vm"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")
)

const (
	configDirName   = "config"
	startupCfgFName = "config_db.json"
	saveCmd         = `sh -c "/backup.sh -u $USERNAME -p $PASSWORD backup"`
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(sonic_vm)
	}, defaultCredentials)
}

type sonic_vm struct {
	nodes.DefaultNode
}

func (n *sonic_vm) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = utils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"))

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	return nil
}

func (n *sonic_vm) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		log.Errorf("Error handling certifcate for %s: %v", n.Cfg.ShortName, err)

		return nil
	}

	return nodes.LoadStartupConfigFileVr(n, configDirName, startupCfgFName)
}

func (n *sonic_vm) SaveConfig(ctx context.Context) error {
	cmd, err := exec.NewExecCmdFromString(saveCmd)
	if err != nil {
		return fmt.Errorf("%s: failed to create execute cmd: %w", n.Cfg.ShortName, err)
	}

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %w", n.Cfg.ShortName, err)
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("%s errors: %s", n.Cfg.ShortName, execResult.GetStdErrString())
	}

	confPath := n.Cfg.LabDir + "/" + configDirName
	log.Infof("saved /etc/sonic/config_db.json backup from %s node to %s\n", n.Cfg.ShortName, confPath)

	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *sonic_vm) CheckInterfaceName() error {
	return nodes.GenericVMInterfaceCheck(n.Cfg.ShortName, n.Endpoints)
}
