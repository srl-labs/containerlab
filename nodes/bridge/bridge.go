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

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	iptCheckCmd = "-vL FORWARD"
	iptAllowCmd = "-I FORWARD -i %s -j ACCEPT"
)

func init() {
	nodes.Register(nodes.NodeKindBridge, func() nodes.Node {
		return new(bridge)
	})
}

type bridge struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *bridge) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	s.cfg.DeploymentStatus = "created" // since we do not create bridges with clab, the status is implied here
	return nil
}
func (s *bridge) Config() *types.NodeConfig    { return s.cfg }
func (*bridge) PreDeploy(_, _, _ string) error { return nil }
func (*bridge) Deploy(_ context.Context) error { return nil }
func (b *bridge) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	return b.installIPTablesBridgeFwdRule()
}
func (*bridge) WithMgmtNet(*types.MgmtNet)               {}
func (s *bridge) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *bridge) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (*bridge) GetContainer(_ context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (*bridge) SaveConfig(_ context.Context) error { return nil }

func (*bridge) GetImages() map[string]string { return map[string]string{} }

func (*bridge) Delete(_ context.Context) error {
	return nil
}

// installIPTablesBridgeFwdRule calls iptables to install `allow` rule for traffic passing through the bridge
// otherwise, communication over the bridge is not permitted
func (b *bridge) installIPTablesBridgeFwdRule() (err error) {

	// first check if a rule already exists for this bridge to not create duplicates
	res, err := exec.Command("iptables", strings.Split(iptCheckCmd, " ")...).Output()

	re, _ := regexp.Compile(fmt.Sprintf("ACCEPT[^\n]+%s", b.cfg.ShortName))

	if re.Match(res) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", b.cfg.ShortName)
		return err
	}
	if err != nil {
		// non nil error typically means that DOCKER-USER chain doesn't exist
		// this happens with old docker installations (centos7 hello) from default repos
		return fmt.Errorf("failed to add iptables forwarding rule for bridge %q", b.cfg.ShortName)
	}

	cmd := fmt.Sprintf(iptAllowCmd, b.cfg.ShortName)

	log.Debugf("Installing iptables rules for bridge %q", b.cfg.ShortName)

	stdOutErr, err := exec.Command("iptables", strings.Split(cmd, " ")...).CombinedOutput()
	if err != nil {
		log.Warnf("iptables install stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to create iptables rules: %w", err)
	}
	return nil
}
