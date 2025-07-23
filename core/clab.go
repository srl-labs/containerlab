// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/cert"
	depMgr "github.com/srl-labs/containerlab/core/dependency_manager"
	containerlaberrors "github.com/srl-labs/containerlab/errors"
	"github.com/srl-labs/containerlab/exec"
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

// NewContainerlabFromTopologyFileOrLabName creates a containerlab instance using either a topology file path
// or a lab name. It returns the initialized CLab structure with the
// topology loaded.
func NewContainerlabFromTopologyFileOrLabName(ctx context.Context,
	topoPath, labName, varsFile, runtimeName string, debug bool, timeout time.Duration, graceful bool,
) (*CLab, error) {
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

// RuntimeInitializer returns a runtime initializer function for a provided runtime name.
// Order of preference: cli flag -> env var -> default value of docker.
func RuntimeInitializer(name string) (string, runtime.Initializer, error) {
	envN := os.Getenv("CLAB_RUNTIME")
	log.Debugf("env runtime var value is %v", envN)

	switch {
	case name != "":
	case envN != "":
		name = envN
	default:
		name = docker.RuntimeName
	}

	runtimeInitializer, ok := runtime.ContainerRuntimes[name]
	if !ok {
		return name, nil, fmt.Errorf("unknown container runtime %q", name)
	}

	return name, runtimeInitializer, nil
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

func (c *CLab) filterClabNodes(nodeFilter []string) error {
	if len(nodeFilter) == 0 {
		return nil
	}

	c.nodeFilter = nodeFilter

	// ensure that the node filter is a subset of the nodes in the topology
	for _, n := range nodeFilter {
		if _, ok := c.Config.Topology.Nodes[n]; !ok {
			return fmt.Errorf("%w: node %q is not present in the topology",
				containerlaberrors.ErrIncorrectInput, n)
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
				err = node.Deploy(ctx, &nodes.DeployParams{Nodes: c.Nodes})
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
	wg.Add(maxWorkers)
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
			go func(node *depMgr.DependencyNode, _ depMgr.DependencyManager,
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
