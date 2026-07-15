package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	clabcert "github.com/srl-labs/containerlab/cert"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// DeployResult holds the outcome of a Deploy call.
type DeployResult struct {
	Containers []clabruntime.GenericContainer
	// Apply summarizes the reconciliation of an already deployed lab or the dry-run plan;
	// it is nil when Deploy performed a fresh full deployment.
	Apply *ApplyResult
}

// Deploy converges the lab to the requested topology. A lab without runtime state is
// deployed from scratch, an already deployed lab is reconciled in place.
func (c *CLab) Deploy(
	ctx context.Context,
	options *DeployOptions,
) (*DeployResult, error) {
	if options == nil {
		var err error
		options, err = NewDeployOptions(0)
		if err != nil {
			return nil, err
		}
	}

	if options.reconfigure {
		if options.dryRun {
			return nil, fmt.Errorf(
				"dry-run cannot be combined with reconfigure: " +
					"reconfigure always destroys and redeploys the full lab",
			)
		}

		containers, err := c.deploy(ctx, options)
		if err != nil {
			return nil, err
		}

		return &DeployResult{Containers: containers}, nil
	}

	currentNodes, err := c.runtimeNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	initialDeploy, err := c.needsInitialDeploy(currentNodes)
	if err != nil {
		return nil, err
	}
	if initialDeploy {
		if len(c.nodeFilter) > 0 {
			if err := c.filterClabNodes(c.nodeFilter); err != nil {
				return nil, err
			}
		}
		if options.dryRun {
			return &DeployResult{
				Apply: &ApplyResult{
					DryRun:      true,
					DeployedLab: true,
					LabName:     c.Config.Name,
				},
			}, nil
		}

		containers, err := c.deploy(ctx, options)
		if err != nil {
			return nil, err
		}

		return &DeployResult{Containers: containers}, nil
	}

	if err := c.checkReconcileDeployOptions(options); err != nil {
		return nil, err
	}

	applyOptions := &ApplyOptions{
		dryRun:             options.dryRun,
		skipPostDeploy:     options.skipPostDeploy,
		skipLabDirFileACLs: options.skipLabDirFileACLs,
		graph:              options.graph,
		finalizeNoop:       true,
		maxWorkers:         options.maxWorkers,
		exportTemplate:     options.exportTemplate,
	}
	applyResult, err := c.apply(ctx, applyOptions, currentNodes)
	if err != nil {
		return nil, err
	}

	if options.dryRun {
		return &DeployResult{Apply: applyResult}, nil
	}

	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return nil, err
	}

	return &DeployResult{Containers: containers, Apply: applyResult}, nil
}

// checkReconcileDeployOptions rejects deploy options that only make sense for a fresh
// deployment when deploy is about to reconcile an already deployed lab.
func (c *CLab) checkReconcileDeployOptions(options *DeployOptions) error {
	if c.managementNetworkOverridden {
		return fmt.Errorf(
			"management network overrides require a fresh deployment, but lab %q is already "+
				"deployed; use --reconfigure to destroy and redeploy it",
			c.Config.Name,
		)
	}

	if options.restoreAll != "" || len(options.restoreNodeSnapshots) > 0 {
		return fmt.Errorf(
			"snapshot restore requires a fresh deployment, but lab %q is already deployed; "+
				"use --reconfigure to destroy and redeploy it",
			c.Config.Name,
		)
	}

	return nil
}

