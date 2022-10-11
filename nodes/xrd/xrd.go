// Copyright 2022 JANOG51 NETCON.

package xrd

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames = []string{"xrd"}
	defEnv    = map[string]string{
		"XR_FIRST_BOOT_CONFIG": "/etc/xrd/first-boot.cfg",
		"XR_MGMT_INTERFACES":   "linux:eth0,xr_name=Mg0/RP0/CPU0/0,chksum",
	}

	//go:embed xrd.conf
	cfgTemplate string
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(xrd)
	})
}

type xrd struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (x *xrd) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {

	x.cfg = cfg
	for _, o := range opts {
		o(x)
	}
	x.cfg.Env = defEnv

	// Check Static MgmtIPv4/Prefix
	if x.cfg.MgmtIPv4Address == "" || x.cfg.MgmtIPv4PrefixLength == 0 {
		return fmt.Errorf("Kind XRd needs mgmt_ipv4 and mgmt_ipv4_prefix. Please set parameters to yml")
	}

	var interfaceEnvCount string
	for i := 0; i < 90; i++ {
		interfaceEnvCount = interfaceEnvCount + fmt.Sprintf("linux:eth%d,xr_name=Gi0/0/0/%d;", i+1, i)
	}
	interfaceEnv := map[string]string{
		"XR_INTERFACES": interfaceEnvCount,
	}

	x.cfg.Env = utils.MergeStringMaps(interfaceEnv, x.cfg.Env)

	cfgFilePath := filepath.Join(x.cfg.LabDir, "xrd.conf")
	x.cfg.Binds = append(x.cfg.Binds,
		fmt.Sprintf("%s:/etc/xrd/first-boot.cfg", cfgFilePath),
	)

	return nil
}
func (x *xrd) Config() *types.NodeConfig { return x.cfg }

func (x *xrd) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(x.cfg.LabDir, 0777)

	return x.createXRDFiles()
}
func (x *xrd) Deploy(ctx context.Context) error {
	cID, err := x.runtime.CreateContainer(ctx, x.cfg)
	if err != nil {
		return err
	}
	_, err = x.runtime.StartContainer(ctx, cID, x.cfg)
	return err
}

func (x *xrd) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for Cisco XRd '%s' node", x.cfg.ShortName)
	return nil
}

func (*xrd) WithMgmtNet(*types.MgmtNet)               {}
func (x *xrd) WithRuntime(r runtime.ContainerRuntime) { x.runtime = r }
func (x *xrd) GetRuntime() runtime.ContainerRuntime   { return x.runtime }

func (x *xrd) Delete(ctx context.Context) error {
	return x.runtime.DeleteContainer(ctx, x.cfg.LongName)
}

func (x *xrd) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: x.cfg.Image,
	}
}

func (x *xrd) SaveConfig(ctx context.Context) error {
	return nil
}

func (x *xrd) createXRDFiles() error {
	nodeCfg := x.Config()
	nodeCfg.ResStartupConfig = filepath.Join(x.cfg.LabDir, "xrd.conf")

	err := nodeCfg.GenerateConfig(nodeCfg.ResStartupConfig, cfgTemplate)
	if err != nil {
		return err
	}
	return err

}

func (x *xrd) xrdPostDeploy() error {
	return nil
}
