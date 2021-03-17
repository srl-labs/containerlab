package clab

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	baseConfigDir = "/etc/containerlab/templates/srl/"
	// prefix is used to distinct containerlab created files/dirs/containers
	prefix = "clab"
	// a name of a docker network that nodes management interfaces connect to
	dockerNetName     = "clab"
	dockerNetIPv4Addr = "172.20.20.0/24"
	dockerNetIPv6Addr = "2001:172:20:20::/80"
	dockerNetMTU      = "1500"
	srlDefaultType    = "ixr6"
	vrsrosDefaultType = "sr-1"
	// default connection mode for vrnetlab based containers
	vrDefConnMode = "tc"
	// NSPath value assigned to host interfaces
	hostNSPath = "__host"
	// veth link mtu. jacked up to 65k to allow jumbo testing of various sizes
	defaultVethLinkMTU = 65000
)

// supported kinds
var kinds = []string{
	"srl",
	"ceos",
	"crpd",
	"sonic-vs",
	"vr-sros",
	"vr-vmx",
	"vr-xrv",
	"vr-xrv9k",
	"vr-veos",
	"linux",
	"bridge",
	"ovs-bridge",
	"mysocketio",
	"host",
}

var defaultConfigTemplates = map[string]string{
	"srl":     "/etc/containerlab/templates/srl/srlconfig.tpl",
	"ceos":    "/etc/containerlab/templates/arista/ceos.cfg.tpl",
	"crpd":    "/etc/containerlab/templates/crpd/juniper.conf",
	"vr-sros": "",
}

var srlTypes = map[string]string{
	"ixr6":  "topology-7250IXR6.yml",
	"ixr10": "topology-7250IXR10.yml",
	"ixrd1": "topology-7220IXRD1.yml",
	"ixrd2": "topology-7220IXRD2.yml",
	"ixrd3": "topology-7220IXRD3.yml",
}

// Config defines lab configuration as it is provided in the YAML file
type Config struct {
	Name       string   `json:"name,omitempty"`
	Mgmt       mgmtNet  `json:"mgmt,omitempty"`
	Topology   Topology `json:"topology,omitempty"`
	ConfigPath string   `yaml:"config_path,omitempty"`
}

// mgmtNet struct defines the management network options
// it is provided via docker network object
type mgmtNet struct {
	Network    string `yaml:"network,omitempty"` // docker network name
	IPv4Subnet string `yaml:"ipv4_subnet,omitempty"`
	IPv6Subnet string `yaml:"ipv6_subnet,omitempty"`
	MTU        string `yaml:"mtu,omitempty"`
}

// Topology represents a lab topology
type Topology struct {
	Defaults NodeConfig            `yaml:"defaults,omitempty"`
	Kinds    map[string]NodeConfig `yaml:"kinds,omitempty"`
	Nodes    map[string]NodeConfig `yaml:"nodes,omitempty"`
	Links    []LinkConfig          `yaml:"links,omitempty"`
}

// NodeConfig represents a configuration a given node can have in the lab definition file
type NodeConfig struct {
	Kind     string `yaml:"kind,omitempty"`
	Group    string `yaml:"group,omitempty"`
	Type     string `yaml:"type,omitempty"`
	Config   string `yaml:"config,omitempty"`
	Image    string `yaml:"image,omitempty"`
	License  string `yaml:"license,omitempty"`
	Position string `yaml:"position,omitempty"`
	Cmd      string `yaml:"cmd,omitempty"`
	// list of bind mount compatible strings
	Binds []string `yaml:"binds,omitempty"`
	// list of port bindings
	Ports []string `yaml:"ports,omitempty"`
	// user-defined IPv4 address in the management network
	MgmtIPv4 string `yaml:"mgmt_ipv4,omitempty"`
	// user-defined IPv6 address in the management network
	MgmtIPv6 string `yaml:"mgmt_ipv6,omitempty"`
	// list of ports to publish with mysocketctl
	Publish []string `yaml:"publish,omitempty"`
	// environment variables
	Env map[string]string `yaml:"env,omitempty"`
	// linux user used in a container
	User string `yaml:"user,omitempty"`
	// container labels
	Labels map[string]string `yaml:"labels,omitempty"`
	// container networking mode. if set to `host` the host networking will be used for this node, else bridged network
	NetworkMode string `yaml:"network-mode,omitempty"`
}

