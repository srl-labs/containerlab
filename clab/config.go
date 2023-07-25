// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pmorjan/kmod"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/nodes"
	clabRuntimes "github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

const (
	// prefix is used to distinct containerlab created files/dirs/containers.
	defaultPrefix = "clab"
	// a name of a docker network that nodes management interfaces connect to.
	dockerNetName     = "clab"
	dockerNetIPv4Addr = "172.20.20.0/24"
	dockerNetIPv6Addr = "2001:172:20:20::/64"
	// NSPath value assigned to host interfaces.
	hostNSPath = "__host"
	// veth link mtu.
	DefaultVethLinkMTU = 9500
	// containerlab's reserved OUI.
	ClabOUI = "aa:c1:ab"

	// clab specific topology variables.
	clabDirVar = "__clabDir__"
	nodeDirVar = "__clabNodeDir__"
)

// Config defines lab configuration as it is provided in the YAML file.
type Config struct {
	Name     string          `json:"name,omitempty"`
	Prefix   *string         `json:"prefix,omitempty"`
	Mgmt     *types.MgmtNet  `json:"mgmt,omitempty"`
	Topology *types.Topology `json:"topology,omitempty"`
	// the debug flag value as passed via cli
	// may be used by other packages to enable debug logging
	Debug bool `json:"debug"`
}

