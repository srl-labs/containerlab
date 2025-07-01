// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/cert"
	depMgr "github.com/srl-labs/containerlab/clab/dependency_manager"
	"github.com/srl-labs/containerlab/clab/exec"
	errs "github.com/srl-labs/containerlab/errors"
	clabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	_ "github.com/srl-labs/containerlab/runtime/all"
	"github.com/srl-labs/containerlab/runtime/docker"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/slices"
)

var ErrNodeNotFound = errors.New("node not found")

type CLab struct {
	Config    *Config `json:"config,omitempty"`
	TopoPaths *types.TopoPaths
	Nodes     map[string]nodes.Node `json:"nodes,omitempty"`
	Links     map[int]links.Link    `json:"links,omitempty"`
	Endpoints []links.Endpoint
	Runtimes  map[string]runtime.ContainerRuntime `json:"runtimes,omitempty"`
	// reg is a registry of node kinds
	Reg  *nodes.NodeRegistry
	Cert *cert.Cert
	// List of SSH public keys extracted from the ~/.ssh/authorized_keys file
	// and ~/.ssh/*.pub files.
	// The keys are used to enable key-based SSH access for the nodes.
	SSHPubKeys []ssh.PublicKey

	dependencyManager depMgr.DependencyManager
	m                 *sync.RWMutex
	timeout           time.Duration
	globalRuntimeName string
	// nodeFilter is a list of node names to be deployed,
	// names are provided exactly as they are listed in the topology file.
	nodeFilter []string
	// checkBindsPaths toggle enables or disables binds paths checks
	// when set to true, bind sources are verified to exist on the host.
	checkBindsPaths bool
	// customOwner is the user-specified owner label for the lab
	customOwner string
}

// WithLabOwner sets the owner label for all nodes in the lab.
// Only users in the clab_admins group can set a custom owner.
func WithLabOwner(owner string) ClabOption {
	return func(c *CLab) error {
		currentUser, err := user.Current()
		if err != nil {
			log.Warn("Failed to get current user when trying to set the custom lab owner", "error", err)
			return nil
		}

		if isClabAdmin, err := utils.UserInUnixGroup(currentUser.Username, "clab_admins"); err == nil && isClabAdmin {
			c.customOwner = owner
		} else if owner != "" {
			log.Warn("Only users in clab_admins group can set custom owner. Using current user as owner.")
		}
		return nil
	}
}

type ClabOption func(c *CLab) error

func WithTimeout(dur time.Duration) ClabOption {
	return func(c *CLab) error {
		if dur <= 0 {
			return errors.New("zero or negative timeouts are not allowed")
		}
		c.timeout = dur
		return nil
	}
}

// WithLabName sets the name of the lab
// to the provided string.
func WithLabName(n string) ClabOption {
	return func(c *CLab) error {
		c.Config.Name = n
		return nil
	}
}

// WithSkippedBindsPathsCheck skips the binds paths checks.
func WithSkippedBindsPathsCheck() ClabOption {
	return func(c *CLab) error {
		c.checkBindsPaths = false
		return nil
	}
}

// WithManagementNetworkName sets the name of the
// management network that is to be used.
func WithManagementNetworkName(n string) ClabOption {
	return func(c *CLab) error {
		c.Config.Mgmt.Network = n
		return nil
	}
}

// WithManagementIpv4Subnet defined the IPv4 subnet
// that will be used for the mgmt network.
func WithManagementIpv4Subnet(s string) ClabOption {
	return func(c *CLab) error {
		c.Config.Mgmt.IPv4Subnet = s
		return nil
	}
}

// WithManagementIpv6Subnet defined the IPv6 subnet
// that will be used for the mgmt network.
func WithManagementIpv6Subnet(s string) ClabOption {
	return func(c *CLab) error {
		c.Config.Mgmt.IPv6Subnet = s
		return nil
	}
}

// WithDependencyManager adds Dependency Manager.
func WithDependencyManager(dm depMgr.DependencyManager) ClabOption {
	return func(c *CLab) error {
		c.dependencyManager = dm
		return nil
	}
}

// WithDebug sets debug mode.
func WithDebug(debug bool) ClabOption {
	return func(c *CLab) error {
		c.Config.Debug = debug
		return nil
	}
}

// WithRuntime option sets a container runtime to be used by containerlab.
func WithRuntime(name string, rtconfig *runtime.RuntimeConfig) ClabOption {
	return func(c *CLab) error {
		name, rInit, err := RuntimeInitializer(name)
		if err != nil {
			return err
		}

		c.globalRuntimeName = name

		r := rInit()
		log.Debugf("Running runtime.Init with params %+v and %+v", rtconfig, c.Config.Mgmt)
		err = r.Init(
			runtime.WithConfig(rtconfig),
			runtime.WithMgmtNet(c.Config.Mgmt),
		)
		if err != nil {
			return fmt.Errorf("failed to init the container runtime: %v", err)
		}

		c.Runtimes[name] = r
		log.Debugf("initialized a runtime with params %+v", r)
		return nil
	}
}

