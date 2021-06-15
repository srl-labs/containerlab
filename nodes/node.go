package nodes

import (
	"context"

	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

type Node interface {
	Init(*types.NodeConfig) error
	GenerateConfig() error
	Config() *types.NodeConfig
	PreDeploy() error
	Deploy(context.Context, runtime.ContainerRuntime) error
	PostDeploy() error
	Destroy() error
}

var Nodes = map[string]Initializer{}

type Initializer func() Node

func Register(name string, initFn Initializer) {
	Nodes[name] = initFn
}

var DefaultConfigTemplates = map[string]string{
	"srl":     "/etc/containerlab/templates/srl/srlconfig.tpl",
	"ceos":    "/etc/containerlab/templates/arista/ceos.cfg.tpl",
	"crpd":    "/etc/containerlab/templates/crpd/juniper.conf",
	"vr-sros": "",
}
