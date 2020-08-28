package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type dockerInfo struct {
	Bridge      string `yaml:"bridge"`
	Ipv4Subnet  string `yaml:"ipv4_subnet"`
	Ipv4Gateway string `yaml:"ipv4_gateway"`
	Ipv6Subnet  string `yaml:"ipv6_subnet"`
	Ipv6Gateway string `yaml:"ipv6_gateway"`
}

type conf struct {
	Prefix      string              `yaml:"Prefix"`
	DockerInfo  dockerInfo          `yaml:"Docker_info"`
	SRLImage    string              `yaml:"SRL_image"`
	ClientImage string              `yaml:"Client_image"`
	SRLConfig   string              `yaml:"SRL_config"`
	SRLLicense  string              `yaml:"SRL_license"`
	Duts        map[string][]string `yaml:"Duts"`
	Links       []struct {
		Endpoints []string `yaml:"endpoints"`
	} `yaml:"Links"`
}

type volume struct {
	Source      string
	Destination string
	ReadOnly    bool
}

// Node is a struct that contains the information of a container element
type Node struct {
	Name       string
	Index      int
	Group      string
	OS         string
	Config     string
	NodeType   string
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
}

// Link is a struct that contains the information of a link between 2 containers
type Link struct {
	a *Endpoint
	b *Endpoint
}

// Endpoint is a sttruct that contains information of a link endpoint
type Endpoint struct {
	Node *Node
	// e1-x, eth, etc
	EndpointName string
}

// Prefix variables to store prefix of the lab
var Prefix string

// DockerInfo variable to store the docker networking information
var DockerInfo dockerInfo

// Nodes variable stores all the node information
var Nodes map[string]*Node

// Links variable stores all the link information
var Links map[int]*Link

// Path variables stores the absolute path of the lab topology structure
var Path string

func parseIPInfo(t *conf) error {
	DockerInfo = t.DockerInfo
	if DockerInfo.Bridge == "" {
		DockerInfo.Bridge = "srlinux_bridge"
	}
	if DockerInfo.Ipv4Subnet == "" {
		DockerInfo.Bridge = "172.19.19.0/24"
	}
	if DockerInfo.Ipv6Subnet == "" {
		DockerInfo.Bridge = "2001:172:19:19::/80"
	}

	_, ipv4Net, err := net.ParseCIDR(DockerInfo.Ipv4Subnet)
	if err != nil {
		return err
	}
	ipv4Gateway := ipv4Net.IP.To4()
	ipv4Gateway[3]++
	DockerInfo.Ipv4Gateway = ipv4Gateway.String()

	_, ipv6Net, err := net.ParseCIDR(DockerInfo.Ipv6Subnet)
	if err != nil {
		return err
	}
	ipv6Gateway := ipv6Net.IP
	ipv6Gateway[15]++
	DockerInfo.Ipv6Gateway = ipv6Gateway.String()

	return nil
}

func parseTopology(t *conf) error {
	// initialize Prefix
	Prefix = t.Prefix
	log.Debug(fmt.Sprintf("Prefix: %s", Prefix))
	// initialize DockerInfo
	err := parseIPInfo(t)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("DockerInfo: %v", DockerInfo))

	Path, _ = filepath.Abs(os.Getenv("PWD"))

	// initialize Nodes and Links variable
	Nodes = make(map[string]*Node)
	Links = make(map[int]*Link)

	// initialize the Node information from the topology map
	idx := 0
	for dut, data := range t.Duts {
		Nodes[dut] = NewNode(t, dut, data, idx)
		idx++
	}
	for i, l := range t.Links {
		// i represnts the endpoint integer and l provide the lik struct
		Links[i] = NewLink(l.Endpoints)
	}
	return nil
}

