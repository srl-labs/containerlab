package sros

import (
	"context"
	"errors"
	"fmt"
	"maps"

	clabutils "github.com/srl-labs/containerlab/utils"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netns"
)

// namespaceNode owns the shared network namespace of a components-based SR-SIM node.
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

func (n *sros) ensureNetnsRunning(ctx context.Context) error {
	switch status := n.netnsNode.GetContainerStatus(ctx); status {
	case clabruntime.Running:
		// The namespace holder intentionally survives regular node lifecycle operations.
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

func (n *sros) cleanupParkingNetNS() error {
	name := clabutils.ParkingNetnsName(n.Cfg.LongName)
	if _, err := clabutils.GetNamedNetNS(name); err != nil {
		return nil
	}

	return netns.DeleteNamed(name)
}

// GetNSPath retrieves the Namespace Path.
func (n *sros) GetNSPath(ctx context.Context) (string, error) {
	if n.isStandaloneNode() || (n.isDistributedCardNode() && n.rootCtrName == "") {
		return n.DefaultNode.GetNSPath(ctx)
	} else if n.isDistributedCardNode() {
		return n.Runtime.GetNSPath(ctx, n.rootCtrName)
	}
	return n.netnsNode.GetNSPath(ctx)
}

// DeleteNetnsSymlink deletes the symlink file created for the container netns.
func (n *sros) DeleteNetnsSymlink() error {
	var errs []error

	// if it is the base node, then we need to delete the symlink for all the components.
	if n.isDistributedBaseNode() {
		for _, componentNode := range n.componentNodes {
			errs = append(errs, componentNode.DeleteNetnsSymlink())
		}
		if n.netnsNode != nil {
			errs = append(errs, n.netnsNode.DeleteNetnsSymlink())
		}
		errs = append(errs, clabutils.DeleteNetnsSymlink(n.Cfg.LongName))
	}

	errs = append(errs, n.DefaultNode.DeleteNetnsSymlink(), n.cleanupParkingNetNS())

	return errors.Join(errs...)
}
