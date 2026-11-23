package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodestailscale "github.com/srl-labs/containerlab/nodes/tailscale"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

type applyEndpointKey struct {
	node  string
	iface string
}

func (k applyEndpointKey) String() string {
	return k.node + ":" + k.iface
}

type applyEndpointRef struct {
	key        applyEndpointKey
	node       clablinks.Node
	bestEffort bool
}

// bridgeNSMigration describes a bridge-side veth end whose owning in-container
// bridge node changed name (a parent move) while the client-side endpoint and
// the veth pair itself stay valid. Migration candidates are validated at
// planning time (parent resolvable, client peer matching and surviving this
// apply), so only resumable moves are planned. Applying the migration moves the
// parked half into the new parent's namespace instead of destroying and
// recreating the pair, which preserves container-local configuration
// (addresses, MACs).
type bridgeNSMigration struct {
	staleRef   applyEndpointRef // bridge-side endpoint to be parked (not deleted)
	newKey     applyEndpointKey // replacement claim under a different bridge node
	relocation *clablinks.BridgePortRelocation
}

// desertedBridgeDevice is an in-container bridge's device left behind in a
// parent namespace that survives the apply after the bridge node moved to
// another parent.
type desertedBridgeDevice struct {
	parentName string
	deviceName string
}

type applyPlan struct {
	currentNodes       map[string]*runtimeNodeGroup
	addedNodeSet       map[string]struct{}
	deletedNodeSet     map[string]struct{}
	recreatedNodeSet   map[string]struct{}
	restartNodeSet     map[string]struct{}
	linkRestartNodeSet map[string]struct{}
	nodeDiffs          map[string]*clabtypes.TopologyDiff
	addedLinks         []clablinks.Link
	plannedLinkSet     map[int]struct{}
	staleEndpoints     []applyEndpointRef
	desiredEndpointSet map[applyEndpointKey]struct{}
	liveEndpointSet    map[applyEndpointKey]struct{}
	liveEndpointInfo   map[applyEndpointKey]clablinks.OwnedInterface
	endpointNodes      map[string]clablinks.Node
	state              *LabState
	parkedNodeSet      map[string]struct{}
	startNodeSet       map[string]struct{}
	// nodeChangeReasons explains per node why apply restarts or recreates it,
	// e.g. "added link" or "config drift: image".
	nodeChangeReasons  map[string]string
	bridgeNSMigrations []bridgeNSMigration
	// desertedBridgeDevices holds in-container bridge devices to delete from
	// surviving parent namespaces after migrated ports are restored elsewhere.
	desertedBridgeDevices []desertedBridgeDevice
}

func (p *applyPlan) empty() bool {
	return len(p.addedNodeSet) == 0 &&
		len(p.deletedNodeSet) == 0 &&
		len(p.recreatedNodeSet) == 0 &&
		len(p.startNodeSet) == 0 &&
		len(p.restartNodeSet) == 0 &&
		len(p.linkRestartNodeSet) == 0 &&
		len(p.addedLinks) == 0 &&
		len(p.staleEndpoints) == 0 &&
		len(p.bridgeNSMigrations) == 0
}

func newApplyPlan(currentNodes map[string]*runtimeNodeGroup, state *LabState) *applyPlan {
	return &applyPlan{
		currentNodes:       currentNodes,
		addedNodeSet:       map[string]struct{}{},
		deletedNodeSet:     map[string]struct{}{},
		recreatedNodeSet:   map[string]struct{}{},
		parkedNodeSet:      map[string]struct{}{},
		startNodeSet:       map[string]struct{}{},
		restartNodeSet:     map[string]struct{}{},
		linkRestartNodeSet: map[string]struct{}{},
		nodeDiffs:          map[string]*clabtypes.TopologyDiff{},
		plannedLinkSet:     map[int]struct{}{},
		desiredEndpointSet: map[applyEndpointKey]struct{}{},
		liveEndpointSet:    map[applyEndpointKey]struct{}{},
		liveEndpointInfo:   map[applyEndpointKey]clablinks.OwnedInterface{},
		endpointNodes:      map[string]clablinks.Node{},
		state:              state,
		nodeChangeReasons:  map[string]string{},
	}
}

func (p *applyPlan) isExternallyManaged(nodeName string) bool {
	if p == nil {
		return false
	}
	node := p.currentNodes[nodeName]
	return node != nil && node.external
}

func (p *applyPlan) isRootNamespaceNode(nodeName string) bool {
	if p == nil {
		return false
	}
	node := p.currentNodes[nodeName]
	return node != nil && node.rootNamespaceBased
}

func (p *applyPlan) isNonContainerNode(nodeName string) bool {
	return p.isExternallyManaged(nodeName) || p.isRootNamespaceNode(nodeName)
}

// isNewNonContainerNode reports whether a non-container node (bridge, host,
// ext-container) is absent from the previously deployed topology. Such nodes
// exist outside the lab, so they are always listed as current nodes, even
// when the topology has just added them.
func (p *applyPlan) isNewNonContainerNode(nodeName string) bool {
	if !p.isNonContainerNode(nodeName) || p.state == nil || p.state.Topology == nil {
		return false
	}
	_, exists := p.state.Topology.Nodes[nodeName]
	return !exists
}

// networkModeContainerTarget returns the referenced node name for a
// "network-mode: container:<name>" config, or "" if networkMode does not
// share another container's network namespace.
func networkModeContainerTarget(networkMode string) string {
	if target, shared := strings.CutPrefix(networkMode, "container:"); shared {
		return target
	}
	return ""
}

// checkApplyNetworkModeTargets rejects apply when a node's network-mode:
// container:<target> target is being removed from the topology while the
// dependent node stays desired. Deleting the target would leave the
// dependent silently attached to a defunct network namespace.
func (c *CLab) checkApplyNetworkModeTargets(plan *applyPlan) error {
	if plan == nil {
		return nil
	}

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		networkMode := c.Nodes[nodeName].Config().NetworkMode
		target := networkModeContainerTarget(networkMode)
		if target == "" {
			continue
		}
		if _, deleted := plan.deletedNodeSet[target]; deleted {
			return fmt.Errorf(
				"node %q has network-mode %q, but %q is being removed from the topology; "+
					"remove or retarget %q as well",
				nodeName,
				networkMode,
				target,
				nodeName,
			)
		}
	}

	return nil
}

