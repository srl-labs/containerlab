package core

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// ApplyResult summarizes the changes applied by an apply operation.
type ApplyResult struct {
	DryRun           bool     `json:"dry-run"`
	DeployedLab      bool     `json:"deployed-lab"`
	LabName          string   `json:"lab-name,omitempty"`
	AddedNodes       []string `json:"added-nodes"`
	DeletedNodes     []string `json:"deleted-nodes"`
	RecreatedNodes   []string `json:"recreated-nodes"`
	StartedNodes     []string `json:"started-nodes"`
	AddedLinks       []string `json:"added-links"`
	DeletedEndpoints []string `json:"deleted-endpoints"`
	RestartedNodes   []string `json:"restarted-nodes"`
	// NodeChangeReasons explains per node why apply restarts or recreates it,
	// e.g. "added link" or "config drift: image".
	NodeChangeReasons map[string]string `json:"node-change-reasons,omitempty"`
}

func applyResultFromPlan(plan *applyPlan) *ApplyResult {
	result := &ApplyResult{
		AddedNodes:        []string{},
		DeletedNodes:      []string{},
		RecreatedNodes:    []string{},
		StartedNodes:      []string{},
		AddedLinks:        []string{},
		DeletedEndpoints:  []string{},
		RestartedNodes:    []string{},
		NodeChangeReasons: map[string]string{},
	}
	if plan == nil {
		return result
	}

	result.AddedNodes = sortedStringSet(plan.addedNodeSet)
	result.DeletedNodes = sortedStringSet(plan.deletedNodeSet)
	result.RecreatedNodes = sortedStringSet(plan.recreatedNodeSet)
	result.StartedNodes = sortedStringSet(plan.startNodeSet)
	result.AddedLinks = applyLinkNames(plan.addedLinks)
	result.DeletedEndpoints = applyDeletedEndpointNames(plan.staleEndpoints)
	result.RestartedNodes = sortedStringSet(unionStringSets(
		plan.restartNodeSet,
		plan.linkRestartNodeSet,
		plan.postLinkRestartNodeSet,
	))

	for nodeName, reason := range plan.nodeChangeReasons {
		result.NodeChangeReasons[nodeName] = reason
	}

	return result
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

	currentNodes, err := c.runtimeNodeGroups(ctx)
	if err != nil {
		return nil, err
	}

	return c.apply(ctx, options, currentNodes)
}

func (c *CLab) apply(
	ctx context.Context,
	options *ApplyOptions,
	currentNodes map[string]*runtimeNodeGroup,
) (*ApplyResult, error) {
	initialDeploy, err := c.needsInitialDeploy(currentNodes)
	if err != nil {
		return nil, err
	}
	if initialDeploy {
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
			SetSkipLabDirFileACLs(options.skipLabDirFileACLs).
			SetGraph(options.graph).
			SetExportTemplate(options.exportTemplate)

		if _, err := c.deploy(ctx, deployOptions); err != nil {
			return nil, err
		}

		return result, nil
	}

	if err := c.setMgmtBridgeFromRuntime(currentNodes); err != nil {
		return nil, err
	}

	if err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge); err != nil {
		return nil, err
	}

	if err := c.resolveApplyLinks(); err != nil {
		return nil, err
	}

	if err := c.checkApplyTopologyDefinition(ctx); err != nil {
		return nil, err
	}

	plan, err := c.planApply(ctx, currentNodes, c.nodeFilter)
	if err != nil {
		return nil, err
	}

	if options.dryRun {
		result := applyResultFromPlan(plan)
		result.DryRun = true
		return result, nil
	}

	if err := c.reconcileNodes(ctx, plan); err != nil {
		return nil, err
	}

	if plan.empty() {
		if plan.mutableNodeSet != nil {
			c.writeApplyState(plan)
		} else if options.finalizeNoop {
			if err := c.prepareApply(ctx, nil, options.skipLabDirFileACLs); err != nil {
				return nil, err
			}
			if _, err := c.finalize(ctx, options.exportTemplate, options.graph); err != nil {
				return nil, err
			}
		} else {
			c.writeState()
		}
		return applyResultFromPlan(plan), nil
	}

	deployNodeNames := plan.deployNodeNames()
	if err := c.prepareApply(ctx, deployNodeNames, options.skipLabDirFileACLs); err != nil {
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

	if err := c.startStoppedNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.deployApplyNodesAndLinks(ctx, plan, deployNodeNames, options.maxWorkers); err != nil {
		return nil, err
	}

	if err := c.postDeployApplyNodes(ctx, deployNodeNames, options.skipPostDeploy); err != nil {
		return nil, err
	}

	if err := c.restartApplyNodes(ctx, plan.linkRestartNodeSet); err != nil {
		return nil, err
	}

	if err := c.updateRuntimeInfoForExistingNodes(ctx); err != nil {
		return nil, err
	}

	if plan.mutableNodeSet != nil {
		c.writeApplyState(plan)
		if err := c.regenerateApplyArtifacts(ctx, options.exportTemplate); err != nil {
			log.Warnf(
				"filtered apply completed and state was persisted, but shared artifacts could not be regenerated: %v",
				err,
			)
		}
	} else {
		if _, err := c.finalize(ctx, options.exportTemplate, options.graph); err != nil {
			return nil, err
		}
	}

	return applyResultFromPlan(plan), nil
}