type LinkConfig struct {
	Endpoints []string
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// Node is a struct that contains the information of a container element
type Node struct {
	ShortName string
	LongName  string
	Fqdn      string
	LabDir    string // LabDir is a directory related to the node, it contains config items and/or other persistent state
	Index     int
	Group     string
	Kind      string
	// path to config template file that is used for config generation
	Config       string
	ResConfig    string // path to config file that is actually mounted to the container and is a result of templation
	NodeType     string
	Position     string
	License      string
	Image        string
	Topology     string
	Sysctls      map[string]string
	User         string
	Entrypoint   string
	Cmd          string
	Env          map[string]string
	Binds        []string    // Bind mounts strings (src:dest:options)
	PortBindings nat.PortMap // PortBindings define the bindings between the container ports and host ports
	PortSet      nat.PortSet // PortSet define the ports that should be exposed on a container
	// container networking mode. if set to `host` the host networking will be used for this node, else bridged network
	NetworkMode          string
	MgmtNet              string // name of the docker network this node is connected to with its first interface
	MgmtIPv4Address      string
	MgmtIPv4PrefixLength int
	MgmtIPv6Address      string
	MgmtIPv6PrefixLength int
	ContainerID          string
	TLSCert              string
	TLSKey               string
	TLSAnchor            string
	NSPath               string   // network namespace path for this node
	Publish              []string //list of ports to publish with mysocketctl
	// container labels
	Labels map[string]string
}

// Link is a struct that contains the information of a link between 2 containers
type Link struct {
	A      *Endpoint
	B      *Endpoint
	MTU    int
	Labels map[string]string
}

// Endpoint is a struct that contains information of a link endpoint
type Endpoint struct {
	Node *Node
	// e1-x, eth, etc
	EndpointName string
}

// initMgmtNetwork sets management network config
func (c *CLab) initMgmtNetwork() error {
	if c.Config.Mgmt.Network == "" {
		c.Config.Mgmt.Network = dockerNetName
	}
	if c.Config.Mgmt.IPv4Subnet == "" && c.Config.Mgmt.IPv6Subnet == "" {
		if c.Config.Mgmt.IPv4Subnet == "" {
			c.Config.Mgmt.IPv4Subnet = dockerNetIPv4Addr
		}
		if c.Config.Mgmt.IPv6Subnet == "" {
			c.Config.Mgmt.IPv6Subnet = dockerNetIPv6Addr
		}
	}
	// init docker network mtu
	if c.Config.Mgmt.MTU == "" {
		c.Config.Mgmt.MTU = dockerNetMTU
	}
	return nil
}

// ParseTopology parses the lab topology
func (c *CLab) ParseTopology() error {
	log.Info("Parsing topology information...")
	log.Debugf("Lab name: %s", c.Config.Name)
	// initialize Management network config
	err := c.initMgmtNetwork()
	if err != nil {
		return err
	}
	log.Debugf("DockerInfo: %v", c.Config.Mgmt)

	if c.Config.ConfigPath == "" {
		c.Config.ConfigPath, _ = filepath.Abs(os.Getenv("PWD"))
	}

	c.Dir = new(cLabDirectory)
	c.Dir.Lab = c.Config.ConfigPath + "/" + prefix + "-" + c.Config.Name
	c.Dir.LabCA = c.Dir.Lab + "/" + "ca"
	c.Dir.LabCARoot = c.Dir.LabCA + "/" + "root"
	c.Dir.LabGraph = c.Dir.Lab + "/" + "graph"

	// initialize Nodes and Links variable
	c.Nodes = make(map[string]*Node)
	c.Links = make(map[int]*Link)

	// initialize the Node information from the topology map
	idx := 0
	for nodeName, node := range c.Config.Topology.Nodes {
		if err = c.NewNode(nodeName, node, idx); err != nil {
			return err
		}
		idx++
	}
	for i, l := range c.Config.Topology.Links {
		// i represents the endpoint integer and l provide the link struct
		c.Links[i] = c.NewLink(l)
	}
	return nil
}

func (c *CLab) kindInitialization(nodeCfg *NodeConfig) string {
	if nodeCfg.Kind != "" {
		return nodeCfg.Kind
	}
	return c.Config.Topology.Defaults.Kind
}

func (c *CLab) bindsInit(nodeCfg *NodeConfig) []string {
	switch {
	case len(nodeCfg.Binds) != 0:
		return nodeCfg.Binds
	case len(c.Config.Topology.Kinds[nodeCfg.Kind].Binds) != 0:
		return c.Config.Topology.Kinds[nodeCfg.Kind].Binds
	case len(c.Config.Topology.Defaults.Binds) != 0:
		return c.Config.Topology.Defaults.Binds
	}
	return nil
}

// portsInit produces the nat.PortMap out of the slice of string representation of port bindings
func (c *CLab) portsInit(nodeCfg *NodeConfig) (nat.PortSet, nat.PortMap, error) {
	if len(nodeCfg.Ports) != 0 {
		ps, pb, err := nat.ParsePortSpecs(nodeCfg.Ports)
		if err != nil {
			return nil, nil, err
		}
		return ps, pb, nil
	}
	return nil, nil, nil
}

func (c *CLab) groupInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Group != "" {
		return nodeCfg.Group
	} else if c.Config.Topology.Kinds[kind].Group != "" {
		return c.Config.Topology.Kinds[kind].Group
	}
	return c.Config.Topology.Defaults.Group
}