// planNetworkModeCascade forces recreation of nodes whose network-mode:
// container:<target> target is itself being recreated. Docker binds
// NetworkMode to the target container's ID at creation time, so a plain
// restart of the dependent cannot rebind it to the target's new container;
// only recreating the dependent does. A reverse dependency graph visits each
// node and dependency at most once, including long namespace-sharing chains.
func (c *CLab) planNetworkModeCascade(plan *applyPlan) {
	if plan == nil || len(plan.recreatedNodeSet) == 0 {
		return
	}

	dependents := make(map[string][]string)
	for name, node := range c.Nodes {
		if target := networkModeContainerTarget(node.Config().NetworkMode); target != "" {
			dependents[target] = append(dependents[target], name)
		}
	}
	queue := make([]string, 0, len(c.Nodes))
	for name := range plan.recreatedNodeSet {
		queue = append(queue, name)
	}
	for i := 0; i < len(queue); i++ {
		target := queue[i]
		for _, nodeName := range dependents[target] {
			if _, recreated := plan.recreatedNodeSet[nodeName]; recreated {
				continue
			}
			if _, added := plan.addedNodeSet[nodeName]; added {
				continue
			}

			plan.recreatedNodeSet[nodeName] = struct{}{}
			delete(plan.restartNodeSet, nodeName)
			delete(plan.linkRestartNodeSet, nodeName)
			plan.nodeChangeReasons[nodeName] = fmt.Sprintf(
				"network-mode target %q recreated", target,
			)
			queue = append(queue, nodeName)
		}
	}
}

func (c *CLab) planApply(
	ctx context.Context,
	currentNodes map[string]*runtimeNodeGroup,
) (*applyPlan, error) {
	state, err := c.LoadState()
	if err != nil {
		log.Warn("Failed to load state file", "error", err)
	}

	plan := newApplyPlan(currentNodes, state)

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		if _, exists := currentNodes[nodeName]; !exists {
			status := c.Nodes[nodeName].GetContainerStatus(ctx)
			if status != clabruntime.NotFound {
				return nil, fmt.Errorf(
					"node %q container %q already exists outside the runtime lab state",
					nodeName,
					c.Nodes[nodeName].Config().LongName,
				)
			}

			plan.addedNodeSet[nodeName] = struct{}{}
			continue
		}

		if plan.isNewNonContainerNode(nodeName) {
			plan.addedNodeSet[nodeName] = struct{}{}
		}
	}

	for _, nodeName := range sortedRuntimeNodeGroupNames(currentNodes) {
		if _, exists := c.Nodes[nodeName]; !exists {
			plan.deletedNodeSet[nodeName] = struct{}{}
		}
	}

	if err := c.checkApplyNetworkModeTargets(plan); err != nil {
		return nil, err
	}

	if err := c.planNodeReconciliation(ctx, plan); err != nil {
		return nil, err
	}

	// Bridge devices of removed BridgeNS nodes must be collected before the
	// removals fail any topology checks below, while the node-replacement
	// sets are final.
	c.planDesertedBridgeDeviceRemovals(plan)

	c.planParkedNodes(ctx, plan)
	c.planStoppedNodes(ctx, plan)

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		for _, ep := range clablinks.RuntimeEndpoints(c.Links[linkIdx]) {
			plan.desiredEndpointSet[endpointKeyFromEndpoint(ep)] = struct{}{}
		}
	}

	if err := c.discoverLiveApplyEndpoints(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.validateDesiredEndpointOwnership(ctx, plan); err != nil {
		return nil, err
	}

	c.planDeletedEndpoints(ctx, plan)

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		link := c.Links[linkIdx]
		if !plan.linkNeedsDeploy(link) {
			continue
		}

		plan.addDeployApplyLink(linkIdx, link)

		for _, ep := range clablinks.RuntimeEndpoints(link) {
			nodeName := ep.GetNode().GetShortName()
			if _, exists := currentNodes[nodeName]; !exists {
				continue
			}
			if _, added := plan.addedNodeSet[nodeName]; added {
				continue
			}

			c.planAffectedApplyNode(ctx, plan, nodeName, "added link")
		}
	}

	// Link reconciliation can request additional recreations. Propagate namespace
	// dependencies only after those decisions, then park every affected live node.
	c.planNetworkModeCascade(plan)
	c.planTailscaleSidecarRecreates(plan)
	c.planParkedNodes(ctx, plan)
	c.planRecreatedNodeLinks(plan)
	for nodeName := range plan.recreatedNodeSet {
		delete(plan.restartNodeSet, nodeName)
		delete(plan.linkRestartNodeSet, nodeName)
		delete(plan.startNodeSet, nodeName)
	}
	if err := c.planNetworkModeRestarts(plan); err != nil {
		return nil, err
	}

	return plan, nil
}

func (p *applyPlan) addDeployApplyLink(linkIdx int, link clablinks.Link) {
	if _, planned := p.plannedLinkSet[linkIdx]; planned {
		return
	}

	p.plannedLinkSet[linkIdx] = struct{}{}
	p.addedLinks = append(p.addedLinks, link)
}

func (c *CLab) planParkedNodes(ctx context.Context, plan *applyPlan) {
	if plan == nil {
		return
	}

	for nodeName := range plan.recreatedNodeSet {
		if _, parked := plan.parkedNodeSet[nodeName]; parked {
			continue
		}
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		if clabruntime.ContainerHasJoinableNetns(node.GetContainerStatus(ctx)) {
			plan.parkedNodeSet[nodeName] = struct{}{}
		}
	}
}

func (c *CLab) planStoppedNodes(ctx context.Context, plan *applyPlan) {
	if plan == nil {
		return
	}

	for nodeName := range plan.currentNodes {
		if plan.isNonContainerNode(nodeName) {
			continue
		}
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		if _, deleted := plan.deletedNodeSet[nodeName]; deleted {
			continue
		}
		if _, added := plan.addedNodeSet[nodeName]; added {
			continue
		}
		if _, recreated := plan.recreatedNodeSet[nodeName]; recreated {
			continue
		}

		switch node.GetContainerStatus(ctx) {
		case clabruntime.Stopped, clabruntime.Created:
			plan.startNodeSet[nodeName] = struct{}{}
		}
	}
}

func (c *CLab) planRecreatedNodeLinks(plan *applyPlan) {
	if plan == nil || len(plan.recreatedNodeSet) == 0 {
		return
	}

	rebuildSet := make(map[string]struct{}, len(plan.recreatedNodeSet))
	for nodeName := range plan.recreatedNodeSet {
		if _, parked := plan.parkedNodeSet[nodeName]; parked {
			continue
		}
		rebuildSet[nodeName] = struct{}{}
	}

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		link := c.Links[linkIdx]
		if !linkTouchesNodeSet(link, rebuildSet) {
			continue
		}

		plan.addDeployApplyLink(linkIdx, link)
	}
}

