// Copyright 2022 JANOG51 NETCON.

package vyos

import (
	"context"
	_ "embed"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames = []string{"vyos"}
	defEnv    = map[string]string{}
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(vyos)
	})
}

type vyos struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (v *vyos) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {

	v.cfg = cfg
	for _, o := range opts {
		o(v)
	}
	v.cfg.Env = defEnv

	v.cfg.Env = utils.MergeStringMaps(defEnv, v.cfg.Env)
	v.cfg.Cmd = "/sbin/init"
	return nil
}
func (v *vyos) Config() *types.NodeConfig { return v.cfg }

func (v *vyos) PreDeploy(_, _, _ string) error {

	return nil
}

func (v *vyos) Deploy(ctx context.Context) error {
	cID, err := v.runtime.CreateContainer(ctx, v.cfg)
	if err != nil {
		return err
	}
	_, err = v.runtime.StartContainer(ctx, cID, v.cfg)
	return err
}

func (v *vyos) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for vyos '%s' node", v.cfg.ShortName)
	return nil
}

func (*vyos) WithMgmtNet(*types.MgmtNet)               {}
func (v *vyos) WithRuntime(r runtime.ContainerRuntime) { v.runtime = r }
func (v *vyos) GetRuntime() runtime.ContainerRuntime   { return v.runtime }

func (v *vyos) Delete(ctx context.Context) error {
	return v.runtime.DeleteContainer(ctx, v.cfg.LongName)
}

func (v *vyos) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: v.cfg.Image,
	}
}

func (v *vyos) SaveConfig(ctx context.Context) error {
	return nil
}
