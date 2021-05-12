package kind

import (
	"strings"

	"github.com/srl-labs/containerlab/types"
)

type Linux struct {
	*types.NodeBase
}

func NewLinuxKind(n *types.NodeBase) (types.Node, error) {
	k := &Linux{
		n,
	}
	if err := k.Init(n); err != nil {
		return k, err
	}
	return k, nil
}

func (k *Linux) Init(n *types.NodeBase) error {
	var err error

	k.Sysctls = make(map[string]string)
	if strings.ToLower(k.NetworkMode) != "host" {
		k.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	return err
}

func (k *Linux) Deploy()     {}
func (k *Linux) PostDeploy() {}
func (k *Linux) Destroy()    {}
