package types

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/cloudflare/cfssl/log"
	"github.com/docker/go-connections/nat"
	"github.com/srl-labs/containerlab/utils"
)

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
	// mac address
	MAC string
}

// mgmtNet struct defines the management network options
// it is provided via docker network object
type MgmtNet struct {
	Network    string `yaml:"network,omitempty"` // docker network name
	Bridge     string // linux bridge backing the docker network
	IPv4Subnet string `yaml:"ipv4_subnet,omitempty"`
	IPv6Subnet string `yaml:"ipv6_subnet,omitempty"`
	MTU        string `yaml:"mtu,omitempty"`
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
	MacAddress           string
	ContainerID          string
	TLSCert              string
	TLSKey               string
	TLSAnchor            string
	NSPath               string   // network namespace path for this node
	Publish              []string //list of ports to publish with mysocketctl
	// container labels
	Labels map[string]string
	// Slice of pointers to local endpoints
	Endpoints []*Endpoint
}

// GenerateConfig generates configuration for the nodes
func (node *Node) GenerateConfig(dst, defaultTemplatePath string) error {
	if utils.FileExists(dst) && (node.Config == defaultTemplatePath) {
		log.Debugf("config file '%s' for node '%s' already exists and will not be generated", dst, node.ShortName)
		return nil
	}
	log.Debugf("generating config for node %s from file %s", node.ShortName, node.Config)
	tpl, err := template.New(filepath.Base(node.Config)).ParseFiles(node.Config)
	if err != nil {
		return err
	}
	dstBytes := new(bytes.Buffer)
	err = tpl.Execute(dstBytes, node)
	if err != nil {
		return err
	}
	log.Debugf("node '%s' generated config: %s", node.ShortName, dstBytes.String())
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(dstBytes.Bytes())
	return err
}

// Data struct storing generic container data
type GenericContainer struct {
	Names           []string
	ID              string
	Image           string
	State           string
	Status          string
	Labels          map[string]string
	Pid             int
	NetworkSettings *GenericMgmtIPs
}

type GenericMgmtIPs struct {
	Set      bool
	IPv4addr string
	IPv4pLen int
	IPv6addr string
	IPv6pLen int
}
