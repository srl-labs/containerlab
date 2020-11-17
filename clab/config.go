package clab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	defaultConfigTemplate = "/etc/containerlab/templates/srl/srlconfig.tpl"
)

// supported kinds
var kinds = []string{"srl", "ceos", "linux", "alpine", "bridge"}

var srlTypes = map[string]string{
	"ixr6":  "topology-7250IXR6.yml",
	"ixr10": "topology-7250IXR10.yml",
	"ixrd1": "topology-7220IXRD1.yml",
	"ixrd2": "topology-7220IXRD2.yml",
	"ixrd3": "topology-7220IXRD3.yml",
}

// mgmtNet struct defines the management network options
// it is provided via docker network object
type mgmtNet struct {
	Network    string // docker network name
	Ipv4Subnet string `yaml:"ipv4_subnet"`
	Ipv6Subnet string `yaml:"ipv6_subnet"`
}

// NodeConfig represents a configuration a given node can have in the lab definition file
type NodeConfig struct {
	Kind     string
	Group    string
	Type     string
	Config   string
	Image    string
	License  string
	Position string
	Cmd      string
	Binds    []string // list of bind mount compatible strings
}

// Topology represents a lab topology
type Topology struct {
	Defaults NodeConfig
	Kinds    map[string]NodeConfig
	Nodes    map[string]NodeConfig
	Links    []link
}

type link struct {
	Endpoints []string
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// Config defines lab configuration as it is provided in the YAML file
type Config struct {
	Name       string
	Mgmt       mgmtNet
	Topology   Topology
	ConfigPath string `yaml:"config_path"`
}

type volume struct {
	Source      string
	Destination string
	ReadOnly    bool
}

// Node is a struct that contains the information of a container element
type Node struct {
	ShortName string
	LongName  string
	Fqdn      string
	LabDir    string
	Index     int
	Group     string
	Kind      string
	Config    string
	NodeType  string
	Position  string
	License   string
	Image     string
	Topology  string
	EnvConf   string
	Sysctls   map[string]string
	User      string
	Cmd       string
	Env       []string
	Binds     []string // Bind mounts strings (src:dest:options)

	TLSCert   string
	TLSKey    string
	TLSAnchor string
}

// Link is a struct that contains the information of a link between 2 containers
type Link struct {
	A      *Endpoint
	B      *Endpoint
	Labels map[string]string
}

// Endpoint is a struct that contains information of a link endpoint
type Endpoint struct {
	Node *Node
	// e1-x, eth, etc
	EndpointName string
}

// ParseIPInfo parses IP information
func (c *cLab) parseIPInfo() error {
	// DockerInfo = t.DockerInfo
	if c.Config.Mgmt.Network == "" {
		c.Config.Mgmt.Network = dockerNetName
	}
	if c.Config.Mgmt.Ipv4Subnet == "" {
		c.Config.Mgmt.Ipv4Subnet = dockerNetIPv4Addr
	}
	if c.Config.Mgmt.Ipv6Subnet == "" {
		c.Config.Mgmt.Ipv6Subnet = dockerNetIPv6Addr
	}
	return nil
}

// ParseTopology parses the lab topology
func (c *cLab) ParseTopology() error {
	log.Info("Parsing topology information ...")
	log.Debugf("Prefix: %s", c.Config.Name)
	// initialize DockerInfo
	err := c.parseIPInfo()
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

func (c *cLab) kindInitialization(nodeCfg *NodeConfig) string {
	if nodeCfg.Kind != "" {
		return nodeCfg.Kind
	}
	return c.Config.Topology.Defaults.Kind
}

func (c *cLab) bindsInit(nodeCfg *NodeConfig) []string {
	if len(nodeCfg.Binds) != 0 {
		return nodeCfg.Binds
	}
	return c.Config.Topology.Kinds[nodeCfg.Kind].Binds
}

func (c *cLab) groupInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Group != "" {
		return nodeCfg.Group
	} else if c.Config.Topology.Kinds[kind].Group != "" {
		return c.Config.Topology.Kinds[kind].Group
	}
	return c.Config.Topology.Defaults.Group
}

func (c *cLab) typeInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Type != "" {
		return nodeCfg.Type
	}
	return c.Config.Topology.Kinds[kind].Type
}

