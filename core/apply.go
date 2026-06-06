package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// ApplyResult summarizes the changes applied by an apply operation.
type ApplyResult struct {
	DryRun           bool
	DeployedLab      bool
	LabName          string
	AddedNodes       []string
	DeletedNodes     []string
	RecreatedNodes   []string
	StartedNodes     []string
	AddedLinks       []string
	DeletedEndpoints []string
	RestartedNodes   []string
}

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

type applyRuntimeNode struct {
	name        string
	containers  []clabruntime.GenericContainer
	distributed bool
}

type applyPlan struct {
	result             *ApplyResult
	currentNodes       map[string]*applyRuntimeNode
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
	boxenImageCache    map[string]bool
}

var errApplyInterfaceUnowned = errors.New("interface is not marked as containerlab-owned")

const (
	boxenImageVendorLabel = "org.opencontainers.image.vendor"
	boxenImageVendorValue = "Boxen"
)

type linkApplyModeNode interface {
	LinkApplyMode() clabnodes.LinkApplyMode
}

func (p *applyPlan) empty() bool {
	return !p.result.DeployedLab &&
		len(p.result.AddedNodes) == 0 &&
		len(p.result.DeletedNodes) == 0 &&
		len(p.result.RecreatedNodes) == 0 &&
		len(p.result.StartedNodes) == 0 &&
		len(p.restartNodeSet) == 0 &&
		len(p.linkRestartNodeSet) == 0 &&
		len(p.result.AddedLinks) == 0 &&
		len(p.result.DeletedEndpoints) == 0
}

// Apply deploys a lab if it is absent, otherwise it aligns the deployed lab with the
// desired topology using runtime state as baseline.
func (c *CLab) Apply(
	ctx context.Context,
	options *ApplyOptions,
) (*ApplyResult, error) {
	if options == nil {
		var err error
		options, err = NewApplyOptions(0)
		if err != nil {
			return nil, err
		}
	}

	if c.Config.Name == "" {
		return nil, fmt.Errorf("missing lab name")
	}
	if !c.TopoPaths.TopologyFileIsSet() {
		return nil, fmt.Errorf(
			"apply requires a topology file; provide --topo when it cannot be derived from --name",
		)
	}

	currentNodes, err := c.runtimeNodeContainers(ctx)
	if err != nil {
		return nil, err
	}
	if len(currentNodes) == 0 {
		result := &ApplyResult{
			DryRun:      options.dryRun,
			DeployedLab: true,
			LabName:     c.Config.Name,
		}

		if options.dryRun {
			return result, nil
		}

		deployOptions, err := NewDeployOptions(options.maxWorkers)
		if err != nil {
			return nil, err
		}
		deployOptions.SetSkipPostDeploy(options.skipPostDeploy).
			SetExportTemplate(options.exportTemplate)

		if _, err := c.Deploy(ctx, deployOptions); err != nil {
			return nil, err
		}

		return result, nil
	}

	if err := c.setApplyMgmtBridgeFromRuntime(currentNodes); err != nil {
		return nil, err
	}

	if err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge); err != nil {
		return nil, err
	}

	if err := c.ResolveLinks(); err != nil {
		return nil, err
	}

	if err := c.checkApplyTopologyDefinition(ctx); err != nil {
		return nil, err
	}

	plan, err := c.planApply(ctx, currentNodes)
	if err != nil {
		return nil, err
	}
	plan.result.DryRun = options.dryRun

	if options.dryRun {
		return plan.result, nil
	}

	if err := c.reconcileNodes(ctx, plan); err != nil {
		return nil, err
	}

	if plan.empty() {
		return plan.result, nil
	}

	deployNodeNames := plan.deployNodeNames()
	if err := c.prepareApply(ctx, deployNodeNames); err != nil {
		return nil, err
	}

	if err := c.deleteApplyEndpoints(ctx, plan.staleEndpoints); err != nil {
		return nil, err
	}

	if err := c.parkRecreatedNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.deleteApplyNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.deployApplyNodes(ctx, deployNodeNames, options.maxWorkers); err != nil {
		return nil, err
	}

	if err := c.restoreRecreatedNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.startStoppedNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.deployApplyLinks(ctx, plan.addedLinks); err != nil {
		return nil, err
	}

	if err := c.postDeployApplyLinks(ctx, deployNodeNames); err != nil {
		return nil, err
	}

	if err := c.postDeployApplyNodes(ctx, deployNodeNames, options.skipPostDeploy); err != nil {
		return nil, err
	}

	if _, err := c.restartApplyNodes(ctx, plan.linkRestartNodeSet); err != nil {
		return nil, err
	}

	if err := c.updateApplyRuntimeInfo(ctx); err != nil {
		return nil, err
	}

	if err := c.regenerateApplyArtifacts(ctx, options.exportTemplate); err != nil {
		return nil, err
	}

	if err := c.WriteState(); err != nil {
		log.Warnf("failed to write state file: %v", err)
	}

	return plan.result, nil
}