// RuntimeInitializer returns a runtime initializer function for a provided runtime name.
func RuntimeInitializer(name string) (string, runtime.Initializer, error) {
	// define runtime name.
	// order of preference: cli flag -> env var -> default value of docker
	envN := os.Getenv("CLAB_RUNTIME")
	log.Debugf("env runtime var value is %v", envN)
	switch {
	case name != "":
	case envN != "":
		name = envN
	default:
		name = docker.RuntimeName
	}
	if rInit, ok := runtime.ContainerRuntimes[name]; ok {
		return name, rInit, nil
	}
	return name, nil, fmt.Errorf("unknown container runtime %q", name)
}

func WithKeepMgmtNet() ClabOption {
	return func(c *CLab) error {
		c.globalRuntime().WithKeepMgmtNet()
		return nil
	}
}

func WithTopoPath(path, varsFile string) ClabOption {
	return func(c *CLab) error {
		file, err := c.ProcessTopoPath(path)
		if err != nil {
			return err
		}
		if err := c.LoadTopologyFromFile(file, varsFile); err != nil {
			return fmt.Errorf("failed to read topology file: %v", err)
		}

		return c.initMgmtNetwork()
	}
}

// WithTopoFromLab loads the topology file path based on a running lab name.
// The lab name is used to look up the container labels of a running lab and
// derive the topology file location. It falls back to WithTopoPath once the
// topology path is discovered.
func WithTopoFromLab(labName string) ClabOption {
	return func(c *CLab) error {
		if labName == "" {
			return fmt.Errorf("lab name is required to derive topology path")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      clabels.Containerlab,
				Operator:   "=",
				Match:      labName,
			},
		}

		containers, err := c.globalRuntime().ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers for lab '%s': %w", labName, err)
		}

		if len(containers) == 0 {
			return fmt.Errorf("lab '%s' not found - no running containers", labName)
		}

		topoFile := containers[0].Labels[clabels.TopoFile]
		if topoFile == "" {
			return fmt.Errorf("could not determine topology file from container labels")
		}

		// Verify topology file exists and is accessible
		if !utils.FileOrDirExists(topoFile) {
			return fmt.Errorf("topology file '%s' referenced by lab '%s' does not exist or is not accessible",
				topoFile, labName)
		}

		log.Debugf("found topology file for lab %s: %s", labName, topoFile)

		return WithTopoPath(topoFile, "")(c)
	}
}

// ProcessTopoPath takes a topology path, which might be the path to a directory or a file
// or stdin or a URL (HTTP/HTTPS/S3) and returns the topology file name if found.
func (c *CLab) ProcessTopoPath(path string) (string, error) {
	var file string
	var err error

	switch {
	case path == "-" || path == "stdin":
		log.Debugf("interpreting topo %q as stdin", path)
		file, err = readFromStdin(c.TopoPaths.ClabTmpDir())
		if err != nil {
			return "", err
		}
	// if the path is not a local file and a URL, download the file and store it in the tmp dir
	case !utils.FileOrDirExists(path) && utils.IsHttpURL(path, true):
		log.Debugf("interpreting topo %q as remote URL", path)
		file, err = downloadTopoFile(path, c.TopoPaths.ClabTmpDir())
		if err != nil {
			return "", err
		}
	// if the path is an S3 URL, download the file and store it in the tmp dir
	case utils.IsS3URL(path):
		log.Debugf("interpreting topo %q as S3 URL", path)
		file, err = downloadTopoFile(path, c.TopoPaths.ClabTmpDir())
		if err != nil {
			return "", err
		}

	case path == "":
		return "", fmt.Errorf("provide a path to the clab topology file")

	default:
		log.Debugf("interpreting topo %q as file path", path)
		file, err = FindTopoFileByPath(path)
		if err != nil {
			return "", err
		}
	}
	return file, nil
}

// FindTopoFileByPath takes a topology path, which might be the path to a directory
// and returns the topology file name if found.
func FindTopoFileByPath(path string) (string, error) {
	finfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// by default we assume the path points to a clab file
	file := path

	// we might have gotten a dirname
	// lets try to find a single *.clab.y*ml
	if finfo.IsDir() {
		matches, err := filepath.Glob(filepath.Join(path, "*.clab.y*ml"))
		if err != nil {
			return "", err
		}

		switch len(matches) {
		case 1:
			// single file found, using it
			file = matches[0]
		case 0:
			// no files found
			return "", fmt.Errorf("no topology files found in directory %q", path)
		default:
			// multiple files found
			var filenames []string
			// extract just filename -> no path
			for _, match := range matches {
				filenames = append(filenames, filepath.Base(match))
			}

			return "", fmt.Errorf("found multiple topology definitions [ %s ] in a given directory %q. Provide the specific filename", strings.Join(filenames, ", "), path)
		}
	}

	return file, nil
}

