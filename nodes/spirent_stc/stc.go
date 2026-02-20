package spirent_stc

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"runtime"

	"github.com/charmbracelet/log"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames       = []string{"spirent_stc"}
	InterfaceRegexp = regexp.MustCompile(`port(?P<port>[1-9])`)
	InterfaceHelp   = "portX (where X is 1-9)"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

const (
	cgroupV1Dir    = "cgroup-v1"
	cgroupMemLimit = "4294967296" // 4GB
)

func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)

	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(spirentStc)
	}, nrea)
}

type spirentStc struct {
	clabnodes.DefaultNode
}

func (n *spirentStc) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	env := map[string]string{
		"SPIRENT_ADMIN":       "--mode container",
		"LIBVIRT_LXC_CMDLINE": "SPIRENT_ADMIN=--mode container",
	}

	n.Cfg.Env = clabutils.MergeStringMaps(env, n.Cfg.Env)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceHelp = InterfaceHelp
	n.InterfaceOffset = 0

	// capture_0 is the last process that starts before it's deemed up.
	n.Cfg.Healthcheck = &clabtypes.HealthcheckConfig{
		Test:     []string{"CMD-SHELL", "pgrep -f capture_0"},
		Interval: 10,
		Timeout:  5,
		Retries:  3,
	}

	// STC uses cgroupv1, mount the fake dirs
	cgroupDir := path.Join(n.Cfg.LabDir, cgroupV1Dir)
	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(cgroupDir, ":/sys/fs/cgroup"),
		fmt.Sprint(path.Join(cgroupDir, "memory.limit_in_bytes"), ":/cgroup/memory/memory.limit_in_bytes"),
	)

	return nil
}

func (n *spirentStc) PreDeploy(_ context.Context, _ *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	return createCgroupV1Files(n.Cfg.LabDir)
}

func (n *spirentStc) PostDeploy(_ context.Context, _ *clabnodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Spirent STC '%s' node", n.Cfg.ShortName)

	return nil
}

// create the fake cgroupv1 files so STC can boot
func createCgroupV1Files(labDir string) error {
	base := path.Join(labDir, cgroupV1Dir)

	cgControllers := map[string]map[string]string{
		"memory": {
			"memory.limit_in_bytes": cgroupMemLimit,
			"memory.usage_in_bytes": "0",
		},
		"devices": {
			"devices.list": "a *:* rwm",
		},
		"cpuset": {
			"cpuset.cpus": fmt.Sprintf("0-%d", runtime.NumCPU()),
			"cpuset.mems": "0",
		},
		"cpu,cpuacct": {
			"cpu.shares":        "1024",
			"cpu.cfs_quota_us":  "-1",
			"cpu.cfs_period_us": "100000",
			"cpu.rt_runtime_us": "0",
			"cpu.rt_period_us":  "1000000",
			"cpuacct.usage":     "0",
		},
	}

	for controller, files := range cgControllers {
		dir := path.Join(base, controller)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating dir %q: %w", controller, err)
		}

		for name, content := range files {
			if err := os.WriteFile(path.Join(dir, name), []byte(content+"\n"), 0644); err != nil {
				return fmt.Errorf("error creating cgroup %q: %w", fmt.Sprintf("%s/%s", controller, name), err)
			}
		}
	}

	// spirent bootup script is checking these two separately to
	// determine if the grousp are present or not,
	os.Symlink("cpu,cpuacct", path.Join(base, "cpu"))
	os.Symlink("cpu,cpuacct", path.Join(base, "cpuacct"))

	// Standalone memory limit file also needed at /cgroup/memory/.
	if err := os.WriteFile(path.Join(base, "memory.limit_in_bytes"), []byte(cgroupMemLimit+"\n"), 0644); err != nil {
		return fmt.Errorf("error creating cgroup mem file %q: %w", "memory.limit_in_bytes", err)
	}

	return nil
}

func (n *spirentStc) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	if !InterfaceRegexp.MatchString(endpointName) {
		return fmt.Errorf("%q interface name %q doesn't match %s",
			n.Cfg.ShortName, endpointName, InterfaceHelp)
	}

	return n.DefaultNode.AddEndpoint(e)
}