// initialize SRL HW type
func (c *CLab) typeInit(nodeCfg *NodeConfig, kind string) string {
	switch {
	case nodeCfg.Type != "":
		return nodeCfg.Type
	case c.Config.Topology.Kinds[kind].Type != "":
		return c.Config.Topology.Kinds[kind].Type
	case c.Config.Topology.Defaults.Type != "":
		return c.Config.Topology.Defaults.Type
	}
	// default type if not defined
	switch kind {
	case "srl":
		return srlDefaultType

	case "vr-sros":
		return vrsrosDefaultType
	}
	return ""
}

// configInit processes the path to a config file that can be provided on
// multiple configuration levels
// returns an errof if the reference path doesn't exist
func (c *CLab) configInit(nodeCfg *NodeConfig, kind string) (string, error) {
	var cfg string
	var err error
	switch {
	case nodeCfg.Config != "":
		cfg = nodeCfg.Config
	case c.Config.Topology.Kinds[kind].Config != "":
		cfg = c.Config.Topology.Kinds[kind].Config
	case c.Config.Topology.Defaults.Config != "":
		cfg = c.Config.Topology.Defaults.Config
	default:
		cfg = defaultConfigTemplates[kind]
	}
	if cfg != "" {
		cfg, err = resolvePath(cfg)
		if err != nil {
			return "", err
		}
		_, err = os.Stat(cfg)
	}
	return cfg, err
}

func (c *CLab) imageInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Image != "" {
		return nodeCfg.Image
	}
	if c.Config.Topology.Kinds[kind].Image != "" {
		return c.Config.Topology.Kinds[kind].Image
	}
	return c.Config.Topology.Defaults.Image
}

func (c *CLab) licenseInit(nodeCfg *NodeConfig, node *Node) (string, error) {
	// path to license file
	var lic string
	var err error
	switch {
	case nodeCfg.License != "":
		lic = nodeCfg.License
	case c.Config.Topology.Kinds[node.Kind].License != "":
		lic = c.Config.Topology.Kinds[node.Kind].License
	case c.Config.Topology.Defaults.License != "":
		lic = c.Config.Topology.Defaults.License
	default:
		lic = ""
	}
	if lic != "" {
		lic, err = resolvePath(lic)
		if err != nil {
			return "", err
		}
		_, err = os.Stat(lic)
	}
	return lic, err
}

func (c *CLab) cmdInit(nodeCfg *NodeConfig, kind string) string {
	switch {
	case nodeCfg.Cmd != "":
		return nodeCfg.Cmd

	case c.Config.Topology.Kinds[kind].Cmd != "":
		return c.Config.Topology.Kinds[kind].Cmd

	case c.Config.Topology.Defaults.Cmd != "":
		return c.Config.Topology.Defaults.Cmd
	}
	return ""
}

// initialize environment variables
func (c *CLab) envInit(nodeCfg *NodeConfig, kind string) map[string]string {
	// merge global envs into kind envs
	m := mergeStringMaps(c.Config.Topology.Defaults.Env, c.Config.Topology.Kinds[kind].Env)
	// merge result of previous merge into node envs
	return mergeStringMaps(m, nodeCfg.Env)
}

