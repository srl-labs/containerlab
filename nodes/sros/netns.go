package sros

import (
	"context"
	"errors"
	"fmt"
	"maps"

	clabutils "github.com/srl-labs/containerlab/utils"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netns"
)

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
