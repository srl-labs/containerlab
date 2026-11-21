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
	// parentNode is the node whose container namespace hosts this bridge
	parentNode clabnodes.Node
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

// SetNodesMap implements clabnodes.NamespaceSharerNodesSetter so core can wire
// the topology's nodes into the bridge after construction. The bridge keeps
// only the parent node hosting its namespace; the rest of the map is not
// stored. This makes the parent container resolvable for link deployment and
// namespace work even when the bridge node itself is never Deploy()ed (the
// reconcile/apply path).
func (n *bridge) SetNodesMap(nodes map[string]clabnodes.Node) {
	if n.containerNs == "" {
		return
	}
	if n.parentNode == nil {
		n.parentNode = nodes[n.containerNs]
	}
}

// resolveParentNode returns the wired parent node or an error explaining the
// missing wiring.
func (n *bridge) resolveParentNode() (clabnodes.Node, error) {
	if n.parentNode == nil {
		return nil, fmt.Errorf(
			"bridge %q: parent container %q not available (nodes not wired)",
			n.GetShortName(),
			n.containerNs,
		)
	}
	return n.parentNode, nil
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
	// Deploy's map is authoritative for wiring the parent node.
	if n.Config().NetworkMode != "" {
		cntName, err := clabutils.ContainerNameFromNetworkMode(n.Config().NetworkMode)
		if err != nil {
			return err
		}
		n.parentNode = dp.Nodes[cntName]

		if n.parentNode == nil {
			return fmt.Errorf(
				"bridge %q: parent container %q not found in topology",
				n.GetShortName(),
				cntName,
			)
		}

		// For BridgeNS, wait for parent container to be running before creating bridge
		if err := n.waitForParentContainer(ctx, n.parentNode); err != nil {
			return err
		}
		err = n.parentNode.ExecFunction(ctx, func(nn ns.NetNS) error {
			bridgeName := n.nameWithoutSeparatorSuffix()
			// check if bridge already exists
			existingLink, err := netlink.LinkByName(bridgeName)
			if err == nil {
				// Bridge already exists, just ensure it's up
				return netlink.LinkSetUp(existingLink)
			}
			// Bridge doesn't exist, create it
			err = netlink.LinkAdd(&netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: bridgeName,
				},
			})
			if err != nil {
				return err
			}
			// retrieve link ref
			netlinkLink, err := netlink.LinkByName(bridgeName)
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

// waitForParentContainer waits for the parent container to have a joinable network namespace.
// This is necessary for BridgeNS bridges which execute operations in the parent container's
// namespace.
func (n *bridge) waitForParentContainer(ctx context.Context, parentNode clabnodes.Node) error {
	// Poll until parent container has a joinable network namespace or context is cancelled
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status := parentNode.GetContainerStatus(ctx)
			if clabruntime.ContainerHasJoinableNetns(status) {
				return nil
			}
			if status == clabruntime.NotFound {
				// Parent container doesn't exist yet, keep waiting
				continue
			}
			if status == clabruntime.Stopped {
				return fmt.Errorf(
					"parent container %q has stopped (status=%s)",
					n.containerNs,
					status,
				)
			}
		}
	}
}

// ExecFunction executes the given function in the bridge's network namespace.
// For BridgeNS bridges (in-container bridges), this executes in the parent container's namespace.
// For host bridges, this executes in the host's root namespace.
func (b *bridge) ExecFunction(ctx context.Context, f func(ns.NetNS) error) error {
	// For BridgeNS bridges, we need to execute in the parent container's namespace
	if b.containerNs != "" {
		parentNode, err := b.resolveParentNode()
		if err != nil {
			// Without the wired parent there is no namespace to execute in.
			return err
		}
		return parentNode.ExecFunction(ctx, f)
	}
	// For host bridges, execute in the current (host) namespace
	curNs, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}
	return curNs.Do(f)
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
		return b.enslaveBridgePortsInNamespace()
	}
	return b.installIPTablesBridgeFwdRule()
}

