package f5_bigipve

import (
	"context"
	"fmt"
	"path"
	"regexp"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"f5_bigip-ve"}
	defaultCredentials = clabnodes.NewCredentials("admin", "Labl@b!234")

	InterfaceRegexp = regexp.MustCompile(`^1\.(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "1.X (where X >= 1) or ethX (where X >= 1)"
)

const (
	defaultQemuMemory = "8192"
	defaultQemuSMP    = "4"
	defaultQemuCPU    = "host"

	configDirName = "config"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, nil)
	r.Register(kindNames, func() clabnodes.Node {
		return new(F5BigIPVE)
	}, nrea)
}

type F5BigIPVE struct {
	clabnodes.DefaultNode
}

func (n *F5BigIPVE) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// eth0 is reserved for management interface.
	n.FirstDataIfIndex = 1

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"F5_HOSTNAME":     n.Cfg.ShortName,
		"USERNAME":        defaultCredentials.GetUsername(),
		"PASSWORD":        defaultCredentials.GetPassword(),
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

	// mount config dir to support config backup / onboarding
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.Env["F5_HOSTNAME"],
		n.Cfg.Env["CONNECTION_MODE"],
	)

	return nil
}

func (n *F5BigIPVE) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	// create config directory that will be bind mounted to vrnetlab container at /config path
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, configDirName), clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return err
}

func (n *F5BigIPVE) CheckInterfaceName() error {
	return clabnodes.GenericVMInterfaceCheck(n.Cfg.ShortName, n.Endpoints)
}

// AddEndpoint maps BIG-IP interface aliases (e.g. 1.1) to container ethX interfaces.
func (n *F5BigIPVE) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	if n.InterfaceRegexp != nil && !clabnodes.VMInterfaceRegexp.MatchString(endpointName) {
		mappedName, err := n.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf(
				"%q interface name %q could not be mapped: %w",
				n.Cfg.ShortName,
				e.GetIfaceName(),
				err,
			)
		}
		log.Debugf(
			"Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)",
			endpointName,
			mappedName,
		)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}

	n.Endpoints = append(n.Endpoints, e)
	return nil
}

// RunExec overrides DefaultNode.RunExec to forward commands to the VM guest
// via SSH, rather than executing them in the vrnetlab container namespace.
func (n *F5BigIPVE) RunExec(ctx context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
	return clabnodes.RunVMExec(ctx, n.Cfg.MgmtIPv4Address,
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], execCmd)
}

// GetMappedInterfaceName wraps the DefaultNode mapping to return an actionable error containing the
// expected interface patterns if alias parsing fails.
func (n *F5BigIPVE) GetMappedInterfaceName(ifName string) (string, error) {
	mappedIfName, err := n.DefaultNode.GetMappedInterfaceName(ifName)
	if err != nil {
		return "", fmt.Errorf("%w (expected %s)", err, n.InterfaceHelp)
	}
	return mappedIfName, nil
}
