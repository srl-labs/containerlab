package srl

import (
	"context"
	"errors"
	"fmt"
	"maps"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const netnsNodeType = "srl-netns"

// namespaceNode owns the network namespace of an SR Linux node, so that the dataplane links and
// the management address survive a restart of the SR Linux container.
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

// usesNetnsNode reports whether the node runs in the network namespace of a holder container.
// Nodes with a user-defined network mode keep the network namespace of their own container.
func (n *srl) usesNetnsNode() bool {
	return n.Cfg.NetworkMode == "" && !n.Cfg.IsRootNamespaceBased
}

func (n *srl) initNetnsNode() error {
	if !n.usesNetnsNode() {
		return nil
	}

	n.netnsNode = new(namespaceNode)

	return n.netnsNode.Init(n.newNetnsConfig(), clabnodes.WithRuntime(n.GetRuntime()))
}

func (n *srl) newNetnsConfig() *clabtypes.NodeConfig {
	labels := maps.Clone(n.Cfg.Labels)

	if labels == nil {
		labels = map[string]string{}
	}

	shortName := n.Cfg.ShortName + "-netns"
	labels[clabconstants.NodeName] = shortName

	longName := n.Cfg.LongName + "-netns"
	labels[clabconstants.LongName] = longName

	labels[clabconstants.NodeType] = netnsNodeType
	labels[clabconstants.InternalNode] = "true"

	labels[clabconstants.RootNodeName] = n.Cfg.ShortName
	labels[clabconstants.RootNodeLongName] = n.Cfg.LongName

	return &clabtypes.NodeConfig{
		ShortName:             shortName,
		LongName:              longName,
		Hostname:              n.Cfg.GetHostname(),
		Fqdn:                  n.Cfg.Fqdn,
		LabDir:                n.Cfg.LabDir,
		Index:                 n.Cfg.Index,
		Kind:                  n.Cfg.Kind,
		NodeType:              netnsNodeType,
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
		MgmtNet:               n.Cfg.MgmtNet,
		MgmtIntf:              n.Cfg.MgmtIntf,
		MgmtIPv4Address:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4AddressAuto:   n.Cfg.MgmtIPv4AddressAuto,
		MgmtIPv4PrefixLength:  n.Cfg.MgmtIPv4PrefixLength,
		MgmtIPv6Address:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6AddressAuto:   n.Cfg.MgmtIPv6AddressAuto,
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

// containerConfig returns the config of the SR Linux container attached to the holder's
// network namespace. The runtime rejects network settings for such containers, as they
// belong to the holder.
func (n *srl) containerConfig() *clabtypes.NodeConfig {
	cfg := *n.Cfg
	cfg.NetworkMode = "container:" + n.netnsNode.GetShortName()
	cfg.DNS = nil
	cfg.ExtraHosts = nil
	cfg.MacAddress = ""
	cfg.PortBindings = nil
	cfg.PortSet = nil

	return &cfg
}

func (n *srl) ensureNetnsRunning(ctx context.Context) error {
	switch status := n.netnsNode.GetContainerStatus(ctx); status {
	case clabruntime.Running:
	case clabruntime.Created, clabruntime.Stopped:
		if err := n.netnsNode.Start(ctx); err != nil {
			return fmt.Errorf("node %q network namespace container start error: %w",
				n.Cfg.ShortName, err)
		}
	case clabruntime.Paused:
		if err := n.Runtime.UnpauseContainer(ctx, n.netnsNode.Config().LongName); err != nil {
			return fmt.Errorf("node %q network namespace container unpause error: %w",
				n.Cfg.ShortName, err)
		}
	case clabruntime.NotFound:
		return fmt.Errorf(
			"node %q network namespace container %q not found; recreate the node",
			n.Cfg.ShortName, n.netnsNode.Config().LongName,
		)
	default:
		return fmt.Errorf("node %q network namespace container %q is %s",
			n.Cfg.ShortName, n.netnsNode.Config().LongName, status)
	}

	return nil
}

// linkNetns points the node's netns symlink to the holder, as the namespace path of the
// SR Linux container changes whenever the container restarts.
func (n *srl) linkNetns(ctx context.Context) error {
	nsPath, err := n.netnsNode.GetNSPath(ctx)
	if err != nil {
		return err
	}

	return clabutils.LinkContainerNS(nsPath, n.Cfg.LongName)
}

// GetNSPath retrieves the Namespace Path.
func (n *srl) GetNSPath(ctx context.Context) (string, error) {
	if n.netnsNode == nil {
		return n.DefaultNode.GetNSPath(ctx)
	}

	return n.netnsNode.GetNSPath(ctx)
}

// DeleteNetnsSymlink deletes the symlink file created for the container netns.
func (n *srl) DeleteNetnsSymlink() error {
	if n.netnsNode == nil {
		return n.DefaultNode.DeleteNetnsSymlink()
	}

	return errors.Join(n.DefaultNode.DeleteNetnsSymlink(), n.netnsNode.DeleteNetnsSymlink())
}
