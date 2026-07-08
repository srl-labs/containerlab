// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause
// The author of this code is Adam Kulagowski (adam.kulagowski@codilime.com), CodiLime
// (codilime.com).

package sonic_vm

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/generic"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/transport"
	"golang.org/x/crypto/ssh"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"sonic-vm"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	generateable     = true
	generateIfFormat = "eth%d"
)

const (
	configDirName   = "config"
	startupCfgFName = "config_db.json"
	saveCmd         = `sh -c "/backup.sh -u $USERNAME -p $PASSWORD backup"`

	scrapliPlatformName = "sonic"
	readyTimeout        = 5 * time.Minute
	readyRetryInterval  = 5 * time.Second
)

var bashPromptPattern = regexp.MustCompile(`[$#]\s*$`)

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
		return new(sonic_vm)
	}, nrea)
}

type sonic_vm struct {
	clabnodes.VRNode
	sshPubKeys []ssh.PublicKey
}

func (n *sonic_vm) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           n.Cfg.Credentials.Username,
		"PASSWORD":           n.Cfg.Credentials.Password,
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
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

func (n *sonic_vm) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	n.sshPubKeys = params.SSHPubKeys

	return clabnodes.LoadStartupConfigFileVr(n, configDirName, startupCfgFName)
}

func (n *sonic_vm) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	if len(n.sshPubKeys) == 0 {
		return nil
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()

	if err := n.deploySSHKeys(deadlineCtx); err != nil {
		return fmt.Errorf("%s: failed to deploy SSH public keys: %w", n.Cfg.ShortName, err)
	}

	log.Infof("Deployed %d SSH public key(s) for node %s", len(n.sshPubKeys), n.Cfg.ShortName)

	return nil
}

// deploySSHKeys waits for the VM to become healthy and then injects the SSH public keys
// via a scrapligo SSH session.
func (n *sonic_vm) deploySSHKeys(ctx context.Context) error {
	commands := buildSSHKeyInjectionCommands(clabutils.MarshalSSHPubKeys(n.sshPubKeys))

	log.Infof("Waiting for %s to be ready to accept SSH connections. This may take a while",
		n.Cfg.ShortName)

	for {
		if !n.IsVrnetlabHealthy(ctx) {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timed out waiting for node to become healthy")
			default:
				log.Debugf("Waiting for %s to become healthy", n.Cfg.ShortName)
				time.Sleep(readyRetryInterval)
				continue
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for SSH to become available")
		default:
			driver, err := generic.NewDriver(
				n.Cfg.MgmtIPv4Address,
				options.WithAuthNoStrictKey(),
				options.WithAuthUsername(n.Cfg.Credentials.Username),
				options.WithAuthPassword(n.Cfg.Credentials.Password),
				options.WithTransportType(transport.StandardTransport),
				options.WithPromptPattern(bashPromptPattern),
				options.WithTimeoutOps(30*time.Second),
			)
			if err != nil {
				return err
			}

			if err := driver.Open(); err != nil {
				log.Debugf("%s: SSH not yet ready - %v", n.Cfg.ShortName, err)
				time.Sleep(readyRetryInterval)
				continue
			}
			defer driver.Close()

			_, err = driver.SendCommands(commands)
			return err
		}
	}
}

func buildSSHKeyInjectionCommands(keys []string) []string {
	cmds := []string{
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh",
		"truncate -s 0 ~/.ssh/authorized_keys",
	}

	for _, key := range keys {
		escaped := strings.ReplaceAll(key, "'", `'\''`)
		cmds = append(cmds, fmt.Sprintf("printf '%%s\\n' '%s' >> ~/.ssh/authorized_keys", escaped))
	}

	cmds = append(cmds, "chmod 600 ~/.ssh/authorized_keys")

	return cmds
}

func (n *sonic_vm) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
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

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *sonic_vm) CheckInterfaceName() error {
	return clabnodes.GenericVMInterfaceCheck(n.Cfg.ShortName, n.Endpoints)
}