func (c *CLab) setApplyMgmtBridgeFromRuntime(
	currentNodes map[string]*applyRuntimeNode,
) error {
	if c.Config.Mgmt.Bridge != "" {
		return nil
	}

	var bridge string
	for _, runtimeNode := range currentNodes {
		for _, ctr := range runtimeNode.containers {
			ctrBridge := ctr.Labels[clabconstants.NodeMgmtNetBr]
			if ctrBridge == "" {
				continue
			}
			if bridge == "" {
				bridge = ctrBridge
				continue
			}
			if bridge != ctrBridge {
				return fmt.Errorf(
					"runtime lab has conflicting management bridge labels: %q and %q",
					bridge,
					ctrBridge,
				)
			}
		}
	}

	if bridge != "" {
		c.Config.Mgmt.Bridge = bridge
	}

	return nil
}

func (c *CLab) runtimeNodeContainers(
	ctx context.Context,
) (map[string]*applyRuntimeNode, error) {
	containers, err := c.ListContainers(ctx, WithListLabName(c.Config.Name))
	if err != nil {
		return nil, err
	}

	result := make(map[string]*applyRuntimeNode)
	var duplicates []string

	for _, ctr := range containers {
		if ctr.Labels[clabconstants.ToolType] != "" {
			continue
		}

		nodeName := ctr.Labels[clabconstants.NodeName]
		if nodeName == "" {
			continue
		}

		groupName := ctr.Labels[clabconstants.RootNodeName]
		distributed := groupName != ""
		if groupName == "" {
			groupName = nodeName
		}

		group, exists := result[groupName]
		if !exists {
			group = &applyRuntimeNode{
				name:        groupName,
				distributed: distributed,
			}
			result[groupName] = group
		}
		group.containers = append(group.containers, ctr)
		group.distributed = group.distributed || distributed

		if !group.distributed && len(group.containers) > 1 {
			duplicates = append(duplicates, nodeName)
			continue
		}
	}

	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return nil, fmt.Errorf(
			"apply supports only single-container nodes, found multiple containers for nodes: %s",
			strings.Join(duplicates, ", "),
		)
	}

	for _, group := range result {
		sort.Slice(group.containers, func(i, j int) bool {
			return applyContainerLess(group.containers[i], group.containers[j])
		})
	}

	return result, nil
}

func (c *CLab) checkApplyTopologyDefinition(ctx context.Context) error {
	if err := c.verifyLinks(ctx); err != nil {
		return err
	}

	if err := c.verifyRootNetNSLinks(); err != nil {
		return err
	}

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		if err := c.Nodes[nodeName].CheckDeploymentConditions(ctx); err != nil {
			return err
		}
	}

	return c.verifyDuplicateAddresses()
}

