package cvce

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabvelocpu "github.com/srl-labs/containerlab/internal/velocpu"
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
	optVCPath      string
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

	if n.Cfg.MacAddress == "" {
		hwa, err := clabutils.GenMac("f0:8e:db")
		if err != nil {
			return err
		}
		n.Cfg.MacAddress = hwa.String()
	}

	// Containers are run in privileged mode so it should not matter now.
	// If it changes, add the capabilities required to run the VeloCloud Edge.
	n.Cfg.CapAdd = append(n.Cfg.CapAdd,
		"CAP_NET_ADMIN",
		"CAP_NET_RAW",
		"CAP_SYS_ADMIN",
		"CAP_SYS_NICE",
	)

	if n.Cfg.CPU == 0 {
		n.Cfg.CPU = 2
	}
	// Pin to host CPUs from the shared velo pool. An explicit cpu-set wins and
	// is reserved out of the pool; otherwise claim a block sized to n.Cfg.CPU.
	// Under oversubscription the allocator may co-locate nodes on a block and
	// caps each with a reduced CPU quota so neither can starve the other.
	if n.Cfg.CPUSet != "" {
		if err := clabvelocpu.Reserve(n.Cfg.CPUSet); err != nil {
			return err
		}
	} else {
		alloc, err := clabvelocpu.Claim(int(n.Cfg.CPU))
		if err != nil {
			return err
		}
		n.Cfg.CPUSet = alloc.CPUSet
		n.Cfg.CPU = alloc.CPUQuota
	}
	if n.Cfg.Memory == "" {
		n.Cfg.Memory = "2048MB"
	}

	if n.Cfg.Entrypoint == "" {
		n.Cfg.Entrypoint = "/sbin/init"
	}
	// The Edge manages its own dataplane and resolver, so by default it runs
	// without the management network. A user-specified network-mode wins.
	if n.Cfg.NetworkMode == "" {
		n.Cfg.NetworkMode = "none"
	}

	n.resolvConfPath = filepath.Join(n.Cfg.LabDir, "resolv.conf")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.resolvConfPath, ":/etc/resolv.conf"))

	// Persist /opt/vc (activation state, certs) across runs by binding a
	// directory under the lab dir. Other paths may need similar handling - TBD.
	n.optVCPath = filepath.Join(n.Cfg.LabDir, "opt-vc")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.optVCPath, ":/opt/vc"))

	return nil
}

func (n *cvce) PreDeploy(_ context.Context, _ *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	clabutils.CreateDirectory(n.optVCPath, clabconstants.PermissionsOpen)
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