func (c *cLab) configInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Config != "" {
		return nodeCfg.Config
	}
	if kindConfig, ok := c.Config.Topology.Kinds[kind]; ok {
		if kindConfig.Config != "" {
			return kindConfig.Config
		}
	}
	if c.Config.Topology.Defaults.Config != "" {
		return c.Config.Topology.Defaults.Config
	}
	return defaultConfigTemplate
}

func (c *cLab) imageInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Image != "" {
		return nodeCfg.Image
	}
	return c.Config.Topology.Kinds[kind].Image
}

func (c *cLab) licenseInit(nodeCfg *NodeConfig, kind string) (string, error) {
	if nodeCfg.License != "" {
		lp, err := filepath.Abs(nodeCfg.License)
		if err != nil {
			return "", err
		}
		return lp, nil
	}
	lp, err := filepath.Abs(c.Config.Topology.Kinds[kind].License)
	if err != nil {
		return "", err
	}
	return lp, nil
}

func (c *cLab) cmdInitialization(nodeCfg *NodeConfig, kind string, defCmd string) string {
	if nodeCfg.Cmd != "" {
		return nodeCfg.Cmd
	}
	if c.Config.Topology.Kinds[kind].Cmd != "" {
		return c.Config.Topology.Kinds[kind].Cmd
	}
	return defCmd
}

func (c *cLab) positionInitialization(nodeCfg *NodeConfig, kind string) string {
	if nodeCfg.Position != "" {
		return nodeCfg.Position
	} else if c.Config.Topology.Kinds[kind].Position != "" {
		return c.Config.Topology.Kinds[kind].Position
	}
	return c.Config.Topology.Defaults.Position
}

// NewNode initializes a new node object
func (c *cLab) NewNode(nodeName string, nodeCfg NodeConfig, idx int) error {
	// initialize a new node
	node := new(Node)
	node.ShortName = nodeName
	node.LongName = prefix + "-" + c.Config.Name + "-" + nodeName
	node.Fqdn = nodeName + "." + c.Config.Name + ".io"
	node.LabDir = c.Dir.Lab + "/" + nodeName
	node.Index = idx

	// initialize the node with global parameters
	// Kind initialization is either coming from `topology.nodes` section or from `topology.defaults`
	// normalize the data to lower case to compare
	node.Kind = strings.ToLower(c.kindInitialization(&nodeCfg))
	node.Binds = c.bindsInit(&nodeCfg)
	switch node.Kind {
	case "ceos":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = c.configInitialization(&nodeCfg, node.Kind)
		//node.License = t.SRLLicense
		node.Image = c.imageInitialization(&nodeCfg, node.Kind)
		//node.NodeType = "ixr6"
		node.Position = c.positionInitialization(&nodeCfg, node.Kind)

		// initialize specifc container information
		node.Cmd = "/sbin/init systemd.setenv=INTFTYPE=eth systemd.setenv=ETBA=1 systemd.setenv=SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT=1 systemd.setenv=CEOS=1 systemd.setenv=EOS_PLATFORM=ceoslab systemd.setenv=container=docker"
		//node.Cmd = "/sbin/init"
		node.Env = []string{
			"CEOS=1",
			"EOS_PLATFORM=ceoslab",
			"container=docker",
			"ETBA=1",
			"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT=1",
			"INTFTYPE=eth"}
		node.User = "root"
		node.Group = c.groupInitialization(&nodeCfg, node.Kind)
		node.NodeType = nodeCfg.Type
		node.Config = nodeCfg.Config

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv4.ip_forward"] = "0"
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
		node.Sysctls["net.ipv6.conf.all.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.default.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.all.autoconf"] = "0"
		node.Sysctls["net.ipv6.conf.default.autoconf"] = "0"

	case "srl":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = c.configInitialization(&nodeCfg, node.Kind)

		lp, err := c.licenseInit(&nodeCfg, node.Kind)
		if err != nil {
			return err
		}
		node.License = lp

		node.Image = c.imageInitialization(&nodeCfg, node.Kind)
		node.Group = c.groupInitialization(&nodeCfg, node.Kind)
		node.NodeType = c.typeInitialization(&nodeCfg, node.Kind)
		node.Position = c.positionInitialization(&nodeCfg, node.Kind)

		if filename, found := srlTypes[node.NodeType]; found {
			node.Topology = baseConfigDir + filename
		} else {
			keys := make([]string, 0, len(srlTypes))
			for key := range srlTypes {
				keys = append(keys, key)
			}
			log.Fatalf("wrong node type. '%s' doesn't exist. should be any of %s", node.NodeType, strings.Join(keys, ", "))
		}

		// initialize specifc container information
		node.Cmd = "sudo bash -c /opt/srlinux/bin/sr_linux"
		node.Env = []string{"SRLINUX=1"}
		node.User = "root"

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv4.ip_forward"] = "0"
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
		node.Sysctls["net.ipv6.conf.all.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.default.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.all.autoconf"] = "0"
		node.Sysctls["net.ipv6.conf.default.autoconf"] = "0"

		// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
		// in (c *cLab) CreateNodeDirStructure
		node.Binds = append(node.Binds, fmt.Sprint(filepath.Join(node.LabDir, "license.key"), ":/opt/srlinux/etc/license.key:ro"))

		// mount config directory
		cfgPath := filepath.Join(node.LabDir, "config")
		node.Binds = append(node.Binds, fmt.Sprint(cfgPath, ":/etc/opt/srlinux/:rw"))

		// mount srlinux.conf
		srlconfPath := filepath.Join(node.LabDir, "srlinux.conf")
		node.Binds = append(node.Binds, fmt.Sprint(srlconfPath, ":/home/admin/.srlinux.conf:rw"))

		// mount srlinux topology
		topoPath := filepath.Join(node.LabDir, "topology.yml")
		node.Binds = append(node.Binds, fmt.Sprint(topoPath, ":/tmp/topology.yml:ro"))

	case "alpine", "linux":
		node.Config = c.configInitialization(&nodeCfg, node.Kind)
		node.License = ""
		node.Image = c.imageInitialization(&nodeCfg, node.Kind)
		node.Group = c.groupInitialization(&nodeCfg, node.Kind)
		node.NodeType = c.typeInitialization(&nodeCfg, node.Kind)
		node.Position = c.positionInitialization(&nodeCfg, node.Kind)
		node.Cmd = c.cmdInitialization(&nodeCfg, node.Kind, "/bin/sh")

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"

	case "bridge":
		node.Group = c.groupInitialization(&nodeCfg, node.Kind)
		node.Position = c.positionInitialization(&nodeCfg, node.Kind)

	default:
		return fmt.Errorf("Node '%s' refers to a kind '%s' which is not supported. Supported kinds are %q", nodeName, node.Kind, kinds)
	}
	c.Nodes[nodeName] = node
	return nil
}