func (c *CLab) planApply(
	ctx context.Context,
	currentNodes map[string]*applyRuntimeNode,
) (*applyPlan, error) {

	state, err := c.LoadState()
	if err != nil {
		log.Warn("Failed to load state file", "error", err)
	}

	plan := &applyPlan{
		result: &ApplyResult{
			AddedNodes:       []string{},
			DeletedNodes:     []string{},
			RecreatedNodes:   []string{},
			StartedNodes:     []string{},
			AddedLinks:       []string{},
			DeletedEndpoints: []string{},
			RestartedNodes:   []string{},
		},
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
		boxenImageCache:    map[string]bool{},
	}

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

			plan.result.AddedNodes = append(plan.result.AddedNodes, nodeName)
			plan.addedNodeSet[nodeName] = struct{}{}
		}
	}

	for _, nodeName := range sortedRuntimeNodeNames(currentNodes) {
		if _, exists := c.Nodes[nodeName]; !exists {
			plan.result.DeletedNodes = append(plan.result.DeletedNodes, nodeName)
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
	plan.result.RecreatedNodes = sortedStringSet(plan.recreatedNodeSet)
	plan.result.StartedNodes = sortedStringSet(plan.startNodeSet)
	plan.result.RestartedNodes = sortedStringSet(unionStringSets(
		plan.restartNodeSet,
		plan.linkRestartNodeSet,
	))

	return plan, nil
}

func (p *applyPlan) addDeployApplyLink(linkIdx int, link clablinks.Link) {
	if p == nil || link == nil {
		return
	}
	if p.plannedLinkSet == nil {
		p.plannedLinkSet = map[int]struct{}{}
	}
	if _, planned := p.plannedLinkSet[linkIdx]; planned {
		return
	}

	p.plannedLinkSet[linkIdx] = struct{}{}
	p.addedLinks = append(p.addedLinks, link)
	if p.result != nil {
		p.result.AddedLinks = append(p.result.AddedLinks, applyLinkName(link))
	}
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
	plan.result.DeletedEndpoints = append(plan.result.DeletedEndpoints, key.String())

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

		ifaceNames, err := discoverOwnedInterfaceNames(ctx, n, c.applyKnownEndpointNames(plan, nodeName))
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

func discoverOwnedInterfaceNames(
	ctx context.Context,
	node clablinks.Node,
	knownIfaceNames map[string]struct{},
) ([]string, error) {
	var ifaceNames []string

	err := node.ExecFunction(ctx, func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			name := link.Attrs().Name
			if name == "lo" || name == "eth0" {
				continue
			}
			if knownIfaceNames != nil {
				if _, known := knownIfaceNames[name]; !known {
					continue
				}
			}
			if !clablinks.HasOwnershipAltName(link) {
				continue
			}

			ifaceNames = append(ifaceNames, name)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(ifaceNames)

	return ifaceNames, nil
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
	if plan == nil || nodeName == "" {
		return
	}
	if plan.restartNodeSet == nil {
		plan.restartNodeSet = map[string]struct{}{}
	}
	if plan.linkRestartNodeSet == nil {
		plan.linkRestartNodeSet = map[string]struct{}{}
	}
	if plan.recreatedNodeSet == nil {
		plan.recreatedNodeSet = map[string]struct{}{}
	}
	if plan.addedNodeSet == nil {
		plan.addedNodeSet = map[string]struct{}{}
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

	switch c.applyNodeLinkApplyMode(ctx, plan, node) {
	case clabnodes.LinkApplyModeLive:
		log.Info("Applying link change without node lifecycle action", "node", nodeName, "change", change)
	case clabnodes.LinkApplyModeRestart:
		plan.linkRestartNodeSet[nodeName] = struct{}{}
	case clabnodes.LinkApplyModeRecreate:
		plan.recreatedNodeSet[nodeName] = struct{}{}
		plan.addedNodeSet[nodeName] = struct{}{}
		delete(plan.restartNodeSet, nodeName)
		delete(plan.linkRestartNodeSet, nodeName)
	}
}

func (c *CLab) applyNodeLinkApplyMode(
	ctx context.Context,
	plan *applyPlan,
	node clabnodes.Node,
) clabnodes.LinkApplyMode {
	mode := applyNodeLinkApplyMode(node)
	if mode == clabnodes.LinkApplyModeLive {
		return mode
	}

	if c.applyNodeUsesBoxenImage(ctx, plan, node) {
		return clabnodes.LinkApplyModeLive
	}

	return mode
}

func (c *CLab) applyNodeUsesBoxenImage(
	ctx context.Context,
	plan *applyPlan,
	node clabnodes.Node,
) bool {
	if node == nil {
		return false
	}

	cfg := node.Config()
	if cfg == nil || cfg.Image == "" {
		return false
	}

	if plan != nil {
		if plan.boxenImageCache == nil {
			plan.boxenImageCache = map[string]bool{}
		}
		if supported, exists := plan.boxenImageCache[cfg.Image]; exists {
			return supported
		}
	}

	supported := c.imageHasBoxenVendorLabel(ctx, node, cfg.Image)
	if plan != nil {
		plan.boxenImageCache[cfg.Image] = supported
	}

	return supported
}

func (c *CLab) imageHasBoxenVendorLabel(
	ctx context.Context,
	node clabnodes.Node,
	image string,
) bool {
	runtime := node.GetRuntime()
	if runtime == nil && c != nil && c.Runtimes != nil && c.globalRuntimeName != "" {
		runtime = c.Runtimes[c.globalRuntimeName]
	}
	if runtime == nil {
		log.Debug("Skipping image label check for apply link hotplug support",
			"image", image,
			"reason", "node runtime is not initialized",
		)
		return false
	}

	inspect, err := runtime.InspectImage(ctx, image)
	if err != nil {
		log.Debug("Failed to inspect image for apply link hotplug support",
			"image", image,
			"error", err,
		)
		return false
	}
	if inspect == nil || inspect.Config.Labels == nil {
		return false
	}

	return inspect.Config.Labels[boxenImageVendorLabel] == boxenImageVendorValue
}

func applyNodeLinkApplyMode(node clabnodes.Node) clabnodes.LinkApplyMode {
	if node == nil {
		return clabnodes.LinkApplyModeRecreate
	}

	modeNode, ok := node.(linkApplyModeNode)
	if !ok {
		return clabnodes.LinkApplyModeRecreate
	}

	switch mode := modeNode.LinkApplyMode(); mode {
	case clabnodes.LinkApplyModeLive,
		clabnodes.LinkApplyModeRestart,
		clabnodes.LinkApplyModeRecreate:
		return mode
	default:
		return clabnodes.LinkApplyModeRecreate
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
			plan.result.RestartedNodes = append(plan.result.RestartedNodes, result.Restarted...)
		case clabtypes.TopologyDiffActionRecreate:
			plan.recreatedNodeSet[nodeName] = struct{}{}
			plan.addedNodeSet[nodeName] = struct{}{}
		}
	}

	return nil
}

// reconcileNodes executes Reconcile on each existing node using cached diffs.
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

		diff := plan.nodeDiffs[nodeName]

		result, err := node.Reconcile(ctx, diff)
		if err != nil {
			return fmt.Errorf("reconcile failed for node %q: %w", nodeName, err)
		}

		switch result.Action {
		case clabtypes.TopologyDiffActionRestart:
			if err := c.waitNodeRunning(ctx, node); err != nil {
				return err
			}
			if err := c.waitNodeHealthyIfAvailable(ctx, node); err != nil {
				return err
			}

		case clabtypes.TopologyDiffActionRecreate:
			plan.recreatedNodeSet[nodeName] = struct{}{}
			plan.addedNodeSet[nodeName] = struct{}{}
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

func (c *CLab) prepareApply(
	ctx context.Context,
	addedNodes []string,
) error {
	skipMgmt := c.skipMgmtNetwork()
	if !skipMgmt {
		if err := c.CreateNetwork(ctx); err != nil {
			return err
		}
	}

	if err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge); err != nil {
		return err
	}

	if err := c.loadKernelModules(); err != nil {
		return err
	}

	log.Info("Creating lab directory", "path", c.TopoPaths.TopologyLabDir())
	clabutils.CreateDirectory(c.TopoPaths.TopologyLabDir(), clabconstants.PermissionsDirDefault)
	if err := clabutils.AdjustFileACLs(c.TopoPaths.TopologyLabDir()); err != nil {
		log.Infof("unable to adjust Labdir file ACLs: %v", err)
	}

	if err := createEmptyFile(c.TopoPaths.AnsibleInventoryFileAbsPath()); err != nil {
		return err
	}
	if err := createEmptyFile(c.TopoPaths.NornirSimpleInventoryFileAbsPath()); err != nil {
		return err
	}
	if err := createEmptyFile(c.TopoPaths.TopoExportFile()); err != nil {
		return err
	}

	if err := c.certificateAuthoritySetup(); err != nil {
		return err
	}

	var err error
	c.SSHPubKeys, err = c.RetrieveSSHPubKeys(ctx)
	if err != nil {
		log.Warn(err)
	}

	if err := c.createAuthzKeysFile(); err != nil {
		return err
	}

	c.populateExtraHosts()

	for _, nodeName := range addedNodes {
		if err := c.Nodes[nodeName].PullImage(ctx); err != nil {
			return err
		}
	}

	return nil
}

func createEmptyFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	return f.Close()
}

func (c *CLab) populateExtraHosts() {
	extraHosts := make([]string, 0, len(c.Nodes))

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		n := c.Nodes[nodeName]
		if n.Config().MgmtIPv4Address != "" {
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv4Address)
		}
		if n.Config().MgmtIPv6Address != "" {
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv6Address)
		}
	}

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		n := c.Nodes[nodeName]
		if !strings.HasPrefix(n.Config().NetworkMode, "container:") {
			n.Config().ExtraHosts = extraHosts
		}
	}
}

