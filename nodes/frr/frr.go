// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var kindNames = []string{"frr", "frrouting"}

const (
	generateable     = true
	generateIfFormat = "eth%d"

	// cfgDir holds the three files bind mounted over the container's /etc/frr.
	cfgDir = "config"

	frrConfFile   = "frr.conf"
	daemonsFile   = "daemons"
	vtyshConfFile = "vtysh.conf"

	etcFRR = "/etc/frr"

	authzKeysPath = "/root/.ssh/authorized_keys"

	// no-header keeps the "Building configuration..." preamble out of the
	// saved file, which is written back as the node's frr.conf.
	saveCmd = `vtysh -c "show running-config no-header"`
)

var (
	//go:embed frr.cfg
	defaultCfgTemplate string

	// vtyshCfg turns on integrated config so that frr.conf is the single
	// configuration file, matching what FRR ships upstream.
	//go:embed vtysh.conf
	vtyshCfg string
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)

	// FRR has no scrapli or napalm platform, so no PlatformAttrs are set.
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(frr)
	}, nrea)
}

type frr struct {
	clabnodes.DefaultNode

	sshPubKeys []ssh.PublicKey
}

func (n *frr) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// The image ships neither frr.conf nor vtysh.conf, and vtysh refuses to
	// start without them, so all three files are always mounted.
	for _, f := range []string{frrConfFile, daemonsFile, vtyshConfFile} {
		n.Cfg.Binds = append(n.Cfg.Binds,
			fmt.Sprint(filepath.Join(n.Cfg.LabDir, cfgDir, f), ":", filepath.Join(etcFRR, f)),
		)
	}

	// FRR programs routes into the kernel, which only forwards if asked to.
	n.Cfg.Sysctls["net.ipv4.ip_forward"] = "1"
	n.Cfg.Sysctls["net.ipv6.conf.all.forwarding"] = "1"

	return nil
}

func (n *frr) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	// Recorded here and written to the container in PostDeploy, once it runs.
	n.sshPubKeys = params.SSHPubKeys

	return n.createFRRFiles()
}

func (n *frr) createFRRFiles() error {
	nodeCfg := n.Config()

	dir := filepath.Join(nodeCfg.LabDir, cfgDir)
	clabutils.CreateDirectory(dir, clabconstants.PermissionsOpen)

	// frr.conf comes from the user's startup-config when given, and from the
	// bundled template otherwise. Routing it through GenerateConfig is what
	// makes enforce-startup-config, suppress-startup-config and templating
	// behave the same way here as for every other kind.
	cfgTemplate := defaultCfgTemplate

	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}

		cfgTemplate = string(c)
	}

	err := n.GenerateConfig(filepath.Join(dir, frrConfFile), cfgTemplate)
	if err != nil {
		return fmt.Errorf("node=%s, failed to generate config: %w", nodeCfg.ShortName, err)
	}

	var daemons []string
	if nodeCfg.Extras != nil && nodeCfg.Extras.FRR != nil {
		daemons = nodeCfg.Extras.FRR.Daemons
	}

	rendered, err := renderDaemons(daemons)
	if err != nil {
		return fmt.Errorf("node=%s: %w", nodeCfg.ShortName, err)
	}

	err = clabutils.CreateFile(filepath.Join(dir, daemonsFile), rendered)
	if err != nil {
		return fmt.Errorf("node=%s, failed to write daemons file: %w", nodeCfg.ShortName, err)
	}

	err = clabutils.CreateFile(filepath.Join(dir, vtyshConfFile), vtyshCfg)
	if err != nil {
		return fmt.Errorf("node=%s, failed to write vtysh.conf: %w", nodeCfg.ShortName, err)
	}

	return nil
}

// PostDeploy adds the public keys containerlab collected from the host to the
// root user's authorized_keys, to enable passwordless ssh.
func (n *frr) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	if len(n.sshPubKeys) == 0 {
		return nil
	}

	log.Debugf("Running postdeploy actions for frr %q node", n.Cfg.ShortName)

	keys := strings.Join(clabutils.MarshalSSHPubKeys(n.sshPubKeys), "\n")

	// The file is removed rather than truncated: a redirect onto an existing
	// file keeps that file's ownership, and sshd rejects an authorized_keys
	// that root does not own.
	script := fmt.Sprintf(`set -e
mkdir -p %[1]s
chmod 700 %[1]s
rm -f %[2]s
printf '%%s\n' %[3]q > %[2]s
chown root:root %[2]s
chmod 600 %[2]s`,
		filepath.Dir(authzKeysPath), authzKeysPath, keys)

	cmd := clabexec.NewExecCmdFromSlice([]string{"bash", "-c", script})

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to add ssh keys to node %q: %w", n.Cfg.ShortName, err)
	}

	if execResult.GetReturnCode() != 0 {
		return fmt.Errorf("failed to add ssh keys to node %q: %s",
			n.Cfg.ShortName, execResult.GetStdErrString())
	}

	return nil
}

func (n *frr) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	cmd, _ := clabexec.NewExecCmdFromString(saveCmd)

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	if execResult.GetReturnCode() != 0 {
		return nil, fmt.Errorf("failed to save config on node %q: %s",
			n.Cfg.ShortName, execResult.GetStdErrString())
	}

	confPath := filepath.Join(n.Cfg.LabDir, cfgDir, frrConfFile)

	err = os.WriteFile(confPath, execResult.GetStdOutByteSlice(),
		clabconstants.PermissionsFileDefault)
	if err != nil {
		return nil, fmt.Errorf("failed to write config by %s path from %s container: %w",
			confPath, n.Cfg.ShortName, err)
	}

	log.Infof("saved FRR configuration from %s node to %s\n", n.Cfg.ShortName, confPath)

	return nil, nil
}