// ParseTopology parses the lab topology.
func (c *CLab) parseTopology() error {
	log.Infof("Parsing & checking topology file: %s", c.TopoPaths.TopologyFilenameBase())

	err := c.TopoPaths.SetLabDir(c.Config.Name)
	if err != nil {
		return err
	}

	if c.Config.Prefix == nil {
		c.Config.Prefix = new(string)
		*c.Config.Prefix = defaultPrefix
	}

	// initialize Nodes and Links variable
	c.Nodes = make(map[string]nodes.Node)
	c.Links = make(map[int]*types.Link)

	// initialize the Node information from the topology map
	nodeNames := make([]string, 0, len(c.Config.Topology.Nodes))
	for nodeName := range c.Config.Topology.Nodes {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	// collect node runtimes in a map[NodeName] -> RuntimeName
	nodeRuntimes := make(map[string]string)

	for nodeName, topologyNode := range c.Config.Topology.Nodes {
		// this case is when runtime was overridden at the node level
		if r := c.Config.Topology.GetNodeRuntime(nodeName); r != "" {
			nodeRuntimes[nodeName] = r
			continue
		}

		// this case if for non-default runtimes overriding the global default
		if r, ok := nodes.NonDefaultRuntimes[topologyNode.GetKind()]; ok {
			nodeRuntimes[nodeName] = r
			continue
		}

		// saving the global default runtime
		nodeRuntimes[nodeName] = c.globalRuntime
	}

	// initialize any extra runtimes
	for _, r := range nodeRuntimes {
		// this is the case for already init'ed runtimes
		if _, ok := c.Runtimes[r]; ok {
			continue
		}

		if rInit, ok := clabRuntimes.ContainerRuntimes[r]; ok {

			newRuntime := rInit()
			defaultConfig := c.Runtimes[c.globalRuntime].Config()
			err := newRuntime.Init(
				clabRuntimes.WithConfig(&defaultConfig),
			)
			if err != nil {
				return fmt.Errorf("failed to init the container runtime: %s", err)
			}

			c.Runtimes[r] = newRuntime
		}
	}

	for idx, nodeName := range nodeNames {
		err = c.NewNode(nodeName, nodeRuntimes[nodeName], c.Config.Topology.Nodes[nodeName], idx)
		if err != nil {
			return err
		}
	}
	for i, l := range c.Config.Topology.Links {
		// i represents the endpoint integer and l provide the link struct
		c.Links[i] = c.NewLink(l)
	}

	// set any containerlab defaults after we've parsed the input
	c.setDefaults()

	return nil
}

// NewNode initializes a new node object.
func (c *CLab) NewNode(nodeName, nodeRuntime string, nodeDef *types.NodeDefinition, idx int) error {
	nodeCfg, err := c.createNodeCfg(nodeName, nodeDef, idx)
	if err != nil {
		return err
	}

	// construct node
	n, err := c.Reg.NewNodeOfKind(nodeCfg.Kind)
	if err != nil {
		return fmt.Errorf("error constructing node %q: %v", nodeCfg.ShortName, err)
	}

	// Init
	err = n.Init(nodeCfg, nodes.WithRuntime(c.Runtimes[nodeRuntime]), nodes.WithMgmtNet(c.Config.Mgmt))
	if err != nil {
		log.Errorf("failed to initialize node %q: %v", nodeCfg.ShortName, err)
		return fmt.Errorf("failed to initialize node %q: %v", nodeCfg.ShortName, err)
	}

	c.Nodes[nodeName] = n

	c.addDefaultLabels(n)

	labelsToEnvVars(n.Config())

	return nil
}

func (c *CLab) createNodeCfg(nodeName string, nodeDef *types.NodeDefinition, idx int) (*types.NodeConfig, error) {
	// default longName follows $prefix-$lab-$nodeName pattern
	longName := fmt.Sprintf("%s-%s-%s", *c.Config.Prefix, c.Config.Name, nodeName)

	switch {
	// when prefix is an empty string longName will match shortName/nodeName
	case *c.Config.Prefix == "":
		longName = nodeName
	case *c.Config.Prefix == "__lab-name":
		longName = fmt.Sprintf("%s-%s", c.Config.Name, nodeName)
	}

	nodeCfg := &types.NodeConfig{
		ShortName:       nodeName, // just the node name as seen in the topo file
		LongName:        longName, // by default clab-$labName-$nodeName
		Fqdn:            strings.Join([]string{nodeName, c.Config.Name, "io"}, "."),
		LabDir:          c.TopoPaths.NodeDir(nodeName),
		Index:           idx,
		Group:           c.Config.Topology.GetNodeGroup(nodeName),
		Kind:            strings.ToLower(c.Config.Topology.GetNodeKind(nodeName)),
		NodeType:        c.Config.Topology.GetNodeType(nodeName),
		Position:        c.Config.Topology.GetNodePosition(nodeName),
		Image:           c.Config.Topology.GetNodeImage(nodeName),
		ImagePullPolicy: c.Config.Topology.GetNodeImagePullPolicy(nodeName),
		User:            c.Config.Topology.GetNodeUser(nodeName),
		Entrypoint:      c.Config.Topology.GetNodeEntrypoint(nodeName),
		Cmd:             c.Config.Topology.GetNodeCmd(nodeName),
		Exec:            c.Config.Topology.GetNodeExec(nodeName),
		Env:             c.Config.Topology.GetNodeEnv(nodeName),
		NetworkMode:     strings.ToLower(c.Config.Topology.GetNodeNetworkMode(nodeName)),
		MgmtIPv4Address: nodeDef.GetMgmtIPv4(),
		MgmtIPv6Address: nodeDef.GetMgmtIPv6(),
		Publish:         c.Config.Topology.GetNodePublish(nodeName),
		Sysctls:         c.Config.Topology.GetSysCtl(nodeName),
		Endpoints:       make([]types.Endpoint, 0),
		Sandbox:         c.Config.Topology.GetNodeSandbox(nodeName),
		Kernel:          c.Config.Topology.GetNodeKernel(nodeName),
		Runtime:         c.Config.Topology.GetNodeRuntime(nodeName),
		CPU:             c.Config.Topology.GetNodeCPU(nodeName),
		CPUSet:          c.Config.Topology.GetNodeCPUSet(nodeName),
		Memory:          c.Config.Topology.GetNodeMemory(nodeName),
		StartupDelay:    c.Config.Topology.GetNodeStartupDelay(nodeName),
		AutoRemove:      c.Config.Topology.GetNodeAutoRemove(nodeName),
		SANs:            c.Config.Topology.GetSANs(nodeName),
		Extras:          c.Config.Topology.GetNodeExtras(nodeName),
		WaitFor:         c.Config.Topology.GetWaitFor(nodeName),
		DNS:             c.Config.Topology.GetNodeDns(nodeName),
		Certificate:     c.Config.Topology.GetCertificateConfig(nodeName),
	}

	var err error

	// Load content of the EnvVarFiles
	envFileContent, err := utils.LoadEnvVarFiles(c.TopoPaths.TopologyFileDir(),
		c.Config.Topology.GetNodeEnvFiles(nodeName))
	if err != nil {
		return nil, err
	}
	// Merge EnvVarFiles content and the existing env variable
	nodeCfg.Env = utils.MergeStringMaps(envFileContent, nodeCfg.Env)

	log.Debugf("node config: %+v", nodeCfg)

	// process startup-config
	err = c.processStartupConfig(nodeCfg)
	if err != nil {
		return nil, err
	}

	nodeCfg.EnforceStartupConfig = c.Config.Topology.GetNodeEnforceStartupConfig(nodeCfg.ShortName)

	// initialize license field
	p, err := c.Config.Topology.GetNodeLicense(nodeCfg.ShortName)
	if err != nil {
		return nil, err
	}
	// resolve the lic path to an abs path
	nodeCfg.License = utils.ResolvePath(p, c.TopoPaths.TopologyFileDir())

	// initialize bind mounts
	binds, err := c.Config.Topology.GetNodeBinds(nodeName)
	if err != nil {
		return nil, err
	}
	err = c.resolveBindPaths(binds, nodeCfg.LabDir)
	if err != nil {
		return nil, err
	}
	nodeCfg.Binds = binds
	nodeCfg.PortSet, nodeCfg.PortBindings, err = c.Config.Topology.GetNodePorts(nodeName)
	if err != nil {
		return nil, err
	}

	nodeCfg.Labels = c.Config.Topology.GetNodeLabels(nodeCfg.ShortName)

	nodeCfg.Config = c.Config.Topology.GetNodeConfigDispatcher(nodeCfg.ShortName)

	return nodeCfg, nil
}

// processStartupConfig processes the raw path of the startup-config as it is defined in the topology file.
// It handles remote files, local files and embedded configs.
// Returns an absolute path to the startup-config file.
func (c *CLab) processStartupConfig(nodeCfg *types.NodeConfig) error {
	// process startup-config
	p, err := c.Config.Topology.GetNodeStartupConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}

	// embedded config is a config that is defined as a multi-line string in the topology file
	// it contains at least one newline
	isEmbeddedConfig := strings.Count(p, "\n") >= 1
	// downloadable config starts with http(s)://
	isDownloadableConfig := utils.IsHttpUri(p)

	if isEmbeddedConfig || isDownloadableConfig {
		// both embedded and downloadable configs are require clab tmp dir to be created
		tmpLoc := c.TopoPaths.ClabTmpDir()
		utils.CreateDirectory(tmpLoc, 0755)

		switch {
		case isEmbeddedConfig:
			log.Debugf("Node %q startup-config is an embedded config: %q", nodeCfg.ShortName, p)
			// for embedded config we create a file with the name embedded.partial.cfg
			// as embedded configs are meant to be partial configs
			absDestFile := c.TopoPaths.StartupConfigDownloadFileAbsPath(
				nodeCfg.ShortName, "embedded.partial.cfg")

			err = utils.CreateFile(absDestFile, p)
			if err != nil {
				return err
			}

			p = absDestFile

		case isDownloadableConfig:
			log.Debugf("Node %q startup-config is a downloadable config %q", nodeCfg.ShortName, p)
			// get file name from an URL
			fname := utils.FilenameForURL(p)

			// Deduce the absolute destination filename for the downloaded content
			absDestFile := c.TopoPaths.StartupConfigDownloadFileAbsPath(nodeCfg.ShortName, fname)

			log.Debugf("Fetching startup-config %q for node %q storing at %q", p, nodeCfg.ShortName, absDestFile)
			// download the file to tmp location
			err := utils.CopyFileContents(p, absDestFile, 0755)
			if err != nil {
				return err
			}

			// adjust the nodeconfig by pointing startup-config to the local downloaded file
			p = absDestFile
		}
	}
	// resolve the startup config path to an abs path
	nodeCfg.StartupConfig = utils.ResolvePath(p, c.TopoPaths.TopologyFileDir())

	return nil
}

