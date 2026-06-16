package cvce

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

var (
	KindNames = []string{"cvce", "arista_cvce"}

	defaultCredentials = clabnodes.NewCredentials("root", "password")
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(KindNames, func() clabnodes.Node {
		return new(cvce)
	}, nrea)
}

type cvce struct {
	clabnodes.DefaultNode

	resolvConfPath string
}

func (n *cvce) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.HostRequirements.MinVCPU = 2
	n.HostRequirements.MinVCPUFailAction = clabtypes.FailBehaviourError

	n.HostRequirements.MinAvailMemoryGb = 2
	n.HostRequirements.MinAvailMemoryGbFailAction = clabtypes.FailBehaviourError

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	hwa, err := clabutils.GenMac("f0:8e:db")
	if err != nil {
		return err
	}
	n.Cfg.MacAddress = hwa.String()

	n.Cfg.CapAdd = []string{
		"CAP_NET_ADMIN",
		"CAP_SYS_NICE",
	}

	if n.Cfg.CPU == 0 {
		n.Cfg.CPU = 2
	}
	if n.Cfg.CPUSet == "" {
		n.Cfg.CPUSet = "0,3"
	}
	if n.Cfg.Memory == "" {
		n.Cfg.Memory = "2048MB"
	}

	n.Cfg.NetworkMode = "none"

	n.resolvConfPath = filepath.Join(n.Cfg.LabDir, "resolv.conf")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.resolvConfPath, ":/etc/resolv.conf"))

	return nil
}

func (n *cvce) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	clabutils.CreateFile(n.resolvConfPath, ResolvConfText)

	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *cvce) CheckInterfaceName() error {
	ifRe := regexp.MustCompile(`eth[0-7]$`)
	for _, e := range n.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("cvce node %q has an interface named %q which doesn't match the required pattern. Interfaces should be named eth0-eth7, where eth0 -> GE1, and so on", n.Cfg.ShortName, e.GetIfaceName())
		}
	}

	return nil
}
