package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Deploy the given topology.
// skipcq: GO-R1005
func (c *CLab) Deploy( //nolint: funlen
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

	skipMgmt, err := c.prepareLabManagementNetwork(ctx)
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
		err = clablinks.DeployEndpoint(ctx, ep)
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

	if err := c.GenerateInventories(); err != nil {
		return nil, err
	}

	topoDataF, err := os.Create(c.TopoPaths.TopoExportFile())
	if err != nil {
		return nil, err
	}
	defer topoDataF.Close()

	if err := c.GenerateExports(ctx, topoDataF, options.exportTemplate); err != nil {
		return nil, err
	}

	// generate graph of the lab topology
	if options.graph {
		if err = c.GenerateDotGraph(ctx); err != nil {
			log.Error(err)
		}
	}

	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return nil, err
	}

	if !skipMgmt {
		log.Info("Adding host entries", "path", "/etc/hosts")

		err = c.appendHostsFileEntries(ctx)
		if err != nil {
			log.Errorf("failed to create hosts file: %v", err)
		}
	}

	log.Info("Adding SSH config for nodes", "path", c.TopoPaths.SSHConfigPath())

	err = c.addSSHConfig()
	if err != nil {
		log.Errorf("failed to create ssh config file: %v", err)
	}

	// save the state file for future applies
	if err = c.WriteState(); err != nil {
		log.Warnf("failed to write state file: %v", err)
	}

	return containers, nil
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
			if err := clablinks.RemoveOwnedInterface(ctx, ep.GetNode(), ep.GetIfaceName()); err != nil {
				return err
			}
		}

		for _, ep := range clablinks.ApplyRuntimeEndpoints(link) {
			if err := clablinks.DeployEndpoint(ctx, ep); err != nil {
				return fmt.Errorf("failed deploying link %s: %w", applyLinkName(link), err)
			}
		}

		if err := link.PostDeploy(ctx); err != nil {
			return fmt.Errorf("failed post-deploying link %s: %w", applyLinkName(link), err)
		}
	}

	return nil
}

func (c *CLab) postDeployApplyLinks(ctx context.Context, nodeNames []string) error {
	for _, nodeName := range nodeNames {
		node := c.Nodes[nodeName]

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