// NewLink initializes a new link object from the link definition provided via topology file.
func (c *CLab) NewLink(l *types.LinkDefinition) *types.Link {
	if len(l.Endpoints) != 2 {
		log.Fatalf("endpoint %q has wrong syntax, unexpected number of items", l.Endpoints) // skipcq: RVV-A0003
	}

	mtu := l.MTU

	if mtu == 0 {
		mtu = DefaultVethLinkMTU
	}

	return &types.Link{
		A:      c.NewEndpoint(l.Endpoints[0]),
		B:      c.NewEndpoint(l.Endpoints[1]),
		MTU:    mtu,
		Labels: l.Labels,
		Vars:   l.Vars,
	}
}

// NewEndpoint initializes a new endpoint object.
func (c *CLab) NewEndpoint(e string) *types.Endpoint {
	// initialize a new endpoint
	endpoint := new(types.Endpoint)

	// split the string to get node name and endpoint name
	split := strings.Split(e, ":")
	if len(split) != 2 {
		log.Fatalf("endpoint %s has wrong syntax", e) // skipcq: GO-S0904, RVV-A0003
	}
	nName := split[0] // node name

	// initialize the endpoint name based on the split function
	endpoint.EndpointName = split[1] // endpoint name
	if len(endpoint.EndpointName) > 15 {
		log.Fatalf("interface '%s' name exceeds maximum length of 15 characters",
			endpoint.EndpointName) // skipcq: RVV-A0003
	}
	// generate unique MAC
	endpoint.MAC = utils.GenMac(ClabOUI)

	// search the node pointer for a node name referenced in endpoint section
	switch nName {
	// "host" is a special reference to host namespace
	// for which we create an special Node with kind "host"
	case "host":
		endpoint.Node = &types.NodeConfig{
			Kind:      "host",
			ShortName: "host",
			NSPath:    hostNSPath,
		}
	case "macvlan":
		endpoint.Node = &types.NodeConfig{
			Kind:      "macvlan",
			ShortName: "macvlan",
			NSPath:    hostNSPath,
		}
	// mgmt-net is a special reference to a bridge of the docker network
	// that is used as the management network
	case "mgmt-net":
		endpoint.Node = &types.NodeConfig{
			Kind:      "bridge",
			ShortName: "mgmt-net",
		}
	default:
		c.m.Lock()
		if n, ok := c.Nodes[nName]; ok {
			endpoint.Node = n.Config()
			n.Config().Endpoints = append(n.Config().Endpoints, *endpoint)
		}
		c.m.Unlock()
	}

	// stop the deployment if the matching node element was not found
	// "host" node name is an exception, it may exist without a matching node
	if endpoint.Node == nil {
		log.Fatalf("not all nodes are specified in the 'topology.nodes' section or the names don't match in the 'links.endpoints' section: %s", nName) // skipcq: GO-S0904, RVV-A0003
	}

	return endpoint
}

