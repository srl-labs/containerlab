package cvx

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	meta "github.com/weaveworks/ignite/pkg/apis/meta/v1alpha1"
	"github.com/weaveworks/ignite/pkg/operations"
)

var (
	defaultCvxKernelImageRef  = "docker.io/networkop/kernel:4.19"
	defaultIgniteSandboxImage = "networkop/ignite:dev"
)

var memoryReqs = map[string]string{
	"4.3.0": "512MB",
	"4.4.0": "768MB",
}

func init() {
	nodes.Register(nodes.NodeKindCVX, func() nodes.Node {
		return new(cvx)
	})
}

type cvx struct {
	cfg     *types.NodeConfig
	vmChans *operations.VMChannels
	runtime runtime.ContainerRuntime
}

func (c *cvx) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	c.cfg = cfg
	for _, o := range opts {
		o(c)
	}

	if c.cfg.Kernel == "" {
		c.cfg.Kernel = defaultCvxKernelImageRef
	}

	if c.cfg.Sandbox == "" {
		c.cfg.Sandbox = defaultIgniteSandboxImage
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

func (c *cvx) Config() *types.NodeConfig { return c.cfg }

func (*cvx) PreDeploy(_, _, _ string) error { return nil }

func (c *cvx) Deploy(ctx context.Context) error {

	intf, err := c.runtime.CreateContainer(ctx, c.cfg)
	if err != nil {
		return err
	}

	if vmChans, ok := intf.(*operations.VMChannels); ok {
		c.vmChans = vmChans
	}

	return nil
}

func (c *cvx) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for cvx '%s' node", c.cfg.ShortName)
	if c.vmChans == nil {
		return nil
	}

	return <-c.vmChans.SpawnFinished
}

func (c *cvx) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = c.cfg.Image

	if c.runtime.GetName() != runtime.IgniteRuntime {
		return images
	}

	images[nodes.KernelKey] = c.cfg.Kernel
	images[nodes.SandboxKey] = c.cfg.Sandbox
	return images
}

func (*cvx) WithMgmtNet(*types.MgmtNet)               {}
func (c *cvx) WithRuntime(r runtime.ContainerRuntime) { c.runtime = r }

func (c *cvx) Delete(ctx context.Context) error {
	return c.runtime.DeleteContainer(ctx, c.Config().LongName)
}

func (s *cvx) GetRuntime() runtime.ContainerRuntime { return s.runtime }

func (c *cvx) SaveConfig(_ context.Context) error {
	log.Debugf("Save operation is currently not supported for %q node kind", c.cfg.Kind)
	return nil
}
