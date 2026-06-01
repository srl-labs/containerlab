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
	addedLinks         []clablinks.Link
	staleEndpoints     []applyEndpointRef
	affectedNodeSet    map[string]struct{}
	desiredEndpointSet map[applyEndpointKey]struct{}
	liveEndpointSet    map[applyEndpointKey]struct{}
}

var errApplyInterfaceUnowned = errors.New("interface is not marked as containerlab-owned")

func (p *applyPlan) empty() bool {
	return !p.result.DeployedLab &&
		len(p.result.AddedNodes) == 0 &&
		len(p.result.DeletedNodes) == 0 &&
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

	if err := c.checkApplySupported(currentNodes); err != nil {
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

	if options.dryRun || plan.empty() {
		return plan.result, nil
	}

	if err := c.prepareApply(ctx, plan.result.AddedNodes); err != nil {
		return nil, err
	}

	if err := c.deleteApplyEndpoints(ctx, plan.staleEndpoints); err != nil {
		return nil, err
	}

	if err := c.deleteApplyNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.deployApplyNodes(ctx, plan.result.AddedNodes, options.maxWorkers); err != nil {
		return nil, err
	}

	if err := c.deployApplyLinks(ctx, plan.addedLinks); err != nil {
		return nil, err
	}

	if err := c.postDeployApplyNodes(ctx, plan.result.AddedNodes, options.skipPostDeploy); err != nil {
		return nil, err
	}

	restarted, err := c.restartApplyNodes(ctx, plan.affectedNodeSet)
	if err != nil {
		return nil, err
	}
	plan.result.RestartedNodes = restarted

	if err := c.updateApplyRuntimeInfo(ctx); err != nil {
		return nil, err
	}

	if err := c.regenerateApplyArtifacts(ctx, options.exportTemplate); err != nil {
		return nil, err
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

func (c *CLab) checkApplySupported(
	currentNodes map[string]*applyRuntimeNode,
) error {
	var unsupported []string

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		n := c.Nodes[nodeName]
		cfg := n.Config()

		switch {
		case cfg.IsRootNamespaceBased:
			unsupported = append(unsupported, fmt.Sprintf("%s: root namespace node", cfg.ShortName))
		case cfg.AutoRemove:
			unsupported = append(unsupported, fmt.Sprintf("%s: auto-remove node", cfg.ShortName))
		case cfg.SkipUniquenessCheck:
			unsupported = append(unsupported, fmt.Sprintf("%s: external/pre-existing node", cfg.ShortName))
		case strings.HasPrefix(cfg.NetworkMode, "container:"):
			unsupported = append(unsupported, fmt.Sprintf("%s: shared container namespace", cfg.ShortName))
		}
	}

	for nodeName, runtimeNode := range currentNodes {
		n, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}

		cfg := n.Config()
		desiredDistributed := len(cfg.Components) > 1
		if runtimeNode.distributed != desiredDistributed {
			unsupported = append(
				unsupported,
				fmt.Sprintf("%s: distributed component layout changed", cfg.ShortName),
			)
		} else if desiredDistributed {
			desiredComponents := desiredApplyComponentNames(n)
			runtimeComponents := runtimeApplyComponentNames(runtimeNode)
			if strings.Join(desiredComponents, "\x00") != strings.Join(runtimeComponents, "\x00") {
				unsupported = append(
					unsupported,
					fmt.Sprintf("%s: distributed component layout changed", cfg.ShortName),
				)
			}
		}

		for _, ctr := range runtimeNode.containers {
			if runtimeNode.distributed {
				checkRuntimeLabel(
					&unsupported,
					ctr,
					clabconstants.RootNodeLongName,
					cfg.LongName,
					cfg.ShortName,
				)
			} else {
				checkRuntimeLabel(&unsupported, ctr, clabconstants.LongName, cfg.LongName, cfg.ShortName)
			}
			checkRuntimeLabel(&unsupported, ctr, clabconstants.NodeKind, cfg.Kind, cfg.ShortName)
			checkRuntimeLabel(&unsupported, ctr, clabconstants.NodeType, cfg.NodeType, cfg.ShortName)
			checkRuntimeLabel(&unsupported, ctr, clabconstants.NodeGroup, cfg.Group, cfg.ShortName)
		}
	}

	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return fmt.Errorf(
			"apply unsupported for: %s; use redeploy or deploy --reconfigure",
			strings.Join(unsupported, "; "),
		)
	}

	return nil
}