func (c *CLab) positionInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Position != "" {
		return nodeCfg.Position
	} else if c.Config.Topology.Kinds[kind].Position != "" {
		return c.Config.Topology.Kinds[kind].Position
	}
	return c.Config.Topology.Defaults.Position
}

func (c *CLab) userInit(nodeCfg *NodeConfig, kind string) string {
	switch {
	case nodeCfg.User != "":
		return nodeCfg.User

	case c.Config.Topology.Kinds[kind].User != "":
		return c.Config.Topology.Kinds[kind].User

	case c.Config.Topology.Defaults.User != "":
		return c.Config.Topology.Defaults.User
	}
	return ""
}

func (c *CLab) publishInit(nodeCfg *NodeConfig, kind string) []string {
	switch {
	case len(nodeCfg.Publish) != 0:
		return nodeCfg.Publish
	case len(c.Config.Topology.Kinds[kind].Publish) != 0:
		return c.Config.Topology.Kinds[kind].Publish
	case len(c.Config.Topology.Defaults.Publish) != 0:
		return c.Config.Topology.Defaults.Publish
	}
	return nil
}

// initialize container labels
func (c *CLab) labelsInit(nodeCfg *NodeConfig, kind string, node *Node) map[string]string {
	defaultLabels := map[string]string{
		"containerlab":      c.Config.Name,
		"clab-node-kind":    kind,
		"clab-node-type":    node.NodeType,
		"clab-node-lab-dir": node.LabDir,
		"clab-topo-file":    c.TopoFile.path,
	}
	// merge global labels into kind envs
	m := mergeStringMaps(c.Config.Topology.Defaults.Labels, c.Config.Topology.Kinds[kind].Labels)
	// merge result of previous merge into node envs
	m2 := mergeStringMaps(m, nodeCfg.Labels)
	// merge with default labels
	return mergeStringMaps(m2, defaultLabels)
}

// NewNode initializes a new node object
func (c *CLab) NewNode(nodeName string, nodeCfg NodeConfig, idx int) error {
	// initialize a new node
	node := new(Node)
	node.ShortName = nodeName
	node.LongName = prefix + "-" + c.Config.Name + "-" + nodeName
	node.Fqdn = nodeName + "." + c.Config.Name + ".io"
	node.LabDir = c.Dir.Lab + "/" + nodeName
	node.Index = idx

	node.NetworkMode = strings.ToLower(nodeCfg.NetworkMode)

	node.MgmtIPv4Address = nodeCfg.MgmtIPv4
	node.MgmtIPv6Address = nodeCfg.MgmtIPv6

	// initialize the node with global parameters
	// Kind initialization is either coming from `topology.nodes` section or from `topology.defaults`
	// normalize the data to lower case to compare
	node.Kind = strings.ToLower(c.kindInitialization(&nodeCfg))

	// initialize bind mounts
	binds := c.bindsInit(&nodeCfg)
	err := resolveBindPaths(binds)
	if err != nil {
		return err
	}
	node.Binds = binds

	ps, pb, err := c.portsInit(&nodeCfg)
	if err != nil {
		return err
	}
	node.PortBindings = pb
	node.PortSet = ps

	// initialize passed env variables which will be merged with kind specific ones
	envs := c.envInit(&nodeCfg, node.Kind)

	user := c.userInit(&nodeCfg, node.Kind)

	node.Publish = c.publishInit(&nodeCfg, node.Kind)

	switch node.Kind {
	case "ceos":
		err = initCeosNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}

	case "srl":
		err = initSRLNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "crpd":
		err = initCrpdNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "sonic-vs":
		err = initSonicNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "vr-sros":
		err = initSROSNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "vr-vmx":
		err = initVrVMXNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "vr-xrv":
		err = initVrXRVNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "vr-xrv9k":
		err = initVrXRV9kNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "vr-veos":
		err = initVrVeosNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}
	case "alpine", "linux", "mysocketio":
		err = initLinuxNode(c, nodeCfg, node, user, envs)
		if err != nil {
			return err
		}

	case "bridge", "ovs-bridge":
		node.Group = c.groupInitialization(&nodeCfg, node.Kind)
		node.Position = c.positionInitialization(&nodeCfg, node.Kind)

	default:
		return fmt.Errorf("Node '%s' refers to a kind '%s' which is not supported. Supported kinds are %q", nodeName, node.Kind, kinds)
	}

	// init labels after all node kinds are processed
	node.Labels = c.labelsInit(&nodeCfg, node.Kind, node)

	c.Nodes[nodeName] = node
	return nil
}