// GetContainerStatus reports the parent container's status for in-container
// bridges so lifecycle scheduling (parking, restart ordering) treats the
// bridge as alive whenever its namespace host exists.
func (b *bridge) GetContainerStatus(ctx context.Context) clabruntime.ContainerStatus {
	if b.containerNs == "" {
		return b.DefaultNode.GetContainerStatus(ctx)
	}
	if parentNode := b.ParentNode(ctx); parentNode != nil {
		return parentNode.GetContainerStatus(ctx)
	}
	return clabruntime.NotFound
}

// ParentNode resolves the node that hosts this bridge's network namespace.
func (b *bridge) ParentNode(_ context.Context) clabnodes.Node {
	if b.containerNs == "" {
		return nil
	}
	return b.parentNode
}

// ParkEndpoints and RestoreEndpoints are no-ops for in-container bridges: the
// bridge-side veth ends live in the parent container's namespace and are
// therefore carried through parent recreation by the parent's own park/restore
// machinery. Parking them separately would move the same interfaces twice.
func (*bridge) ParkEndpoints(_ context.Context) error { return nil }

func (*bridge) RestoreEndpoints(_ context.Context) error { return nil }

// enslaveBridgePortsInNamespace re-enslaves this bridge's tracked endpoint
// interfaces (bridge-side veth ends) to the bridge inside the parent's
// namespace. Moving interfaces between namespaces (e.g. parking during parent
// recreation) clears the master, so restore paths need this fixup. Ports
// already enslaved are left untouched; missing ports are skipped and remain
// the link deployment path's responsibility.
func (b *bridge) enslaveBridgePortsInNamespace() error {
	bridgeName := b.nameWithoutSeparatorSuffix()
	nodeName := b.GetShortName()

	return b.ExecFunction(context.Background(), func(_ ns.NetNS) error {
		bridgeLink, err := netlink.LinkByName(bridgeName)
		if err != nil {
			log.Debugf(
				"bridge %q not found while enslaving ports, skipping: %v",
				bridgeName,
				err,
			)
			return nil
		}

		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, ep := range b.GetEndpoints() {
			for _, link := range links {
				if !clablinks.HasOwnershipAltNameFor(
					link,
					nodeName,
					ep.GetIfaceName(),
				) {
					continue
				}

				if link.Attrs().MasterIndex != bridgeLink.(*netlink.Bridge).Index {
					if err := netlink.LinkSetMaster(link, bridgeLink); err != nil {
						return fmt.Errorf("failed to enslave %q to %q: %w",
							link.Attrs().Name, bridgeName, err)
					}
					log.Info(
						"Re-enslaved restored bridge port to bridge",
						"node",
						nodeName,
						"port",
						link.Attrs().Name,
					)
				}

				if link.Attrs().OperState != netlink.OperUp {
					if err := netlink.LinkSetUp(link); err != nil {
						return fmt.Errorf("failed to bring up %q: %w",
							link.Attrs().Name, err)
					}
				}
			}
		}

		return nil
	})
}

func (b *bridge) GetNSPath(ctx context.Context) (string, error) {
	if b.containerNs != "" {
		parentNode, err := b.resolveParentNode()
		if err != nil {
			return "", err
		}
		return parentNode.GetNSPath(ctx)
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

// LinkApplyMode returns live: bridge enslaving/enslave-detach is hot-swappable
// netlink work, so adding/removing/retargeting bridge ports must not restart or
// recreate the bridge node. Intact links keep their endpoints untouched.
func (*bridge) LinkApplyMode(context.Context) clabnodes.LinkApplyMode {
	return clabnodes.LinkApplyModeLive
}

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
	parentNode, err := b.resolveParentNode()
	if err != nil {
		return err
	}
	err = parentNode.AddLinkToContainer(ctx, link, func(nn ns.NetNS) error {
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

// installIPTablesBridgeFwdRule installs `allow` rule for the traffic routed in and out of the
// bridge
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
