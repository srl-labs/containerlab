package f5_bigip

import (
	"context"
	"fmt"
	"path"
	"regexp"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"f5_bigip_ve", "vr-f5_bigip_ve", "vr-f5_bigip"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`^1\.(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "1.X (where X >= 1) or ethX (where X >= 1)"
)

const (
	// scrapligo/scrapligocfg does not have a stable BIG-IP platform driver today.
	// Keep this explicit and fail fast on `save` with a clear message.
	scrapliPlatformName = "notsupported"

	defaultRootPassword = "default"
	defaultQemuMemory   = "8192"
	defaultQemuSMP      = "4"
	defaultQemuCPU      = "host"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, nil)
	r.Register(kindNames, func() clabnodes.Node {
		return new(f5BigIPVE)
	}, nrea)
}

type f5BigIPVE struct {
	clabnodes.VRNode
}

func (n *f5BigIPVE) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"F5_HOSTNAME":     n.Cfg.ShortName,
		"USERNAME":        defaultCredentials.GetUsername(),
		"PASSWORD":        defaultCredentials.GetPassword(),
		"ROOT_PASSWORD":   defaultRootPassword,
		"CONNECTION_MODE": clabnodes.VrDefConnMode,
		"QEMU_MEMORY":     defaultQemuMemory,
		"QEMU_SMP":        defaultQemuSMP,
		"QEMU_CPU":        defaultQemuCPU,
	}

	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// Keep forced password-change handling deterministic: if user overrides PASSWORD, align
	// F5_NEW_PASSWORD unless explicitly set.
	if _, ok := n.Cfg.Env["F5_NEW_PASSWORD"]; !ok {
		n.Cfg.Env["F5_NEW_PASSWORD"] = n.Cfg.Env["PASSWORD"]
	}

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf(
		"--hostname %s --username %s --password %s --root-password %s --connection-mode %s --ram %s --smp %s --cpu %s --trace",
		n.Cfg.Env["F5_HOSTNAME"],
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.Env["ROOT_PASSWORD"],
		n.Cfg.Env["CONNECTION_MODE"],
		n.Cfg.Env["QEMU_MEMORY"],
		n.Cfg.Env["QEMU_SMP"],
		n.Cfg.Env["QEMU_CPU"],
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

// GetMappedInterfaceName wraps the DefaultNode mapping to return an actionable error containing the
// expected interface patterns if alias parsing fails.
func (n *f5BigIPVE) GetMappedInterfaceName(ifName string) (string, error) {
	mappedIfName, err := n.VRNode.DefaultNode.GetMappedInterfaceName(ifName)
	if err != nil {
		return "", fmt.Errorf("%w (expected %s)", err, n.InterfaceHelp)
	}
	return mappedIfName, nil
}

func (n *f5BigIPVE) SaveConfig(_ context.Context) error {
	return fmt.Errorf(
		"save config is not supported for %q kind (no compatible scrapli platform driver)",
		n.Cfg.Kind,
	)
}