// readFromStdin reads the topology file from stdin
// creates a temp file with topology contents
// and returns a path to the temp file.
func readFromStdin(tempDir string) (string, error) {
	tmpFile, err := os.CreateTemp(tempDir, "topo-*.clab.yml")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.ReadFrom(os.Stdin)
	if err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func downloadTopoFile(url, tempDir string) (string, error) {
	tmpFile, err := os.CreateTemp(tempDir, "topo-*.clab.yml")
	if err != nil {
		return "", err
	}

	err = utils.CopyFile(url, tmpFile.Name(), 0644)

	return tmpFile.Name(), err
}

// NewContainerlabFromTopologyFileOrLabName creates a containerlab instance using either a topology file path
// or a lab name. It returns the initialized CLab structure with the
// topology loaded.
func NewContainerlabFromTopologyFileOrLabName(ctx context.Context, topoPath, labName, varsFile, runtimeName string, debug bool, timeout time.Duration, graceful bool) (*CLab, error) {
	if topoPath == "" && labName == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory and no topology path or lab name provided: %w", err)
		}
		topoPath = cwd
	}

	opts := []ClabOption{
		WithTimeout(timeout),
		WithRuntime(runtimeName, &runtime.RuntimeConfig{
			Debug:            debug,
			Timeout:          timeout,
			GracefulShutdown: graceful,
		}),
		WithDebug(debug),
	}

	switch {
	case topoPath != "":
		opts = append(opts, WithTopoPath(topoPath, varsFile))
	case labName != "":
		opts = append(opts, WithTopoFromLab(labName))
	}

	return NewContainerLab(opts...)
}

// WithNodeFilter option sets a filter for nodes to be deployed.
// A filter is a list of node names to be deployed,
// names are provided exactly as they are listed in the topology file.
// Since this is altering the clab.config.Topology.[Nodes,Links] it must only
// be called after WithTopoFile.
func WithNodeFilter(nodeFilter []string) ClabOption {
	return func(c *CLab) error {
		return c.filterClabNodes(nodeFilter)
	}
}

func (c *CLab) filterClabNodes(nodeFilter []string) error {
	if len(nodeFilter) == 0 {
		return nil
	}

	c.nodeFilter = nodeFilter

	// ensure that the node filter is a subset of the nodes in the topology
	for _, n := range nodeFilter {
		if _, ok := c.Config.Topology.Nodes[n]; !ok {
			return fmt.Errorf("%w: node %q is not present in the topology", errs.ErrIncorrectInput, n)
		}
	}

	log.Infof("Applying node filter: %q", nodeFilter)

	// filter nodes
	for name := range c.Config.Topology.Nodes {
		if exists := slices.Contains(nodeFilter, name); !exists {
			log.Debugf("Excluding node %s", name)
			delete(c.Config.Topology.Nodes, name)
		}
	}

	return nil
}

// NewContainerLab function defines a new container lab.
func NewContainerLab(opts ...ClabOption) (*CLab, error) {
	c := &CLab{
		Config: &Config{
			Mgmt:     new(types.MgmtNet),
			Topology: types.NewTopology(),
		},
		m:               new(sync.RWMutex),
		Nodes:           make(map[string]nodes.Node),
		Links:           make(map[int]links.Link),
		Runtimes:        make(map[string]runtime.ContainerRuntime),
		Cert:            &cert.Cert{},
		checkBindsPaths: true,
	}

	// init a new NodeRegistry
	c.Reg = nodes.NewNodeRegistry()
	c.RegisterNodes()

	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}

	var err error
	if c.TopoPaths.TopologyFileIsSet() {
		err = c.parseTopology()
	}

	// Extract the host systems DNS servers and populate the
	// Nodes DNS Config with these if not specifically provided
	fileSystem := os.DirFS("/")
	if err := c.extractDNSServers(fileSystem); err != nil {
		return nil, err
	}

	return c, err
}

// initMgmtNetwork sets management network config.
func (c *CLab) initMgmtNetwork() error {
	log.Debugf("method initMgmtNetwork was called mgmt params %+v", c.Config.Mgmt)
	if c.Config.Mgmt.Network == "" {
		c.Config.Mgmt.Network = dockerNetName
	}

	if c.Config.Mgmt.IPv4Subnet == "" && c.Config.Mgmt.IPv6Subnet == "" {
		c.Config.Mgmt.IPv4Subnet = dockerNetIPv4Addr
		c.Config.Mgmt.IPv6Subnet = dockerNetIPv6Addr
	}

	// by default external access is enabled if not set by a user
	if c.Config.Mgmt.ExternalAccess == nil {
		c.Config.Mgmt.ExternalAccess = new(bool)
		*c.Config.Mgmt.ExternalAccess = true
	}

	log.Debugf("New mgmt params are %+v", c.Config.Mgmt)

	return nil
}

func (c *CLab) globalRuntime() runtime.ContainerRuntime {
	return c.Runtimes[c.globalRuntimeName]
}

// GetNode retrieve a node from the clab instance.
func (c *CLab) GetNode(name string) (nodes.Node, error) {
	if node, exists := c.Nodes[name]; exists {
		return node, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, name)
}

// createNodes schedules nodes creation and returns a waitgroup for all nodes
// with the exec collection created from the exec config of each node.
// The exec collection is returned to the caller to ensure that the execution log
// is printed after the nodes are created.
// Nodes interdependencies are created in this function.
func (c *CLab) createNodes(ctx context.Context, maxWorkers uint, skipPostDeploy bool) (*sync.WaitGroup, *exec.ExecCollection, error) {
	for _, node := range c.Nodes {
		c.dependencyManager.AddNode(node)
	}

	// nodes with static mgmt IP should be scheduled before the dynamic ones
	err := c.createStaticDynamicDependency()
	if err != nil {
		return nil, nil, err
	}

	// create user-defined node dependencies done with `wait-for` property of the deployment stage
	err = c.createWaitForDependency()
	if err != nil {
		return nil, nil, err
	}

	// create a set of dependencies, that makes the ignite nodes start one after the other
	err = c.createIgniteSerialDependency()
	if err != nil {
		return nil, nil, err
	}

	// make network namespace shared containers start in the right order
	c.createNamespaceSharingDependency()

	// Add possible additional dependencies here

	// make sure that there are no unresolvable dependencies, which would deadlock.
	err = c.dependencyManager.CheckAcyclicity()
	if err != nil {
		return nil, nil, err
	}

	// start scheduling
	NodesWg, execCollection := c.scheduleNodes(ctx, int(maxWorkers), skipPostDeploy)

	return NodesWg, execCollection, nil
}

