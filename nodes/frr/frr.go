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

	// authzKeysTargets are the "user:home" pairs whose authorized_keys file is
	// populated: root for a shell and admin for vtysh, both reached with the
	// same key.
	authzKeysTargets = "root:/root admin:/home/admin"

	// authzKeysGuard skips a user the image does not have. admin ships only in
	// the containerlab flavour, so a plain release image still gets root's keys
	// instead of an error.
	authzKeysGuard = `id -u "$user" >/dev/null 2>&1 || continue`

	// authzKeysChown hands the file to the user who logs in with it; sshd
	// rejects an authorized_keys that user does not own.
	authzKeysChown = `chown -R "$user:" "$home/.ssh"`

	// authzKeysScript writes the public keys, passed to it as $1, to the
	// authorized_keys of every user in authzKeysTargets that exists.
	authzKeysScript = `set -e
for entry in ` + authzKeysTargets + `; do
	user=${entry%%:*}
	home=${entry#*:}

	` + authzKeysGuard + `

	mkdir -p "$home/.ssh"
	chmod 700 "$home/.ssh"
	# The file is removed rather than truncated: a redirect onto an existing
	# file keeps that file's ownership, and sshd rejects an authorized_keys
	# that its user does not own.
	rm -f "$home/.ssh/authorized_keys"
	printf '%s\n' "$1" > "$home/.ssh/authorized_keys"
	chmod 600 "$home/.ssh/authorized_keys"
	` + authzKeysChown + `
done`

	// passwordGuard skips a user the image does not have. A plain release image
	// has no admin, and root there is still reachable with the host's keys.
	passwordGuard = `id -u "$1" >/dev/null 2>&1 || exit 0`

	// passwordScript sets the password of the user named by $1 to $2. Both
	// travel as arguments rather than in the script text, so no character in
	// the password can be read as shell.
	passwordScript = passwordGuard + `
printf '%s:%s\n' "$1" "$2" | chpasswd`

	// no-header keeps the "Building configuration..." preamble out of the
	// saved file, which is written back as the node's frr.conf.
	saveCmd = `vtysh -c "show running-config no-header"`
)

var (
	// defaultCredentials is the admin user the containerlab image ships, whose
	// login shell is vtysh. It is what "ssh <node>" uses, so a bare ssh to a
	// node reaches the routing CLI; "ssh root@<node>" still gets a shell. The
	// image has no password for admin: PostDeploy sets this one.
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

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
	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials, generateNodeAttributes, nil)

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

	// The whole directory is mounted rather than the three files individually.
	// FRR saves a configuration by renaming frr.conf to frr.conf.sav and
	// writing a new one, and a single-file bind mount cannot be renamed, so
	// per-file mounts make every "write memory" report
	//
	//	Error renaming /etc/frr/frr.conf to /etc/frr/frr.conf.sav: Resource busy
	//
	// and lose the backup. Mounting the directory costs nothing: the image
	// ships only daemons in /etc/frr, and containerlab writes that file too.
	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, cfgDir), ":", etcFRR),
	)

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

// PostDeploy gives the node's users what they need to log in: the public keys
// containerlab collected from the host, for root and admin, and a password for
// the node's credentials user.
func (n *frr) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for frr %q node", n.Cfg.ShortName)

	err := n.addSSHKeys(ctx)
	if err != nil {
		return err
	}

	return n.setPassword(ctx)
}

// addSSHKeys adds the public keys containerlab collected from the host to the
// root and admin users' authorized_keys, to enable passwordless ssh. root gets
// a shell and admin gets vtysh, and both are reached with the same key.
func (n *frr) addSSHKeys(ctx context.Context) error {
	if len(n.sshPubKeys) == 0 {
		return nil
	}

	keys := strings.Join(clabutils.MarshalSSHPubKeys(n.sshPubKeys), "\n")

	cmd := clabexec.NewExecCmdFromSlice(
		[]string{"bash", "-c", authzKeysScript, "--", keys})

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

// setPassword sets the password of the node's credentials user: admin with
// password admin, unless the topology's credentials say otherwise. The image
// ships admin without a password, so a well-known one is not baked into it;
// this is what makes the kind's default, or a topology's own credentials, work
// for a password login.
func (n *frr) setPassword(ctx context.Context) error {
	user := n.Cfg.Credentials.Username
	password := n.Cfg.Credentials.Password

	if user == "" || password == "" {
		return nil
	}

	// chpasswd reads one user:password pair per line, so a line break in the
	// password would start a second pair, for whichever user it names.
	if strings.ContainsAny(password, "\r\n") {
		return fmt.Errorf("node %q: password must not contain a line break", n.Cfg.ShortName)
	}

	cmd := clabexec.NewExecCmdFromSlice(
		[]string{"bash", "-c", passwordScript, "--", user, password})

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to set password on node %q: %w", n.Cfg.ShortName, err)
	}

	if execResult.GetReturnCode() != 0 {
		return fmt.Errorf("failed to set password on node %q: %s",
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
