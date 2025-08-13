// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/runtime/docker/firewall"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

var kindNames = []string{"bridge"}

// bridgeNodeSep is a separator used in the bridge name to distinguish
// bridges attached to different namespaces in the topology.
var bridgeNodeSep = "|"

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(bridge)
	}, nrea)
}

type bridge struct {
	clabnodes.DefaultNode
	containerNs string
	nodesMap    map[string]clabnodes.Node
}

func (s *bridge) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *clabnodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	if s.Cfg.NetworkMode == "" || s.Cfg.NetworkMode == "host" {
		s.Cfg.IsRootNamespaceBased = true
	} else {
		var err error
		s.containerNs, err = clabutils.ContainerNameFromNetworkMode(s.Config().NetworkMode)
		if err != nil {
			return err
		}
	}
	return nil
}

// nameWithoutSeparatorSuffix returns the bridge name without the separator suffix
// used to distinguish bridges attached to different namespaces in the topology.
// For example, if the bridge name is "br0|ns1", it will return "br0".
func (n *bridge) nameWithoutSeparatorSuffix() string {
	s := n.GetShortName()
	if idx := strings.Index(s, bridgeNodeSep); idx != -1 {
		return s[:idx]
	}
	return s
}

func (n *bridge) Deploy(ctx context.Context, dp *clabnodes.DeployParams) error {
	// store nodes map for later use
	n.nodesMap = dp.Nodes

	// if the NetworkMode is set, then the bridge is setup within a namespace, so it must be created.
	if n.Config().NetworkMode != "" {
		cntName, err := clabutils.ContainerNameFromNetworkMode(n.Config().NetworkMode)
		if err != nil {
			return err
		}
		err = dp.Nodes[cntName].ExecFunction(ctx, func(nn ns.NetNS) error {
			// add the bridge
			err := netlink.LinkAdd(&netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: n.nameWithoutSeparatorSuffix(),
				},
			})
			if err != nil {
				return err
			}
			// retrieve link ref
			netlinkLink, err := netlink.LinkByName(n.nameWithoutSeparatorSuffix())
			if err != nil {
				return err
			}
			// bring the link up
			err = netlink.LinkSetUp(netlinkLink)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	n.SetState(clabnodesstate.Deployed)
	return nil
}

func (*bridge) Delete(_ context.Context) error {
	// we are not deleting iptables rules set up in the post deploy stage
	// because we can't guarantee that the bridge is not used by another topology.
	return nil
}

func (*bridge) GetImages(_ context.Context) map[string]string { return map[string]string{} }

// DeleteNetnsSymlink is a noop for bridge nodes.
func (b *bridge) DeleteNetnsSymlink() (err error) { return nil }

func (b *bridge) PostDeploy(_ context.Context, _ *clabnodes.PostDeployParams) error {
	if b.containerNs != "" {
		return nil
	}
	return b.installIPTablesBridgeFwdRule()
}

func (b *bridge) GetNSPath(ctx context.Context) (string, error) {
	if b.containerNs != "" {
		node, ok := b.nodesMap[b.containerNs]
		if !ok {
			return "", fmt.Errorf("unable to find node %s", b.containerNs)
		}
		return node.GetNSPath(ctx)
	}
	curns, err := ns.GetCurrentNS()
	if err != nil {
		return "", err
	}
	return curns.Path(), nil
}

func (b *bridge) CheckDeploymentConditions(ctx context.Context) error {
	err := b.VerifyHostRequirements()
	if err != nil {
		return err
	}

	if strings.HasPrefix(b.Cfg.NetworkMode, "container:") {
		if b.Cfg.NetworkMode[10:] != b.GetShortName()[len(b.nameWithoutSeparatorSuffix())+1:] {
			return fmt.Errorf("container based bridge requires container name as suffix %s != %s",
				b.Cfg.NetworkMode[10:], b.GetShortName()[len(b.nameWithoutSeparatorSuffix())+1:])
		}
	}

	// check bridge exists only if host ns
	if b.containerNs == "" {
		err = b.ExecFunction(ctx, func(nn ns.NetNS) error {
			// check bridge exists
			_, err = clabutils.BridgeByName(b.nameWithoutSeparatorSuffix())
			if err != nil {
				return err
			}
			return nil
		})
	}

	return err
}

func (*bridge) PullImage(_ context.Context) error { return nil }

// UpdateConfigWithRuntimeInfo is a noop for bridges.
func (*bridge) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers is a noop for bridges.
func (*bridge) GetContainers(_ context.Context) ([]clabruntime.GenericContainer, error) {
	return nil, nil
}

// RunExec is a noop for bridge kind.
func (b *bridge) RunExec(_ context.Context, _ *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
	log.Warnf("Exec operation is not implemented for kind %q", b.Config().Kind)
	return nil, clabexec.ErrRunExecNotSupported
}

func (b *bridge) AddLinkToContainer(ctx context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	if b.Cfg.NetworkMode != "" {
		return b.addLinkToContainerNamespace(ctx, link, f)
	}
	return b.addLinkToContainerHost(ctx, link, f)
}

func (b *bridge) addLinkToContainerNamespace(ctx context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	cntName, err := clabutils.ContainerNameFromNetworkMode(b.Config().NetworkMode)
	if err != nil {
		return err
	}
	err = b.nodesMap[cntName].AddLinkToContainer(ctx, link, func(nn ns.NetNS) error {
		// get the bridge as netlink.Link
		br, err := netlink.LinkByName(b.nameWithoutSeparatorSuffix())
		if err != nil {
			return err
		}

		// assign the bridge to the link as master
		err = netlink.LinkSetMaster(link, br)
		if err != nil {
			return err
		}

		// execute the given function
		return f(nn)
	})
	return err
}

func (b *bridge) addLinkToContainerHost(_ context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	curNamespace, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}

	// get the bridge as netlink.Link
	br, err := netlink.LinkByName(b.nameWithoutSeparatorSuffix())
	if err != nil {
		return err
	}

	// assign the bridge to the link as master
	err = netlink.LinkSetMaster(link, br)
	if err != nil {
		return err
	}

	// execute the given function
	return curNamespace.Do(f)
}

func (b *bridge) GetLinkEndpointType() clablinks.LinkEndpointType {
	if b.containerNs != "" {
		return clablinks.LinkEndpointTypeBridgeNS
	}
	return clablinks.LinkEndpointTypeBridge
}

// installIPTablesBridgeFwdRule installs `allow` rule for the traffic routed in and out of the bridge
// otherwise, communication over the bridge is not permitted on most systems.
func (b *bridge) installIPTablesBridgeFwdRule() (err error) {
	f, err := firewall.NewFirewallClient()
	if err != nil {
		return err
	}
	log.Debugf("setting up bridge firewall rules using %s as the firewall interface", f.Name())

	r := definitions.FirewallRule{
		Interface: b.nameWithoutSeparatorSuffix(),
		Direction: definitions.InDirection,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
		Table:     definitions.FilterTable,
		Chain:     definitions.ForwardChain,
	}
	err = f.InstallForwardingRules(&r)
	if err != nil {
		return err
	}

	r = definitions.FirewallRule{
		Interface: b.nameWithoutSeparatorSuffix(),
		Direction: definitions.OutDirection,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
		Table:     definitions.FilterTable,
		Chain:     definitions.ForwardChain,
	}

	return f.InstallForwardingRules(&r)
}
