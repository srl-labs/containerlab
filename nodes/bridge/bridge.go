// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// autoBridgeCreated tracks whether we created the bridge during PreDeploy
	// so we know if we should clean it up during Delete
	autoBridgeCreated bool
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

func (b *bridge) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	// Only auto-create bridges in the host namespace (not in container namespace)
	if b.containerNs != "" {
		return nil
	}

	// Check if the bridge already exists
	err := b.ExecFunction(ctx, func(nn ns.NetNS) error {
		_, err := clabutils.BridgeByName(b.nameWithoutSeparatorSuffix())
		return err
	})

	// If bridge exists, we don't create it (and won't delete it either)
	if err == nil {
		b.autoBridgeCreated = false
		log.Debugf("Bridge %s already exists, not auto-creating", b.nameWithoutSeparatorSuffix())
		return nil
	}

	// If there was an error other than "not found", return it
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
		return fmt.Errorf("error checking bridge existence: %w", err)
	}

	// Bridge does not exist, create it
	log.Infof("Creating bridge %s", b.nameWithoutSeparatorSuffix())
	err = b.ExecFunction(ctx, func(nn ns.NetNS) error {
		// add the bridge
		err := netlink.LinkAdd(&netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: b.nameWithoutSeparatorSuffix(),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create bridge: %w", err)
		}
		// retrieve link ref
		netlinkLink, err := netlink.LinkByName(b.nameWithoutSeparatorSuffix())
		if err != nil {
			return fmt.Errorf("failed to get bridge link: %w", err)
		}
		// bring the link up
		err = netlink.LinkSetUp(netlinkLink)
		if err != nil {
			return fmt.Errorf("failed to bring bridge up: %w", err)
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Mark that we created this bridge so we can clean it up later
	b.autoBridgeCreated = true
	log.Debugf("Successfully created bridge %s", b.nameWithoutSeparatorSuffix())

	return nil
}

func (n *bridge) Deploy(ctx context.Context, dp *clabnodes.DeployParams) error {
	// store nodes map for later use
	n.nodesMap = dp.Nodes

	// if the NetworkMode is set, then the bridge is setup within a namespace, so it must be
	// created.
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

func (b *bridge) Delete(ctx context.Context) error {
	log.Debugf("Bridge Delete called for %s, containerNs: %s", 
		b.nameWithoutSeparatorSuffix(), b.containerNs)
	
	// we are not deleting iptables rules set up in the post deploy stage
	// because we can't guarantee that the bridge is not used by another topology.

	// Only auto-delete bridges in the host namespace that appear to be containerlab-managed
	// This is determined by naming patterns or other characteristics
	if b.containerNs != "" {
		log.Debugf("Bridge %s is in container namespace, not auto-deleting", 
			b.nameWithoutSeparatorSuffix())
		return nil
	}
	
	// Be conservative about which bridges to auto-delete
	// Only delete bridges that look like they were created for this specific topology
	if !b.shouldAutoDelete() {
		log.Debugf("Bridge %s doesn't appear to be auto-manageable, not deleting", 
			b.nameWithoutSeparatorSuffix())
		return nil
	}

	bridgeName := b.nameWithoutSeparatorSuffix()
	
	// Wait a short time to allow container network cleanup to progress
	// This gives time for the container deletion process to remove interfaces
	time.Sleep(100 * time.Millisecond)
	
	// Try to delete the bridge if no slave interfaces remain
	// Poll for a few seconds to handle timing
	maxAttempts := 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		deleted, err := b.tryDeleteBridge(ctx, bridgeName)
		if err != nil {
			log.Debugf("Error during bridge deletion attempt %d: %v", attempt+1, err)
			return nil // Don't fail the overall deletion
		}
		
		if deleted {
			return nil
		}
		
		// Wait a bit before trying again, but not too long to avoid blocking
		if attempt < maxAttempts-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	
	log.Debugf("Bridge %s still has slave interfaces after %d attempts, leaving it", bridgeName, maxAttempts)
	return nil
}

// tryDeleteBridge attempts to delete the bridge if no slave interfaces remain
// Returns (deleted, error)
func (b *bridge) tryDeleteBridge(ctx context.Context, bridgeName string) (bool, error) {
	err := b.ExecFunction(ctx, func(nn ns.NetNS) error {
		br, err := netlink.LinkByName(bridgeName)
		if err != nil {
			// Bridge doesn't exist, consider it deleted
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
				log.Debugf("Bridge %s already deleted", bridgeName)
				return nil
			}
			return fmt.Errorf("error checking bridge: %w", err)
		}

		// Get all links and check if any are enslaved to this bridge
		links, err := netlink.LinkList()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		hasSlaves := false
		slaveNames := []string{}
		for _, link := range links {
			// Skip the bridge itself and loopback interface
			if link.Attrs().Index == br.Attrs().Index || link.Type() == "loopback" {
				continue
			}
			
			// Check if this link is enslaved to our bridge
			if link.Attrs().MasterIndex == br.Attrs().Index {
				hasSlaves = true
				slaveNames = append(slaveNames, link.Attrs().Name)
			}
		}

		if hasSlaves {
			log.Debugf("Bridge %s still has slave interfaces: %v, waiting", bridgeName, slaveNames)
			return fmt.Errorf("has slaves") // Signal that we should try again
		}

		// No slave interfaces remain, delete the bridge
		log.Infof("Deleting bridge %s (no slave interfaces remain)", bridgeName)
		err = netlink.LinkDel(br)
		if err != nil {
			return fmt.Errorf("failed to delete bridge %s: %w", bridgeName, err)
		}

		log.Debugf("Successfully deleted bridge %s", bridgeName)
		return nil
	})
	
	if err != nil {
		if strings.Contains(err.Error(), "has slaves") {
			return false, nil // Not deleted, but no error
		}
		return false, err // Real error
	}
	
	return true, nil // Successfully deleted
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

	// For host namespace bridges, we now create them automatically in PreDeploy
	// so we don't need to check if they exist here
	// We still check for container namespace bridges since they should exist within their container
	if b.containerNs != "" {
		// For bridges in container namespaces, the bridge will be created during Deploy
		// so we don't need to check for existence here either
	}

	return nil
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

func (b *bridge) AddLinkToContainer(
	ctx context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	if b.Cfg.NetworkMode != "" {
		return b.addLinkToContainerNamespace(ctx, link, f)
	}
	return b.addLinkToContainerHost(ctx, link, f)
}

func (b *bridge) addLinkToContainerNamespace(
	ctx context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
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

func (b *bridge) addLinkToContainerHost(
	_ context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
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

// shouldAutoDelete determines if this bridge should be auto-deleted based on heuristics
func (b *bridge) shouldAutoDelete() bool {
	bridgeName := b.nameWithoutSeparatorSuffix()
	
	// Only delete bridges that were likely created for containerlab use
	// This includes bridges that:
	// 1. Were created during PreDeploy execution (if we can track it)
	// 2. Have naming patterns that suggest containerlab usage
	
	// If we tracked that this bridge was auto-created, always allow deletion
	if b.autoBridgeCreated {
		return true
	}
	
	// Otherwise, use naming heuristics
	// Common containerlab bridge naming patterns:
	// - Contains "clab" in the name
	// - Contains the topology name
	// - Follows certain patterns like "br-something"
	
	if strings.Contains(bridgeName, "clab") {
		return true
	}
	
	// For bridges named like "test-br", "lab-br", etc., be more careful
	// Only auto-delete if they match specific patterns AND we're confident
	commonPatterns := []string{"test-", "lab-", "demo-"}
	for _, pattern := range commonPatterns {
		if strings.HasPrefix(bridgeName, pattern) && strings.HasSuffix(bridgeName, "-br") {
			return true
		}
	}
	
	// Be conservative - don't delete bridges we're not sure about
	return false
}
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
