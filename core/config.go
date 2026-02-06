// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	gogitplumbing "github.com/go-git/go-git/v5/plumbing"

	"github.com/charmbracelet/log"
	"github.com/pmorjan/kmod"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	// prefix is used to distinct containerlab created files/dirs/containers.
	defaultPrefix = "clab"
	// a name of a docker network that nodes management interfaces connect to.
	dockerNetName     = "clab"
	dockerNetIPv4Addr = "172.20.20.0/24"
	dockerNetIPv6Addr = "3fff:172:20:20::/64"

	// DefaultVethLinkMTU is the veth link mtu.
	DefaultVethLinkMTU = 9500

	// clab specific topology variables.
	clabDirVar  = "__clabDir__"
	nodeDirVar  = "__clabNodeDir__"
	nodeNameVar = "__clabNodeName__"

	// clab name specific variables
	gitBranchVar = "__gitBranch__"
	gitHashVar   = "__gitHash__"

	// clab validation variables
	containerNamePattern = "^[a-zA-Z0-9][a-zA-Z0-9-._]+$"
	dnsIncompatibleChars = "._"
	maxNameLength        = 60
)

var containerNamePatternRe = regexp.MustCompile(containerNamePattern)

// Config defines lab configuration as it is provided in the YAML file.
type Config struct {
	Name     string              `json:"name,omitempty"`
	Prefix   *string             `json:"prefix,omitempty"`
	Mgmt     *clabtypes.MgmtNet  `json:"mgmt,omitempty"`
	Settings *clabtypes.Settings `json:"settings,omitempty"`
	Topology *clabtypes.Topology `json:"topology,omitempty"`
	// the debug flag value as passed via cli
	// may be used by other packages to enable debug logging
	Debug bool `json:"debug"`
}

