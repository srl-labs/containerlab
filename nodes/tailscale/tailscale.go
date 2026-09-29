package tailscale

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	KindName      = "tailscale"
	DefaultImage  = "tailscale/tailscale:stable"
	sidecarSuffix = "-ts"
	tsStateDir    = "/var/lib/tailscale"
	sidecarLabDir = "/clab"
	authKeyFile   = "authkey"
	// tsSocket is the containerboot default tailscaled socket.
	tsSocket      = "/tmp/tailscaled.sock"
	logoutTimeout = 10 * time.Second
)

// authKeyEnv makes tailscale up read the key from the lab dir so it stays out of the
// container env and exported node configs.
const authKeyEnv = "file:" + sidecarLabDir + "/" + authKeyFile

var kindNames = []string{KindName}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, nil, nil).
		WithPrivilegedByDefault(false)

	r.Register(kindNames, func() clabnodes.Node {
		return new(tailscale)
	}, nrea)
}

// SidecarName returns the topology name of the Tailscale sidecar for parent.
func SidecarName(parent string) string {
	return parent + sidecarSuffix
}

type tailscale struct {
	clabnodes.DefaultNode
}

// PreDestroy logs the sidecar out of the tailnet. Best effort.
func (n *tailscale) PreDestroy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, logoutTimeout)
	defer cancel()

	res, err := n.RunExec(ctx, clabexec.NewExecCmdFromSlice(
		[]string{"tailscale", "--socket=" + tsSocket, "logout"},
	))
	switch {
	case err != nil:
		log.Warn("Tailscale logout failed", "node", n.Cfg.ShortName, "error", err)
	case res.GetReturnCode() != 0:
		log.Warn("Tailscale logout failed", "node", n.Cfg.ShortName,
			"error", strings.TrimSpace(res.GetStdErrString()))
	default:
		log.Info("Logged out of the tailnet", "node", n.Cfg.ShortName)
	}

	return nil
}

func (*tailscale) LinkApplyMode(context.Context) clabnodes.LinkApplyMode {
	return clabnodes.LinkApplyModeLive
}

func (n *tailscale) PreDeploy(context.Context, *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	return writeAuthKey(n.Cfg, n.Mgmt)
}

// writeAuthKey stores the auth key read by authKeyEnv; a user-provided TS_AUTHKEY is left alone.
func writeAuthKey(cfg *clabtypes.NodeConfig, mgmt *clabtypes.MgmtNet) error {
	if cfg.LabDir == "" || cfg.Env["TS_AUTHKEY"] != authKeyEnv ||
		mgmt == nil || mgmt.Tailscale == nil || mgmt.Tailscale.AuthKey == "" {
		return nil
	}

	return os.WriteFile(
		filepath.Join(cfg.LabDir, authKeyFile),
		[]byte(mgmt.Tailscale.AuthKey),
		0o600,
	)
}

func (n *tailscale) Deploy(ctx context.Context, params *clabnodes.DeployParams) error {
	setEnvDefault(n.Cfg, "TS_DEST_IP", parentMgmtIP(n.Cfg, params))

	return n.DefaultNode.Deploy(ctx, params)
}

func (n *tailscale) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	if n.Cfg.RestartPolicy == "" {
		n.Cfg.RestartPolicy = "always"
	}

	if n.Cfg.Image == "" {
		n.Cfg.Image = DefaultImage
	}

	for _, o := range opts {
		o(n)
	}

	if n.Cfg.LabDir != "" {
		n.Cfg.Binds = append(n.Cfg.Binds, n.Cfg.LabDir+":"+sidecarLabDir)
	}

	// TS_DEST_IP requires a kernel TUN; containerboot rejects it with userspace networking.
	n.Cfg.CapAdd = append(n.Cfg.CapAdd, "NET_ADMIN")
	n.Cfg.Devices = append(n.Cfg.Devices, "/dev/net/tun")
	if n.Cfg.Sysctls == nil {
		n.Cfg.Sysctls = map[string]string{}
	}
	n.Cfg.Sysctls["net.ipv4.ip_forward"] = "1"
	n.Cfg.Sysctls["net.ipv6.conf.all.forwarding"] = "1"

	applyTailscaleEnv(cfg, n.Mgmt)

	return nil
}

func applyTailscaleEnv(cfg *clabtypes.NodeConfig, mgmt *clabtypes.MgmtNet) {
	if cfg.Env == nil {
		cfg.Env = map[string]string{}
	}

	setEnvDefault(cfg, "TS_USERSPACE", "false")
	setEnvDefault(cfg, "TS_ACCEPT_DNS", "false")
	setEnvDefault(cfg, "TS_HOSTNAME", tailscaleHostname(cfg))

	if mgmt == nil || mgmt.Tailscale == nil {
		return
	}

	if mgmt.Tailscale.AuthKey != "" {
		setEnvDefault(cfg, "TS_AUTHKEY", authKeyEnv)
	}
}

func setEnvDefault(cfg *clabtypes.NodeConfig, key, value string) {
	if value == "" {
		return
	}
	if _, exists := cfg.Env[key]; !exists {
		cfg.Env[key] = value
	}
}

func parentMgmtIP(cfg *clabtypes.NodeConfig, params *clabnodes.DeployParams) string {
	if cfg == nil || params == nil {
		return ""
	}
	parentName := cfg.Labels[clabconstants.RootNodeName]
	parent, ok := params.Nodes[parentName]
	if !ok || parent == nil || parent.Config() == nil {
		return ""
	}
	return mgmtIP(parent.Config())
}

func mgmtIP(cfg *clabtypes.NodeConfig) string {
	if cfg.MgmtIPv4Address != "" {
		return cfg.MgmtIPv4Address
	}
	return cfg.MgmtIPv6Address
}

func tailscaleHostname(cfg *clabtypes.NodeConfig) string {
	name := cfg.Labels[clabconstants.RootNodeName]
	if name == "" {
		name = cfg.ShortName
	}
	if lab := cfg.Labels[clabconstants.Containerlab]; lab != "" {
		return "clab-" + lab + "-" + name
	}

	return "clab-" + name
}
