// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package plvision_sonic

import (
	"context"
	"fmt"
	"path"

	"github.com/charmbracelet/log"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"plvision_sonic"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	generateable     = true
	generateIfFormat = "eth%d"
)

const (
	configDirName   = "config"
	startupCfgFName = "config_db.json"
	saveCmd         = `sh -c "/backup.sh -u $USERNAME -p $PASSWORD backup"`

	scrapliPlatformName = "sonic"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformAttrs,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(plvision_sonic)
	}, nrea)
}

type plvision_sonic struct {
	clabnodes.DefaultNode
}

func (n *plvision_sonic) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	defEnv := map[string]string{
		"USERNAME":           n.Cfg.Credentials.Username,
		"PASSWORD":           n.Cfg.Credentials.Password,
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"),
	)

	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
	)

	return nil
}

func (n *plvision_sonic) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	return clabnodes.LoadStartupConfigFileVr(n, configDirName, startupCfgFName)
}

func (n *plvision_sonic) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	cmd, err := clabexec.NewExecCmdFromString(saveCmd)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create execute cmd: %w", n.Cfg.ShortName, err)
	}

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute cmd: %w", n.Cfg.ShortName, err)
	}

	if execResult.GetStdErrString() != "" {
		return nil, fmt.Errorf("%s errors: %s", n.Cfg.ShortName, execResult.GetStdErrString())
	}

	confPath := n.Cfg.LabDir + "/" + configDirName
	log.Infof(
		"saved /etc/sonic/config_db.json backup from %s node to %s\n",
		n.Cfg.ShortName,
		confPath,
	)

	return nil, nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file is correct.
func (n *plvision_sonic) CheckInterfaceName() error {
	return clabnodes.GenericVMInterfaceCheck(n.Cfg.ShortName, n.Endpoints)
}
