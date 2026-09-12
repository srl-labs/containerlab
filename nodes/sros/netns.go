package sros

import (
	"fmt"
	"maps"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// namespaceNode owns the shared network namespace of a components-based SR-SIM node
type namespaceNode struct {
	clabnodes.DefaultNode
}

func (n *namespaceNode) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg
	n.StopSignal = clabtypes.SIGTERM

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *sros) newNetnsConfig() *clabtypes.NodeConfig {

	labels := maps.Clone(n.Cfg.Labels)

	if labels == nil {
		labels = map[string]string{}
	}

	shortName := fmt.Sprintf("%s-%s", n.Cfg.ShortName, "netns")
	labels[clabconstants.NodeName] = shortName

	longName := fmt.Sprintf("%s-%s", n.Cfg.LongName, "netns")
	labels[clabconstants.LongName] = longName

	labels[clabconstants.NodeType] = "srsim-netns"
	labels[clabconstants.InternalNode] = "true"

	// this node will be the root node
	labels[clabconstants.RootNodeName] = n.Cfg.ShortName
	labels[clabconstants.RootNodeLongName] = n.Cfg.LongName

	return &clabtypes.NodeConfig{
		ShortName:             shortName,
		LongName:              longName,
		LabDir:                n.Cfg.LabDir,
		Index:                 n.Cfg.Index,
		Kind:                  n.Cfg.Kind,
		NodeType:              "srsim-netns",
		Image:                 n.Cfg.Image,
		ImagePullPolicy:       n.Cfg.ImagePullPolicy,
		RestartPolicy:         n.Cfg.RestartPolicy,
		Sysctls:               maps.Clone(n.Cfg.Sysctls),
		User:                  "0:0",
		Entrypoint:            "/bin/sh",
		Cmd:                   `-c "trap 'exit 0' TERM INT; sleep infinity & wait $!"`,
		Env:                   map[string]string{},
		PortBindings:          n.Cfg.PortBindings,
		PortSet:               n.Cfg.PortSet,
		NetworkMode:           n.Cfg.NetworkMode,
		MgmtNet:               n.Cfg.MgmtNet,
		MgmtIntf:              n.Cfg.MgmtIntf,
		MgmtIPv4Address:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4PrefixLength:  n.Cfg.MgmtIPv4PrefixLength,
		MgmtIPv6Address:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6PrefixLength:  n.Cfg.MgmtIPv6PrefixLength,
		MgmtIPv4Gateway:       n.Cfg.MgmtIPv4Gateway,
		MgmtIPv6Gateway:       n.Cfg.MgmtIPv6Gateway,
		MacAddress:            n.Cfg.MacAddress,
		Aliases:               n.Cfg.Aliases,
		DNS:                   n.Cfg.DNS,
		ExtraHosts:            n.Cfg.ExtraHosts,
		Labels:                labels,
		Runtime:               n.Cfg.Runtime,
		ResultingPortBindings: n.Cfg.ResultingPortBindings,
	}
}