// NewLink initializes a new link object
func (c *cLab) NewLink(l link) *Link {
	// initialize a new link
	link := new(Link)
	link.Labels = l.Labels

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
func (c *cLab) NewEndpoint(e string) *Endpoint {
	// initialize a new endpoint
	endpoint := new(Endpoint)

	// split the string to get node name and endpoint name
	split := strings.Split(e, ":")
	if len(split) != 2 {
		log.Fatalf("endpoint %s has wrong syntax", e)
	}
	nName := split[0]  // node name
	epName := split[1] // endpoint name
	// search the node pointer based in the name of the split function
	for name, n := range c.Nodes {
		if name == split[0] {
			endpoint.Node = n
			break
		}
	}
	if endpoint.Node == nil {
		log.Fatalf("Not all nodes are specified in the 'topology.nodes' section or the names don't match in the 'links.endpoints' section: %s", nName)
	}

	// initialize the endpoint name based on the split function
	if c.Nodes[nName].Kind == "bridge" {
		endpoint.EndpointName = c.Config.Name + epName
	} else {
		endpoint.EndpointName = epName
	}
	if len(endpoint.EndpointName) > 15 {
		log.Fatalf("interface '%s' name exceeds maximum length of 15 characters", endpoint.EndpointName)
	}
	return endpoint
}

// VerifyBridgeExists verifies if every node of kind=bridge exists on the lab host
func (c *cLab) VerifyBridgesExist() error {
	for name, node := range c.Nodes {
		if node.Kind == "bridge" {
			if _, err := netlink.LinkByName(name); err != nil {
				return fmt.Errorf("Bridge %s is referenced in the endpoints section but was not found in the default network namespace", name)
			}
		}
	}
	return nil
}
