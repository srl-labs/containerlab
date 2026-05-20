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
	key  applyEndpointKey
	node clabnodes.Node
}

type applyPlan struct {
	result             *ApplyResult
	currentNodes       map[string]clabruntime.GenericContainer
	addedNodeSet       map[string]struct{}
	addedLinks         []clablinks.Link
	staleEndpoints     []applyEndpointRef
	affectedNodeSet    map[string]struct{}
	desiredEndpointSet map[applyEndpointKey]struct{}
	liveEndpointSet    map[applyEndpointKey]struct{}
}

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

func (c *CLab) runtimeNodeContainers(
	ctx context.Context,
) (map[string]clabruntime.GenericContainer, error) {
	containers, err := c.ListContainers(ctx, WithListLabName(c.Config.Name))
	if err != nil {
		return nil, err
	}

	result := make(map[string]clabruntime.GenericContainer)
	var duplicates []string

	for _, ctr := range containers {
		if ctr.Labels[clabconstants.ToolType] != "" {
			continue
		}

		nodeName := ctr.Labels[clabconstants.NodeName]
		if nodeName == "" {
			continue
		}

		if _, exists := result[nodeName]; exists {
			duplicates = append(duplicates, nodeName)
			continue
		}

		result[nodeName] = ctr
	}

	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return nil, fmt.Errorf(
			"apply supports only single-container nodes, found multiple containers for nodes: %s",
			strings.Join(duplicates, ", "),
		)
	}

	return result, nil
}

func (c *CLab) checkApplySupported(
	currentNodes map[string]clabruntime.GenericContainer,
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
		case len(cfg.Components) > 0:
			unsupported = append(unsupported, fmt.Sprintf("%s: multi-component node", cfg.ShortName))
		}
	}

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		link := c.Links[linkIdx]
		if link.GetType() != clablinks.LinkTypeVEth {
			unsupported = append(
				unsupported,
				fmt.Sprintf("link %s: unsupported type %q", applyLinkName(link), link.GetType()),
			)
		}
	}

	for nodeName, ctr := range currentNodes {
		n, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}

		cfg := n.Config()
		checkRuntimeLabel(&unsupported, ctr, clabconstants.LongName, cfg.LongName, cfg.ShortName)
		checkRuntimeLabel(&unsupported, ctr, clabconstants.NodeKind, cfg.Kind, cfg.ShortName)
		checkRuntimeLabel(&unsupported, ctr, clabconstants.NodeType, cfg.NodeType, cfg.ShortName)
		checkRuntimeLabel(&unsupported, ctr, clabconstants.NodeGroup, cfg.Group, cfg.ShortName)
	}

	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return fmt.Errorf("apply unsupported for: %s", strings.Join(unsupported, "; "))
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
	currentNodes map[string]clabruntime.GenericContainer,
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
		for _, ep := range c.Links[linkIdx].GetEndpoints() {
			plan.desiredEndpointSet[endpointKeyFromEndpoint(ep)] = struct{}{}
		}
	}

	if err := c.discoverLiveApplyEndpoints(ctx, plan); err != nil {
		return nil, err
	}

	for _, key := range sortedEndpointKeys(plan.liveEndpointSet) {
		if _, exists := plan.desiredEndpointSet[key]; exists {
			continue
		}

		n := c.Nodes[key.node]
		ref := applyEndpointRef{key: key, node: n}
		plan.staleEndpoints = append(plan.staleEndpoints, ref)
		plan.result.DeletedEndpoints = append(plan.result.DeletedEndpoints, key.String())
		plan.affectedNodeSet[key.node] = struct{}{}
	}

	for _, linkIdx := range sortedLinkIndexes(c.Links) {
		link := c.Links[linkIdx]
		if !plan.linkNeedsDeploy(link) {
			continue
		}

		plan.addedLinks = append(plan.addedLinks, link)
		plan.result.AddedLinks = append(plan.result.AddedLinks, applyLinkName(link))

		for _, ep := range link.GetEndpoints() {
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

func (c *CLab) discoverLiveApplyEndpoints(
	ctx context.Context,
	plan *applyPlan,
) error {
	for _, nodeName := range sortedNodeNames(c.Nodes) {
		if _, exists := plan.currentNodes[nodeName]; !exists {
			continue
		}

		n := c.Nodes[nodeName]
		status := n.GetContainerStatus(ctx)
		if !clabruntime.ContainerHasJoinableNetns(status) {
			return fmt.Errorf(
				"node %q is %s; apply requires existing nodes to have a joinable network namespace",
				nodeName,
				status,
			)
		}

		ifaceNames, err := discoverOwnedVethInterfaceNames(ctx, n)
		if err != nil {
			return fmt.Errorf("failed to discover runtime interfaces for node %q: %w", nodeName, err)
		}

		for _, ifaceName := range ifaceNames {
			plan.liveEndpointSet[applyEndpointKey{node: nodeName, iface: ifaceName}] = struct{}{}
		}
	}

	return nil
}

func discoverOwnedVethInterfaceNames(ctx context.Context, node clabnodes.Node) ([]string, error) {
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
			if link.Type() != "veth" {
				continue
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

func (p *applyPlan) linkNeedsDeploy(link clablinks.Link) bool {
	for _, ep := range link.GetEndpoints() {
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
			return err
		}
	}

	return nil
}

func (c *CLab) deleteApplyNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range plan.result.DeletedNodes {
		ctr := plan.currentNodes[nodeName]
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

		for _, ep := range link.GetEndpoints() {
			if err := removeInterfaceFromNode(ctx, ep.GetNode(), ep.GetIfaceName()); err != nil {
				return err
			}
		}

		for _, ep := range link.GetEndpoints() {
			deployable, ok := ep.(clablinks.DeployableEndpoint)
			if !ok {
				return fmt.Errorf("endpoint %q is not deployable", ep.GetIfaceName())
			}

			if err := deployable.Deploy(ctx); err != nil {
				return fmt.Errorf("failed deploying link %s: %w", applyLinkName(link), err)
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
	return node.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifaceName)
		if _, notfound := err.(netlink.LinkNotFoundError); notfound {
			return nil
		}
		if err != nil {
			return err
		}
		if link.Type() != "veth" || !clablinks.HasOwnershipAltName(link) {
			return fmt.Errorf(
				"interface %q on node %q exists but is not marked as containerlab-owned",
				ifaceName,
				node.GetShortName(),
			)
		}

		return netlink.LinkDel(link)
	})
}

func endpointKeyFromEndpoint(ep clablinks.Endpoint) applyEndpointKey {
	return applyEndpointKey{
		node:  ep.GetNode().GetShortName(),
		iface: ep.GetIfaceName(),
	}
}

func applyLinkName(link clablinks.Link) string {
	endpoints := link.GetEndpoints()
	names := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		names = append(names, endpointKeyFromEndpoint(ep).String())
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

func sortedRuntimeNodeNames(nodes map[string]clabruntime.GenericContainer) []string {
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
