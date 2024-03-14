package cvx

import (
	"context"
	"fmt"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	meta "github.com/weaveworks/ignite/pkg/apis/meta/v1alpha1"
	"github.com/weaveworks/ignite/pkg/operations"
)

var (
	kindnames                 = []string{"cvx", "cumulus_cvx"}
	defaultCvxKernelImageRef  = "docker.io/networkop/kernel:4.19"
	defaultIgniteSandboxImage = "networkop/ignite:dev"
)

var memoryReqs = map[string]string{
	"4.3.0": "512MB",
	"4.4.0": "768MB",
}

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(cvx)
	}, nil)
	nodes.SetNonDefaultRuntimePerKind(kindnames, ignite.RuntimeName)
}

type cvx struct {
	nodes.DefaultNode
	vmChans *operations.VMChannels
}

func (c *cvx) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	c.DefaultNode = *nodes.NewDefaultNode(c)

	c.Cfg = cfg
	for _, o := range opts {
		o(c)
	}

	if c.Cfg.Kernel == "" {
		c.Cfg.Kernel = defaultCvxKernelImageRef
	}

	if c.Cfg.Sandbox == "" {
		c.Cfg.Sandbox = defaultIgniteSandboxImage
	}

	ociRef, err := meta.NewOCIImageRef(cfg.Image)
	if err != nil {
		return fmt.Errorf("failed to parse OCI image ref %q: %s", cfg.Image, err)
	}

	// if Memory is not statically set, apply the defaults
	if cfg.Memory == "" {
		mem, ok := memoryReqs[ociRef.Ref().Tag()]
		cfg.Memory = mem

		// by default setting the limit to 768MB
		if !ok {
			cfg.Memory = "768MB"
		}
	}

	return nil
}

func (c *cvx) Deploy(ctx context.Context, _ *nodes.DeployParams) error {
	// CreateContainer is no-op in case of ignite runtime
	cID, err := c.Runtime.CreateContainer(ctx, c.Cfg)
	if err != nil {
		return err
	}
	intf, err := c.Runtime.StartContainer(ctx, cID, c)
	if err != nil {
		return err
	}
	if vmChans, ok := intf.(*operations.VMChannels); ok {
		c.vmChans = vmChans
	}

	c.SetState(state.Deployed)

	return nil
}

func (c *cvx) PostDeploy(_ context.Context, _ *nodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for cvx '%s' node", c.Cfg.ShortName)
	if c.vmChans == nil {
		return nil
	}

	return <-c.vmChans.SpawnFinished
}

func (c *cvx) GetImages(_ context.Context) map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = c.Cfg.Image

	if c.Runtime.GetName() != ignite.RuntimeName {
		return images
	}

	images[nodes.KernelKey] = c.Cfg.Kernel
	images[nodes.SandboxKey] = c.Cfg.Sandbox
	return images
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (c *cvx) CheckInterfaceName() error {
	// allow swpX interface names
	// https://regex101.com/r/SV0k1J/1
	ifRe := regexp.MustCompile(`swp[\d\.]+$`)
	for _, e := range c.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("%q interface name %q doesn't match the required pattern. It should be named as swpX, where X is >=0", c.Cfg.ShortName, e.GetIfaceName())
		}
	}

	return nil
}