// NewNode initializes a new node object
func NewNode(t *conf, dutName string, data []string, idx int) *Node {
	// initialize a new node
	node := new(Node)
	node.Name = dutName
	node.Index = idx
	// normalize the data to lower case to compare
	node.OS = strings.ToLower(data[0])
	switch node.OS {
	case "srl":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = t.SRLConfig
		node.License = t.SRLLicense
		node.Image = t.SRLImage
		node.NodeType = "ixr6"
		//

		node.Cmd = "sudo bash -c /opt/srlinux/bin/sr_linux"
		node.Env = []string{"SRLINUX=1"}
		node.User = "root"

		for i, d := range data {
			switch i {
			case 1:
				// 2nd element in the string slice, should be the node group
				if d != "" {
					node.Group = d
				}
			case 2:
				// 3rd element in the string slice, should be the node type
				node.NodeType = d
				switch node.NodeType {
				case "ixr6":
					node.Topology = "srl_config/templates/topology-7250IXR6.yml"
				case "ixr10":
					node.Topology = "srl_config/templates/topology-7250IXR10.yml"
				case "ixrd1":
					node.Topology = "srl_config/templates/topology-7220IXD1.yml"
				case "isrd2":
					node.Topology = "srl_config/templates/topology-7220IXD2.yml"
				case "ixrd3":
					node.Topology = "srl_config/templates/topology-7220IXD3.yml"
				default:
					log.Error("wrong node type; should be ixr6, ixr10, ixrd1, ixrd2, ixrd3")
					os.Exit(1)
				}
			case 3:
				// 4th element in the string slice, should be config
				// if there is a more specific config override the global config
				if d != "" {
					node.Config = d
				}
			default:
			}
		}

		// err := createNodeDirStructure(node, dutName)
		// if err != nil {
		// 	log.Error(err)
		// }

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv4.ip_forward"] = "0"
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
		node.Sysctls["net.ipv6.conf.all.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.default.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.all.autoconf"] = "0"
		node.Sysctls["net.ipv6.conf.default.autoconf"] = "0"

		node.Mounts = make(map[string]volume)
		var v volume
		labPath := Path + "/" + "lab" + "_" + Prefix + "/"
		labDutPath := labPath + dutName + "/"
		v.Source = labPath + "license.key"
		v.Destination = "/opt/srlinux/etc/license.key"
		v.ReadOnly = true
		log.Debug("License key: ", v.Source)
		node.Mounts["license"] = v

		v.Source = labDutPath + "config/"
		v.Destination = "/etc/opt/srlinux/"
		v.ReadOnly = false
		log.Debug("Config: ", v.Source)
		node.Mounts["config"] = v

		v.Source = labDutPath + "srlinux.conf"
		v.Destination = "/home/admin/.srlinux.conf"
		v.ReadOnly = false
		log.Debug("Env Config: ", v.Source)
		node.Mounts["envConf"] = v

		// v.Source = labDutPath + "tls/"
		// v.Destination = "/etc/opt/srlinux/tls/"
		// v.ReadOnly = false
		// log.Debug("TLS Dir: ", v.Source)
		// node.Mounts["tls"] = v

		// v.Source = labDutPath + "checkpoint/"
		// v.Destination = "/etc/opt/srlinux/checkpoint/"
		// v.ReadOnly = false
		// log.Debug("checkPoint Dir: ", v.Source)
		// node.Mounts["checkPoint"] = v

		v.Source = labDutPath + "topology.yml"
		v.Destination = "/tmp/topology.yml"
		v.ReadOnly = true
		log.Debug("Topology File: ", v.Source)
		node.Mounts["topology"] = v

		node.Volumes = make(map[string]struct{})
		node.Volumes = map[string]struct{}{
			node.Mounts["license"].Destination:    struct{}{},
			node.Mounts["config"].Destination:    struct{}{},
			node.Mounts["envConf"].Destination:    struct{}{},
			node.Mounts["topology"].Destination:   struct{}{},
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
		node.Image = t.ClientImage
		node.Cmd = "/bin/bash"

		for i, d := range data {
			switch i {
			case 1:
				// 2nd element in the string slice, should be group
				// if there is a more specific config override the global config
				if d != "" {
					node.Group = d
				}
			case 2:
				// 3rd element not used for now
			case 3:
				// 4th element not used for now
			default:
			}
		}
	}
	return node
}

// NewLink initializes a new link object
func NewLink(e []string) *Link {
	// initialize a new link
	link := new(Link)

	for i, d := range e {
		// i indicates the number and d presents the string, which need to be
		// split in node and endpoint name
		if i == 0 {
			link.a = NewEndpoint(d)
		} else {
			link.b = NewEndpoint(d)
		}
	}
	return link
}

// NewEndpoint initializes a new endpoint object
func NewEndpoint(e string) *Endpoint {
	// initialize a new endpoint
	endpoint := new(Endpoint)

	// split the string to get node name and endpoint name
	split := strings.Split(e, ":")
	// search the node pointer based in the name of the split function
	found := false
	for name, n := range Nodes {
		if name == split[0] {
			endpoint.Node = n
			found = true
		} else {

		}
	}
	if !found {
		log.Error("Not all nodes are specified in the duts section or the names dont match in the duts/endpoint section", split[0])
		os.Exit(1)
	}

	// initialize the endpoint name based on the split function
	endpoint.EndpointName = split[1]

	return endpoint
}