// NewLink initializes a new link object
func (c *CLab) NewLink(l LinkConfig) *Link {
	// initialize a new link
	link := new(Link)
	link.Labels = l.Labels

	if link.MTU <= 0 {
		link.MTU = defaultVethLinkMTU
	}

	for i, d := range l.Endpoints {
		// i indicates the number and d presents the string, which need to be
		// split in node and endpoint name
		if i == 0 {
			link.A = c.NewEndpoint(d)
		} else {
			link.B = c.NewEndpoint(d)
		}
	}
	return link
}

// NewEndpoint initializes a new endpoint object
func (c *CLab) NewEndpoint(e string) *Endpoint {
	// initialize a new endpoint
	endpoint := new(Endpoint)

	// split the string to get node name and endpoint name
	split := strings.Split(e, ":")
	if len(split) != 2 {
		log.Fatalf("endpoint %s has wrong syntax", e)
	}
	nName := split[0]  // node name
	epName := split[1] // endpoint name
	// search the node pointer for a node name referenced in endpoint section
	// if node name is not "host", since "host" is a special reference to host namespace
	// for which we create an special Node with kind "host"
	if nName == "host" {
		endpoint.Node = &Node{
			Kind:      "host",
			ShortName: "host",
			NSPath:    hostNSPath,
		}
	} else {
		for name, n := range c.Nodes {
			if name == split[0] {
				endpoint.Node = n
				break
			}
		}
	}

	// stop the deployment if the matching node element was not found
	// "host" node name is an exception, it may exist without a matching node
	if endpoint.Node == nil {
		log.Fatalf("Not all nodes are specified in the 'topology.nodes' section or the names don't match in the 'links.endpoints' section: %s", nName)
	}

	// initialize the endpoint name based on the split function
	endpoint.EndpointName = epName
	if len(endpoint.EndpointName) > 15 {
		log.Fatalf("interface '%s' name exceeds maximum length of 15 characters", endpoint.EndpointName)
	}
	return endpoint
}

// CheckTopologyDefinition runs topology checks and returns any errors found
func (c *CLab) CheckTopologyDefinition(ctx context.Context) error {
	err := c.verifyBridgesExist()
	if err != nil {
		return err
	}
	err = c.verifyLinks()
	if err != nil {
		return err
	}
	if err = c.VerifyContainersUniqueness(ctx); err != nil {
		return err
	}
	if err = c.verifyVirtSupport(); err != nil {
		return err
	}
	if err = c.verifyHostIfaces(); err != nil {
		return err
	}
	if err = c.VerifyImages(ctx); err != nil {
		return err
	}
	return nil
}

// VerifyBridgeExists verifies if every node of kind=bridge/ovs-bridge exists on the lab host
func (c *CLab) verifyBridgesExist() error {
	for name, node := range c.Nodes {
		if node.Kind == "bridge" || node.Kind == "ovs-bridge" {
			if _, err := netlink.LinkByName(name); err != nil {
				return fmt.Errorf("bridge %s is referenced in the endpoints section but was not found in the default network namespace", name)
			}
		}
	}
	return nil
}

func (c *CLab) verifyLinks() error {
	endpoints := map[string]struct{}{}
	dups := []string{}
	for _, lc := range c.Config.Topology.Links {
		for _, e := range lc.Endpoints {
			if _, ok := endpoints[e]; ok {
				dups = append(dups, e)
			}
			endpoints[e] = struct{}{}
		}
	}
	if len(dups) != 0 {
		return fmt.Errorf("endpoints %q appeared more than once in the links section of the topology file", dups)
	}
	return nil
}

// VerifyImages will check if image referred in the node config
// either pullable or present or is available in the local registry
// if it is not available it will emit an error
func (c *CLab) VerifyImages(ctx context.Context) error {
	images := map[string]struct{}{}
	for _, node := range c.Nodes {
		// skip image verification for bridge kinds
		if node.Kind == "bridge" || node.Kind == "ovs-bridge" {
			return nil
		}
		if node.Image == "" {
			return fmt.Errorf("missing required image for node %s", node.ShortName)
		}
		if node.Image != "" {
			images[node.Image] = struct{}{}
		}
	}

	for image := range images {
		err := c.PullImageIfRequired(ctx, image)
		if err != nil {
			return err
		}
	}
	return nil
}