// ParseTopology parses the lab topology.
func (c *CLab) parseTopology() error {
	log.Info("Parsing & checking topology", "file", c.TopoPaths.TopologyFilenameBase())

	if strings.Contains(c.Config.Name, gitBranchVar) || strings.Contains(c.Config.Name, gitHashVar) {
		r := c.magicTopoNameReplacer()
		oldName := c.Config.Name
		c.Config.Name = r.Replace(c.Config.Name)
		log.Debugf("Topology name contains Git variables, substituted topology name: %q -> %q", oldName, c.Config.Name)
	}

	err := c.TopoPaths.SetLabDirByPrefix(c.Config.Name)
	if err != nil {
		return err
	}

	if c.Config.Prefix == nil {
		c.Config.Prefix = new(string)
		*c.Config.Prefix = defaultPrefix
	}

	// initialize Nodes and Links variable
	c.Nodes = make(map[string]clabnodes.Node)
	c.Links = make(map[int]clablinks.Link)

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
		if topologyNode != nil {
			if r, ok := clabnodes.NonDefaultRuntimes[topologyNode.Kind]; ok {
				nodeRuntimes[nodeName] = r
				continue
			}
		}

		// saving the global default runtime
		nodeRuntimes[nodeName] = c.globalRuntimeName
	}

	// initialize any extra runtimes
	for _, r := range nodeRuntimes {
		// this is the case for already init'ed runtimes
		if _, ok := c.Runtimes[r]; ok {
			continue
		}

		if rInit, ok := clabruntime.ContainerRuntimes[r]; ok {
			newRuntime := rInit()
			defaultConfig := c.Runtimes[c.globalRuntimeName].Config()

			err := newRuntime.Init(
				clabruntime.WithConfig(&defaultConfig),
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

	return nil
}

// NewNode initializes a new node object.
func (c *CLab) NewNode(
	nodeName,
	nodeRuntime string,
	nodeDef *clabtypes.NodeDefinition,
	idx int,
) error {
	nodeCfg, err := c.createNodeCfg(nodeName, nodeDef, idx)
	if err != nil {
		return err
	}

	// construct node
	n, err := c.Reg.NewNodeOfKind(nodeCfg.Kind)
	if err != nil {
		return fmt.Errorf("error constructing node %q: %v", nodeCfg.ShortName, err)
	}

	// adding default labels to the node config
	// so that the labels are present in the node config
	// and can be copied to the child components
	// at the time of the node init
	c.addDefaultLabels(nodeCfg)
	labelsToEnvVars(nodeCfg)

	// Init
	err = n.Init(nodeCfg, clabnodes.WithRuntime(c.Runtimes[nodeRuntime]),
		clabnodes.WithMgmtNet(c.Config.Mgmt))
	if err != nil {
		log.Errorf("failed to initialize node %q: %v", nodeCfg.ShortName, err)

		return fmt.Errorf("failed to initialize node %q: %v", nodeCfg.ShortName, err)
	}

	c.Nodes[nodeName] = n
	// adding default labels 2nd time in case node init
	// overwrote original values for the default labels
	c.addDefaultLabels(n.Config())
	labelsToEnvVars(n.Config())

	return nil
}

func (c *CLab) createNodeCfg( //nolint: funlen
	nodeName string,
	nodeDef *clabtypes.NodeDefinition,
	idx int,
) (*clabtypes.NodeConfig, error) {
	// default longName follows $prefix-$lab-$nodeName pattern
	longName := fmt.Sprintf("%s-%s-%s", *c.Config.Prefix, c.Config.Name, nodeName)

	switch *c.Config.Prefix {
	// when prefix is an empty string longName will match shortName/nodeName
	case "":
		longName = nodeName
	case "__lab-name":
		longName = fmt.Sprintf("%s-%s", c.Config.Name, nodeName)
	}

	if !containerNamePatternRe.MatchString(longName) {
		return nil, fmt.Errorf("Node name contains invalid characters: %s", longName)
	}

	if len(longName) > maxNameLength {
		log.Warn("Node name will not resolve via DNS", "name", longName, "reason", fmt.Sprintf("name exceeds %d characters", maxNameLength))
	}

	if strings.ContainsAny(longName, dnsIncompatibleChars) {
		log.Warn("Node name will not resolve via DNS", "name", longName, "reason", "name contains invalid characters such as '.' and/or '_'")
	}

	nodeCfg := &clabtypes.NodeConfig{
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
		NetworkMode:     c.Config.Topology.GetNodeNetworkMode(nodeName),
		Sysctls:         c.Config.Topology.GetSysCtl(nodeName),
		Sandbox:         c.Config.Topology.GetNodeSandbox(nodeName),
		Kernel:          c.Config.Topology.GetNodeKernel(nodeName),
		Runtime:         c.Config.Topology.GetNodeRuntime(nodeName),
		Devices:         c.Config.Topology.GetNodeDevices(nodeName),
		CapAdd:          c.Config.Topology.GetNodeCapAdd(nodeName),
		ShmSize:         c.Config.Topology.GetNodeShmSize(nodeName),
		CPU:             c.Config.Topology.GetNodeCPU(nodeName),
		CPUSet:          c.Config.Topology.GetNodeCPUSet(nodeName),
		Memory:          c.Config.Topology.GetNodeMemory(nodeName),
		StartupDelay:    c.Config.Topology.GetNodeStartupDelay(nodeName),
		AutoRemove:      c.Config.Topology.GetNodeAutoRemove(nodeName),
		RestartPolicy:   c.Config.Topology.GetRestartPolicy(nodeName),
		Extras:          c.Config.Topology.GetNodeExtras(nodeName),
		DNS:             c.Config.Topology.GetNodeDns(nodeName),
		Certificate:     c.Config.Topology.GetCertificateConfig(nodeName),
		Healthcheck:     c.Config.Topology.GetHealthCheckConfig(nodeName),
		Aliases:         c.Config.Topology.GetNodeAliases(nodeName),
		Components:      c.Config.Topology.GetComponents(nodeName),
	}

	if nodeDef != nil {
		nodeCfg.MgmtIPv4Address = nodeDef.MgmtIPv4
		nodeCfg.MgmtIPv6Address = nodeDef.MgmtIPv6
	}

	var err error

	nodeCfg.Stages, err = c.Config.Topology.GetStages(nodeName)
	if err != nil {
		return nil, err
	}

	// load environment variables
	err = addEnvVarsToNodeCfg(c, nodeCfg)
	if err != nil {
		return nil, err
	}

	log.Debugf("node config: %+v", nodeCfg)

	// process startup-config
	err = c.processStartupConfig(nodeCfg)
	if err != nil {
		return nil, err
	}

	nodeCfg.EnforceStartupConfig = c.Config.Topology.GetNodeEnforceStartupConfig(
		nodeCfg.ShortName,
	)
	nodeCfg.SuppressStartupConfig = c.Config.Topology.GetNodeSuppressStartupConfig(
		nodeCfg.ShortName,
	)

	// initialize license field
	p := c.Config.Topology.GetNodeLicense(nodeCfg.ShortName)
	// resolve the lic path to an abs path
	nodeCfg.License = clabutils.ResolvePath(p, c.TopoPaths.TopologyFileDir())

	// initialize bind mounts
	binds, err := c.Config.Topology.GetNodeBinds(nodeName)
	if err != nil {
		return nil, err
	}

	err = c.resolveBindPaths(binds, nodeName)
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

	c.processNodeExecs(nodeCfg)

	return nodeCfg, nil
}

// processStartupConfig processes the raw path of the startup-config as it is defined in the .
// topology file It handles remote files (HTTP/HTTPS/S3), local files and embedded configs.
// As a result the `nodeCfg.StartupConfig` will be set to an absPath of the startup config file.
func (c *CLab) processStartupConfig(nodeCfg *clabtypes.NodeConfig) error {
	// replace __clabNodeName__ magic var in startup-config path with node short name
	r := c.magicVarReplacer(nodeCfg.ShortName)
	p := r.Replace(c.Config.Topology.GetNodeStartupConfig(nodeCfg.ShortName))

	// embedded config is a config that is defined as a multi-line string in the topology file
	// it contains at least one newline
	isEmbeddedConfig := strings.Count(p, "\n") >= 1
	// downloadable config starts with http(s):// or s3://
	isDownloadableConfig := clabutils.IsHttpURL(p, false) || clabutils.IsS3URL(p)

	if isEmbeddedConfig || isDownloadableConfig {
		switch {
		case isEmbeddedConfig:
			log.Debugf("Node %q startup-config is an embedded config: %q", nodeCfg.ShortName, p)
			// for embedded config we create a file with the name embedded.partial.cfg
			// as embedded configs are meant to be partial configs
			absDestFile := c.TopoPaths.StartupConfigDownloadFileAbsPath(
				nodeCfg.ShortName, "embedded.partial.cfg")

			err := clabutils.CreateFile(absDestFile, p)
			if err != nil {
				return err
			}

			p = absDestFile

		case isDownloadableConfig:
			log.Debugf("Node %q startup-config is a downloadable config %q", nodeCfg.ShortName, p)
			// get file name from an URL
			fname := clabutils.FilenameForURL(context.Background(), p)

			// Deduce the absolute destination filename for the downloaded content
			dstFileAbsPath := c.TopoPaths.StartupConfigDownloadFileAbsPath(nodeCfg.ShortName, fname)

			log.Debugf(
				"Fetching startup-config %q for node %q storing at %q",
				p,
				nodeCfg.ShortName,
				dstFileAbsPath,
			)

			// download the file to tmp location
			out, cleanup, err := clabutils.CreateFileWithPermissions(
				dstFileAbsPath,
				clabconstants.PermissionsDirDefault,
			)
			if err != nil {
				return err
			}
			defer cleanup()

			err = clabutils.CopyFileContents(context.Background(), p, out)
			if err != nil {
				return err
			}

			// adjust the NodeConfig by pointing startup-config to the local downloaded file
			p = dstFileAbsPath
		}
	}
	// resolve the startup config path to an abs path
	nodeCfg.StartupConfig = clabutils.ResolvePath(p, c.TopoPaths.TopologyFileDir())

	return nil
}

// checkTopologyDefinition runs topology checks and returns any errors found.
// This function runs after topology file is parsed and all nodes/links are initialized.
func (c *CLab) checkTopologyDefinition(ctx context.Context) error {
	if err := c.verifyLinks(ctx); err != nil {
		return err
	}

	if err := c.verifyRootNetNSLinks(); err != nil {
		return err
	}

	// Check deployment conditions for all nodes (sequentially)
	for _, node := range c.Nodes {
		err := node.CheckDeploymentConditions(ctx)
		if err != nil {
			return err
		}
	}

	if err := c.verifyDuplicateAddresses(); err != nil {
		return err
	}

	if err := c.verifyContainersUniqueness(ctx); err != nil {
		return err
	}

	// Pull images for all nodes concurrently
	return c.pullImagesForNodes(ctx)
}

// verifyRootNetNSLinks makes sure, that there will be no overlap in
// interface names for Root Network Namespace bases nodes.
func (c *CLab) verifyRootNetNSLinks() error {
	rootEpNames := map[string]string{}

	// iterate through nodes
	for _, n := range c.Nodes {
		// check if they are RootNamespace based
		if n.Config().IsRootNamespaceBased {
			// if so, add their ep names to the list of rootEpNames
			for _, e := range n.GetEndpoints() {
				if val, exists := rootEpNames[e.GetIfaceName()]; exists {
					return fmt.Errorf(
						"root network namespace endpoint %q defined by multiple nodes [%s, %s]",
						e.GetIfaceName(), val, e.GetNode().GetShortName(),
					)
				}

				rootEpNames[e.GetIfaceName()] = e.GetNode().GetShortName()
			}
		}
	}

	// we also need to take the two special nodes host and mgmt-br into account
	for _, n := range []clablinks.Node{
		clablinks.GetHostLinkNode(),
		clablinks.GetMgmtBrLinkNode(),
	} {
		// if so, add their ep names to the list of rootEpNames
		for _, e := range n.GetEndpoints() {
			if val, exists := rootEpNames[e.GetIfaceName()]; exists {
				return fmt.Errorf(
					"root network namespace endpoint %q defined by multiple nodes [%s, %s]",
					e.GetIfaceName(), val, e.GetNode().GetShortName(),
				)
			}

			rootEpNames[e.GetIfaceName()] = e.GetNode().GetShortName()
		}
	}

	return nil
}

// verifyLinks checks if all the endpoints in the links section of the topology file
// appear only once.
func (c *CLab) verifyLinks(ctx context.Context) error {
	var err error

	var verificationErrors []error

	for _, e := range c.Endpoints {
		err = e.Verify(ctx, c.globalRuntime().Config().VerifyLinkParams)
		if err != nil {
			verificationErrors = append(verificationErrors, err)
		}
	}

	if len(verificationErrors) > 0 {
		return errors.Join(verificationErrors...)
	}

	return nil
}

// LoadKernelModules loads containerlab-required kernel modules.
func (*CLab) loadKernelModules() error {
	modules := []string{"ip_tables", "ip6_tables"}

	opts := []kmod.Option{
		kmod.SetInitFunc(clabutils.ModInitFunc),
	}

	for _, m := range modules {
		isLoaded, err := clabutils.IsKernelModuleLoaded(m)
		if err != nil {
			return err
		}

		if isLoaded {
			log.Debugf("kernel module %q is already loaded", m)

			continue
		}

		log.Debugf("kernel module %q is not loaded. Trying to load", m)
		// trying to load the kernel modules.
		km, err := kmod.New(opts...)
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
				return fmt.Errorf(
					"management IP address %s appeared more than once in the topology file",
					ip,
				)
			}
		}
	}

	return nil
}

