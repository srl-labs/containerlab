// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package bridge

import (
	"context"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	cExec "github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/runtime/docker/firewall"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

var kindNames = []string{"bridge"}

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := nodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes)

	r.Register(kindNames, func() nodes.Node {
		return new(bridge)
	}, nrea)
}

type bridge struct {
	nodes.DefaultNode
}

func (s *bridge) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	s.Cfg.IsRootNamespaceBased = true
	return nil
}

func (n *bridge) Deploy(_ context.Context, _ *nodes.DeployParams) error {
	n.SetState(state.Deployed)
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

func (b *bridge) PostDeploy(_ context.Context, _ *nodes.PostDeployParams) error {
	return b.installIPTablesBridgeFwdRule()
}

func (b *bridge) CheckDeploymentConditions(_ context.Context) error {
	err := b.VerifyHostRequirements()
	if err != nil {
		return err
	}
	// check bridge exists
	_, err = utils.BridgeByName(b.Cfg.ShortName)
	if err != nil {
		return err
	}
	return nil
}

func (*bridge) PullImage(_ context.Context) error { return nil }

// UpdateConfigWithRuntimeInfo is a noop for bridges.
func (*bridge) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers is a noop for bridges.
func (*bridge) GetContainers(_ context.Context) ([]runtime.GenericContainer, error) { return nil, nil }

// RunExec is a noop for bridge kind.
func (b *bridge) RunExec(_ context.Context, _ *cExec.ExecCmd) (*cExec.ExecResult, error) {
	log.Warnf("Exec operation is not implemented for kind %q", b.Config().Kind)

	return nil, cExec.ErrRunExecNotSupported
}

func (b *bridge) AddLinkToContainer(ctx context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	ns, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}

	// get the bridge as netlink.Link
	br, err := netlink.LinkByName(b.Cfg.ShortName)
	if err != nil {
		return err
	}

	// assign the bridge to the link as master
	err = netlink.LinkSetMaster(link, br)
	if err != nil {
		return err
	}

	// execute the given function
	return ns.Do(f)
}

func (b *bridge) GetLinkEndpointType() links.LinkEndpointType {
	return links.LinkEndpointTypeBridge
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
		Interface: b.Cfg.ShortName,
		Direction: definitions.InDirection,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
		Table:     definitions.FilterTable,
		Chain:     definitions.ForwardChain,
	}
	err = f.InstallForwardingRules(r)
	if err != nil {
		return err
	}

	r = definitions.FirewallRule{
		Interface: b.Cfg.ShortName,
		Direction: definitions.OutDirection,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
		Table:     definitions.FilterTable,
		Chain:     definitions.ForwardChain,
	}

	return f.InstallForwardingRules(r)
}