// create a set of dependencies, that makes the ignite nodes start one after the other.
func (c *CLab) createIgniteSerialDependency() error {
	var prevIgniteNode *depMgr.DependencyNode
	// iterate through the nodes
	for _, n := range c.dependencyManager.GetNodes() {
		// find nodes that should run with IgniteRuntime
		if n.GetRuntime().GetName() == ignite.RuntimeName {
			if prevIgniteNode != nil {
				err := n.AddDepender(types.WaitForCreate, prevIgniteNode, types.WaitForCreate)
				if err != nil {
					return err
				}
			}
			prevIgniteNode = n
		}
	}
	return nil
}

// createNamespaceSharingDependency adds dependency between the containerlab nodes that share a common network namespace.
// When a node_a in the topology configured to be started in the netns of a node_b as such:
//
// node_a:
//
//	network-mode: container:node_b
//
// then node_a depends on node_b, and waits for node_b to be scheduled first.
func (c *CLab) createNamespaceSharingDependency() {
	for _, n := range c.dependencyManager.GetNodes() {
		nodeConfig := n.Config()
		netModeArr := strings.SplitN(nodeConfig.NetworkMode, ":", 2)
		if netModeArr[0] != "container" {
			// we only care about nodes with shared netns network-mode ("container:<CONTAINERNAME>")
			continue
		}

		referenceNodeName := netModeArr[1]
		referenceNode, err := c.dependencyManager.GetNode(referenceNodeName)
		if err != nil {
			log.Warnf("node %s referenced in namespace sharing not found in topology definition, considering it an external dependency.", referenceNodeName)
			continue
		}

		referenceNode.AddDepender(types.WaitForCreate, n, types.WaitForCreate)
	}
}