// verifyContainersUniqueness ensures that nodes defined in the topology do not have names of the
// existing containers additionally it checks that the lab name is unique and no containers are
// currently running with the same lab name label.
func (c *CLab) verifyContainersUniqueness(ctx context.Context) error {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	containers, err := c.ListContainers(nctx)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return nil
	}

	var dups []string

	for idx := range c.Nodes {
		if c.Nodes[idx].Config().SkipUniquenessCheck {
			continue
		}

		for cIdx := range containers {
			if c.Nodes[idx].Config().LongName == containers[cIdx].Names[0] {
				dups = append(dups, c.Nodes[idx].Config().LongName)
			}
		}
	}

	if len(dups) != 0 {
		return fmt.Errorf(
			"containers %q already exist. Add '--reconfigure' flag to the deploy command to "+
				"first remove the containers and then deploy the lab",
			dups,
		)
	}

	// check that none of the existing containers has a label that matches
	// the lab name of a currently deploying lab
	// this ensures lab uniqueness
	for idx := range containers {
		if containers[idx].Labels[clabconstants.Containerlab] == c.Config.Name {
			return fmt.Errorf(
				"the '%s' lab has already been deployed. Destroy the lab before deploying a "+
					"lab with the same name", c.Config.Name,
			)
		}
	}

	return nil
}