// CheckTopologyDefinition runs topology checks and returns any errors found.
// This function runs after topology file is parsed and all nodes/links are initialized.
func (c *CLab) CheckTopologyDefinition(ctx context.Context) error {
	var err error

	for _, node := range c.Nodes {
		err := node.CheckDeploymentConditions(ctx)
		if err != nil {
			return err
		}
	}
	if err = c.verifyLinks(); err != nil {
		return err
	}
	if err = c.verifyDuplicateAddresses(); err != nil {
		return err
	}
	if err = c.verifyRootNetnsInterfaceUniqueness(); err != nil {
		return err
	}
	if err = c.VerifyContainersUniqueness(ctx); err != nil {
		return err
	}
	if err = c.verifyHostIfaces(); err != nil {
		return err
	}
	return nil
}

// verifyLinks checks if all the endpoints in the links section of the topology file
// appear only once.
func (c *CLab) verifyLinks() error {
	endpoints := map[string]struct{}{}
	// dups accumulates duplicate links
	dups := []string{}
	for _, l := range c.Links {
		for _, e := range []*types.Endpoint{l.A, l.B} {
			e_string := e.String()
			if _, ok := endpoints[e_string]; ok {
				// macvlan interface can appear multiple times
				if strings.Contains(e_string, "macvlan") {
					continue
				}

				dups = append(dups, e_string)
			}
			endpoints[e_string] = struct{}{}
		}
	}
	if len(dups) != 0 {
		sort.Strings(dups) // sort for deterministic error message
		return fmt.Errorf("endpoints %q appeared more than once in the links section of the topology file", dups)
	}
	return nil
}