// createStaticDynamicDependency creates the dependencies between the nodes such that all nodes with dynamic mgmt IP
// are dependent on the nodes with static mgmt IP. This results in nodes with static mgmt IP to be scheduled before dynamic ones.
func (c *CLab) createStaticDynamicDependency() error {
	staticIPNodes := make(map[string]*depMgr.DependencyNode)
	dynIPNodes := make(map[string]*depMgr.DependencyNode)

	// divide the nodes into static and dynamic mgmt IP nodes.
	for name, n := range c.dependencyManager.GetNodes() {
		if n.Config().MgmtIPv4Address != "" || n.Config().MgmtIPv6Address != "" {
			staticIPNodes[name] = n
			continue
		}
		dynIPNodes[name] = n
	}

	// go through all the dynamic ip nodes
	for _, dynNode := range dynIPNodes {
		// and add their wait group to the the static nodes, while increasing the waitgroup
		for _, staticNode := range staticIPNodes {

			err := staticNode.AddDepender(types.WaitForCreate, dynNode, types.WaitForCreate)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// createWaitForDependency adds dependencies defined in the configuration via the wait-for field
// of the deployment stages.
func (c *CLab) createWaitForDependency() error {
	for _, dependerNode := range c.dependencyManager.GetNodes() {
		// add node's waitFor nodes to the dependency manager
		for dependerStage, waitForNodes := range dependerNode.Config().Stages.GetWaitFor() {
			for _, dependee := range waitForNodes {

				dependeeNode, err := c.dependencyManager.GetNode(dependee.Node)
				if err != nil {
					return fmt.Errorf("dependee node %s not found", dependee.Node)
				}

				err = dependeeNode.AddDepender(dependerStage, dependerNode, dependee.Stage)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// skipcq: GO-R1005
func (c *CLab) scheduleNodes(ctx context.Context, maxWorkers int, skipPostDeploy bool) (*sync.WaitGroup, *exec.ExecCollection) {
	concurrentChan := make(chan *depMgr.DependencyNode)

	execCollection := exec.NewExecCollection()

	workerFunc := func(i int, input chan *depMgr.DependencyNode, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case node, ok := <-input:
				if node == nil || !ok {
					log.Debugf("Worker %d terminating...", i)
					return
				}

				log.Debugf("Worker %d received node: %+v", i, node.Config())

				// Apply startup delay
				delay := node.Config().StartupDelay
				if delay > 0 {
					log.Infof("node %q is being delayed for %d seconds", node.Config().ShortName, delay)
					time.Sleep(time.Duration(delay) * time.Second)
				}

				// Pre-deploy stage
				err := node.PreDeploy(
					ctx,
					&nodes.PreDeployParams{
						Cert:         c.Cert,
						TopologyName: c.Config.Name,
						TopoPaths:    c.TopoPaths,
						SSHPubKeys:   c.SSHPubKeys,
					},
				)
				if err != nil {
					log.Errorf("failed pre-deploy stage for node %q: %v", node.Config().ShortName, err)
					continue
				}

				// Deploy
				err = node.Deploy(ctx, &nodes.DeployParams{})
				if err != nil {
					log.Errorf("failed deploy stage for node %q: %v", node.Config().ShortName, err)
					continue
				}

				// we need to update the node's state with runtime info (e.g. the mgmt net ip addresses)
				// before continuing with the post-deploy stage (for e.g. certificate creation)
				err = node.UpdateConfigWithRuntimeInfo(ctx)
				if err != nil {
					log.Errorf("failed to update node runtime information for node %s: %v", node.Config().ShortName, err)
				}

				node.Done(ctx, types.WaitForCreate)

				node.EnterStage(ctx, types.WaitForCreateLinks)

				// Deploy the Nodes link endpoints
				err = node.DeployEndpoints(ctx)
				if err != nil {
					log.Errorf("failed deploy links for node %q: %v", node.Config().ShortName, err)
					continue
				}

				node.Done(ctx, types.WaitForCreateLinks)

				// start config stage
				node.EnterStage(ctx, types.WaitForConfigure)

				// if postdeploy should be skipped we do not call it
				if !skipPostDeploy {
					err = node.PostDeploy(ctx, &nodes.PostDeployParams{Nodes: c.Nodes})
					if err != nil {
						log.Errorf("failed to run postdeploy task for node %s: %v", node.Config().ShortName, err)
					}
				}

				node.Done(ctx, types.WaitForConfigure)

				// run execs
				err = node.RunExecFromConfig(ctx, execCollection)
				if err != nil {
					log.Errorf("failed to run exec commands for %s: %v", node.GetShortName(), err)
				}

				// health state processing
				if node.MustWait(types.WaitForHealthy) {
					node.EnterStage(ctx, types.WaitForHealthy)
					// if there is a dependecy on the healthy state of this node, enter the checking procedure
					for {
						healthy, err := node.IsHealthy(ctx)
						if err != nil {
							log.Errorf("error checking for node health %v. Continuing deployment anyways", err)
							break
						}
						if healthy {
							log.Infof("node %q turned healthy, continuing", node.GetShortName())
							node.Done(ctx, types.WaitForHealthy)
							break
						}
						time.Sleep(time.Second)
					}
				}

				// exit state processing
				if node.MustWait(types.WaitForExit) {
					node.EnterStage(ctx, types.WaitForExit)
					// if there is a dependency on the healthy state of this node, enter the checking procedure
					for {
						status := node.GetContainerStatus(ctx)
						if status == runtime.Stopped {
							log.Infof("node %q stopped", node.GetShortName())
							node.Done(ctx, types.WaitForExit)
							break
						}
						time.Sleep(time.Second)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}

	numScheduledNodes := len(c.Nodes)
	if numScheduledNodes < maxWorkers {
		maxWorkers = numScheduledNodes
	}
	wg := new(sync.WaitGroup)

	// start concurrent workers
	wg.Add(int(maxWorkers))
	// it's safe to not check if all nodes are serial because in that case
	// maxWorkers will be 0
	for i := 0; i < maxWorkers; i++ {
		go workerFunc(i, concurrentChan, wg)
	}

	// Waitgroup protects the channel towards the workers of being closed too early
	workerFuncChWG := new(sync.WaitGroup)

	// schedule nodes via a go func to create links in parallel
	go func() {
		for _, dn := range c.dependencyManager.GetNodes() {
			workerFuncChWG.Add(1)
			// start a func for all the containers, then will wait for their own waitgroups
			// to be set to zero by their depending containers, then enqueue to the creation channel
			go func(node *depMgr.DependencyNode, dm depMgr.DependencyManager,
				workerChan chan<- *depMgr.DependencyNode, wfcwg *sync.WaitGroup,
			) {
				// we are entering the create stage here and not in the workerFunc
				// to avoid blocking the worker.
				// Block can happen when you have less workers than nodes
				// and nodes consume the worker but become stuck in waiting since
				// no more workers available to process other nodes that would unblock
				// the nodes stuck in waiting.
				// Entering the Create stage here would not consume a worker and let other nodes
				// to be scheduled.

				node.EnterStage(ctx, types.WaitForCreate)

				// wait for possible external dependencies
				c.waitForExternalNodeDependencies(ctx, node.Config().ShortName)
				// when all nodes that this node depends on are created, push it into the channel
				workerChan <- node
				// indicate we are done, such that only when all of these functions are done, the workerChan is being closed
				wfcwg.Done()
			}(dn, c.dependencyManager, concurrentChan, workerFuncChWG) // execute this function straight away
		}

		// Gate to make sure the channel is not closed before all the nodes made it though the channel
		workerFuncChWG.Wait()
		// close the channel and thereby terminate the workerFuncs
		close(concurrentChan)
	}()

	return wg, execCollection
}

// waitForExternalNodeDependencies makes nodes that have a reference to an external container network-namespace (network-mode: container:<NAME>)
// to wait until the referenced container is in started status.
// The wait time is 15 minutes by default.
func (c *CLab) waitForExternalNodeDependencies(ctx context.Context, nodeName string) {
	if _, exists := c.Nodes[nodeName]; !exists {
		log.Errorf("unable to find referenced node %q", nodeName)
		return
	}
	nodeConfig := c.Nodes[nodeName].Config()
	netModeArr := strings.SplitN(nodeConfig.NetworkMode, ":", 2)
	if netModeArr[0] != "container" {
		// we only care about nodes with NetMode "container:<CONTAINERNAME>"
		return
	}
	// the referenced container might be an external pre-existing or a container created also by the given clab topology.
	contName := netModeArr[1]

	// if the container does not exist in the list of container, it must be an external dependency
	// it can be ignored for internal processing so -> continue
	if _, exists := c.Nodes[contName]; exists {
		return
	}

	runtime.WaitForContainerRunning(ctx, c.Runtimes[c.globalRuntimeName], contName, nodeName)
}

func (c *CLab) deleteNodes(ctx context.Context, workers uint, serialNodes map[string]struct{}) {
	wg := new(sync.WaitGroup)

	concurrentChan := make(chan nodes.Node)
	serialChan := make(chan nodes.Node)

	workerFunc := func(i uint, input chan nodes.Node, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case n := <-input:
				if n == nil {
					log.Debugf("Worker %d terminating...", i)
					return
				}
				err := n.Delete(ctx)
				if err != nil {
					log.Errorf("could not remove container %q: %v", n.Config().LongName, err)
				}
			case <-ctx.Done():
				return
			}
		}
	}

	// start concurrent workers
	wg.Add(int(workers))
	for i := uint(0); i < workers; i++ {
		go workerFunc(i, concurrentChan, wg)
	}

	// start the serial worker
	if len(serialNodes) > 0 {
		wg.Add(1)
		go workerFunc(workers, serialChan, wg)
	}

	// send nodes to workers
	for _, n := range c.Nodes {
		if _, ok := serialNodes[n.Config().LongName]; ok {
			serialChan <- n
			continue
		}
		concurrentChan <- n
	}

	// close channel to terminate the workers
	close(concurrentChan)
	close(serialChan)

	// also call delete on the special nodes
	for _, n := range c.getSpecialLinkNodes() {
		err := n.Delete(ctx)
		if err != nil {
			log.Warn(err)
		}
	}

	wg.Wait()
}

// ListContainers lists all containers using provided filter.
func (c *CLab) ListContainers(ctx context.Context, filter []*types.GenericFilter) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer

	for _, r := range c.Runtimes {
		ctrs, err := r.ListContainers(ctx, filter)
		if err != nil {
			return containers, fmt.Errorf("could not list containers: %v", err)
		}
		containers = append(containers, ctrs...)
	}
	return containers, nil
}

// ListNodesContainers lists all containers based on the nodes stored in clab instance.
func (c *CLab) ListNodesContainers(ctx context.Context) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer

	for _, n := range c.Nodes {
		cts, err := n.GetContainers(ctx)
		if err != nil {
			return containers, fmt.Errorf("could not get container for node %s: %v", n.Config().LongName, err)
		}

		containers = append(containers, cts...)
	}

	return containers, nil
}

// ListNodesContainersIgnoreNotFound lists all containers based on the nodes stored in clab instance, ignoring errors for non found containers.
func (c *CLab) ListNodesContainersIgnoreNotFound(ctx context.Context) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer

	for _, n := range c.Nodes {
		cts, err := n.GetContainers(ctx)
		if err != nil {
			continue
		}
		containers = append(containers, cts...)
	}

	return containers, nil
}

// GetLinkNodes returns all CLab.Nodes nodes as links.Nodes enriched with the special nodes - host and mgmt-net.
// The CLab nodes are copied to a new map and thus clab.Node interface is converted to link.Node.
func (c *CLab) getLinkNodes() map[string]links.Node {
	// resolveNodes is a map of all nodes in the topology
	// that is artificially created to combat circular dependencies.
	// If no circ deps were in place we could've used c.Nodes map instead.
	// The map is used to resolve links between the nodes by passing it in the ResolveParams struct.
	resolveNodes := make(map[string]links.Node, len(c.Nodes))
	for k, v := range c.Nodes {
		resolveNodes[k] = v
	}

	// add the virtual host and mgmt-bridge nodes to the resolve nodes
	specialNodes := c.getSpecialLinkNodes()
	for _, n := range specialNodes {
		resolveNodes[n.GetShortName()] = n
	}

	return resolveNodes
}

// GetSpecialLinkNodes returns a map of special nodes that are used to resolve links.
// Special nodes are host and mgmt-bridge nodes that are not typically present in the topology file
// but are required to resolve links.
func (*CLab) getSpecialLinkNodes() map[string]links.Node {
	// add the virtual host and mgmt-bridge nodes to the resolve nodes
	specialNodes := map[string]links.Node{
		"host":     links.GetHostLinkNode(),
		"mgmt-net": links.GetMgmtBrLinkNode(),
	}

	return specialNodes
}

// ResolveLinks resolves raw links to the actual link types and stores them in the CLab.Links map.
func (c *CLab) ResolveLinks() error {
	resolveParams := &links.ResolveParams{
		Nodes:          c.getLinkNodes(),
		MgmtBridgeName: c.Config.Mgmt.Bridge,
		NodesFilter:    c.nodeFilter,
	}

	for i, l := range c.Config.Topology.Links {
		l, err := l.Link.Resolve(resolveParams)
		if err != nil {
			return err
		}

		// if the link is nil, it means that it was filtered out
		if l == nil {
			continue
		}

		c.Endpoints = append(c.Endpoints, l.GetEndpoints()...)
		c.Links[i] = l
	}

	return nil
}

// extractDNSServers extracts DNS servers from the resolv.conf files
// and populates the Nodes DNS Config with these if not specifically provided.
func (c *CLab) extractDNSServers(filesys fs.FS) error {
	// extract DNS servers from the relevant resolv.conf files
	DNSServers, err := utils.ExtractDNSServersFromResolvConf(filesys,
		[]string{"etc/resolv.conf", "run/systemd/resolve/resolv.conf"})
	if err != nil {
		return err
	}

	// no DNS Servers found, return
	if len(DNSServers) == 0 {
		return nil
	}

	// if no dns servers are explicitly configured,
	// we set the DNS servers that we've extracted.
	for _, n := range c.Nodes {
		config := n.Config()
		// skip nodes in container network mode since docker doesn't allow
		// setting dns config for them
		if strings.HasPrefix(config.NetworkMode, "container") {
			log.Debugf("Skipping DNS config for node %s as it is in container network mode", config.ShortName)
			continue
		}

		if config.DNS == nil {
			config.DNS = &types.DNSConfig{}
		}

		if n.Config().DNS.Servers == nil {
			n.Config().DNS.Servers = DNSServers
		}
	}

	return nil
}

// Deploy the given topology.
// skipcq: GO-R1005
func (c *CLab) Deploy(ctx context.Context, options *DeployOptions) ([]runtime.GenericContainer, error) {
	var err error

	err = c.ResolveLinks()
	if err != nil {
		return nil, err
	}

	log.Debugf("lab Conf: %+v", c.Config)
	if options.reconfigure {
		_ = c.Destroy(ctx, uint(len(c.Nodes)), true)
		log.Info("Removing directory", "path", c.TopoPaths.TopologyLabDir())
		if err := os.RemoveAll(c.TopoPaths.TopologyLabDir()); err != nil {
			return nil, err
		}
	}

	// create management network or use existing one
	if err = c.CreateNetwork(ctx); err != nil {
		return nil, err
	}

	err = links.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return nil, err
	}

	if err = c.checkTopologyDefinition(ctx); err != nil {
		return nil, err
	}

	if err = c.loadKernelModules(); err != nil {
		return nil, err
	}

	log.Info("Creating lab directory", "path", c.TopoPaths.TopologyLabDir())
	utils.CreateDirectory(c.TopoPaths.TopologyLabDir(), 0755)

	if !options.skipLabDirFileACLs {
		// adjust ACL for Labdir such that SUDO_UID Users will
		// also have access to lab directory files
		err = utils.AdjustFileACLs(c.TopoPaths.TopologyLabDir())
		if err != nil {
			log.Infof("unable to adjust Labdir file ACLs: %v", err)
		}
	}

	// create an empty ansible inventory file that will get populated later
	// we create it here first, so that bind mounts of ansible-inventory.yml file could work
	ansibleInvFPath := c.TopoPaths.AnsibleInventoryFileAbsPath()
	_, err = os.Create(ansibleInvFPath)
	if err != nil {
		return nil, err
	}

	// create an empty nornir simple inventory file that will get populated later
	// we create it here first, so that bind mounts of nornir-simple-inventory.yml file could work
	nornirSimpleInvFPath := c.TopoPaths.NornirSimpleInventoryFileAbsPath()
	_, err = os.Create(nornirSimpleInvFPath)
	if err != nil {
		return nil, err
	}

	// in an similar fashion, create an empty topology data file
	topoDataFPath := c.TopoPaths.TopoExportFile()
	topoDataF, err := os.Create(topoDataFPath)
	if err != nil {
		return nil, err
	}

	if err := c.certificateAuthoritySetup(); err != nil {
		return nil, err
	}

	c.SSHPubKeys, err = c.RetrieveSSHPubKeys()
	if err != nil {
		log.Warn(err)
	}

	if err := c.createAuthzKeysFile(); err != nil {
		return nil, err
	}

	// extraHosts holds host entries for nodes with static IPv4/6 addresses
	// these entries will be used by container runtime to populate /etc/hosts file
	extraHosts := make([]string, 0, len(c.Nodes))

	for _, n := range c.Nodes {
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

	for _, n := range c.Nodes {
		n.Config().ExtraHosts = extraHosts
	}

	nodesWg, execCollection, err := c.createNodes(ctx, options.maxWorkers, options.skipPostDeploy)
	if err != nil {
		return nil, err
	}

	// also call deploy on the special nodes endpoints (only host is required for the
	// vxlan stitched endpoints)
	eps := c.getSpecialLinkNodes()["host"].GetEndpoints()
	for _, ep := range eps {
		err = ep.Deploy(ctx)
		if err != nil {
			log.Warnf("failed deploying endpoint %s", ep)
		}
	}

	if nodesWg != nil {
		nodesWg.Wait()
	}

	// write to log
	execCollection.Log()

	if err := c.GenerateInventories(); err != nil {
		return nil, err
	}

	if err := c.GenerateExports(ctx, topoDataF, options.exportTemplate); err != nil {
		return nil, err
	}

	// generate graph of the lab topology
	if options.graph {
		if err = c.GenerateDotGraph(); err != nil {
			log.Error(err)
		}
	}

	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("Adding host entries", "path", "/etc/hosts")
	err = c.appendHostsFileEntries(ctx)
	if err != nil {
		log.Errorf("failed to create hosts file: %v", err)
	}

	log.Info("Adding SSH config for nodes", "path", c.TopoPaths.SSHConfigPath())
	err = c.addSSHConfig()
	if err != nil {
		log.Errorf("failed to create ssh config file: %v", err)
	}

	return containers, nil
}

// certificateAuthoritySetup sets up the certificate authority parameters.
func (c *CLab) certificateAuthoritySetup() error {
	// init the Cert storage and CA
	c.Cert.CertStorage = cert.NewLocalDirCertStorage(c.TopoPaths)
	c.Cert.CA = cert.NewCA()

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

		// if external CA cert and and key are set, propagate to topopaths
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
	caCertInput := &cert.CACSRInput{
		CommonName:   c.Config.Name + " lab CA",
		Country:      "US",
		Expiry:       validityDuration,
		Organization: "containerlab",
		KeySize:      keySize,
	}

	return c.LoadOrGenerateCA(caCertInput)
}

// Destroy the given topology.
func (c *CLab) Destroy(ctx context.Context, maxWorkers uint, keepMgmtNet bool) error {
	containers, err := c.ListNodesContainersIgnoreNotFound(ctx)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return nil
	}

	if maxWorkers == 0 {
		maxWorkers = uint(len(c.Nodes))
	}

	// a set of workers that do not support concurrency
	serialNodes := make(map[string]struct{})
	for _, n := range c.Nodes {
		if n.GetRuntime().GetName() == ignite.RuntimeName {
			serialNodes[n.Config().LongName] = struct{}{}
			// decreasing the num of maxWorkers as they are used for concurrent nodes
			maxWorkers = maxWorkers - 1
		}
	}

	// Serializing ignite workers due to busy device error
	if _, ok := c.Runtimes[ignite.RuntimeName]; ok {
		maxWorkers = 1
	}

	log.Info("Destroying lab", "name", c.Config.Name)
	c.deleteNodes(ctx, maxWorkers, serialNodes)

	log.Info("Removing host entries", "path", "/etc/hosts")
	err = c.DeleteEntriesFromHostsFile()
	if err != nil {
		return fmt.Errorf("error while trying to clean up the hosts file: %w", err)
	}

	log.Info("Removing SSH config", "path", c.TopoPaths.SSHConfigPath())
	err = c.RemoveSSHConfig(c.TopoPaths)
	if err != nil {
		log.Errorf("failed to remove ssh config file: %v", err)
	}

	// delete container network namespaces symlinks
	for _, node := range c.Nodes {
		err = node.DeleteNetnsSymlink()
		if err != nil {
			return fmt.Errorf("error while deleting netns symlinks: %w", err)
		}
	}

	// delete lab management network
	if c.Config.Mgmt.Network != "bridge" && !keepMgmtNet {
		log.Debugf("Calling DeleteNet method. *CLab.Config.Mgmt value is: %+v", c.Config.Mgmt)
		if err = c.globalRuntime().DeleteNet(ctx); err != nil {
			// do not log error message if deletion error simply says that such network doesn't exist
			if err.Error() != fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network) {
				log.Error(err)
			}
		}
	}
	return nil
}

// Exec execute commands on running topology nodes.
func (c *CLab) Exec(ctx context.Context, cmds []string, options *ExecOptions) (*exec.ExecCollection, error) {
	err := links.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return nil, err
	}

	cnts, err := c.ListContainers(ctx, options.filters)
	if err != nil {
		return nil, err
	}

	// make sure filter returned containers
	if len(cnts) == 0 {
		return nil, fmt.Errorf("filter did not match any containers")
	}

	// prepare the exec collection and the exec command
	resultCollection := exec.NewExecCollection()

	// build execs from the string input
	var execCmds []*exec.ExecCmd
	for _, execCmdStr := range cmds {
		execCmd, err := exec.NewExecCmdFromString(execCmdStr)
		if err != nil {
			return nil, err
		}
		execCmds = append(execCmds, execCmd)
	}

	// run the exec commands on all the containers matching the filter
	for _, cnt := range cnts {
		// iterate over the commands
		for _, execCmd := range execCmds {
			// execute the commands
			execResult, err := cnt.RunExec(ctx, execCmd)
			if err != nil {
				// skip nodes that do not support exec
				if err == exec.ErrRunExecNotSupported {
					continue
				}
			}

			resultCollection.Add(cnt.Names[0], execResult)
		}
	}

	return resultCollection, nil
}

// CheckConnectivity checks the connectivity to all container runtimes, returns an error if it encounters any, otherwise nil.
func (c *CLab) CheckConnectivity(ctx context.Context) error {
	for _, r := range c.Runtimes {
		err := r.CheckConnection(ctx)
		if err != nil {
			return fmt.Errorf("could not connect to container runtime: %v", err)
		}
	}

	return nil
}