// deploy creates a lab that has no managed runtime nodes yet.
// skipcq: GO-R1005
func (c *CLab) deploy( //nolint: funlen
	ctx context.Context,
	options *DeployOptions,
) ([]clabruntime.GenericContainer, error) {
	var err error

	err = c.ResolveLinks()
	if err != nil {
		return nil, err
	}

	log.Debugf("lab Conf: %+v", c.Config)

	if options.reconfigure {
		_ = c.destroy(ctx, uint(len(c.Nodes)), true)
		log.Info("Removing directory", "path", c.TopoPaths.TopologyLabDir())

		if err := os.RemoveAll(c.TopoPaths.TopologyLabDir()); err != nil {
			return nil, err
		}
	}

	_, err = c.prepareLabManagementNetwork(ctx)
	if err != nil {
		return nil, err
	}

	if err := c.checkTopologyDefinition(ctx); err != nil {
		return nil, err
	}

	if err := c.prepareDeployArtifacts(ctx, options.skipLabDirFileACLs); err != nil {
		return nil, err
	}

	// Apply snapshot restore configuration to nodes
	if err := c.configureSnapshotRestore(options); err != nil {
		return nil, err
	}

	nodesWg, execCollection, nodeFailCh, err := c.createNodes(
		ctx,
		options.maxWorkers,
		options.skipPostDeploy,
	)
	if err != nil {
		return nil, err
	}

	if nodesWg != nil {
		nodesWg.Wait()
	}

	close(nodeFailCh)
	var nodeFailErrs []error
	for nodeErr := range nodeFailCh {
		nodeFailErrs = append(nodeFailErrs, nodeErr)
	}
	if len(nodeFailErrs) > 0 {
		return nil, fmt.Errorf(
			"deployment failed for one or more nodes: %w",
			errors.Join(nodeFailErrs...),
		)
	}

	// also call deploy on the special nodes endpoints (only host is required for the
	// vxlan stitched endpoints).
	// this must happen after all node workers have finished so that veth pairs created
	// as part of vxlan-stitch links are already deployed before the stitch TC rules are applied.
	eps := c.getSpecialLinkNodes()["host"].GetEndpoints()
	for _, ep := range eps {
		err = ep.Deploy(ctx)
		if err != nil {
			log.Warnf("failed deploying endpoint %s", ep)
		}
	}

	// Stitch links after node workers and host endpoints have finished.
	// The veth pair is already created by node workers and the VxLAN interface is created
	// by host endpoint deploy; Stitch applies the TC redirect rules to bridge them.
	for _, link := range c.Links {
		if err = link.PostDeploy(ctx); err != nil {
			log.Warnf("failed post-deploying link: %v", err)
		}
	}

	execCollection.Log()

	return c.finalize(ctx, options.exportTemplate, options.graph)
}

func (c *CLab) prepareLabManagementNetwork(ctx context.Context) (bool, error) {
	skipMgmt := c.skipMgmtNetwork()
	if !skipMgmt {
		if err := c.CreateNetwork(ctx); err != nil {
			return skipMgmt, err
		}
	}

	if err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge); err != nil {
		return skipMgmt, err
	}

	return skipMgmt, nil
}

func (c *CLab) prepareDeployArtifacts(ctx context.Context, skipLabDirFileACLs bool) error {
	if err := c.loadKernelModules(); err != nil {
		return err
	}

	c.prepareLabDirectory(skipLabDirFileACLs)

	if err := c.createPlaceholderArtifacts(); err != nil {
		return err
	}

	if err := c.setupNodeAuth(ctx); err != nil {
		return err
	}

	c.populateExtraHosts()

	return nil
}

func (c *CLab) prepareLabDirectory(skipFileACLs bool) {
	log.Info("Creating lab directory", "path", c.TopoPaths.TopologyLabDir())
	clabutils.CreateDirectory(c.TopoPaths.TopologyLabDir(), clabconstants.PermissionsDirDefault)

	if skipFileACLs {
		return
	}

	if err := clabutils.AdjustFileACLs(c.TopoPaths.TopologyLabDir()); err != nil {
		log.Infof("unable to adjust Labdir file ACLs: %v", err)
	}
}

