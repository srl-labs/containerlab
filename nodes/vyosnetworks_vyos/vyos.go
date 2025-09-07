// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vyosnetworks_vyos

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	scrapliPlatformName = "vyatta_vyos"
	NapalmPlatformName  = "vyos"

	vyattacfg_gid = 102
)

var (
	KindNames          = []string{"vyosnetworks_vyos"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	//go:embed vyos.config.boot
	cfgTemplate string

	// The container CLI may become available while the init is still committing
	// so the python makes it wait till commits are complete.
	saveCmd = []string{
		"sudo python3 -c \"import sys;sys.path.insert(1, '/usr/lib/python3/dist-packages/vyos/utils/');from commit import wait_for_commit_lock;wait_for_commit_lock()\"",
		"commit",
		"save",
	}
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	log.Debug("Registering vyos ")
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, platformAttrs)

	r.Register(KindNames, func() clabnodes.Node {
		return new(vyos)
	}, nrea)
}

type vyos struct {
	clabnodes.DefaultNode
	configDir  string
	SSHPubKeys []ssh.PublicKey
	creds      *clabnodes.Credentials
}

func (n *vyos) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	log.Debug("Initializating Vyos node")
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.creds = defaultCredentials

	// mount config dir
	n.configDir = filepath.Join(n.Cfg.LabDir, "config")
	n.Cfg.ResStartupConfig = filepath.Join(n.configDir, "config.boot")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/opt/vyatta/etc/config", n.configDir))
	return nil
}

func (n *vyos) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	if err := n.fixdirACL(); err != nil {
		return err
	}

	issueTrue := true
	n.Cfg.Certificate = &clabtypes.CertificateConfig{
		Issue: &issueTrue,
	}
	cert, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	key, err := pkcs1To8(cert.Key)
	if err != nil {
		return err
	}

	ca, err := params.Cert.LoadCaCert()
	if err != nil {
		return err
	}

	n.Cfg.TLSCert = string(cert.Cert)
	n.Cfg.TLSKey = string(key)
	n.Cfg.TLSAnchor = string(ca.Cert)

	n.SSHPubKeys = params.SSHPubKeys

	return n.createVyosFiles(ctx)
}

func (n *vyos) SaveConfig(ctx context.Context) error {
	cli, err := n.newCli()
	if err != nil {
		return err
	}
	defer cli.Close()

	if err := n.save(ctx, cli); err != nil {
		return err
	}

	return nil
}

func (n *vyos) PostDeploy(ctx context.Context, params *clabnodes.PostDeployParams) error {
	nodeCfg := n.Config()
	cli, err := n.newCli()
	if err != nil {
		return err
	}

	defer cli.Close()

	log.Debug("Configuring management interface", "int", nodeCfg.MgmtIntf)

	var cfgs []string

	addressCmd := func(a string, p int) string {
		log.Debug("Setting mgmt address", "address", a, "subnet", p)
		return fmt.Sprintf(
			"set interfaces ethernet %s address %s/%d",
			nodeCfg.MgmtIntf,
			a,
			p,
		)
	}

	if nodeCfg.MgmtIPv4Address != "" {
		ip := nodeCfg.MgmtIPv4Address
		prefix := nodeCfg.MgmtIPv4PrefixLength
		cfgs = append(
			cfgs,
			addressCmd(ip, prefix),
		)
	}

	if nodeCfg.MgmtIPv6Address != "" {
		ip := nodeCfg.MgmtIPv6Address
		prefix := nodeCfg.MgmtIPv6PrefixLength
		cfgs = append(
			cfgs,
			addressCmd(ip, prefix),
		)
	}

	if n.SSHPubKeys != nil {
		cfgs = slices.Concat(cfgs, n.authorizedKeyCmds())
	}

	resp, err := cli.SendConfigs(cfgs)
	log.Debug("CLI", "response", resp.JoinedResult())
	if err != nil {
		return err
	} else if resp.Failed != nil {
		return errors.New("failed to configure management interface")
	}
	if err := n.save(ctx, cli); err != nil {
		return err
	}
	log.Info("PostDeploy complete", "node", n.Cfg.ShortName)
	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *vyos) CheckInterfaceName() error {
	// allow eth and et interfaces
	// https://regex101.com/r/umQW5Z/2
	ifRe := regexp.MustCompile(`eth[1-9]$`)
	for _, e := range n.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("vyos node %q has an interface named %q which doesn't match the required pattern. Interfaces may only be named ethX where X is any number greater than 0", n.Cfg.ShortName, e.GetIfaceName())
		}
	}

	return nil
}