// LoadKernelModules loads containerlab-required kernel modules.
func (c *CLab) LoadKernelModules() error {
	modules := []string{"ip_tables", "ip6_tables"}

	for _, m := range modules {
		isLoaded, err := utils.IsKernelModuleLoaded(m)
		if err != nil {
			return err
		}

		if isLoaded {
			log.Debugf("kernel module %q is already loaded", m)

			continue
		}

		log.Debugf("kernel module %q is not loaded. Trying to load", m)
		// trying to load the kernel modules.
		km, err := kmod.New()
		if err != nil {
			log.Warnf("Unable to init module loader: %v. Skipping...", err)

			return nil
		}

		err = km.Load(m, "", 0)
		if err != nil {
			log.Warnf("Unable to load kernel module %q automatically %q", m, err)

			return nil
		}

		log.Debugf("kernel module %q loaded successfully", m)
	}

	return nil
}

// verifyDuplicateAddresses checks that every static IP address in the topology is unique.
func (c *CLab) verifyDuplicateAddresses() error {
	dupIps := map[string]struct{}{}
	for _, node := range c.Nodes {
		ips := []string{node.Config().MgmtIPv4Address, node.Config().MgmtIPv6Address}
		if ips[0] == "" && ips[1] == "" {
			continue
		}

		for _, ip := range ips {
			if _, exists := dupIps[ip]; !exists {
				if ip == "" {
					continue
				}
				dupIps[ip] = struct{}{}
			} else {
				return fmt.Errorf("management IP address %s appeared more than once in the topology file", ip)
			}
		}
	}

	return nil
}

// VerifyContainersUniqueness ensures that nodes defined in the topology do not have names of the existing containers
// additionally it checks that the lab name is unique and no containers are currently running with the same lab name label.
func (c *CLab) VerifyContainersUniqueness(ctx context.Context) error {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	containers, err := c.ListContainers(nctx, nil)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return nil
	}

	dups := []string{}
	for _, n := range c.Nodes {
		for _, cnt := range containers {
			if n.Config().LongName == cnt.Names[0] {
				dups = append(dups, n.Config().LongName)
			}
		}
	}
	if len(dups) != 0 {
		return fmt.Errorf("containers %q already exist. Add '--reconfigure' flag to the deploy command to first remove the containers and then deploy the lab", dups)
	}

	// check that none of the existing containers has a label that matches
	// the lab name of a currently deploying lab
	// this ensures lab uniqueness
	for _, cnt := range containers {
		if cnt.Labels[labels.Containerlab] == c.Config.Name {
			return fmt.Errorf("the '%s' lab has already been deployed. Destroy the lab before deploying a lab with the same name", c.Config.Name)
		}
	}

	return nil
}

// verifyHostIfaces ensures that host interfaces referenced in the topology
// do not exist already in the root namespace
// and ensure that nodes that are configured with host networking mode do not have any interfaces defined.
func (c *CLab) verifyHostIfaces() error {
	for _, l := range c.Links {
		for _, ep := range []*types.Endpoint{l.A, l.B} {
			if ep.Node.ShortName == "host" {
				if nl, _ := netlink.LinkByName(ep.EndpointName); nl != nil {
					return fmt.Errorf("host interface %s referenced in topology already exists", ep.EndpointName)
				}
			}
			if ep.Node.NetworkMode == "host" {
				return fmt.Errorf("node '%s' is defined with host network mode, it can't have any links. Remove '%s' node links from the topology definition",
					ep.Node.ShortName, ep.Node.ShortName)
			}
		}
	}
	return nil
}