func (c *CLab) deleteApplyEndpoints(
	ctx context.Context,
	refs []applyEndpointRef,
) error {
	for _, ref := range refs {
		log.Info("Deleting link endpoint", "endpoint", ref.key.String())
		if err := removeInterfaceFromNode(ctx, ref.node, ref.key.iface); err != nil {
			if ref.bestEffort && errors.Is(err, errApplyInterfaceUnowned) {
				log.Debugf("skipping best-effort endpoint delete: %v", err)
				continue
			}
			return err
		}
	}

	return nil
}

func (c *CLab) parkRecreatedNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range sortedStringSet(plan.parkedNodeSet) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		parker, ok := node.(interface{ ParkEndpoints(context.Context) error })
		if !ok {
			return fmt.Errorf("node %q does not support endpoint parking", nodeName)
		}
		log.Info("Parking links for recreate", "node", nodeName)
		if err := parker.ParkEndpoints(ctx); err != nil {
			return fmt.Errorf("failed parking endpoints for node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) restoreRecreatedNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range sortedStringSet(plan.parkedNodeSet) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		restorer, ok := node.(interface{ RestoreEndpoints(context.Context) error })
		if !ok {
			return fmt.Errorf("node %q does not support endpoint restore", nodeName)
		}
		log.Info("Restoring links after recreate", "node", nodeName)
		if err := restorer.RestoreEndpoints(ctx); err != nil {
			return fmt.Errorf("failed restoring endpoints for node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) startStoppedNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range sortedStringSet(plan.startNodeSet) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		log.Info("Starting stopped node", "node", nodeName)
		if err := node.Start(ctx); err != nil {
			return fmt.Errorf("failed starting node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) deleteApplyNodes(ctx context.Context, plan *applyPlan) error {
	nodeNames := make([]string, 0, len(plan.deletedNodeSet)+len(plan.recreatedNodeSet))
	for nodeName := range plan.deletedNodeSet {
		nodeNames = append(nodeNames, nodeName)
	}
	for nodeName := range plan.recreatedNodeSet {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	for _, nodeName := range nodeNames {
		runtimeNode := plan.currentNodes[nodeName]
		if runtimeNode == nil {
			return fmt.Errorf("runtime node %q not found", nodeName)
		}

		for i := len(runtimeNode.containers) - 1; i >= 0; i-- {
			ctr := runtimeNode.containers[i]
			if len(ctr.Names) == 0 {
				return fmt.Errorf("runtime container for node %q has no name", nodeName)
			}

			runtime := ctr.Runtime
			if runtime == nil {
				runtime = c.globalRuntime()
			}

			action := "Deleting node"
			if _, recreate := plan.recreatedNodeSet[nodeName]; recreate {
				action = "Deleting node for recreate"
			}
			log.Info(action, "node", nodeName, "container", ctr.Names[0])
			if err := runtime.DeleteContainer(ctx, ctr.Names[0]); err != nil {
				return fmt.Errorf("failed deleting node %q: %w", nodeName, err)
			}

			_ = clabutils.DeleteNetnsSymlink(ctr.Names[0])
		}
	}

	return nil
}

func (c *CLab) deployApplyNodes(
	ctx context.Context,
	nodeNames []string,
	maxWorkers uint,
) error {
	if len(nodeNames) == 0 {
		return nil
	}

	if maxWorkers == 0 || int(maxWorkers) > len(nodeNames) {
		maxWorkers = uint(len(nodeNames))
	}

	input := make(chan string)
	errCh := make(chan error, len(nodeNames))

	for range maxWorkers {
		go func() {
			for nodeName := range input {
				errCh <- c.deployApplyNode(ctx, nodeName)
			}
		}()
	}

	for _, nodeName := range nodeNames {
		input <- nodeName
	}
	close(input)

	var errs []error
	for range nodeNames {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (c *CLab) deployApplyNode(ctx context.Context, nodeName string) error {
	node := c.Nodes[nodeName]

	log.Info("Creating node", "node", nodeName)
	if err := node.PreDeploy(ctx, &clabnodes.PreDeployParams{
		Cert:         c.Cert,
		TopologyName: c.Config.Name,
		TopoPaths:    c.TopoPaths,
		SSHPubKeys:   c.SSHPubKeys,
	}); err != nil {
		return fmt.Errorf("node %q pre-deploy: %w", nodeName, err)
	}

	if err := node.Deploy(ctx, &clabnodes.DeployParams{Nodes: c.Nodes}); err != nil {
		return fmt.Errorf("node %q deploy: %w", nodeName, err)
	}

	if err := node.UpdateConfigWithRuntimeInfo(ctx); err != nil {
		log.Errorf("failed to update node runtime information for node %s: %v", nodeName, err)
	}

	return nil
}

func (c *CLab) deployApplyLinks(ctx context.Context, links []clablinks.Link) error {
	for _, link := range links {
		log.Info("Creating link", "link", applyLinkName(link))

		for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
			if err := removeInterfaceFromNode(ctx, ep.GetNode(), ep.GetIfaceName()); err != nil {
				return err
			}
		}

		for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
			deployable, ok := ep.(clablinks.DeployableEndpoint)
			if !ok {
				return fmt.Errorf("endpoint %q is not deployable", ep.GetIfaceName())
			}

			if err := deployable.Deploy(ctx); err != nil {
				return fmt.Errorf("failed deploying link %s: %w", applyLinkName(link), err)
			}
		}

		if stitchedLink, ok := link.(*clablinks.VxlanStitched); ok {
			if err := stitchedLink.Stitch(); err != nil {
				return fmt.Errorf("failed stitching link %s: %w", applyLinkName(link), err)
			}
		}
	}

	return nil
}

type postDeployEndpointsNode interface {
	PostDeployEndpoints(context.Context) error
}

func (c *CLab) postDeployApplyLinks(ctx context.Context, nodeNames []string) error {
	for _, nodeName := range nodeNames {
		node, ok := c.Nodes[nodeName].(postDeployEndpointsNode)
		if !ok {
			continue
		}

		if err := node.PostDeployEndpoints(ctx); err != nil {
			return fmt.Errorf("node %q post-deploy links: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) postDeployApplyNodes(
	ctx context.Context,
	nodeNames []string,
	skipPostDeploy bool,
) error {
	execCollection := clabexec.NewExecCollection()

	for _, nodeName := range nodeNames {
		node := c.Nodes[nodeName]

		if !skipPostDeploy {
			if err := node.PostDeploy(ctx, &clabnodes.PostDeployParams{Nodes: c.Nodes}); err != nil {
				return fmt.Errorf("node %q post-deploy: %w", nodeName, err)
			}
		}

		if err := node.RunExecFromConfig(ctx, execCollection); err != nil {
			log.Errorf("failed to run exec commands for %s: %v", nodeName, err)
		}
	}

	execCollection.Log()

	return nil
}

func (p *applyPlan) setEndpointNode(nodeName string, node clablinks.Node) {
	if p == nil || nodeName == "" || node == nil {
		return
	}
	if p.endpointNodes == nil {
		p.endpointNodes = map[string]clablinks.Node{}
	}
	p.endpointNodes[nodeName] = node
}

func (p *applyPlan) endpointNode(nodeName string) (clablinks.Node, bool) {
	if p == nil || p.endpointNodes == nil {
		return nil, false
	}
	node, ok := p.endpointNodes[nodeName]
	return node, ok
}

func (c *CLab) restartApplyNodes(
	ctx context.Context,
	nodeSet map[string]struct{},
) ([]string, error) {
	nodeNames := sortedStringSet(nodeSet)
	restarted := make([]string, 0, len(nodeNames))

	for _, nodeName := range nodeNames {
		node := c.Nodes[nodeName]
		if node == nil {
			continue
		}

		log.Info("Restarting node after link apply", "node", nodeName)
		if err := node.Stop(ctx); err != nil {
			return restarted, err
		}
		if err := node.Start(ctx); err != nil {
			return restarted, err
		}
		if err := c.waitNodeRunning(ctx, node); err != nil {
			return restarted, err
		}
		if err := c.waitNodeHealthyIfAvailable(ctx, node); err != nil {
			return restarted, err
		}

		restarted = append(restarted, nodeName)
	}

	return restarted, nil
}

func (c *CLab) waitNodeRunning(ctx context.Context, node clabnodes.Node) error {
	deadline := time.Now().Add(c.timeout)

	for {
		status := node.GetContainerStatus(ctx)
		if clabruntime.ContainerHasJoinableNetns(status) {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("node %q did not reach running state, current state %s",
				node.GetShortName(), status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (c *CLab) waitNodeHealthyIfAvailable(ctx context.Context, node clabnodes.Node) error {
	if node.Config().Healthcheck == nil {
		return nil
	}

	deadline := time.Now().Add(c.timeout)

	for {
		healthy, err := node.IsHealthy(ctx)
		if healthy {
			return nil
		}
		if err != nil && strings.Contains(err.Error(), "no health information") {
			return nil
		}
		if err != nil {
			log.Debugf("error checking node %q health: %v", node.GetShortName(), err)
		}

		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("node %q did not become healthy: %w", node.GetShortName(), err)
			}
			return fmt.Errorf("node %q did not become healthy", node.GetShortName())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (c *CLab) updateApplyRuntimeInfo(ctx context.Context) error {
	for _, nodeName := range sortedNodeNames(c.Nodes) {
		node := c.Nodes[nodeName]
		if node.GetContainerStatus(ctx) == clabruntime.NotFound {
			continue
		}
		if err := node.UpdateConfigWithRuntimeInfo(ctx); err != nil {
			return fmt.Errorf("failed to update runtime info for node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) regenerateApplyArtifacts(ctx context.Context, exportTemplate string) error {
	if err := c.GenerateInventories(); err != nil {
		return err
	}

	topoDataF, err := os.Create(c.TopoPaths.TopoExportFile())
	if err != nil {
		return err
	}
	defer topoDataF.Close()

	if err := c.GenerateExports(ctx, topoDataF, exportTemplate); err != nil {
		return err
	}

	if !c.skipMgmtNetwork() {
		log.Info("Updating host entries", "path", "/etc/hosts")
		if err := c.appendHostsFileEntries(ctx); err != nil {
			log.Errorf("failed to update hosts file: %v", err)
		}
	}

	log.Info("Updating SSH config for nodes", "path", c.TopoPaths.SSHConfigPath())
	if err := c.addSSHConfig(); err != nil {
		log.Errorf("failed to create ssh config file: %v", err)
	}

	return nil
}

func removeInterfaceFromNode(ctx context.Context, node clablinks.Node, ifaceName string) error {
	if node == nil || ifaceName == "" {
		return nil
	}

	return node.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifaceName)
		if _, notfound := err.(netlink.LinkNotFoundError); notfound {
			return nil
		}
		if err != nil {
			return err
		}
		if !clablinks.HasOwnershipAltName(link) {
			return fmt.Errorf(
				"%w: interface %q on node %q",
				errApplyInterfaceUnowned,
				ifaceName,
				node.GetShortName(),
			)
		}

		return netlink.LinkDel(link)
	})
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

func sortedNodeNames(nodes map[string]clabnodes.Node) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedRuntimeNodeNames(nodes map[string]*applyRuntimeNode) []string {
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

func applyContainerNodeName(ctr clabruntime.GenericContainer) string {
	if name := ctr.Labels[clabconstants.NodeName]; name != "" {
		return name
	}
	if len(ctr.Names) > 0 {
		return ctr.Names[0]
	}
	return ctr.ID
}

func applyContainerLess(a, b clabruntime.GenericContainer) bool {
	aName := applyContainerNodeName(a)
	bName := applyContainerNodeName(b)

	aOrder := applyComponentSortOrder(aName)
	bOrder := applyComponentSortOrder(bName)
	if aOrder != bOrder {
		return aOrder < bOrder
	}

	return aName < bName
}

func applyComponentSortOrder(name string) int {
	idx := strings.LastIndex(name, "-")
	if idx < 0 || idx == len(name)-1 {
		return 0
	}

	slot := strings.ToUpper(name[idx+1:])
	if len(slot) == 1 && slot[0] >= 'A' && slot[0] <= 'Z' {
		return 100 - int(slot[0])
	}

	var n int
	for _, r := range slot {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}

	return n
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