func (c *CLab) createPlaceholderArtifacts() error {
	paths := []string{
		c.TopoPaths.AnsibleInventoryFileAbsPath(),
		c.TopoPaths.NornirSimpleInventoryFileAbsPath(),
		c.TopoPaths.TopoExportFile(),
	}

	for _, path := range paths {
		if err := createEmptyFile(path); err != nil {
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

func (c *CLab) setupNodeAuth(ctx context.Context) error {
	if err := c.certificateAuthoritySetup(); err != nil {
		return err
	}

	var err error
	c.SSHPubKeys, err = c.RetrieveSSHPubKeys(ctx)
	if err != nil {
		log.Warn(err)
	}

	return c.createAuthzKeysFile()
}

func (c *CLab) populateExtraHosts() {
	extraHosts := make([]string, 0, len(c.Nodes))

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		n := c.Nodes[nodeName]
		if n.Config().MgmtIPv4Address != "" {
			log.Debugf("Adding static ipv4 /etc/hosts entry for %s:%s",
				n.Config().ShortName, n.Config().MgmtIPv4Address)
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv4Address)
		}
		if n.Config().MgmtIPv6Address != "" {
			log.Debugf("Adding static ipv6 /etc/hosts entry for %s:%s",
				n.Config().ShortName, n.Config().MgmtIPv6Address)
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

// certificateAuthoritySetup sets up the certificate authority parameters.
func (c *CLab) certificateAuthoritySetup() error {
	// init the Cert storage and CA
	c.Cert.CertStorage = clabcert.NewLocalDirCertStorage(c.TopoPaths)
	c.Cert.CA = clabcert.NewCA()

	s := c.Config.Settings

	// Set defaults for the CA parameters
	keySize := 2048
	validityDuration := time.Until(time.Now().AddDate(1, 0, 0)) // 1 year as default

	// check that Settings.CertificateAuthority exists.
	if s != nil && s.CertificateAuthority != nil {
		// if ValidityDuration is set use the value
		if s.CertificateAuthority.ValidityDuration != 0 {
			validityDuration = s.CertificateAuthority.ValidityDuration
		}

		// if KeyLength is set use the value
		if s.CertificateAuthority.KeySize != 0 {
			keySize = s.CertificateAuthority.KeySize
		}

		// if external CA cert and key are set, propagate to topopaths
		extCACert := s.CertificateAuthority.Cert
		extCAKey := s.CertificateAuthority.Key

		// override external ca and key from env vars
		if v := os.Getenv("CLAB_CA_KEY_FILE"); v != "" {
			extCAKey = v
		}

		if v := os.Getenv("CLAB_CA_CERT_FILE"); v != "" {
			extCACert = v
		}

		if extCACert != "" && extCAKey != "" {
			err := c.TopoPaths.SetExternalCaFiles(extCACert, extCAKey)
			if err != nil {
				return err
			}
		}
	}

	// define the attributes used to generate the CA Cert
	caCertInput := &clabcert.CACSRInput{
		CommonName:   c.Config.Name + " lab CA",
		Country:      "US",
		Expiry:       validityDuration,
		Organization: "containerlab",
		KeySize:      keySize,
	}

	return c.LoadOrGenerateCA(caCertInput)
}

// createNodes schedules nodes creation and returns a waitgroup for all nodes
// with the exec collection created from the exec config of each node.
// The exec collection is returned to the caller to ensure that the execution log
// is printed after the nodes are created.
// Nodes interdependencies are created in this function.
func (c *CLab) createNodes(
	ctx context.Context,
	maxWorkers uint,
	skipPostDeploy bool,
) (*sync.WaitGroup, *clabexec.ExecCollection, chan error, error) {
	for _, node := range c.Nodes {
		c.dependencyManager.AddNode(node)
	}

	// nodes with static mgmt IP should be scheduled before the dynamic ones
	err := c.createStaticDynamicDependency()
	if err != nil {
		return nil, nil, nil, err
	}

	// create user-defined node dependencies done with `wait-for` property of the deployment stage
	err = c.createWaitForDependency()
	if err != nil {
		return nil, nil, nil, err
	}

	// make network namespace shared containers start in the right order
	c.createNamespaceSharingDependency()

	// Add possible additional dependencies here

	// make sure that there are no unresolvable dependencies, which would deadlock.
	err = c.dependencyManager.CheckAcyclicity()
	if err != nil {
		return nil, nil, nil, err
	}

	// Buffered so workers never block reporting a failure (at most one failure path per node).
	// With zero nodes, nothing sends on this channel; an empty buffer is fine.
	nodeFailCh := make(chan error, len(c.Nodes))

	// start scheduling
	NodesWg, execCollection := c.scheduleNodes(ctx, int(maxWorkers), skipPostDeploy, nodeFailCh)

	return NodesWg, execCollection, nodeFailCh, nil
}

// DeployNodes runs the node creation phase for the selected nodes.
func (c *CLab) DeployNodes(
	ctx context.Context,
	nodeNames []string,
	maxWorkers uint,
) error {
	if err := c.deployNodesUntilRunning(ctx, nodeNames, maxWorkers); err != nil {
		return err
	}
	return c.waitNodesHealthy(ctx, nodeNames)
}

// deployNodesUntilRunning creates dependency batches without waiting for
// healthchecks that may require topology interfaces. Apply attaches or restores
// those interfaces before calling waitNodesHealthy.
func (c *CLab) deployNodesUntilRunning(
	ctx context.Context,
	nodeNames []string,
	maxWorkers uint,
) error {
	if len(nodeNames) == 0 {
		return nil
	}
	batches, err := c.applyDeployBatches(nodeNames)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := c.deployNodeBatch(ctx, batch, maxWorkers); err != nil {
			return err
		}
		for _, nodeName := range batch {
			node := c.Nodes[nodeName]
			if err := c.waitNodeRunning(ctx, node); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CLab) waitNodesHealthy(ctx context.Context, nodeNames []string) error {
	for _, nodeName := range nodeNames {
		node, exists := c.Nodes[nodeName]
		if !exists {
			return fmt.Errorf("node %q not found", nodeName)
		}
		if err := c.waitNodeHealthyIfAvailable(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLab) deployNodeBatch(
	ctx context.Context,
	nodeNames []string,
	maxWorkers uint,
) error {

	if maxWorkers == 0 || int(maxWorkers) > len(nodeNames) {
		maxWorkers = uint(len(nodeNames))
	}
	for _, nodeName := range nodeNames {
		if _, exists := c.Nodes[nodeName]; !exists {
			return fmt.Errorf("node %q not found", nodeName)
		}
	}

	input := make(chan string)
	errCh := make(chan error, len(nodeNames))

	for range maxWorkers {
		go func() {
			for nodeName := range input {
				log.Info("Creating node", "node", nodeName)
				errCh <- c.deployNode(ctx, c.Nodes[nodeName])
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

func (c *CLab) applyDeployBatches(nodeNames []string) ([][]string, error) {
	pending := make(map[string]map[string]struct{}, len(nodeNames))
	deploySet := make(map[string]struct{}, len(nodeNames))
	for _, nodeName := range nodeNames {
		deploySet[nodeName] = struct{}{}
	}
	for nodeName := range deploySet {
		dependencies := map[string]struct{}{}
		node := c.Nodes[nodeName]
		if provider, shared := c.applySharedNetNSProvider(node); shared {
			if _, deploying := deploySet[provider]; deploying {
				dependencies[provider] = struct{}{}
			}
		}
		if node != nil && node.Config() != nil && node.Config().Stages != nil {
			for _, waitForList := range node.Config().Stages.GetWaitFor() {
				for _, waitFor := range waitForList {
					if waitFor == nil {
						continue
					}
					if _, deploying := deploySet[waitFor.Node]; deploying {
						dependencies[waitFor.Node] = struct{}{}
					}
				}
			}
		}
		pending[nodeName] = dependencies
	}

	var batches [][]string
	completed := map[string]struct{}{}
	for len(pending) > 0 {
		var ready []string
		for nodeName, dependencies := range pending {
			allComplete := true
			for dependency := range dependencies {
				if _, complete := completed[dependency]; !complete {
					allComplete = false
					break
				}
			}
			if allComplete {
				ready = append(ready, nodeName)
			}
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf(
				"apply node dependencies contain a cycle among %v",
				sortedStringSet(deploySet),
			)
		}
		sort.Strings(ready)
		batches = append(batches, ready)
		for _, nodeName := range ready {
			delete(pending, nodeName)
			completed[nodeName] = struct{}{}
		}
	}

	return batches, nil
}

func (c *CLab) deployNode(ctx context.Context, node clabnodes.Node) error {
	nodeName := node.GetShortName()
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

// DeployLinks deploys selected links through their endpoints and runs their post-deploy hooks.
func (c *CLab) DeployLinks(ctx context.Context, links []clablinks.Link) error {
	return c.deployLinks(ctx, links, nil)
}

func (c *CLab) deployLinks(
	ctx context.Context,
	links []clablinks.Link,
	additionalNodeNames []string,
) error {
	touchedNodes := make(map[string]struct{}, len(additionalNodeNames))
	for _, nodeName := range additionalNodeNames {
		touchedNodes[nodeName] = struct{}{}
	}
	for _, link := range links {
		log.Info("Creating link", "link", applyLinkName(link))

		for _, ep := range clablinks.RuntimeEndpoints(link) {
			if err := ep.Deploy(ctx); err != nil {
				return fmt.Errorf("failed deploying link %s: %w", applyLinkName(link), err)
			}
			touchedNodes[ep.GetNode().GetShortName()] = struct{}{}
		}

		if err := link.PostDeploy(ctx); err != nil {
			return fmt.Errorf("failed post-deploying link %s: %w", applyLinkName(link), err)
		}
	}

	for _, nodeName := range sortedStringSet(touchedNodes) {
		node, exists := c.Nodes[nodeName]
		if !exists {
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

func (c *CLab) finalize(
	ctx context.Context,
	exportTemplate string,
	graph bool,
) ([]clabruntime.GenericContainer, error) {
	if err := c.GenerateInventories(); err != nil {
		return nil, err
	}

	topoDataF, err := os.Create(c.TopoPaths.TopoExportFile())
	if err != nil {
		return nil, err
	}
	defer topoDataF.Close()

	if err := c.GenerateExports(ctx, topoDataF, exportTemplate); err != nil {
		return nil, err
	}

	if graph {
		if err := c.GenerateDotGraph(ctx); err != nil {
			log.Error(err)
		}
	}

	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return nil, err
	}

	if !c.skipMgmtNetwork() {
		log.Info("Adding host entries", "path", "/etc/hosts")
		if err := c.appendHostsFileEntries(ctx); err != nil {
			log.Errorf("failed to create hosts file: %v", err)
		}
	}

	log.Info("Adding SSH config for nodes", "path", c.TopoPaths.SSHConfigPath())
	if err := c.addSSHConfig(); err != nil {
		log.Errorf("failed to create ssh config file: %v", err)
	}

	c.writeState()

	return containers, nil
}

func (c *CLab) writeState() {
	if err := c.WriteState(); err != nil {
		log.Warnf("failed to write state file: %v", err)
	}
}

// configureSnapshotRestore configures nodes for snapshot restoration.
// It resolves snapshot files for each node and adds the necessary volume mounts
// and environment variables to restore from snapshots.
func (c *CLab) configureSnapshotRestore(options *DeployOptions) error {
	// Build restore map from per-node specifications
	restoreMap := make(map[string]string)
	for _, mapping := range options.restoreNodeSnapshots {
		parts := strings.SplitN(mapping, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid restore format: %s (expected: node=path)", mapping)
		}
		restoreMap[parts[0]] = parts[1]
	}

	// Track statistics for logging
	var restoredCount, freshCount int

	// Configure each node
	for _, node := range c.Nodes {
		nodeName := node.Config().ShortName

		// Resolve snapshot path for this node
		snapshotPath, shouldRestore := resolveNodeSnapshot(nodeName, restoreMap, options.restoreAll)

		if shouldRestore {
			// Validate snapshot file exists
			if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
				return fmt.Errorf("snapshot file not found for node %s: %s", nodeName, snapshotPath)
			}

			// Get absolute path for bind mount
			absPath, err := filepath.Abs(snapshotPath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for %s: %w", snapshotPath, err)
			}

			// Add volume mount for snapshot (read-only)
			node.Config().Binds = append(node.Config().Binds,
				fmt.Sprintf("%s:/snapshot.tar:ro", absPath))

			// Add restore environment variable
			if node.Config().Env == nil {
				node.Config().Env = make(map[string]string)
			}
			node.Config().Env["RESTORE_SNAPSHOT"] = "1"

			log.Infof("Node %s will restore from: %s", nodeName, snapshotPath)
			restoredCount++
		} else {
			freshCount++
		}
	}

	// Log deployment mode summary if any restores are configured
	if restoredCount > 0 {
		log.Infof("Deploying %d nodes from snapshots, %d nodes fresh", restoredCount, freshCount)
	}

	return nil
}

// resolveNodeSnapshot determines the snapshot file path for a given node.
// Priority: per-node override > restore-all directory > none.
func resolveNodeSnapshot(
	nodeName string,
	restoreMap map[string]string,
	restoreAll string,
) (string, bool) {
	// 1. Check per-node overrides first (highest priority)
	if snapshotPath, ok := restoreMap[nodeName]; ok {
		return snapshotPath, true
	}

	// 2. Check restore-all directory
	if restoreAll != "" {
		// Look for {nodename}.tar in directory
		snapshotPath := filepath.Join(restoreAll, nodeName+".tar")
		if _, err := os.Stat(snapshotPath); err == nil {
			return snapshotPath, true
		}

		// Not found - this is OK, just deploy normally
		log.Debugf("No snapshot found for node %s in %s, deploying normally", nodeName, restoreAll)
		return "", false
	}

	// 3. No restore specified
	return "", false
}
