package nodes

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

type DefaultNode struct {
	Cfg     *types.NodeConfig
	Mgmt    *types.MgmtNet
	Runtime runtime.ContainerRuntime
}

func (d *DefaultNode) GetRuntime() runtime.ContainerRuntime { return d.Runtime }
func (d *DefaultNode) Config() *types.NodeConfig            { return d.Cfg }
func (d *DefaultNode) WithMgmtNet(mgmt *types.MgmtNet)      { d.Mgmt = mgmt }
func (d *DefaultNode) WithRuntime(r runtime.ContainerRuntime) {
	d.Runtime = r
}

func (d *DefaultNode) SaveConfig(_ context.Context) error {
	log.Debugf("Save operation is currently not supported for %q node kind", d.Cfg.Kind)
	return nil
}

func (d *DefaultNode) PreDeploy(_, _, _ string) error {
	return nil
}

func (d *DefaultNode) Deploy(ctx context.Context) error {
	cID, err := d.Runtime.CreateContainer(ctx, d.Cfg)
	if err != nil {
		return err
	}
	_, err = d.Runtime.StartContainer(ctx, cID, d.Cfg)
	return err
}

func (d *DefaultNode) Delete(ctx context.Context) error {
	return d.Runtime.DeleteContainer(ctx, d.Cfg.LongName)
}

func (d *DefaultNode) PostDeploy(_ context.Context, _ map[string]Node) error {
	return nil
}

func (d *DefaultNode) GetImages() map[string]string {
	return map[string]string{
		ImageKey: d.Cfg.Image,
	}
}