// deployApplyNodesAndLinks advances one dependency batch at a time. Links whose
// endpoint containers are available are attached before the batch health check,
// so shared-netns children cannot start against an unready provider namespace.
func (c *CLab) deployApplyNodesAndLinks(
	ctx context.Context,
	plan *applyPlan,
	nodeNames []string,
	maxWorkers uint,
) error {
	batches, err := c.applyDeployBatches(nodeNames)
	if err != nil {
		return err
	}

	available := map[string]struct{}{"host": {}, "mgmt-net": {}}
	for nodeName := range plan.currentNodes {
		if _, deleted := plan.deletedNodeSet[nodeName]; deleted {
			continue
		}
		if _, recreated := plan.recreatedNodeSet[nodeName]; recreated {
			continue
		}
		available[nodeName] = struct{}{}
	}
	pendingLinks := append([]clablinks.Link(nil), plan.addedLinks...)
	postLinkRestarted := map[string]struct{}{}

	for _, batch := range batches {
		// Existing providers may gain a link in the same filtered apply that
		// introduces a new shared-netns child. Attach those links and satisfy
		// provider health before the child process can observe the namespace.
		ready, remaining := applyPartitionReadyLinks(pendingLinks, available)
		if err := c.deployFreshApplyLinks(ctx, ready, nil); err != nil {
			return err
		}
		pendingLinks = remaining
		if err := c.restartRecreatedApplyLinkNodes(
			ctx, plan, ready, pendingLinks, postLinkRestarted,
		); err != nil {
			return err
		}
		for _, nodeName := range batch {
			provider, shared := c.applySharedNetNSProvider(c.Nodes[nodeName])
			if !shared {
				continue
			}
			if _, exists := available[provider]; !exists {
				continue
			}
			if err := c.waitNodeHealthyIfAvailable(ctx, c.Nodes[provider]); err != nil {
				return err
			}
		}

		if err := c.deployNodeBatch(ctx, batch, maxWorkers); err != nil {
			return err
		}
		for _, nodeName := range batch {
			if err := c.waitNodeRunning(ctx, c.Nodes[nodeName]); err != nil {
				return err
			}
			available[nodeName] = struct{}{}
		}
		if err := c.restoreRecreatedNodeBatch(ctx, plan, batch, pendingLinks); err != nil {
			return err
		}

		ready, remaining = applyPartitionReadyLinks(pendingLinks, available)
		if err := c.deployFreshApplyLinks(ctx, ready, batch); err != nil {
			return err
		}
		pendingLinks = remaining
		if err := c.restartRecreatedApplyLinkNodes(
			ctx, plan, ready, pendingLinks, postLinkRestarted,
		); err != nil {
			return err
		}

		if err := c.waitNodesHealthy(ctx, batch); err != nil {
			return err
		}
	}
	ready, remaining := applyPartitionReadyLinks(pendingLinks, available)
	if err := c.deployFreshApplyLinks(ctx, ready, nil); err != nil {
		return err
	}
	pendingLinks = remaining
	if err := c.restartRecreatedApplyLinkNodes(
		ctx, plan, ready, pendingLinks, postLinkRestarted,
	); err != nil {
		return err
	}

	if len(pendingLinks) != 0 {
		return fmt.Errorf("apply links still depend on unavailable nodes: %v", applyLinkNames(pendingLinks))
	}

	return nil
}

func (c *CLab) restartRecreatedApplyLinkNodes(
	ctx context.Context,
	plan *applyPlan,
	links []clablinks.Link,
	pendingLinks []clablinks.Link,
	restarted map[string]struct{},
) error {
	nodeSet := map[string]struct{}{}
	for _, link := range links {
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			nodeName := endpointKeyFromEndpoint(ep).node
			if _, required := plan.postLinkRestartNodeSet[nodeName]; !required {
				continue
			}
			if _, done := restarted[nodeName]; done {
				continue
			}
			nodeSet[nodeName] = struct{}{}
		}
	}
	for nodeName := range nodeSet {
		if linkTouchesNodeSetInList(pendingLinks, map[string]struct{}{nodeName: {}}) {
			delete(nodeSet, nodeName)
		}
	}
	if err := c.restartApplyNodes(ctx, nodeSet); err != nil {
		return err
	}
	for nodeName := range nodeSet {
		restarted[nodeName] = struct{}{}
	}
	return nil
}

