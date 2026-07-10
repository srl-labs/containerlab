package core

import (
	"context"
	"fmt"

	clablinks "github.com/srl-labs/containerlab/links"
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
	// NodeChangeReasons explains per node why apply restarts or recreates it,
	// e.g. "added link" or "config drift: image".
	NodeChangeReasons map[string]string
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

	if options.dryRun {
		result := applyResultFromPlan(plan)
		result.DryRun = true
		return result, nil
	}

	if err := c.reconcileNodes(ctx, plan); err != nil {
		return nil, err
	}

	if plan.empty() {
		if options.finalizeNoop {
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

	if err := c.DeployNodes(ctx, deployNodeNames, options.maxWorkers); err != nil {
		return nil, err
	}

	if err := c.restoreRecreatedNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.startStoppedNodes(ctx, plan); err != nil {
		return nil, err
	}

	if err := c.removeApplyLinkEndpoints(ctx, plan.addedLinks); err != nil {
		return nil, err
	}

	if err := c.deployLinks(ctx, plan.addedLinks, deployNodeNames); err != nil {
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

	if _, err := c.finalize(ctx, options.exportTemplate, options.graph); err != nil {
		return nil, err
	}

	return applyResultFromPlan(plan), nil
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