func checkRuntimeLabel(
	unsupported *[]string,
	ctr clabruntime.GenericContainer,
	label,
	want,
	nodeName string,
) {
	got, exists := ctr.Labels[label]
	if !exists || got == want {
		return
	}

	*unsupported = append(
		*unsupported,
		fmt.Sprintf("%s: runtime label %s=%q, desired %q", nodeName, label, got, want),
	)
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
	plan := &applyPlan{
		result: &ApplyResult{
			AddedNodes:       []string{},
			DeletedNodes:     []string{},
			AddedLinks:       []string{},
			DeletedEndpoints: []string{},
			RestartedNodes:   []string{},
		},
		currentNodes:       currentNodes,
		addedNodeSet:       map[string]struct{}{},
		affectedNodeSet:    map[string]struct{}{},
		desiredEndpointSet: map[applyEndpointKey]struct{}{},
		liveEndpointSet:    map[applyEndpointKey]struct{}{},
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
		}
	}

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		for _, ep := range clablinks.ApplyRuntimeEndpoints(c.Links[linkIdx]) {
			plan.desiredEndpointSet[endpointKeyFromEndpoint(ep)] = struct{}{}
		}
	}

	if err := c.discoverLiveApplyEndpoints(ctx, plan); err != nil {
		return nil, err
	}

	c.planDeletedEndpoints(plan)

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		link := c.Links[linkIdx]
		if !plan.linkNeedsDeploy(link) {
			continue
		}

		plan.addedLinks = append(plan.addedLinks, link)
		plan.result.AddedLinks = append(plan.result.AddedLinks, applyLinkName(link))

		for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
			nodeName := ep.GetNode().GetShortName()
			if _, exists := currentNodes[nodeName]; !exists {
				continue
			}
			if _, added := plan.addedNodeSet[nodeName]; added {
				continue
			}

			plan.affectedNodeSet[nodeName] = struct{}{}
		}
	}

	return plan, nil
}

func (c *CLab) planDeletedEndpoints(plan *applyPlan) {
	plannedEndpointSet := map[applyEndpointKey]struct{}{}

	for _, key := range sortedEndpointKeys(plan.liveEndpointSet) {
		if _, exists := plan.desiredEndpointSet[key]; exists {
			continue
		}
		c.addStaleApplyEndpoint(plan, plannedEndpointSet, key)
	}
}

func (c *CLab) addStaleApplyEndpoint(
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

	node, exists := c.applyLinkNode(key.node)
	if !exists {
		return
	}

	plannedEndpointSet[key] = struct{}{}
	plan.staleEndpoints = append(plan.staleEndpoints, applyEndpointRef{key: key, node: node})
	plan.result.DeletedEndpoints = append(plan.result.DeletedEndpoints, key.String())

	if _, exists := c.Nodes[key.node]; exists {
		if _, added := plan.addedNodeSet[key.node]; !added {
			plan.affectedNodeSet[key.node] = struct{}{}
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

		if !isApplySpecialNode(nodeName) {
			status := c.Nodes[nodeName].GetContainerStatus(ctx)
			if !clabruntime.ContainerHasJoinableNetns(status) {
				return fmt.Errorf(
					"node %q is %s; apply requires existing nodes to have a joinable network namespace",
					nodeName,
					status,
				)
			}
		}

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

func (p *applyPlan) linkNeedsDeploy(link clablinks.Link) bool {
	for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
		key := endpointKeyFromEndpoint(ep)
		if _, added := p.addedNodeSet[key.node]; added {
			return true
		}
		if _, live := p.liveEndpointSet[key]; !live {
			return true
		}
	}

	return false
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

func (c *CLab) deleteApplyNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range plan.result.DeletedNodes {
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

			log.Info("Deleting node", "node", nodeName, "container", ctr.Names[0])
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
				log.Errorf("failed to run postdeploy task for node %s: %v", nodeName, err)
			}
		}

		if err := node.RunExecFromConfig(ctx, execCollection); err != nil {
			log.Errorf("failed to run exec commands for %s: %v", nodeName, err)
		}
	}

	execCollection.Log()

	return nil
}

func (c *CLab) restartApplyNodes(
	ctx context.Context,
	affectedNodeSet map[string]struct{},
) ([]string, error) {
	nodeNames := sortedStringSet(affectedNodeSet)
	restarted := make([]string, 0, len(nodeNames))

	for _, nodeName := range nodeNames {
		node := c.Nodes[nodeName]
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

func desiredApplyComponentNames(n clabnodes.Node) []string {
	cfg := n.Config()
	names := make([]string, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		if component == nil {
			continue
		}

		slot := strings.ToLower(strings.TrimSpace(component.Slot))
		if slot == "" {
			continue
		}
		names = append(names, cfg.ShortName+"-"+slot)
	}

	sortApplyComponentNames(names)

	return names
}

func runtimeApplyComponentNames(runtimeNode *applyRuntimeNode) []string {
	if runtimeNode == nil {
		return nil
	}

	names := make([]string, 0, len(runtimeNode.containers))
	for _, ctr := range runtimeNode.containers {
		name := applyContainerNodeName(ctr)
		if name == "" {
			continue
		}
		names = append(names, name)
	}

	sortApplyComponentNames(names)

	return names
}

func sortApplyComponentNames(names []string) {
	sort.Slice(names, func(i, j int) bool {
		iOrder := applyComponentSortOrder(names[i])
		jOrder := applyComponentSortOrder(names[j])
		if iOrder != jOrder {
			return iOrder < jOrder
		}

		return names[i] < names[j]
	})
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
