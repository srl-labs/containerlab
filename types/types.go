// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package types

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

// Link is a struct that contains the information of a link between 2 containers
type Link struct {
	A      *Endpoint
	B      *Endpoint
	MTU    int
	Labels map[string]string
	Vars   map[string]interface{}
}

func (link *Link) String() string {
	return fmt.Sprintf("link [%s:%s, %s:%s]", link.A.Node.ShortName,
		link.A.EndpointName, link.B.Node.ShortName, link.B.EndpointName)
}

// Endpoint is a struct that contains information of a link endpoint
type Endpoint struct {
	Node *NodeConfig
	// e1-x, eth, etc
	EndpointName string
	// mac address
	MAC string
}

// MgmtNet struct defines the management network options
type MgmtNet struct {
	Network        string `yaml:"network,omitempty" json:"network,omitempty"` // container runtime network name
	Bridge         string `yaml:"bridge,omitempty" json:"bridge,omitempty"`   // linux bridge backing the runtime network
	IPv4Subnet     string `yaml:"ipv4_subnet,omitempty" json:"ipv4-subnet,omitempty"`
	IPv4Gw         string `yaml:"ipv4-gw,omitempty" json:"ipv4-gw,omitempty"`
	IPv6Subnet     string `yaml:"ipv6_subnet,omitempty" json:"ipv6-subnet,omitempty"`
	IPv6Gw         string `yaml:"ipv6-gw,omitempty" json:"ipv6-gw,omitempty"`
	MTU            string `yaml:"mtu,omitempty" json:"mtu,omitempty"`
	ExternalAccess *bool  `yaml:"external-access,omitempty" json:"external-access,omitempty"`
}

