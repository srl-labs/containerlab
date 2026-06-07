package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
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
	endpointNodes      map[string]clablinks.Node
	state              *LabState
	parkedNodeSet      map[string]struct{}
	startNodeSet       map[string]struct{}
}

func (p *applyPlan) empty() bool {
	return len(p.addedNodeSet) == 0 &&
		len(p.deletedNodeSet) == 0 &&
		len(p.recreatedNodeSet) == 0 &&
		len(p.startNodeSet) == 0 &&
		len(p.restartNodeSet) == 0 &&
		len(p.linkRestartNodeSet) == 0 &&
		len(p.addedLinks) == 0 &&
		len(p.staleEndpoints) == 0
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
		endpointNodes:      map[string]clablinks.Node{},
		state:              state,
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
		}
	}

	for _, nodeName := range sortedRuntimeNodeGroupNames(currentNodes) {
		if _, exists := c.Nodes[nodeName]; !exists {
			plan.deletedNodeSet[nodeName] = struct{}{}
		}
	}

	if err := c.planNodeReconciliation(ctx, plan); err != nil {
		return nil, err
	}

	c.planParkedNodes(ctx, plan)
	c.planStoppedNodes(ctx, plan)

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		for _, ep := range clablinks.ApplyRuntimeEndpoints(c.Links[linkIdx]) {
			plan.desiredEndpointSet[endpointKeyFromEndpoint(ep)] = struct{}{}
		}
	}

	if err := c.discoverLiveApplyEndpoints(ctx, plan); err != nil {
		return nil, err
	}

	c.planDeletedEndpoints(ctx, plan)

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		link := c.Links[linkIdx]
		if !plan.linkNeedsDeploy(link) {
			continue
		}

		plan.addDeployApplyLink(linkIdx, link)

		for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
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

	c.planRecreatedNodeLinks(plan)
	for nodeName := range plan.recreatedNodeSet {
		delete(plan.restartNodeSet, nodeName)
		delete(plan.linkRestartNodeSet, nodeName)
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

	for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
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
		} else if !isApplySpecialNode(nodeName) {
			status := c.Nodes[nodeName].GetContainerStatus(ctx)
			if !clabruntime.ContainerHasJoinableNetns(status) {
				return fmt.Errorf(
					"node %q is %s; apply requires existing nodes to have a joinable network namespace",
					nodeName,
					status,
				)
			}
		}

		plan.setEndpointNode(nodeName, n)

		ifaceNames, err := clablinks.DiscoverOwnedInterfaceNames(
			ctx,
			n,
			c.applyKnownEndpointNames(plan, nodeName),
		)
		if err != nil {
			return fmt.Errorf("failed to discover runtime interfaces for node %q: %w", nodeName, err)
		}

		for _, ifaceName := range ifaceNames {
			plan.liveEndpointSet[applyEndpointKey{node: nodeName, iface: ifaceName}] = struct{}{}
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

	switch clabnodes.LinkApplyModeForNode(ctx, node) {
	case clabnodes.LinkApplyModeLive:
		log.Info("Applying link change without node lifecycle action", "node", nodeName, "change", change)
	case clabnodes.LinkApplyModeRestart:
		plan.linkRestartNodeSet[nodeName] = struct{}{}
	case clabnodes.LinkApplyModeRecreate:
		plan.recreatedNodeSet[nodeName] = struct{}{}
		delete(plan.restartNodeSet, nodeName)
		delete(plan.linkRestartNodeSet, nodeName)
	}
}

func resolveNodeConfigFromTopology(topo *clabtypes.Topology, nodeName string) *clabtypes.NodeConfig {
	if topo == nil {
		return nil
	}

	binds, _ := topo.GetNodeBinds(nodeName)
	portSet, _, _ := topo.GetNodePorts(nodeName)

	return &clabtypes.NodeConfig{
		ShortName:   nodeName,
		NodeType:    topo.GetNodeType(nodeName),
		Image:       topo.GetNodeImage(nodeName),
		Entrypoint:  topo.GetNodeEntrypoint(nodeName),
		Cmd:         topo.GetNodeCmd(nodeName),
		Exec:        topo.GetNodeExec(nodeName),
		Env:         topo.GetNodeEnv(nodeName),
		Binds:       binds,
		Devices:     topo.GetNodeDevices(nodeName),
		CapAdd:      topo.GetNodeCapAdd(nodeName),
		ShmSize:     topo.GetNodeShmSize(nodeName),
		PortSet:     portSet,
		User:        topo.GetNodeUser(nodeName),
		NetworkMode: topo.GetNodeNetworkMode(nodeName),
		Runtime:     topo.GetNodeRuntime(nodeName),
		CPU:         topo.GetNodeCPU(nodeName),
		CPUSet:      topo.GetNodeCPUSet(nodeName),
		Memory:      topo.GetNodeMemory(nodeName),
		License:     topo.GetNodeLicense(nodeName),
		Components:  topo.GetComponents(nodeName),
	}
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

		oldNodeConfig := resolveNodeConfigFromTopology(oldTopo, nodeName)
		newNodeConfig := resolveNodeConfigFromTopology(c.Config.Topology, nodeName)
		diff := node.ComputeDiff(oldNodeConfig, newNodeConfig)
		plan.nodeDiffs[nodeName] = diff

		result, err := node.GetReconcilePlan(ctx, diff)
		if err != nil {
			return fmt.Errorf("reconcile planning failed for node %q: %w", nodeName, err)
		}

		switch result.Action {
		case clabtypes.TopologyDiffActionRestart:
			plan.restartNodeSet[nodeName] = struct{}{}
		case clabtypes.TopologyDiffActionRecreate:
			plan.recreatedNodeSet[nodeName] = struct{}{}
		}
	}

	return nil
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
	for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
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
	refs []applyEndpointRef,
) error {
	for _, ref := range refs {
		log.Info("Deleting link endpoint", "endpoint", ref.key.String())
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
	endpoints := clablinks.ApplyRuntimeEndpoints(link)
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
