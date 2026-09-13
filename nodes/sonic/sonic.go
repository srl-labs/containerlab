// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sonic

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	// mgmtIfName is the management interface of a sonic-vs node. It is created
	// by the runtime rather than by a link, so it is not an endpoint of the node
	// today - but a node with `network-mode: none` may declare one, and the
	// management interface is not a wire behind a SONiC port, so it is filtered
	// out explicitly.
	mgmtIfName = "eth0"

	scrapliPlatformName = "sonic"
)

var kindNames = []string{"sonic-vs"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, platformAttrs)

	r.Register(kindNames, func() clabnodes.Node {
		return new(sonic)
	}, nrea)
}

type sonic struct {
	clabnodes.DefaultNode
}

func (s *sonic) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *clabnodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// the entrypoint is reset to prevent it from starting before all interfaces are connected
	// all main sonic agents are started in a post-deploy phase
	s.Cfg.Entrypoint = "/bin/bash"
	return nil
}

func (s *sonic) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(s.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}
	return nil
}

// wireInterfaceCmds returns the commands that quiet the kernel on the
// containerlab-created wire interfaces of a sonic-vs node. The management
// interface is skipped: it is not a wire behind a SONiC port.
func wireInterfaceCmds(ifNames []string) []string {
	cmds := make([]string, 0, len(ifNames)*2)

	for _, ifName := range ifNames {
		if ifName == mgmtIfName {
			continue
		}

		cmds = append(cmds,
			fmt.Sprintf("ip link set arp off dev %s", ifName),
			fmt.Sprintf("sysctl -w net.ipv6.conf.%s.disable_ipv6=1", ifName),
		)
	}

	return cmds
}

// quietWireInterfaces marks the wire interfaces of the node ARP-off and
// IPv6-off.
//
// A sonic-vs node has two kernel interfaces per link: the veth containerlab
// creates (ethN, the wire) and the tap syncd derives from it (eth1 becomes
// Ethernet0, eth2 becomes Ethernet4, and so on - the port SONiC configures).
// Left alone, the kernel answers ARP for the port's address on the wire with
// the wire's MAC and brings up an IPv6 link-local there, so a neighbour can
// cache the wrong MAC and learn a device that isn't the port. Quieting the wire
// leaves only the port answering. This is what SONiC's own vs harness does
// (sonic-swss, tests/conftest.py, VirtualServer).
//
// The commands are idempotent, so this may run more than once for a node - it
// runs at post-deploy time and again for links added to a running node. A
// failure is logged and does not fail the deploy.
func (s *sonic) quietWireInterfaces(ctx context.Context) error {
	ifNames := make([]string, 0, len(s.Endpoints))

	for _, e := range s.Endpoints {
		ifNames = append(ifNames, e.GetIfaceName())
	}

	for _, cmdStr := range wireInterfaceCmds(ifNames) {
		cmd, err := clabexec.NewExecCmdFromString(cmdStr)
		if err != nil {
			log.Warn("failed to quiet sonic-vs wire interface",
				"node", s.Cfg.ShortName, "cmd", cmdStr, "error", err)

			continue
		}

		execResult, err := s.RunExec(ctx, cmd)
		if err != nil || (execResult != nil && execResult.GetReturnCode() != 0) {
			if err == nil {
				err = fmt.Errorf("returned %d: %s",
					execResult.GetReturnCode(), execResult.GetStdErrString())
			}

			log.Warn("failed to quiet sonic-vs wire interface",
				"node", s.Cfg.ShortName, "cmd", cmdStr, "error", err)
		}
	}

	return nil
}

// PostDeployEndpoints runs sonic-vs endpoint fixups after dataplane links
// exist. It also covers links added to an already running node.
func (s *sonic) PostDeployEndpoints(ctx context.Context) error {
	return s.quietWireInterfaces(ctx)
}

// Start starts a stopped sonic-vs node and re-applies the wire fixups. Stopping
// a node parks its endpoints in a separate network namespace and starting it
// moves them back; a netdev that crosses a namespace has its IPv6 configuration
// rebuilt, so disable_ipv6 falls back to 0 and a link-local address reappears
// on the wire.
func (s *sonic) Start(ctx context.Context) error {
	if err := s.DefaultNode.Start(ctx); err != nil {
		return err
	}

	return s.PostDeployEndpoints(ctx)
}

func (s *sonic) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for sonic-vs '%s' node", s.Cfg.ShortName)

	// quiet the wires before the SONiC agents start.
	if err := s.quietWireInterfaces(ctx); err != nil {
		return err
	}

	cmd, _ := clabexec.NewExecCmdFromString("supervisord")
	err := s.RunExecNotWait(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed post-deploy node %q: %w", s.Cfg.ShortName, err)
	}

	cmd, _ = clabexec.NewExecCmdFromString("supervisorctl start bgpd")
	err = s.RunExecNotWait(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed post-deploy node %q: %w", s.Cfg.ShortName, err)
	}

	return nil
}