// NodeConfig is a struct that contains the information of a container element
type NodeConfig struct {
	ShortName            string            `json:"shortname,omitempty"` // name of the Node inside topology YAML
	LongName             string            `json:"longname,omitempty"`  // containerlab-prefixed unique container name
	Fqdn                 string            `json:"fqdn,omitempty"`
	LabDir               string            `json:"labdir,omitempty"` // LabDir is a directory related to the node, it contains config items and/or other persistent state
	Index                int               `json:"index,omitempty"`
	Group                string            `json:"group,omitempty"`
	Kind                 string            `json:"kind,omitempty"`
	StartupConfig        string            `json:"startup-config,omitempty"`          // path to config template file that is used for startup config generation
	StartupDelay         uint              `json:"startup-delay,omitempty"`           // optional delay (in seconds) to wait before creating this node
	EnforceStartupConfig bool              `json:"enforce-startup-config,omitempty"`  // when set to true will enforce the use of startup-config, even when config is present in the lab directory
	ResStartupConfig     string            `json:"startup-config-abs-path,omitempty"` // path to config file that is actually mounted to the container and is a result of templation
	Config               *ConfigDispatcher `json:"config,omitempty"`
	ResConfig            string            `json:"config-abs-path,omitempty"` // path to config file that is actually mounted to the container and is a result of templation
	NodeType             string            `json:"type,omitempty"`
	Position             string            `json:"position,omitempty"`
	License              string            `json:"license,omitempty"`
	Image                string            `json:"image,omitempty"`
	Sysctls              map[string]string `json:"sysctls,omitempty"`
	User                 string            `json:"user,omitempty"`
	Entrypoint           string            `json:"entrypoint,omitempty"`
	Cmd                  string            `json:"cmd,omitempty"`
	Exec                 []string          `json:"exec,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	Binds                []string          `json:"binds,omitempty"`        // Bind mounts strings (src:dest:options).
	PortBindings         nat.PortMap       `json:"portbindings,omitempty"` // PortBindings define the bindings between the container ports and host ports
	PortSet              nat.PortSet       `json:"portset,omitempty"`      // PortSet define the ports that should be exposed on a container
	NetworkMode          string            `json:"networkmode,omitempty"`  // container networking mode. if set to `host` the host networking will be used for this node, else bridged network
	MgmtNet              string            `json:"mgmt-net,omitempty"`     // name of the docker network this node is connected to with its first interface
	MgmtIntf             string            `json:"mgmt-intf,omitempty"`    // can be used to be rendered by the default node template
	MgmtIPv4Address      string            `json:"mgmt-ipv4-address,omitempty"`
	MgmtIPv4PrefixLength int               `json:"mgmt-ipv4-prefix-length,omitempty"`
	MgmtIPv6Address      string            `json:"mgmt-ipv6-address,omitempty"`
	MgmtIPv6PrefixLength int               `json:"mgmt-ipv6-prefix-length,omitempty"`
	MacAddress           string            `json:"mac-address,omitempty"`
	ContainerID          string            `json:"containerid,omitempty"`
	TLSCert              string            `json:"tls-cert,omitempty"`
	TLSKey               string            `json:"-"` // Do not marshal into JSON - highly sensitive data
	TLSAnchor            string            `json:"tls-anchor,omitempty"`
	NSPath               string            `json:"nspath,omitempty"`      // network namespace path for this node
	Publish              []string          `json:"publish,omitempty"`     // list of ports to publish with mysocketctl
	ExtraHosts           []string          `json:"extra-hosts,omitempty"` // Extra /etc/hosts entries for all nodes.
	Labels               map[string]string `json:"labels,omitempty"`      // container labels
	Endpoints            []Endpoint        `json:"-"`                     // Slice of pointers to local endpoints, DO NOT marshal into JSON as it creates a cyclical error
	// Ignite sandbox and kernel imageNames
	Sandbox string `json:"sandbox,omitempty"`
	Kernel  string `json:"kernel,omitempty"`
	// Configured container runtime
	Runtime string `json:"runtime,omitempty"`
	// Resource limits
	CPU    float64 `json:"cpu,omitempty"`
	CPUSet string  `json:"cpuset,omitempty"`
	Memory string  `json:"memory,omitempty"`
	// Host requirements
	HostRequirements HostRequirements `json:"host-requirements,omitempty"`

	DeploymentStatus string `json:"deployment-status,omitempty"` // status that is set by containerlab to indicate deployment stage

	// Extras
	Extras *Extras `json:"extras,omitempty"` // Extra node parameters
}

type HostRequirements struct {
	SSE3 bool `json:"sse3,omitempty"` // sse3 cpu instruction
}

// GenerateConfig generates configuration for the nodes
// out of the template based on the node configuration and saves the result to dst
func (node *NodeConfig) GenerateConfig(dst, templ string) error {

	// If the config file is already present in the node dir
	// we do not regenerate the config unless EnforceStartupConfig is explicitly set to true and startup-config points to a file
	// this will persist the changes that users make to a running config when booted from some startup config
	if utils.FileExists(dst) && (node.StartupConfig == "" || !node.EnforceStartupConfig) {
		log.Infof("config file '%s' for node '%s' already exists and will not be generated/reset", dst, node.ShortName)
		return nil
	} else if node.EnforceStartupConfig {
		log.Infof("Startup config for '%s' node enforced: '%s'", node.ShortName, dst)
	}
	log.Debugf("generating config for node %s from file %s", node.ShortName, node.StartupConfig)
	tpl, err := template.New(filepath.Base(node.StartupConfig)).Parse(templ)
	if err != nil {
		return err
	}
	dstBytes := new(bytes.Buffer)
	err = tpl.Execute(dstBytes, node)
	if err != nil {
		return err
	}
	log.Debugf("node '%s' generated config: %s", node.ShortName, dstBytes.String())
	f, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666) // skipcq: GSC-G302
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(dstBytes.Bytes())
	return err
}

func DisableTxOffload(n *NodeConfig) error {
	// skip this if node runs in host mode
	if strings.ToLower(n.NetworkMode) == "host" {
		return nil
	}
	// disable tx checksum offload for linux containers on eth0 interfaces
	nodeNS, err := ns.GetNS(n.NSPath)
	if err != nil {
		return err
	}
	err = nodeNS.Do(func(_ ns.NetNS) error {
		// disabling offload on eth0 interface
		err := utils.EthtoolTXOff("eth0")
		if err != nil {
			log.Infof("Failed to disable TX checksum offload for 'eth0' interface for Linux '%s' node: %v", n.ShortName, err)
		}
		return err
	})
	return err
}

// Data struct storing generic container data
type GenericContainer struct {
	Names           []string
	ID              string
	ShortID         string // trimmed ID for display purposes
	Image           string
	State           string
	Status          string
	Labels          map[string]string
	Pid             int
	NetworkSettings GenericMgmtIPs
	Mounts          []ContainerMount
}

type ContainerMount struct {
	Source      string
	Destination string
}

func (ctr *GenericContainer) GetContainerIPv4() string {
	if ctr.NetworkSettings.IPv4addr == "" {
		return "N/A"
	}
	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv4addr, ctr.NetworkSettings.IPv4pLen)

}

func (ctr *GenericContainer) GetContainerIPv6() string {
	if ctr.NetworkSettings.IPv6addr == "" {
		return "N/A"
	}
	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv6addr, ctr.NetworkSettings.IPv6pLen)
}

type GenericMgmtIPs struct {
	IPv4addr string
	IPv4pLen int
	IPv6addr string
	IPv6pLen int
}

type GenericFilter struct {
	// defined by now "label"
	FilterType string
	// defines e.g. the label name for FilterType "label"
	Field string
	// = | != | exists
	Operator string
	// match value
	Match string
}

func FilterFromLabelStrings(labels []string) []*GenericFilter {
	gfl := []*GenericFilter{}
	var gf *GenericFilter
	for _, s := range labels {
		gf = &GenericFilter{
			FilterType: "label",
		}
		if strings.Contains(s, "=") {
			gf.Operator = "="
			subs := strings.Split(s, "=")
			gf.Field = strings.TrimSpace(subs[0])
			gf.Match = strings.TrimSpace(subs[1])
		} else {
			gf.Operator = "exists"
			gf.Field = strings.TrimSpace(s)
		}

		gfl = append(gfl, gf)
	}
	return gfl
}

// ConfigDispatcher represents the config of a configuration machine
// that is responsible to execute configuration commands on the nodes
// after they started
type ConfigDispatcher struct {
	Vars map[string]interface{} `yaml:"vars,omitempty"`
}

func (cd *ConfigDispatcher) GetVars() map[string]interface{} {
	if cd == nil {
		return nil
	}
	return cd.Vars
}

// Extras contains extra node parameters which are not entitled to be part of a generic node config
type Extras struct {
	SRLAgents       []string `yaml:"srl-agents,omitempty"`         // Nokia SR Linux agents. As of now just the agents spec files can be provided here
	MysocketProxy   string   `yaml:"mysocket-proxy,omitempty"`     // Proxy address that mysocketctl will use
	CeosCopyToFlash []string `yaml:"ceos-copy-to-flash,omitempty"` // paths to files which are to be copied to ceos flash dir
}

// ContainerDetails contains information that is commonly outputted to tables or graphs
type ContainerDetails struct {
	LabName     string `json:"lab_name,omitempty"`
	LabPath     string `json:"labPath,omitempty"`
	Name        string `json:"name,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	Image       string `json:"image,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Group       string `json:"group,omitempty"`
	State       string `json:"state,omitempty"`
	IPv4Address string `json:"ipv4_address,omitempty"`
	IPv6Address string `json:"ipv6_address,omitempty"`
}

type MySocketIoEntry struct {
	SocketId  *string `json:"socket_id,omitempty"`
	DnsName   *string `json:"dns_name,omitempty"`
	Ports     []int   `json:"ports,omitempty"`
	Type      *string `json:"type,omitempty"`
	CloudAuth bool    `json:"cloud_auth,omitempty"`
	Name      *string `json:"name,omitempty"`
}

type LabData struct {
	Containers []ContainerDetails `json:"containers"`
	MySocketIo []*MySocketIoEntry `json:"mysocketio"`
}