// resolveBindPaths resolves the host paths in a bind string, such as
// /hostpath:/remotepath(:options) string it allows host path to have `~` and relative path to an
// absolute path the list of binds will be changed in place. if the host path doesn't exist, the
// error will be returned.
func (c *CLab) resolveBindPaths(binds []string, nodeName string) error {
	// checks are skipped when, for example, the destroy operation is run
	if !c.checkBindsPaths {
		return nil
	}

	for i := range binds {
		// host path is a first element in a /hostpath:/remotepath(:options) string
		elems := strings.Split(binds[i], ":")

		if len(elems) == 1 {
			// if there is only one element, it means that we have an anonymous
			// volume, in this case we don't need to resolve the path
			continue
		}

		// replace special variables
		r := c.magicVarReplacer(nodeName)
		hp := r.Replace(elems[0])
		hp = clabutils.ResolvePath(hp, c.TopoPaths.TopologyFileDir())

		_, err := os.Stat(hp)
		if err != nil {
			// check if the hostpath mount has a reference to ansible-inventory.yml or
			// topology-data.json if that is the case, we do not emit an error on missing file,
			// since these files will be created by containerlab upon lab deployment
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
// Update the addDefaultLabels function in clab/config.go
// addDefaultLabels adds default labels to node's config struct.
func (c *CLab) addDefaultLabels(cfg *clabtypes.NodeConfig) {
	if cfg.Labels == nil {
		cfg.Labels = map[string]string{}
	}

	cfg.Labels[clabconstants.Containerlab] = c.Config.Name
	cfg.Labels[clabconstants.NodeName] = cfg.ShortName
	cfg.Labels[clabconstants.LongName] = cfg.LongName
	cfg.Labels[clabconstants.NodeKind] = cfg.Kind
	cfg.Labels[clabconstants.NodeType] = cfg.NodeType
	cfg.Labels[clabconstants.NodeGroup] = cfg.Group
	cfg.Labels[clabconstants.NodeLabDir] = cfg.LabDir
	cfg.Labels[clabconstants.TopoFile] = c.TopoPaths.TopologyFilenameAbsPath()

	// Use custom owner if set, otherwise use current user
	owner := c.customOwner
	if owner == "" {
		owner = os.Getenv("SUDO_USER")
		if owner == "" {
			owner = os.Getenv("USER")
		}
	}

	cfg.Labels[clabconstants.Owner] = owner

	gitBranch, gitHash := c.getGitInfo()

	if gitBranch != "none" {
		cfg.Labels[clabconstants.GitBranch] = gitBranch
		if gitHash != "none" {
			cfg.Labels[clabconstants.GitHash] = gitHash
		}
	}

}

// labelsToEnvVars adds labels to env vars with CLAB_LABEL_ prefix added
// and labels value sanitized.
func labelsToEnvVars(n *clabtypes.NodeConfig) {
	if n.Env == nil {
		n.Env = map[string]string{}
	}

	for k, v := range n.Labels {
		// add the value to the node env with a prefixed and special chars cleaned up key
		n.Env["CLAB_LABEL_"+clabutils.ToEnvKey(k)] = v
	}
}

// addEnvVarsToNodeCfg adds env vars that come from different sources to node config struct.
func addEnvVarsToNodeCfg(c *CLab, nodeCfg *clabtypes.NodeConfig) error {
	// Load content of the EnvVarFiles
	envFileContent, err := clabutils.LoadEnvVarFiles(c.TopoPaths.TopologyFileDir(),
		c.Config.Topology.GetNodeEnvFiles(nodeCfg.ShortName))
	if err != nil {
		return err
	}
	// Merge EnvVarFiles content and the existing env variable
	nodeCfg.Env = clabutils.MergeStringMaps(envFileContent, nodeCfg.Env)

	// Default set of no_proxy entries
	noProxyDefaults := []string{"localhost", "127.0.0.1", "::1", "*.local"}

	// check if either of the no_proxy variables exists
	noProxyLower, existsLower := nodeCfg.Env["no_proxy"]
	noProxyUpper, existsUpper := nodeCfg.Env["NO_PROXY"]

	var noProxy string

	switch {
	case existsLower:
		noProxy = noProxyLower
	case existsUpper:
		noProxy = noProxyUpper
	default:
		noProxy = strings.Join(noProxyDefaults, ",")
	}

	if existsLower || existsUpper {
		for _, defaultValue := range noProxyDefaults {
			if !strings.Contains(noProxy, defaultValue) {
				noProxy = noProxy + "," + defaultValue
			}
		}
	}

	// add all clab nodes to the no_proxy variable, if they have a static IP assigned,
	// add this as well
	var noProxyList []string //nolint:prealloc
	for key := range c.Config.Topology.Nodes {
		noProxyList = append(noProxyList, key)

		nodeConfig := c.Config.Topology.Nodes[key]
		if nodeConfig == nil {
			continue
		}

		if nodeConfig.MgmtIPv4 != "" {
			noProxyList = append(noProxyList, nodeConfig.MgmtIPv4)
		}

		if nodeConfig.MgmtIPv6 != "" {
			noProxyList = append(noProxyList, nodeConfig.MgmtIPv6)
		}
	}

	// add mgmt subnet range for the sake of completeness - some OS support it, others don't
	if c.Config.Mgmt.IPv4Subnet != "" {
		noProxyList = append(noProxyList, c.Config.Mgmt.IPv4Subnet)
	}

	if c.Config.Mgmt.IPv6Subnet != "" {
		noProxyList = append(noProxyList, c.Config.Mgmt.IPv6Subnet)
	}

	// sort for better readability
	sort.Strings(noProxyList)

	noProxy = noProxy + "," + strings.Join(noProxyList, ",")

	nodeCfg.Env["no_proxy"] = noProxy
	nodeCfg.Env["NO_PROXY"] = noProxy

	return nil
}

// processNodeExecs replaces (in place) magic variables in node execs.
func (c *CLab) processNodeExecs(nodeCfg *clabtypes.NodeConfig) {
	for i, e := range nodeCfg.Exec {
		r := c.magicVarReplacer(nodeCfg.ShortName)
		nodeCfg.Exec[i] = r.Replace(e)
	}
}

// magicVarReplacer returns a string replacer that replaces all supported magic variables.
func (c *CLab) magicVarReplacer(nodeName string) *strings.Replacer {
	if nodeName == "" {
		return &strings.Replacer{}
	}

	return strings.NewReplacer(
		clabDirVar, c.TopoPaths.TopologyLabDir(),
		nodeDirVar, c.TopoPaths.NodeDir(nodeName),
		nodeNameVar, nodeName,
	)
}

// magicTopoNameReplacer returns a string replacer that replaces all git branch variables in the topology name.
func (c *CLab) magicTopoNameReplacer() *strings.Replacer {

	gitBranch, gitHash := c.getGitInfo()

	if gitHash == "none" && gitBranch == "none" {
		log.Warnf("topology name uses git variables, but no Git repository found at %q - variables will be replaced with 'none'", c.TopoPaths.TopologyFileDir())
	}

	// Replace illegal characters in branch name
	gitBranch = strings.ReplaceAll(gitBranch, "/", "-")

	log.Debugf("Git repo variables will be the following: branch: %q hash: %q", gitBranch, gitHash)

	return strings.NewReplacer(
		gitBranchVar, gitBranch,
		gitHashVar, gitHash,
	)
}

func (c *CLab) getGitInfo() (string, string) {
	// Return cached values if available (non-empty gitBranch indicates cached)
	if c.gitBranch != "" {
		// If hash wasn't set, return "none" for hash
		if c.gitHash == "" {
			return c.gitBranch, "none"
		}
		return c.gitBranch, c.gitHash
	}

	// Initialize defaults
	gitBranch := "none"
	gitHash := "none"

	repo, err := gogit.PlainOpen(c.TopoPaths.TopologyFileDir())
	if err != nil {
		// Not a git repository - cache and return defaults
		c.gitBranch = gitBranch
		c.gitHash = gitHash
		return gitBranch, gitHash
	}

	repoHead, err := repo.Head()
	if err != nil {
		// Try symbolic head
		symbolicHead, err := repo.Storer.Reference(gogitplumbing.HEAD)
		if err != nil || symbolicHead.Type() != gogitplumbing.SymbolicReference {
			log.Debugf("Could not determine Git branch/hash: %v", err)
			// Cache the defaults
			c.gitBranch = gitBranch
			c.gitHash = gitHash
			return gitBranch, gitHash
		}
		gitBranch = strings.TrimPrefix(symbolicHead.Target().String(), "refs/heads/")
		// For symbolic refs, we might not have a hash readily available
		gitHash = "none"
	} else {
		gitBranch = repoHead.Name().Short()
		// Use short hash (7 characters) instead of full 40-character hash
		gitHash = repoHead.Hash().String()[:7]
	}

	// Cache the results
	c.gitBranch = gitBranch
	c.gitHash = gitHash

	return gitBranch, gitHash
}