// verifyRootNetnsInterfaceUniqueness ensures that interafaces that appear in the root ns (bridge, ovs-bridge and host)
// are uniquely defined in the topology file.
func (c *CLab) verifyRootNetnsInterfaceUniqueness() error {
	rootNsIfaces := map[string]struct{}{}
	for _, l := range c.Links {
		endpoints := [2]*types.Endpoint{l.A, l.B}
		for _, e := range endpoints {
			if e.Node.IsRootNamespaceBased {
				if _, ok := rootNsIfaces[e.EndpointName]; ok {
					return fmt.Errorf(`interface %s defined for node %s has already been used in other bridges, ovs-bridges or host interfaces.
					Make sure that nodes of these kinds use unique interface names`, e.EndpointName, e.Node.ShortName)
				}
				rootNsIfaces[e.EndpointName] = struct{}{}
			}
		}
	}
	return nil
}

// resolveBindPaths resolves the host paths in a bind string, such as /hostpath:/remotepath(:options) string
// it allows host path to have `~` and relative path to an absolute path
// the list of binds will be changed in place.
// if the host path doesn't exist, the error will be returned.
func (c *CLab) resolveBindPaths(binds []string, nodedir string) error {
	for i := range binds {
		// host path is a first element in a /hostpath:/remotepath(:options) string
		elems := strings.Split(binds[i], ":")

		// replace special variable
		r := strings.NewReplacer(clabDirVar, c.TopoPaths.TopologyLabDir(), nodeDirVar, nodedir)
		hp := r.Replace(elems[0])
		hp = utils.ResolvePath(hp, c.TopoPaths.TopologyFileDir())

		_, err := os.Stat(hp)
		if err != nil {
			// check if the hostpath mount has a reference to ansible-inventory.yml or topology-data.json
			// if that is the case, we do not emit an error on missing file, since these files
			// will be created by containerlab upon lab deployment
			if hp != c.TopoPaths.AnsibleInventoryFileAbsPath() &&
				hp != c.TopoPaths.TopoExportFile() {
				return fmt.Errorf("failed to verify bind path: %v", err)
			}
		}
		elems[0] = hp
		binds[i] = strings.Join(elems, ":")
	}

	return nil
}

// sets defaults after the topology has been parsed.
func (c *CLab) setDefaults() {
	for _, n := range c.Nodes {
		// Injecting the env var with expected number of links
		numLinks := map[string]string{
			types.CLAB_ENV_INTFS: fmt.Sprintf("%d", len(n.Config().Endpoints)),
		}
		n.Config().Env = utils.MergeStringMaps(n.Config().Env, numLinks)

	}
}

// returns nodeCfg.ShortName based on the provided containerName and labName.
func getShortName(labName string, labPrefix *string, containerName string) (string, error) {
	if *labPrefix == "" {
		return containerName, nil
	}

	result := strings.Split(containerName, "-"+labName+"-")
	if len(result) != 2 {
		return "", fmt.Errorf("failed to parse container name %q", containerName)
	}
	return result[1], nil
}

// HasKind returns true if kind k is found in the list of nodes.
func (c *CLab) HasKind(k string) bool {
	for _, n := range c.Nodes {
		if n.Config().Kind == k {
			log.Warn("found")
			return true
		}
	}

	return false
}

// addDefaultLabels adds default labels to node's config struct.
func (c *CLab) addDefaultLabels(n nodes.Node) {
	cfg := n.Config()
	if cfg.Labels == nil {
		cfg.Labels = map[string]string{}
	}

	cfg.Labels[labels.Containerlab] = c.Config.Name
	cfg.Labels[labels.NodeName] = cfg.ShortName
	cfg.Labels[labels.NodeKind] = cfg.Kind
	cfg.Labels[labels.NodeType] = cfg.NodeType
	cfg.Labels[labels.NodeGroup] = cfg.Group
	cfg.Labels[labels.NodeLabDir] = cfg.LabDir
	cfg.Labels[labels.TopoFile] = c.TopoPaths.TopologyFilenameAbsPath()
}

// labelsToEnvVars adds labels to env vars with CLAB_LABEL_ prefix added
// and labels value sanitized.
func labelsToEnvVars(n *types.NodeConfig) {
	if n.Env == nil {
		n.Env = map[string]string{}
	}

	for k, v := range n.Labels {
		// add the value to the node env with a prefixed and special chars cleaned up key
		n.Env["CLAB_LABEL_"+utils.ToEnvKey(k)] = v
	}
}