// VerifyContainersUniqueness ensures that nodes defined in the topology do not have names of the existing containers
func (c *CLab) VerifyContainersUniqueness(ctx context.Context) error {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var labels []string
	containers, err := c.ListContainers(nctx, labels)
	if err != nil {
		return fmt.Errorf("could not list containers: %v", err)
	}
	if len(containers) == 0 {
		return nil
	}

	dups := []string{}
	for _, n := range c.Nodes {
		for _, cnt := range containers {
			if "/"+n.LongName == cnt.Names[0] {
				dups = append(dups, n.LongName)
			}
		}
	}
	if len(dups) != 0 {
		return fmt.Errorf("containers %q already exist. Add '--reconfigure' flag to the deploy command to first remove the containers and then deploy the lab", dups)
	}
	return err
}

// verifyHostIfaces ensures that host interfaces referenced in the topology
// do not exist already in the root namespace
func (c *CLab) verifyHostIfaces() error {
	for _, l := range c.Links {
		if l.A.Node.ShortName == "host" {
			if nl, _ := netlink.LinkByName(l.A.EndpointName); nl != nil {
				return fmt.Errorf("host interface %s referenced in topology already exists", l.A.EndpointName)
			}
		}
		if l.B.Node.ShortName == "host" {
			if nl, _ := netlink.LinkByName(l.B.EndpointName); nl != nil {
				return fmt.Errorf("host interface %s referenced in topology already exists", l.B.EndpointName)
			}
		}
	}
	return nil
}

// verifyVirtSupport checks if virtualization supported by vcpu if vrnetlab nodes are used
func (c *CLab) verifyVirtSupport() error {
	virtNeeded := false
	for _, n := range c.Nodes {
		if strings.HasPrefix(n.Kind, "vr-") {
			virtNeeded = true
			break
		}
	}
	if !virtNeeded {
		return nil
	}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "vmx") || strings.Contains(scanner.Text(), "svm") {
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return fmt.Errorf("virtualization seems to be not supported and it is required by vrnetlab routers. Check if virtualization can been enabled")
}

//resolvePath resolves a string path by expanding `~` to home dir or getting Abs path for the given path
func resolvePath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	var err error
	switch {
	// resolve ~/ path
	case p[0] == '~':
		p, err = homedir.Expand(p)
		if err != nil {
			return "", err
		}
	default:
		p, err = filepath.Abs(p)
		if err != nil {
			return "", err
		}
	}
	return p, nil
}

// resolveBindPaths resolves the host paths in a bind string, such as /hostpath:/remotepath(:options) string
// it allows host path to have `~` and returns absolute path for a relative path
// if the host path doesn't exist, the error will be returned
func resolveBindPaths(binds []string) error {
	for i := range binds {
		// host path is a first element in a /hostpath:/remotepath(:options) string
		elems := strings.Split(binds[i], ":")
		hp, err := resolvePath(elems[0])
		if err != nil {
			return err
		}
		_, err = os.Stat(hp)
		if err != nil {
			return fmt.Errorf("failed to verify bind path: %v", err)
		}
		elems[0] = hp
		binds[i] = strings.Join(elems, ":")
	}
	return nil
}

// convertEnvs convert env variables passed as a map to a list of them
func convertEnvs(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}

// mergeStringMaps merges map m1 into m2 and return a resulting map as a new map
// maps that are passed for merging will not be changed
func mergeStringMaps(m1, m2 map[string]string) map[string]string {
	if m1 == nil {
		return m2
	}
	if m2 == nil {
		return m1
	}
	// make a copy of a map
	m := make(map[string]string)
	for k, v := range m1 {
		m[k] = v
	}

	for k, v := range m2 {
		m[k] = v
	}
	return m
}

// CheckResources runs container host resources check
func (c *CLab) CheckResources() error {
	vcpu := runtime.NumCPU()
	log.Debugf("Number of vcpu: %d", vcpu)
	if vcpu < 2 {

		log.Warn("Only 1 vcpu detected on this container host. Most containerlab nodes require at least 2 vcpu")
	}
	return nil
}