func linkTouchesNodeSet(link clablinks.Link, nodeSet map[string]struct{}) bool {
	if len(nodeSet) == 0 {
		return false
	}

	for _, ep := range clablinks.RuntimeEndpoints(link) {
		key := endpointKeyFromEndpoint(ep)
		if _, exists := nodeSet[key.node]; exists {
			return true
		}
	}

	return false
}

func (c *CLab) planDeletedEndpoints(ctx context.Context, plan *applyPlan) {
	plannedEndpointSet := map[applyEndpointKey]struct{}{}

	for _, key := range sortedEndpointKeys(plan.liveEndpointSet) {
		if _, exists := plan.desiredEndpointSet[key]; exists {
			continue
		}
		c.addStaleApplyEndpoint(ctx, plan, plannedEndpointSet, key)
	}

	c.planBridgeNSMigrations(ctx, plan)
}

// planBridgeNSMigrations finds stale BridgeNS bridge-side endpoints that the
// desired topology re-claims under a different in-container bridge node (the
// bridge moved to another parent). Such veth ends are migrated (parked and
// restored into the new parent's namespace) instead of deleted, which keeps
// the pairwise integrity of the link and the client-side configuration.
//
// Detection runs on three sources: live-discovered stale endpoints (existing
// bridge nodes), the stored old topology (deleted bridge nodes, whose ports
// are not discovered live), and parked ports of interrupted migrations that
// the next apply no longer plans as a move (recovery).
func (c *CLab) planBridgeNSMigrations(ctx context.Context, plan *applyPlan) {
	if plan == nil {
		return
	}

	c.recoverParkedBridgeNSPorts(plan)

	// Live-discovered stale bridge-side ends (the bridge node still exists).
	for _, ref := range plan.staleEndpoints {
		if ref.node == nil ||
			ref.node.GetLinkEndpointType() != clablinks.LinkEndpointTypeBridgeNS {
			continue
		}
		// Get the client peer from the stale link
		staleClientPeer := c.getClientPeerFromNode(ref.key, ref.node)
		c.addBridgeNSMigrationCandidate(plan, ref.key, ref.node, staleClientPeer)
	}

	// Deleted in-container bridge nodes are absent from the desired topology,
	// so their ports are not discovered live; recover them from the old state,
	// deriving the client-side peer from the stored link definition.
	if plan.state != nil && plan.state.Topology != nil {
		for _, link := range plan.state.Topology.Links {
			if link == nil || link.Link == nil {
				continue
			}

			endpoints := clablinks.RawLinkEndpoints(link.Link)
			if len(endpoints) != 2 {
				continue
			}

			var bridgeEndpointPoint *[2]string
			for i, point := range endpoints {
				if strings.Contains(point[0], "|") {
					bridgeEndpointPoint = &endpoints[i]
				}
			}
			if bridgeEndpointPoint == nil {
				continue
			}

			// The client peer is the non-bridge endpoint of the stored link.
			var clientPeer applyEndpointKey
			for _, ep := range endpoints {
				if ep[0] == bridgeEndpointPoint[0] && ep[1] == bridgeEndpointPoint[1] {
					continue
				}
				clientPeer = applyEndpointKey{node: ep[0], iface: ep[1]}
			}

			c.addBridgeNSMigrationCandidate(
				plan,
				applyEndpointKey{node: bridgeEndpointPoint[0], iface: bridgeEndpointPoint[1]},
				nil,
				clientPeer,
			)
		}
	}
}

// recoverParkedBridgePorts plans restores for desired in-container bridge
// ports found parked in their owner bridge's parking netns. Such ports belong
// to a migration that was interrupted before restore, for which this apply no
// longer plans the same move (the move is superseded or reverted): without
// recovery, the parked half would stay invisible to every later flow, leaving
// the client link dead until destroy. Recovery restores the parked half
// through the regular restore path of the planned migration.
func (c *CLab) recoverParkedBridgeNSPorts(plan *applyPlan) {
	if plan == nil {
		return
	}

	for nodeName, node := range c.Nodes {
		if !c.isBridgeNSNode(nodeName) {
			continue
		}

		longName := node.Config().LongName

		for key := range plan.desiredEndpointSet {
			if key.node != nodeName || key.iface == "" {
				continue
			}

			if _, live := plan.liveEndpointSet[key]; live {
				continue
			}

			if c.isBridgeNSMigrationPlanned(plan, key) {
				continue
			}

			relocation, parked, err := clablinks.FindParkedBridgePortByOwnership(
				longName,
				nodeName,
				key.iface,
			)
			// Lookup failures stay out of planning; the deployment path
			// recreates the pair when the port never shows up.
			if err != nil {
				log.Debug("Failed to inspect the parking netns for parked bridge port recovery",
					"bridge", nodeName, "port", key.iface, "error", err)
				continue
			}
			if !parked {
				continue
			}

			plan.bridgeNSMigrations = append(plan.bridgeNSMigrations, bridgeNSMigration{
				// The parked port's owner is this desired bridge itself; the
				// restore paths treat same-owner migrations as label-and-
				// master fixups only.
				staleRef: applyEndpointRef{
					key:  applyEndpointKey{node: nodeName, iface: key.iface},
					node: node,
				},
				newKey:     key,
				relocation: relocation,
			})
			log.Info("Planning recovery of parked bridge port",
				"bridge", nodeName, "port", key.iface)
		}
	}
}

