package cvx

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	"github.com/weaveworks/ignite/pkg/operations"
)

var (
	defaultCvxKernelImageRef  = "docker.io/networkop/kernel:4.19"
	defaultIgniteSandboxImage = "networkop/ignite:dev"
)

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

	return nil
}

func (c *cvx) Config() *types.NodeConfig { return c.cfg }

func (c *cvx) PreDeploy(configName, labCADir, labCARoot string) error { return nil }

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

func (c *cvx) PostDeploy(ctx context.Context, ns map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for cvx '%s' node", c.cfg.ShortName)
	if c.vmChans == nil {
		return nil
	}

	if err := <-c.vmChans.SpawnFinished; err != nil {
		return err
	}

	return nil
}

func (c *cvx) GetImages() []string {
	return []string{c.cfg.Image, c.cfg.Kernel, c.cfg.Sandbox}
}

func (c *cvx) WithMgmtNet(*types.MgmtNet) {}
func (c *cvx) WithRuntime(r runtime.ContainerRuntime) {

	// fallback to the global runtime if overridden
	if c.Config().Runtime != "" {
		c.runtime = r
		return
	}

	// By default, running in ignite runtime
	if rInit, ok := runtime.ContainerRuntimes[ignite.RuntimeName]; ok {
		c.runtime = rInit()
		err := c.runtime.Init(
			runtime.WithConfig(&runtime.RuntimeConfig{
				Timeout:          r.GetTimeout(),
				Debug:            r.GetDebug(),
				GracefulShutdown: r.GetGracefulShutdown(),
			}),
		)
		if err != nil {
			log.Fatalf("failed to init the container runtime: %s", err)
		}

	}
}

func (c *cvx) Delete(ctx context.Context) error {
	return c.runtime.DeleteContainer(ctx, c.GetName())
}

func (c *cvx) GetName() string                      { return c.cfg.LongName }
func (s *cvx) GetRuntime() runtime.ContainerRuntime { return s.runtime }

func (c *cvx) SaveConfig(ctx context.Context) error {
	log.Debugf("Save operation is currently not supported for %q node kind", c.cfg.Kind)
	return nil
}
