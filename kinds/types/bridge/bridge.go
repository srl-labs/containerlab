// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package bridge

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	cExec "github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

var kindnames = []string{"bridge"}

const (
	iptCheckCmd = "-vL FORWARD -w 5"
	iptAllowCmd = "-I FORWARD -i %s -j ACCEPT -w 5"
)

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(bridge)
	}, nil)
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

func (*bridge) Delete(_ context.Context) error                { return nil }
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

// installIPTablesBridgeFwdRule calls iptables to install `allow` rule for traffic passing through the bridge
// otherwise, communication over the bridge is not permitted.
func (b *bridge) installIPTablesBridgeFwdRule() (err error) {
	// first check if a rule already exists for this bridge to not create duplicates
	res, err := exec.Command("iptables", strings.Split(iptCheckCmd, " ")...).Output()

	re, _ := regexp.Compile(fmt.Sprintf("ACCEPT[^\n]+%s", b.Cfg.ShortName))

	if re.Match(res) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", b.Cfg.ShortName)
		return err
	}
	if err != nil {
		return fmt.Errorf("failed to add iptables forwarding rule for bridge %q: %w", b.Cfg.ShortName, err)
	}

	cmd := fmt.Sprintf(iptAllowCmd, b.Cfg.ShortName)

	log.Debugf("Installing iptables rules for bridge %q", b.Cfg.ShortName)

	stdOutErr, err := exec.Command("iptables", strings.Split(cmd, " ")...).CombinedOutput()

	log.Debugf("iptables install stdout for bridge %s:%s", b.Cfg.ShortName, stdOutErr)

	if err != nil {
		log.Warnf("iptables install stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to create iptables rules: %w", err)
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