// addBridgeNSMigrationCandidate registers a bridge NS migration when the
// desired topology re-claims the given bridge-side endpoint under a different
// in-container bridge node with the same logical port name. Candidates are
// validated at planning time: the old bridge's parent must resolve from the
// desired topology, the old link's client-side endpoint must match the new
// link's client endpoint, and the client side must survive this apply.
func (c *CLab) addBridgeNSMigrationCandidate(
	plan *applyPlan,
	staleKey applyEndpointKey,
	staleNode clablinks.Node,
	staleClientPeer applyEndpointKey,
) {
	if staleKey.node == "" || staleKey.iface == "" {
		return
	}
	if _, desired := plan.desiredEndpointSet[staleKey]; desired {
		return
	}
	if c.isBridgeNSMigrationPlanned(plan, staleKey) {
		return
	}

	if !c.canResolveBridgeMigParent(staleKey.node) {
		log.Debug("Skipping bridge port migration; parent of old bridge is not in the desired topology",
			"bridge", staleKey.node)
		return
	}

	if staleClientPeer.node == "" && staleNode != nil {
		staleClientPeer = c.getClientPeerFromNode(staleKey, staleNode)
	}
	if staleClientPeer.node == "" {
		log.Debug("Skipping bridge port migration; client-side peer of the old link is unknown",
			"bridge", staleKey.String())
		return
	}

	// Client-side validation is candidate-independent; run it once.
	if c.isClientPeerDropped(staleClientPeer, plan) {
		log.Debug("Skipping bridge port migration; client side is deleted, recreated or not desired",
			"bridge", staleKey.String(), "client", staleClientPeer.String())
		return
	}

	newClientPeers := c.desiredBridgeClientPeers()

	for candidate := range plan.desiredEndpointSet {
		if candidate.iface != staleKey.iface || candidate.node == staleKey.node {
			continue
		}
		if !c.isBridgeNSNode(candidate.node) {
			continue
		}

		if newClientPeers[candidate] != staleClientPeer {
			log.Debug("Skipping bridge port migration; rewired client endpoint would destroy the pair",
				"bridge", staleKey.node, "port", staleKey.iface,
				"oldClient", staleClientPeer.String(),
				"newClient", newClientPeers[candidate].String())
			continue
		}

		plan.bridgeNSMigrations = append(plan.bridgeNSMigrations, bridgeNSMigration{
			staleRef: applyEndpointRef{key: staleKey, node: staleNode},
			newKey:   candidate,
		})
		log.Info(
			"Planning bridge port migration",
			"from",
			staleKey.String(),
			"to",
			candidate.String(),
		)
		return
	}
}

// desiredBridgeClientPeers builds a bidirectional endpoint map from the
// desired veth links once, so migration candidate validation does not rescan
// the link set per candidate.
func (c *CLab) desiredBridgeClientPeers() map[applyEndpointKey]applyEndpointKey {
	peers := make(map[applyEndpointKey]applyEndpointKey)
	for _, link := range c.Links {
		veth, ok := link.(*clablinks.LinkVEth)
		if !ok || len(veth.Endpoints) != 2 {
			continue
		}
		var a, b applyEndpointKey
		if veth.Endpoints[0].GetNode() != nil {
			a = applyEndpointKey{
				veth.Endpoints[0].GetNode().GetShortName(),
				veth.Endpoints[0].GetIfaceName(),
			}
		}
		if veth.Endpoints[1].GetNode() != nil {
			b = applyEndpointKey{
				veth.Endpoints[1].GetNode().GetShortName(),
				veth.Endpoints[1].GetIfaceName(),
			}
		}
		if a.node == "" || b.node == "" {
			continue
		}
		peers[a] = b
		peers[b] = a
	}
	return peers
}

// getClientPeerFromNode extracts the client-side peer of the given bridge-side
// endpoint from the live bridge node's endpoint links.
func (c *CLab) getClientPeerFromNode(
	bridgeKey applyEndpointKey,
	node clablinks.Node,
) applyEndpointKey {
	for _, ep := range node.GetEndpoints() {
		if ep == nil || ep.GetLink() == nil {
			continue
		}
		veth, ok := ep.GetLink().(*clablinks.LinkVEth)
		// Only consider the link that actually holds bridgeKey; a bridge node
		// with multiple ports exposes one link per port.
		if !ok || len(veth.Endpoints) != 2 {
			continue
		}
		for i := range veth.Endpoints {
			e := veth.Endpoints[i]
			if e.GetNode() == nil ||
				e.GetNode().GetShortName() != bridgeKey.node ||
				e.GetIfaceName() != bridgeKey.iface {
				continue
			}
			peer := veth.Endpoints[1-i]
			if peer.GetNode() == nil {
				return applyEndpointKey{}
			}
			return applyEndpointKey{
				peer.GetNode().GetShortName(),
				peer.GetIfaceName(),
			}
		}
	}
	return applyEndpointKey{}
}

// isClientPeerDropped reports whether the migration's client side loses its
// peer during this apply: node deleted or recreated (container teardown
// destroys the peer), or the client endpoint no longer desired (the pair gets
// torn down by endpoint deletion). In all such cases migration cannot keep the
// pair intact and the delete/recreate path applies instead.
func (c *CLab) isClientPeerDropped(
	clientPeer applyEndpointKey,
	plan *applyPlan,
) bool {
	if _, deleted := plan.deletedNodeSet[clientPeer.node]; deleted {
		return true
	}
	if _, recreated := plan.recreatedNodeSet[clientPeer.node]; recreated {
		return true
	}
	if _, desired := plan.desiredEndpointSet[clientPeer]; !desired {
		return true
	}
	return false
}

// canResolveBridgeMigParent reports whether the parent container of the given
// bridge node name is part of the desired topology, which is required to park
// a port out of the old bridge's namespace. Unresolvable parents belong to the
// delete/recreate path instead of the migration path.
func (c *CLab) canResolveBridgeMigParent(bridgeNodeName string) bool {
	parentName := c.extractBridgeParentName(bridgeNodeName)
	if parentName == "" {
		return false
	}
	_, exists := c.Nodes[parentName]
	return exists
}

