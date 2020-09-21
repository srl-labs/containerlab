package clab

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

const baseConfigDir = "/etc/containerlab/templates/srl/"

var srlTypes = map[string]string{
	"ixr6":  "topology-7250IXR6.yml",
	"ixr10": "topology-7250IXR10.yml",
	"ixrd1": "topology-7220IXRD1.yml",
	"ixrd2": "topology-7220IXRD2.yml",
	"ixrd3": "topology-7220IXRD3.yml",
}

type dockerInfo struct {
	Bridge      string `yaml:"bridge"`
	Ipv4Subnet  string `yaml:"ipv4_subnet"`
	Ipv4Gateway string `yaml:"ipv4_gateway"`
	Ipv6Subnet  string `yaml:"ipv6_subnet"`
	Ipv6Gateway string `yaml:"ipv6_gateway"`
}

type dutInfo struct {
	Kind     string `yaml:"kind"`
	Group    string `yaml:"group"`
	Type     string `yaml:"type"`
	Config   string `yaml:"config"`
	Image    string `yaml:"image"`
	License  string `yaml:"license"`
	Position string `yaml:"position"`
}

type link struct {
	Endpoints []string          `yaml:"endpoints"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// Conf holds the configuration file input
type Conf struct {
	Prefix      string     `yaml:"Prefix"`
	DockerInfo  dockerInfo `yaml:"Docker_info"`
	ClientImage string     `yaml:"Client_image"`
	Duts        struct {
		GlobalDefaults dutInfo            `yaml:"global_defaults"`
		KindDefaults   map[string]dutInfo `yaml:"kind_defaults"`
		DutSpecifics   map[string]dutInfo `yaml:"dut_specifics"`
	} `yaml:"Duts"`
	Links      []link `yaml:"Links"`
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
	// CertDir    string
	Index      int
	Group      string
	Kind       string
	Config     string
	NodeType   string
	Position   string
	License    string
	Image      string
	Topology   string
	Path       string
	EnvConf    string
	TLS        string
	Checkpoint string
	Sysctls    map[string]string
	User       string
	Cmd        string
	Env        []string
	Mounts     map[string]volume
	Volumes    map[string]struct{}
	Binds      []string
	Pid        int
	Cid        string
	MgmtIPv4   string
	MgmtIPv6   string
	MgmtMac    string
	AS         uint32

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

// Endpoint is a sttruct that contains information of a link endpoint
type Endpoint struct {
	Node *Node
	// e1-x, eth, etc
	EndpointName string
}

// ParseIPInfo parses IP information
func (c *cLab) parseIPInfo() error {
	// DockerInfo = t.DockerInfo
	if c.Conf.DockerInfo.Bridge == "" {
		c.Conf.DockerInfo.Bridge = "srlinux_bridge"
	}
	if c.Conf.DockerInfo.Ipv4Subnet == "" {
		c.Conf.DockerInfo.Bridge = "172.19.19.0/24"
	}
	if c.Conf.DockerInfo.Ipv6Subnet == "" {
		c.Conf.DockerInfo.Bridge = "2001:172:19:19::/80"
	}

	_, ipv4Net, err := net.ParseCIDR(c.Conf.DockerInfo.Ipv4Subnet)
	if err != nil {
		return err
	}
	ipv4Gateway := ipv4Net.IP.To4()
	ipv4Gateway[3]++
	c.Conf.DockerInfo.Ipv4Gateway = ipv4Gateway.String()

	_, ipv6Net, err := net.ParseCIDR(c.Conf.DockerInfo.Ipv6Subnet)
	if err != nil {
		return err
	}
	ipv6Gateway := ipv6Net.IP
	ipv6Gateway[15]++
	c.Conf.DockerInfo.Ipv6Gateway = ipv6Gateway.String()

	return nil
}

// ParseTopology parses the lab topology
func (c *cLab) ParseTopology() error {
	log.Info("Parsing topology information ...")
	log.Debugf("Prefix: %s", c.Conf.Prefix)
	// initialize DockerInfo
	err := c.parseIPInfo()
	if err != nil {
		return err
	}
	log.Debugf("DockerInfo: %v", c.Conf.DockerInfo)

	if c.Conf.ConfigPath == "" {
		c.Conf.ConfigPath, _ = filepath.Abs(os.Getenv("PWD"))
	}

	c.Dir = new(cLabDirectory)
	c.Dir.Lab = c.Conf.ConfigPath + "/" + "containerlab" + "-" + c.Conf.Prefix
	c.Dir.LabCA = c.Dir.Lab + "/" + "ca"
	c.Dir.LabCARoot = c.Dir.LabCA + "/" + "root"
	c.Dir.LabGraph = c.Dir.Lab + "/" + "graph"

	// initialize Nodes and Links variable
	c.Nodes = make(map[string]*Node)
	c.Links = make(map[int]*Link)

	// initialize the Node information from the topology map
	idx := 0
	for dut, data := range c.Conf.Duts.DutSpecifics {
		c.NewNode(dut, data, idx)
		idx++
	}
	for i, l := range c.Conf.Links {
		// i represnts the endpoint integer and l provide the link struct
		c.Links[i] = c.NewLink(l)
	}
	return nil
}

func (c *cLab) kindInitialization(dut *dutInfo) string {
	if dut.Kind != "" {
		return dut.Kind
	}
	return c.Conf.Duts.GlobalDefaults.Kind
}

func (c *cLab) groupInitialization(dut *dutInfo, kind string) string {
	if dut.Group != "" {
		return dut.Group
	} else if c.Conf.Duts.KindDefaults[kind].Group != "" {
		return c.Conf.Duts.KindDefaults[kind].Group
	}
	return c.Conf.Duts.GlobalDefaults.Group
}

func (c *cLab) typeInitialization(dut *dutInfo, kind string) string {
	if dut.Type != "" {
		return dut.Type
	}
	return c.Conf.Duts.KindDefaults[kind].Type
}

func (c *cLab) configInitialization(dut *dutInfo, kind string) string {
	if dut.Config != "" {
		return dut.Config
	}
	return c.Conf.Duts.KindDefaults[kind].Config
}

func (c *cLab) imageInitialization(dut *dutInfo, kind string) string {
	if dut.Image != "" {
		return dut.Image
	}
	return c.Conf.Duts.KindDefaults[kind].Image
}

func (c *cLab) licenseInitialization(dut *dutInfo, kind string) string {
	if dut.License != "" {
		return dut.License
	}
	return c.Conf.Duts.KindDefaults[kind].License
}

func (c *cLab) positionInitialization(dut *dutInfo, kind string) string {
	if dut.Position != "" {
		return dut.Position
	} else if c.Conf.Duts.KindDefaults[kind].Position != "" {
		return c.Conf.Duts.KindDefaults[kind].Position
	}
	return c.Conf.Duts.GlobalDefaults.Position
}

// NewNode initializes a new node object
func (c *cLab) NewNode(dutName string, dut dutInfo, idx int) {
	// initialize a new node
	node := new(Node)
	node.ShortName = dutName
	node.LongName = "containerlab" + "-" + c.Conf.Prefix + "-" + dutName
	node.Fqdn = dutName + "." + c.Conf.Prefix + ".io"
	node.LabDir = c.Dir.Lab + "/" + dutName
	// node.CertDir = c.Dir.LabCA + "/" + dutName
	node.Index = idx

	// initialize the node with global parameters
	// Kind initialization is either coming from dut_specific or from global
	// normalize the data to lower case to compare
	node.Kind = strings.ToLower(c.kindInitialization(&dut))
	switch node.Kind {
	case "ceos":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = c.configInitialization(&dut, node.Kind)
		//node.License = t.SRLLicense
		node.Image = c.imageInitialization(&dut, node.Kind)
		//node.NodeType = "ixr6"
		node.Position = c.positionInitialization(&dut, node.Kind)

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
		node.Group = c.groupInitialization(&dut, node.Kind)
		node.NodeType = dut.Type
		node.Config = dut.Config

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv4.ip_forward"] = "0"
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
		node.Sysctls["net.ipv6.conf.all.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.default.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.all.autoconf"] = "0"
		node.Sysctls["net.ipv6.conf.default.autoconf"] = "0"

	case "srl":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = c.configInitialization(&dut, node.Kind)
		node.License = c.licenseInitialization(&dut, node.Kind)
		node.Image = c.imageInitialization(&dut, node.Kind)
		node.Group = c.groupInitialization(&dut, node.Kind)
		node.NodeType = c.typeInitialization(&dut, node.Kind)
		node.Position = c.positionInitialization(&dut, node.Kind)

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

		node.Mounts = make(map[string]volume)
		var v volume
		v.Source = c.Dir.Lab + "/" + "license.key"
		v.Destination = "/opt/srlinux/etc/license.key"
		v.ReadOnly = true
		log.Debug("License key: ", v.Source)
		node.Mounts["license"] = v

		v.Source = node.LabDir + "/" + "config/"
		v.Destination = "/etc/opt/srlinux/"
		v.ReadOnly = false
		log.Debug("Config: ", v.Source)
		node.Mounts["config"] = v

		v.Source = node.LabDir + "/" + "srlinux.conf"
		v.Destination = "/home/admin/.srlinux.conf"
		v.ReadOnly = false
		log.Debug("Env Config: ", v.Source)
		node.Mounts["envConf"] = v

		v.Source = node.LabDir + "/" + "topology.yml"
		v.Destination = "/tmp/topology.yml"
		v.ReadOnly = true
		log.Debug("Topology File: ", v.Source)
		node.Mounts["topology"] = v

		node.Volumes = make(map[string]struct{})
		node.Volumes = map[string]struct{}{
			node.Mounts["license"].Destination:  {},
			node.Mounts["config"].Destination:   {},
			node.Mounts["envConf"].Destination:  {},
			node.Mounts["topology"].Destination: {},
		}

		bindLicense := node.Mounts["license"].Source + ":" + node.Mounts["license"].Destination + ":" + "ro"
		bindConfig := node.Mounts["config"].Source + ":" + node.Mounts["config"].Destination + ":" + "rw"
		bindEnvConf := node.Mounts["envConf"].Source + ":" + node.Mounts["envConf"].Destination + ":" + "rw"
		bindTopology := node.Mounts["topology"].Source + ":" + node.Mounts["topology"].Destination + ":" + "ro"

		node.Binds = append(node.Binds, bindLicense)
		node.Binds = append(node.Binds, bindConfig)
		node.Binds = append(node.Binds, bindEnvConf)
		node.Binds = append(node.Binds, bindTopology)

	case "alpine", "linux":
		node.Config = c.configInitialization(&dut, node.Kind)
		node.License = c.licenseInitialization(&dut, node.Kind)
		node.Image = c.imageInitialization(&dut, node.Kind)
		node.Group = c.groupInitialization(&dut, node.Kind)
		node.NodeType = c.typeInitialization(&dut, node.Kind)
		node.Position = c.positionInitialization(&dut, node.Kind)

		node.Cmd = "/bin/bash"

	case "bridge":
		node.Group = c.groupInitialization(&dut, node.Kind)
		node.Position = c.positionInitialization(&dut, node.Kind)

	default:
		panic("Node Kind, OS is not properly initialized; should be provided in Duts.dut_specifics.kind parameters or Duts.global_defaults.kind")
	}
	c.Nodes[dutName] = node
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
		panic(fmt.Sprintf("endpoint %s has wrong syntax", e))
	}
	// search the node pointer based in the name of the split function
	for name, n := range c.Nodes {
		if name == split[0] {
			endpoint.Node = n
			break
		}
	}
	if endpoint.Node == nil {
		log.Fatalf("Not all nodes are specified in the duts section or the names don't match in the duts/endpoint section: %s", split[0])
	}

	// initialize the endpoint name based on the split function
	if c.Nodes[split[0]].Kind == "bridge" {
		endpoint.EndpointName = "veth" + c.Conf.Prefix + split[1]
	} else {
		endpoint.EndpointName = split[1]
	}
	if len(endpoint.EndpointName) > 15 {
		log.Fatalf("interface '%s' name too long > 15", endpoint.EndpointName)
	}
	return endpoint
}
