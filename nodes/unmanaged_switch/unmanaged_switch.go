// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package unmanaged_switch

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	defaultImage = "alpine:3"
	defaultCmd   = "sleep infinity"

	// bridgeName is the name of the bridge inside the switch container.
	bridgeName = "br0"
)

var kindNames = []string{"unmanaged-switch"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(unmanagedSwitch)
	}, nrea)
}

// unmanagedSwitch attaches all of its endpoints to a bridge in its own netns.
type unmanagedSwitch struct {
	clabnodes.DefaultNode
}

func (*unmanagedSwitch) LinkApplyMode(context.Context) clabnodes.LinkApplyMode {
	return clabnodes.LinkApplyModeLive
}

func (n *unmanagedSwitch) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	// sleep ignores SIGTERM as PID 1
	n.StopSignal = clabtypes.SIGKILL

	if n.Cfg.Image == "" {
		n.Cfg.Image = defaultImage
	}
	if n.Cfg.Cmd == "" {
		n.Cfg.Cmd = defaultCmd
	}

	for _, o := range opts {
		o(n)
	}

	// the switch has no management interface
	if n.Cfg.NetworkMode != "" && n.Cfg.NetworkMode != "none" {
		return fmt.Errorf("network-mode %q is not supported by %s node %q",
			n.Cfg.NetworkMode, n.Cfg.Kind, n.Cfg.ShortName)
	}
	n.Cfg.NetworkMode = "none"

	// keep the switch from sending IPv6 ND and MLD messages
	if n.Cfg.Sysctls == nil {
		n.Cfg.Sysctls = map[string]string{}
	}
	n.Cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "1"
	n.Cfg.Sysctls["net.ipv6.conf.default.disable_ipv6"] = "1"

	return nil
}

// Start recreates the bridge, which is lost on stop, and reattaches the restored endpoints.
func (n *unmanagedSwitch) Start(ctx context.Context) error {
	if err := n.DefaultNode.Start(ctx); err != nil {
		return err
	}

	return n.ExecFunction(ctx, func(_ ns.NetNS) error {
		br, err := ensureBridge()
		if err != nil {
			return err
		}

		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			if !clablinks.HasOwnershipAltName(link) {
				continue
			}

			if err := netlink.LinkSetMaster(link, br); err != nil {
				return err
			}
		}

		return nil
	})
}

func (n *unmanagedSwitch) AddLinkToContainer(
	ctx context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	return n.DefaultNode.AddLinkToContainer(ctx, link, func(nn ns.NetNS) error {
		if err := f(nn); err != nil {
			return err
		}

		br, err := ensureBridge()
		if err != nil {
			return err
		}

		return netlink.LinkSetMaster(link, br)
	})
}

// ensureBridge creates the bridge in the current netns if it does not exist yet.
func ensureBridge() (netlink.Link, error) {
	snooping := false
	err := netlink.LinkAdd(&netlink.Bridge{
		LinkAttrs:         netlink.LinkAttrs{Name: bridgeName},
		MulticastSnooping: &snooping,
	})
	// links may be attached concurrently
	if err != nil && !errors.Is(err, syscall.EEXIST) {
		return nil, err
	}

	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, err
	}

	return br, netlink.LinkSetUp(br)
}

// CheckInterfaceName rejects eth0, which is lost on stop/start.
func (n *unmanagedSwitch) CheckInterfaceName() error {
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" {
			return fmt.Errorf("eth0 interface name is not allowed for %s node", n.Cfg.ShortName)
		}
	}

	return n.CheckInterfaceOverlap()
}