// extractBridgeParentName extracts the parent container name from an
// name-encoded in-container bridge node name ("bridgeName|parentName").
func (c *CLab) extractBridgeParentName(bridgeNodeName string) string {
	parts := strings.SplitN(bridgeNodeName, "|", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// isBridgeNSMigrationPlanned reports whether the given bridge endpoint is
// already covered by a planned migration.
func (c *CLab) isBridgeNSMigrationPlanned(
	plan *applyPlan,
	key applyEndpointKey,
) bool {
	for _, mig := range plan.bridgeNSMigrations {
		if mig.staleRef.key == key {
			return true
		}
	}
	return false
}

// isBridgeNSNode reports whether the node name refers to an in-container
// bridge node of the desired topology.
func (c *CLab) isBridgeNSNode(nodeName string) bool {
	node, exists := c.Nodes[nodeName]
	if !exists {
		return false
	}
	return node.GetLinkEndpointType() == clablinks.LinkEndpointTypeBridgeNS ||
		node.Config().Kind == "bridge" && strings.Contains(nodeName, "|")
}

func (c *CLab) addStaleApplyEndpoint(
	ctx context.Context,
	plan *applyPlan,
	plannedEndpointSet map[applyEndpointKey]struct{},
	key applyEndpointKey,
) {
	if key.node == "" || key.iface == "" {
		return
	}
	if _, exists := plannedEndpointSet[key]; exists {
		return
	}
	if _, desired := plan.desiredEndpointSet[key]; desired {
		return
	}

	node, exists := plan.endpointNode(key.node)
	if !exists {
		node, exists = c.applyLinkNode(key.node)
	}
	if !exists {
		return
	}

	plannedEndpointSet[key] = struct{}{}
	plan.staleEndpoints = append(plan.staleEndpoints, applyEndpointRef{key: key, node: node})

	if _, exists := c.Nodes[key.node]; exists {
		if _, added := plan.addedNodeSet[key.node]; !added {
			c.planAffectedApplyNode(ctx, plan, key.node, "deleted endpoint")
		}
	}

	c.addDerivedHostStaleEndpoints(plan, plannedEndpointSet, key)
}

func (c *CLab) addDerivedHostStaleEndpoints(
	plan *applyPlan,
	plannedEndpointSet map[applyEndpointKey]struct{},
	key applyEndpointKey,
) {
	if key.node == "host" {
		return
	}
	if _, exists := c.Nodes[key.node]; !exists {
		return
	}

	hostNode, exists := c.applyLinkNode("host")
	if !exists {
		return
	}

	for _, iface := range []string{
		fmt.Sprintf("ve-%s_%s", key.node, key.iface),
		fmt.Sprintf("vx-%s_%s", key.node, key.iface),
	} {
		hostKey := applyEndpointKey{node: "host", iface: iface}
		if _, planned := plannedEndpointSet[hostKey]; planned {
			continue
		}
		if _, desired := plan.desiredEndpointSet[hostKey]; desired {
			continue
		}

		plannedEndpointSet[hostKey] = struct{}{}
		plan.staleEndpoints = append(
			plan.staleEndpoints,
			applyEndpointRef{key: hostKey, node: hostNode, bestEffort: true},
		)
	}
}

func (c *CLab) discoverLiveApplyEndpoints(
	ctx context.Context,
	plan *applyPlan,
) error {
	desiredNodes := c.applyEndpointDiscoveryNodes(plan)
	for _, nodeName := range sortedLinkNodeNames(desiredNodes) {
		n := desiredNodes[nodeName]
		if _, exists := plan.currentNodes[nodeName]; !exists && !isApplySpecialNode(nodeName) {
			continue
		}

		if _, recreate := plan.recreatedNodeSet[nodeName]; recreate {
			if _, parked := plan.parkedNodeSet[nodeName]; !parked {
				continue
			}
		}
		if _, added := plan.addedNodeSet[nodeName]; added {
			if _, parked := plan.parkedNodeSet[nodeName]; !parked {
				continue
			}
		}
		if _, start := plan.startNodeSet[nodeName]; start {
			parkingNode, ok := c.applyParkingLinkNode(nodeName)
			if !ok {
				continue
			}
			n = parkingNode
		} else if !isApplySpecialNode(nodeName) && !plan.isRootNamespaceNode(nodeName) {
			status := c.Nodes[nodeName].GetContainerStatus(ctx)
			if !clabruntime.ContainerHasJoinableNetns(status) {
				if plan.isExternallyManaged(nodeName) {
					return fmt.Errorf(
						"externally managed node %q is %s; "+
							"start it outside containerlab before running apply",
						nodeName,
						status,
					)
				}
				return fmt.Errorf(
					"node %q is %s; apply requires existing nodes to have a joinable network namespace",
					nodeName,
					status,
				)
			}
		}

		plan.setEndpointNode(nodeName, n)

		// For in-container bridges (BridgeNS), discover interfaces attached to the
		// bridge in the parent container's namespace. Matching is done inside the
		// namespace callback via ownership altnames, so this is order-independent
		// w.r.t. the remote endpoints' discovery.
		if node, ok := c.Nodes[nodeName]; ok &&
			node.GetLinkEndpointType() == clablinks.LinkEndpointTypeBridgeNS {
			parentNode, err := c.bridgeNSParentNode(nodeName)
			if err != nil {
				return err
			}
			if err := c.discoverBridgeNSEndpoints(ctx, plan, nodeName, parentNode); err != nil {
				return fmt.Errorf(
					"failed to discover runtime interfaces for in-container bridge %q: %w",
					nodeName,
					err,
				)
			}
			continue
		}

		interfaces, err := clablinks.DiscoverOwnedInterfacesFor(
			ctx,
			n,
			nodeName,
			c.applyKnownEndpointNames(plan, nodeName),
		)
		if err != nil {
			return fmt.Errorf(
				"failed to discover runtime interfaces for node %q: %w",
				nodeName,
				err,
			)
		}

		for _, iface := range interfaces {
			key := applyEndpointKey{node: nodeName, iface: iface.Name}
			plan.liveEndpointSet[key] = struct{}{}
			plan.liveEndpointInfo[key] = iface
		}
	}

	return nil
}

// bridgeNSParentNode resolves the parent container node whose network namespace
// hosts the given BridgeNS bridge node. Shared by discovery, validation and
// deletion paths touching BridgeNS endpoint lifecycles.
func (c *CLab) bridgeNSParentNode(bridgeName string) (clablinks.Node, error) {
	node, exists := c.Nodes[bridgeName]
	if !exists {
		return nil, fmt.Errorf("in-container bridge %q not found in topology", bridgeName)
	}
	parentName := networkModeContainerTarget(node.Config().NetworkMode)
	if parentName == "" {
		return nil, fmt.Errorf(
			"in-container bridge %q has invalid network-mode: %s",
			bridgeName,
			node.Config().NetworkMode,
		)
	}
	parentNode, exists := c.Nodes[parentName]
	if !exists {
		return nil, fmt.Errorf(
			"in-container bridge %q: parent container %q not found",
			bridgeName,
			parentName,
		)
	}
	return parentNode, nil
}

// bridgeNSNameInNamespace strips the "<bridge>|<node>" suffix, yielding the
// kernel bridge interface name inside the parent container namespace.
func bridgeNSNameInNamespace(bridgeName string) string {
	if idx := strings.Index(bridgeName, "|"); idx != -1 {
		return bridgeName[:idx]
	}
	return bridgeName
}

// discoverBridgeNSEndpoints discovers interfaces attached to an in-container
// bridge. Interfaces are discovered in the parent container's namespace and
// mapped to their logical endpoint names via ownership altnames, so no ordering
// dependency with the remote endpoints' discovery exists. Orphaned
// bridge-attached interfaces (owned but claimed by no desired endpoint) are
// keyed by their kernel names so stale-clean up can remove them.
func (c *CLab) discoverBridgeNSEndpoints(
	ctx context.Context,
	plan *applyPlan,
	bridgeName string,
	nsNode clablinks.Node,
) error {
	desiredNames := make(map[string]struct{})
	for key := range plan.desiredEndpointSet {
		if key.node == bridgeName && key.iface != "" {
			desiredNames[key.iface] = struct{}{}
		}
	}

	matched, orphans, err := clablinks.DiscoverEndpointsForBridgeNS(
		ctx,
		nsNode,
		bridgeName,
		bridgeNSNameInNamespace(bridgeName),
		desiredNames,
	)
	if err != nil {
		return err
	}

	for name, iface := range matched {
		key := applyEndpointKey{node: bridgeName, iface: name}
		plan.liveEndpointSet[key] = struct{}{}
		plan.liveEndpointInfo[key] = iface
	}

	for _, iface := range orphans {
		key := applyEndpointKey{node: bridgeName, iface: iface.Name}
		plan.liveEndpointSet[key] = struct{}{}
		plan.liveEndpointInfo[key] = iface
	}

	return nil
}

func (c *CLab) validateDesiredEndpointOwnership(
	ctx context.Context,
	plan *applyPlan,
) error {
	for _, key := range sortedEndpointKeys(plan.desiredEndpointSet) {
		if _, added := plan.addedNodeSet[key.node]; added {
			continue
		}

		node, exists := plan.endpointNode(key.node)
		if !exists {
			node, exists = c.applyLinkNode(key.node)
		}
		if !exists {
			continue
		}

		// Name-based ownership validation is invalid for BridgeNS endpoints:
		// kernel iface names are sanitized versions of the logical names, and
		// a plain LinkByName(logical) would miss or hit the wrong interface.
		// BridgeNS matching is done at discovery time via ownership altnames.
		isBridgeNSEndpoint := false
		if bridgeNode, ok := c.Nodes[key.node]; ok {
			isBridgeNSEndpoint = bridgeNode.GetLinkEndpointType() == clablinks.LinkEndpointTypeBridgeNS
		}

		if isBridgeNSEndpoint {
			continue
		}

		if err := clablinks.ValidateOwnedInterfaceFor(
			ctx,
			node,
			key.node,
			key.iface,
		); err != nil {
			return fmt.Errorf(
				"desired endpoint %s cannot be reconciled: %w",
				key.String(),
				err,
			)
		}
	}

	return nil
}

func (c *CLab) applyParkingLinkNode(nodeName string) (clablinks.Node, bool) {
	node, exists := c.Nodes[nodeName]
	if !exists || node.Config() == nil || node.Config().LongName == "" {
		return nil, false
	}

	parkPath, err := clabutils.GetNamedNetNS(
		clabutils.ParkingNetnsName(node.Config().LongName),
	)
	if err != nil {
		return nil, false
	}

	return clablinks.NewParkingNode(node.Config().LongName, parkPath), true
}

func (c *CLab) applyKnownEndpointNames(
	plan *applyPlan,
	nodeName string,
) map[string]struct{} {
	if !isApplySpecialNode(nodeName) {
		return nil
	}

	names := map[string]struct{}{}
	for key := range plan.desiredEndpointSet {
		if key.node == nodeName {
			names[key.iface] = struct{}{}
		}
	}

	return names
}

func (c *CLab) applyEndpointDiscoveryNodes(plan *applyPlan) map[string]clablinks.Node {
	nodes := make(map[string]clablinks.Node, len(c.Nodes)+2)
	for nodeName, node := range c.Nodes {
		nodes[nodeName] = node
	}

	for _, key := range sortedEndpointKeys(plan.desiredEndpointSet) {
		if _, exists := nodes[key.node]; exists {
			continue
		}
		if node, exists := c.applyLinkNode(key.node); exists {
			nodes[key.node] = node
		}
	}

	return nodes
}

func (c *CLab) applyLinkNode(nodeName string) (clablinks.Node, bool) {
	if node, exists := c.Nodes[nodeName]; exists {
		return node, true
	}
	node, exists := c.getSpecialLinkNodes()[nodeName]
	return node, exists
}

func isApplySpecialNode(nodeName string) bool {
	return nodeName == "host" || nodeName == "mgmt-net"
}

func (c *CLab) planAffectedApplyNode(
	ctx context.Context,
	plan *applyPlan,
	nodeName string,
	change string,
) {
	if nodeName == "" {
		return
	}
	if _, planned := plan.linkRestartNodeSet[nodeName]; planned {
		return
	}
	if _, planned := plan.recreatedNodeSet[nodeName]; planned {
		return
	}

	node, exists := c.Nodes[nodeName]
	if !exists {
		return
	}
	if plan.isExternallyManaged(nodeName) {
		log.Info(
			"Applying link change without node lifecycle action",
			"node", nodeName,
			"change", change,
			"externally-managed", true,
		)
		return
	}

	overridden := clabnodes.LinkApplyModeOverrideForNode(node) != ""

	switch clabnodes.LinkApplyModeForNode(ctx, node) {
	case clabnodes.LinkApplyModeLive:
		log.Info(
			"Applying link change without node lifecycle action",
			"node",
			nodeName,
			"change",
			change,
		)
	case clabnodes.LinkApplyModeRestart:
		plan.linkRestartNodeSet[nodeName] = struct{}{}
		plan.nodeChangeReasons[nodeName] = change
		log.Info("Node will be restarted to apply link change", "node", nodeName, "change", change)
	case clabnodes.LinkApplyModeRecreate:
		plan.recreatedNodeSet[nodeName] = struct{}{}
		delete(plan.restartNodeSet, nodeName)
		delete(plan.linkRestartNodeSet, nodeName)
		plan.nodeChangeReasons[nodeName] = change

		if overridden {
			log.Info("Node will be recreated to apply link change",
				"node", nodeName, "change", change, "link-apply-mode", "recreate")
		} else {
			log.Info(
				"Node will be recreated to apply link change since its kind does not declare "+
					"live link support; if the NOS detects hot-plugged interfaces, set "+
					"link-apply-mode: live on the node to avoid the recreation",
				"node", nodeName,
				"change", change,
			)
		}
	}
}

func (c *CLab) resolveNodeConfigFromTopology(
	topo *clabtypes.Topology,
	nodeName string,
) *clabtypes.NodeConfig {
	if topo == nil {
		return nil
	}
	if topo.GetNodeKind(nodeName) == clabnodestailscale.KindName &&
		topo.GetNodeLabels(nodeName)[clabconstants.InternalNode] == "true" {
		topo = &clabtypes.Topology{Nodes: map[string]*clabtypes.NodeDefinition{
			nodeName: topo.Nodes[nodeName],
		}}
	}

	binds, _ := topo.GetNodeBinds(nodeName)
	portSet, _, _ := topo.GetNodePorts(nodeName)
	kind := topo.GetNodeKind(nodeName)
	privileged := topo.GetNodePrivileged(
		nodeName,
		c.privilegedByDefault(strings.ToLower(kind)),
	)

	nodeCfg := &clabtypes.NodeConfig{
		ShortName:    nodeName,
		Hostname:     topo.GetNodeHostname(nodeName),
		Kind:         kind,
		NodeType:     topo.GetNodeType(nodeName),
		Image:        topo.GetNodeImage(nodeName),
		Entrypoint:   topo.GetNodeEntrypoint(nodeName),
		Cmd:          topo.GetNodeCmd(nodeName),
		Exec:         topo.GetNodeExec(nodeName),
		Env:          topo.GetNodeEnv(nodeName),
		Binds:        binds,
		Devices:      topo.GetNodeDevices(nodeName),
		CapAdd:       topo.GetNodeCapAdd(nodeName),
		Privileged:   privileged,
		CgroupnsMode: topo.GetNodeCgroupnsMode(nodeName),
		CgroupParent: topo.GetNodeCgroupParent(nodeName),
		PidMode:      topo.GetNodePidMode(nodeName),
		Tmpfs:        topo.GetNodeTmpfs(nodeName),
		SecurityOpts: topo.GetNodeSecurityOpts(nodeName),
		ShmSize:      topo.GetNodeShmSize(nodeName),
		PortSet:      portSet,
		User:         topo.GetNodeUser(nodeName),
		NetworkMode:  topo.GetNodeNetworkMode(nodeName),
		MgmtNet:      topo.GetNodeMgmtNet(nodeName),
		Runtime:      topo.GetNodeRuntime(nodeName),
		CPU:          topo.GetNodeCPU(nodeName),
		CPUSet:       topo.GetNodeCPUSet(nodeName),
		Memory:       topo.GetNodeMemory(nodeName),
		License:      topo.GetNodeLicense(nodeName),
	}

	kindSpecificConfig, err := c.decodeKindSpecificConfig(topo, nodeName, strings.ToLower(kind))
	if err != nil {
		// A state file can hold keys a later release rejects; such a node counts as changed.
		kindSpecificConfig = clabnodes.InvalidKindSpecificConfig{Err: err.Error()}
	}

	nodeCfg.KindSpecificConfig = kindSpecificConfig

	return nodeCfg
}

func (c *CLab) planNodeReconciliation(ctx context.Context, plan *applyPlan) error {
	if plan == nil || plan.currentNodes == nil {
		return nil
	}

	var oldTopo *clabtypes.Topology
	if plan.state != nil {
		oldTopo = plan.state.Topology
	}

	for nodeName := range plan.currentNodes {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		if _, added := plan.addedNodeSet[nodeName]; added {
			continue
		}

		oldNodeConfig := c.resolveNodeConfigFromTopology(oldTopo, nodeName)
		newNodeConfig := c.resolveNodeConfigFromTopology(c.Config.Topology, nodeName)
		diff := node.ComputeDiff(oldNodeConfig, newNodeConfig)
		plan.nodeDiffs[nodeName] = diff

		result, err := node.GetReconcilePlan(ctx, diff)
		if err != nil {
			return fmt.Errorf("reconcile planning failed for node %q: %w", nodeName, err)
		}
		if plan.isNonContainerNode(nodeName) && result.Action != clabtypes.TopologyDiffActionNone {
			return fmt.Errorf(
				"node %q is externally managed and cannot be %s for %s",
				nodeName,
				result.Action,
				configDriftReason(diff),
			)
		}

		switch result.Action {
		case clabtypes.TopologyDiffActionRestart:
			plan.restartNodeSet[nodeName] = struct{}{}
			plan.nodeChangeReasons[nodeName] = configDriftReason(diff)
		case clabtypes.TopologyDiffActionRecreate:
			plan.recreatedNodeSet[nodeName] = struct{}{}
			plan.nodeChangeReasons[nodeName] = configDriftReason(diff)
		}
	}

	return nil
}

func configDriftReason(diff *clabtypes.TopologyDiff) string {
	if diff == nil || len(diff.Fields) == 0 {
		return "config drift"
	}

	return "config drift: " + strings.Join(diff.Fields, ", ")
}

func (c *CLab) reconcileNodes(ctx context.Context, plan *applyPlan) error {
	if plan == nil || plan.currentNodes == nil {
		return nil
	}

	for nodeName := range plan.currentNodes {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		if _, added := plan.addedNodeSet[nodeName]; added {
			continue
		}
		if plan.isRootNamespaceNode(nodeName) {
			continue
		}
		// Recreated nodes are handled exclusively by the apply pipeline
		// (park -> delete -> deploy -> restore). Calling Reconcile here would
		// delete the container before parkRecreatedNodes can capture its
		// interfaces, breaking the recreate flow.
		if _, recreated := plan.recreatedNodeSet[nodeName]; recreated {
			continue
		}

		diff := plan.nodeDiffs[nodeName]

		result, err := node.Reconcile(ctx, diff)
		if err != nil {
			return fmt.Errorf("reconcile failed for node %q: %w", nodeName, err)
		}

		if result.Action == clabtypes.TopologyDiffActionRestart {
			if err := c.waitNodeRunning(ctx, node); err != nil {
				return err
			}
			if err := c.waitNodeHealthyIfAvailable(ctx, node); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *applyPlan) linkNeedsDeploy(link clablinks.Link) bool {
	for _, ep := range clablinks.RuntimeEndpoints(link) {
		key := endpointKeyFromEndpoint(ep)
		if _, parked := p.parkedNodeSet[key.node]; !parked {
			if _, added := p.addedNodeSet[key.node]; added {
				return true
			}
		}
		if _, live := p.liveEndpointSet[key]; !live {
			return true
		}
	}

	return !p.linkIntact(link)
}

func (p *applyPlan) linkIntact(link clablinks.Link) bool {
	endpoints := clablinks.RuntimeEndpoints(link)
	if len(endpoints) != 2 || len(p.liveEndpointInfo) == 0 {
		return true
	}

	left, leftOK := p.liveEndpointInfo[endpointKeyFromEndpoint(endpoints[0])]
	right, rightOK := p.liveEndpointInfo[endpointKeyFromEndpoint(endpoints[1])]
	if !leftOK || !rightOK {
		return false
	}

	if bridgeMasterMismatch(endpoints[0], left) || bridgeMasterMismatch(endpoints[1], right) {
		return false
	}

	return left.PeerIndex != 0 &&
		right.PeerIndex != 0 &&
		left.PeerIndex == right.Index &&
		right.PeerIndex == left.Index
}

// bridgeMasterMismatch reports whether the live interface of a bridge endpoint is
// attached to a bridge other than the one the endpoint resolves to, e.g. after a
// mgmt-net link moved to another management network. mgmt-net endpoints whose
// management network bridge is not resolved are not checked.
func bridgeMasterMismatch(ep clablinks.Endpoint, live clablinks.OwnedInterface) bool {
	node := ep.GetNode()
	endpointType := node.GetLinkEndpointType()

	// Handle regular bridge endpoints
	if endpointType == clablinks.LinkEndpointTypeBridge {
		if clablinks.IsUnresolvedMgmtBridgeEndpoint(ep) {
			return false
		}
		return live.MasterName != node.GetShortName()
	}

	// Handle in-container bridge (BridgeNS) endpoints
	if endpointType == clablinks.LinkEndpointTypeBridgeNS {
		// For BridgeNS, the bridge name inside the parent's namespace is the
		// node name without the "<bridge>|<parent>" suffix.
		return live.MasterName != bridgeNSNameInNamespace(node.GetShortName())
	}

	return false
}

func (p *applyPlan) deployNodeNames() []string {
	if p == nil {
		return nil
	}

	nodeNames := make([]string, 0, len(p.addedNodeSet)+len(p.recreatedNodeSet))

	for nodeName := range p.addedNodeSet {
		nodeNames = append(nodeNames, nodeName)
	}
	for nodeName := range p.recreatedNodeSet {
		if _, added := p.addedNodeSet[nodeName]; !added {
			nodeNames = append(nodeNames, nodeName)
		}
	}

	sort.Strings(nodeNames)
	return nodeNames
}

func (c *CLab) deleteApplyEndpoints(
	ctx context.Context,
	plan *applyPlan,
) error {
	refs := plan.staleEndpoints
	for _, ref := range refs {
		// Migrated endpoints are parked and restored later in apply instead
		// of being deleted here; report the intent before any delete log.
		if ref.node.GetLinkEndpointType() == clablinks.LinkEndpointTypeBridgeNS {
			if c.isBridgeNSMigrationPlanned(plan, ref.key) {
				log.Info("Preserving bridge port for migration", "endpoint", ref.key.String())
				continue
			}
		}

		log.Info("Deleting link endpoint", "endpoint", ref.key.String())

		// For BridgeNS bridges, use RemoveInterface instead of RemoveOwnedInterface.
		// The bridge-attached interface's ownership marker refers to the bridge
		// endpoint owner, not to the parent container this call executes in, so
		// per-node ownership checks would always fail here.
		if ref.node.GetLinkEndpointType() == clablinks.LinkEndpointTypeBridgeNS {
			parentNode, err := c.bridgeNSParentNode(ref.key.node)
			if err != nil {
				return err
			}
			if err := clablinks.RemoveInterface(ctx, parentNode, ref.key.iface); err != nil {
				if ref.bestEffort && errors.Is(err, clablinks.ErrInterfaceUnowned) {
					log.Debugf("skipping best-effort endpoint delete: %v", err)
					continue
				}
				return err
			}
			continue
		}

		if err := clablinks.RemoveOwnedInterface(ctx, ref.node, ref.key.iface); err != nil {
			if ref.bestEffort && errors.Is(err, clablinks.ErrInterfaceUnowned) {
				log.Debugf("skipping best-effort endpoint delete: %v", err)
				continue
			}
			return err
		}
	}

	return nil
}

func (p *applyPlan) setEndpointNode(nodeName string, node clablinks.Node) {
	if nodeName == "" || node == nil {
		return
	}
	p.endpointNodes[nodeName] = node
}

func (p *applyPlan) endpointNode(nodeName string) (clablinks.Node, bool) {
	node, ok := p.endpointNodes[nodeName]
	return node, ok
}

func endpointKeyFromEndpoint(ep clablinks.Endpoint) applyEndpointKey {
	if ep == nil || ep.GetNode() == nil {
		return applyEndpointKey{}
	}
	nodeName := ep.GetNode().GetShortName()
	if ep.IsNodeless() && ep.GetNode().GetLinkEndpointType() == clablinks.LinkEndpointTypeBridge {
		nodeName = "mgmt-net"
	}
	return applyEndpointKey{
		node:  nodeName,
		iface: ep.GetIfaceName(),
	}
}

func applyLinkName(link clablinks.Link) string {
	endpoints := clablinks.RuntimeEndpoints(link)
	names := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		key := endpointKeyFromEndpoint(ep)
		if key.node == "" || key.iface == "" {
			continue
		}
		names = append(names, key.String())
	}
	sort.Strings(names)

	return strings.Join(names, " -- ")
}

func applyLinkNames(links []clablinks.Link) []string {
	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, applyLinkName(link))
	}
	return names
}

func applyDeletedEndpointNames(refs []applyEndpointRef) []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.bestEffort {
			continue
		}
		names = append(names, ref.key.String())
	}
	return names
}

func sortedNodeNames(nodes map[string]clabnodes.Node) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedLinkNodeNames(nodes map[string]clablinks.Node) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedLinkIndexes(links map[int]clablinks.Link) []int {
	indexes := make([]int, 0, len(links))
	for idx := range links {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	return indexes
}

func sortedEndpointKeys(endpointSet map[applyEndpointKey]struct{}) []applyEndpointKey {
	keys := make([]applyEndpointKey, 0, len(endpointSet))
	for key := range endpointSet {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].node == keys[j].node {
			return keys[i].iface < keys[j].iface
		}
		return keys[i].node < keys[j].node
	})
	return keys
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func unionStringSets(sets ...map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}
	for _, set := range sets {
		for value := range set {
			result[value] = struct{}{}
		}
	}
	return result
}