func linkTouchesNodeSetInList(links []clablinks.Link, nodeSet map[string]struct{}) bool {
	for _, link := range links {
		if linkTouchesNodeSet(link, nodeSet) {
			return true
		}
	}
	return false
}

func (c *CLab) deployFreshApplyLinks(
	ctx context.Context,
	links []clablinks.Link,
	additionalNodeNames []string,
) error {
	if err := c.removeApplyLinkEndpoints(ctx, links); err != nil {
		return err
	}
	return c.deployLinks(ctx, links, additionalNodeNames)
}

func applyPartitionReadyLinks(
	links []clablinks.Link,
	available map[string]struct{},
) (ready, pending []clablinks.Link) {
	for _, link := range links {
		linkReady := true
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			key := endpointKeyFromEndpoint(ep)
			if _, exists := available[key.node]; !exists {
				linkReady = false
				break
			}
		}
		if linkReady {
			ready = append(ready, link)
		} else {
			pending = append(pending, link)
		}
	}
	return ready, pending
}

// resolveApplyLinks keeps the full desired topology available to reconciliation.
// ResolveLinks normally uses nodeFilter to discard links during a fresh filtered
// deploy; apply instead uses that filter only to limit which nodes may change.
func (c *CLab) resolveApplyLinks() error {
	nodeFilter := c.nodeFilter
	c.nodeFilter = nil
	err := c.ResolveLinks()
	c.nodeFilter = nodeFilter
	return err
}

func (c *CLab) writeApplyState(plan *applyPlan) {
	var err error
	if plan == nil || plan.mutableNodeSet == nil {
		err = c.WriteState()
	} else {
		state := plan.state
		if state == nil {
			state = &LabState{Topology: clabtypes.NewTopology()}
		}
		if state.NodeConfigs == nil {
			state.NodeConfigs = map[string]*clabtypes.NodeConfig{}
		}
		for nodeName := range plan.mutableNodeSet {
			node, exists := c.Nodes[nodeName]
			if !exists {
				delete(state.NodeConfigs, nodeName)
				continue
			}
			state.NodeConfigs[nodeName] = c.applyStateNodeConfig(nodeName, node.Config())
		}
		err = c.writeLabState(state)
	}
	if err != nil {
		log.Warnf("failed to write state file: %v", err)
	}
}

func (c *CLab) applyStateNodeConfig(
	nodeName string,
	runtimeConfig *clabtypes.NodeConfig,
) *clabtypes.NodeConfig {
	if runtimeConfig == nil {
		return nil
	}

	applied := *runtimeConfig
	desired := resolveNodeConfigFromTopology(c.Config.Topology, nodeName)
	if desired == nil {
		return &applied
	}

	applied.NodeType = desired.NodeType
	applied.Kind = desired.Kind
	applied.Image = desired.Image
	applied.Entrypoint = desired.Entrypoint
	applied.Cmd = desired.Cmd
	applied.Exec = desired.Exec
	applied.Env = desired.Env
	applied.Binds = desired.Binds
	applied.Devices = desired.Devices
	applied.CapAdd = desired.CapAdd
	applied.ShmSize = desired.ShmSize
	applied.PortSet = desired.PortSet
	applied.User = desired.User
	applied.NetworkMode = desired.NetworkMode
	if desired.Runtime != "" {
		applied.Runtime = desired.Runtime
	}
	applied.CPU = desired.CPU
	applied.CPUSet = desired.CPUSet
	applied.Memory = desired.Memory
	applied.License = desired.License
	applied.Components = desired.Components

	return &applied
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

func (c *CLab) prepareApply(
	ctx context.Context,
	addedNodes []string,
	skipLabDirFileACLs bool,
) error {
	if _, err := c.prepareLabManagementNetwork(ctx); err != nil {
		return err
	}

	if err := c.prepareDeployArtifacts(ctx, skipLabDirFileACLs); err != nil {
		return err
	}

	for _, nodeName := range addedNodes {
		if err := c.Nodes[nodeName].PullImage(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (*CLab) removeApplyLinkEndpoints(ctx context.Context, links []clablinks.Link) error {
	for _, link := range links {
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			if err := clablinks.RemoveOwnedInterface(ctx, ep.GetNode(), ep.GetIfaceName()); err != nil {
				return err
			}
		}
	}

	return nil
}
