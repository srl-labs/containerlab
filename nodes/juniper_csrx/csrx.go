// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package juniper_csrx

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	configDir        = "config"
	junosConfig      = "juniper.conf"
	sshdConfig       = "sshd_config"
	licenseDir       = "license"
	licenseFile      = "license.lic"
	containerConfig  = "/config/juniper.conf"
	containerLicense = "/config/license/license.lic"

	generateable     = true
	generateIfFormat = "eth%d"

	scrapliPlatformName = "juniper_junos"
	NapalmPlatformName  = "junos"
)

var (
	kindNames = []string{"juniper_csrx"}
	//go:embed csrx.cfg
	defaultCfgTemplate string

	//go:embed sshd_config
	sshdCfg string

	defaultCredentials = clabnodes.NewCredentials("root", "clab123")

	saveCmd       = "cli show conf"
	sshRestartCmd = "service ssh restart"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformOpts := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformOpts,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(csrx)
	}, nrea)
}

type csrx struct {
	clabnodes.DefaultNode
}

func (s *csrx) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *clabnodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}

	// mount config and log dirs
	s.Cfg.Binds = append(s.Cfg.Binds,
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, configDir), ":/config"),
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, "log"), ":/var/log"),
		// mount sshd_config
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, configDir, sshdConfig), ":/etc/ssh/sshd_config"),
		// Pre-create the cSRX password sentinel so rc.local skips the initial
		// "encrypted-password *disabled*" commit and honors our juniper.conf hash instead.
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, "csrx_password_config_file"), ":/var/local/csrx_password_config_file"),
	)

	// On cSRX 22.x, rc.local only loads /config/juniper.conf when the
	// CSRX_JUNOS_CONFIG env var points at it. 24.x loads it unconditionally,
	// so setting this is a no-op on newer images.
	if s.Cfg.Env == nil {
		s.Cfg.Env = map[string]string{}
	}
	if _, ok := s.Cfg.Env["CSRX_JUNOS_CONFIG"]; !ok {
		s.Cfg.Env["CSRX_JUNOS_CONFIG"] = containerConfig
	}

	return nil
}

func (s *csrx) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(s.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}
	return createCSRXFiles(s)
}

func (s *csrx) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for csrx %q node", s.Cfg.ShortName)

	cmd, _ := clabexec.NewExecCmdFromString(sshRestartCmd)
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if execResult.GetStdErrString() != "" {
		// If "ssh: unrecognized service" appears in the output we are probably
		// on Junos >=23.4, where the SSH service was renamed to junos-ssh and
		// is fully managed by MGD
		if strings.Contains(execResult.GetStdErrString(), "ssh: unrecognized service") {
			log.Debug(`Caught "ssh: unrecognized service" error, ignoring`)
		} else {
			return fmt.Errorf("csrx post-deploy sshd restart failed: %s", execResult.GetStdErrString())
		}
	}

	if s.Config().License != "" {
		d, err := clabutils.SpawnCLIviaExec("juniper_junos", s.Cfg.LongName, s.Runtime.GetName())
		if err != nil {
			return err
		}

		defer d.Close()

		resp, err := d.SendCommand(
			fmt.Sprintf("request system license add %s", containerLicense),
		)
		if err != nil {
			return err
		} else if resp.Failed != nil {
			return fmt.Errorf(
				"csrx post-deploy license add failed: %w",
				resp.Failed,
			)
		}
		log.Debugf("csrx post-deploy license add completed")
	}

	return nil
}

func (s *csrx) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	cmd, _ := clabexec.NewExecCmdFromString(saveCmd)
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	if execResult.GetStdErrString() != "" {
		return nil, fmt.Errorf("csrx save-config failed: %s", execResult.GetStdErrString())
	}

	// path by which to save a config
	confPath := csrxConfigPath(s.Cfg.LabDir)
	err = os.WriteFile(confPath, execResult.GetStdOutByteSlice(),
		clabconstants.PermissionsOpen) // skipcq: GO-S2306
	if err != nil {
		return nil, fmt.Errorf(
			"failed to write config by %s path from %s container: %v",
			confPath,
			s.Cfg.ShortName,
			err,
		)
	}
	log.Infof("saved csrx configuration from %s node to %s\n", s.Cfg.ShortName, confPath)

	return &clabnodes.SaveConfigResult{
		ConfigPath: confPath,
	}, nil
}

func createCSRXFiles(node clabnodes.Node) error {
	nodeCfg := node.Config()
	// create config and logs directory that will be bind mounted to csrx
	clabutils.CreateDirectory(filepath.Join(nodeCfg.LabDir, configDir),
		clabconstants.PermissionsOpen)
	clabutils.CreateDirectory(filepath.Join(nodeCfg.LabDir, "log"),
		clabconstants.PermissionsOpen)

	// copy csrx config from default template or user-provided conf file
	cfg := csrxConfigPath(nodeCfg.LabDir)
	nodeCfg.ResStartupConfig = cfg
	var cfgTemplate string

	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	if cfgTemplate == "" {
		cfgTemplate = defaultCfgTemplate
	}

	err := node.GenerateConfig(cfg, cfgTemplate)
	if err != nil {
		return fmt.Errorf("node=%s, failed to generate config: %w", nodeCfg.ShortName, err)
	}

	// write csrx sshd conf file to csrx node dir
	// Note: this only applies to older versions of Junos (before 23). In later
	// versions the config file is placed in /var/etc/sshd_config and is owned
	// by MGD.
	dst := filepath.Join(nodeCfg.LabDir, configDir, sshdConfig)
	err = clabutils.CreateFile(dst, sshdCfg)
	if err != nil {
		return fmt.Errorf("failed to write sshd_config file %v", err)
	}
	log.Debug("Writing sshd_config succeeded")

	// Pre-create the cSRX password sentinel file. Its mere existence makes
	// rc.local skip the block that force-sets root-authentication to
	// "*disabled*", which would otherwise mask the hash from juniper.conf.
	pwSentinel := filepath.Join(nodeCfg.LabDir, "csrx_password_config_file")
	if err = clabutils.CreateFile(pwSentinel, ""); err != nil {
		return fmt.Errorf("failed to write csrx password sentinel file %v", err)
	}

	if nodeCfg.License != "" {
		// copy license file to node specific lab directory
		src := nodeCfg.License
		dst = csrxLicensePath(nodeCfg.LabDir)

		if err := os.MkdirAll(filepath.Dir(dst),
			clabconstants.PermissionsOpen); err != nil { // skipcq: GSC-G301
			return err
		}

		if err = clabutils.CopyFile(context.Background(), src, dst,
			clabconstants.PermissionsFileDefault); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}
	return nil
}

func csrxConfigPath(labDir string) string {
	return filepath.Join(labDir, configDir, junosConfig)
}

func csrxLicensePath(labDir string) string {
	return filepath.Join(labDir, configDir, licenseDir, licenseFile)
}
