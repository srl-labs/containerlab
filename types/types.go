// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package types

import (
	"fmt"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"github.com/srl-labs/containerlab/virt"
)

// Link is a struct that contains the information of a link between 2 containers.
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

// Endpoint is a struct that contains information of a link endpoint.
type Endpoint struct {
	Node *NodeConfig
	// e1-x, eth, etc
	EndpointName string
	// mac address
	MAC string
}

// MgmtNet struct defines the management network options.
type MgmtNet struct {
	Network string `yaml:"network,omitempty" json:"network,omitempty"` // container runtime network name
	Bridge  string `yaml:"bridge,omitempty" json:"bridge,omitempty"`
	// linux bridge backing the runtime network
	IPv4Subnet     string `yaml:"ipv4_subnet,omitempty" json:"ipv4-subnet,omitempty"`
	IPv4Gw         string `yaml:"ipv4-gw,omitempty" json:"ipv4-gw,omitempty"`
	IPv6Subnet     string `yaml:"ipv6_subnet,omitempty" json:"ipv6-subnet,omitempty"`
	IPv6Gw         string `yaml:"ipv6-gw,omitempty" json:"ipv6-gw,omitempty"`
	MTU            string `yaml:"mtu,omitempty" json:"mtu,omitempty"`
	ExternalAccess *bool  `yaml:"external-access,omitempty" json:"external-access,omitempty"`
}

// NodeConfig is a struct that contains the information of a container element.
type NodeConfig struct {
	// name of the Node inside topology YAML
	ShortName string `json:"shortname,omitempty"`
	// containerlab-prefixed unique container name
	LongName string `json:"longname,omitempty"`
	Fqdn     string `json:"fqdn,omitempty"`
	// LabDir is a directory related to the node, it contains config items and/or other persistent state
	LabDir string `json:"labdir,omitempty"`
	Index  int    `json:"index,omitempty"`
	Group  string `json:"group,omitempty"`
	Kind   string `json:"kind,omitempty"`
	// path to config template file that is used for startup config generation
	StartupConfig string `json:"startup-config,omitempty"`
	// optional delay (in seconds) to wait before creating this node
	StartupDelay uint `json:"startup-delay,omitempty"`
	// when set to true will enforce the use of startup-config, even when config is present in the lab directory
	EnforceStartupConfig bool `json:"enforce-startup-config,omitempty"`
	// when set to true will auto-remove a stopped/failed container
	AutoRemove *bool `json:"auto-remove,omitempty"`
	// path to config file that is actually mounted to the container and is a result of templation
	ResStartupConfig string            `json:"startup-config-abs-path,omitempty"`
	Config           *ConfigDispatcher `json:"config,omitempty"`
	// path to config file that is actually mounted to the container and is a result of templation
	ResConfig  string            `json:"config-abs-path,omitempty"`
	NodeType   string            `json:"type,omitempty"`
	Position   string            `json:"position,omitempty"`
	License    string            `json:"license,omitempty"`
	Image      string            `json:"image,omitempty"`
	Sysctls    map[string]string `json:"sysctls,omitempty"`
	User       string            `json:"user,omitempty"`
	Entrypoint string            `json:"entrypoint,omitempty"`
	Cmd        string            `json:"cmd,omitempty"`
	// Exec is a list of commands to execute inside the container backing the node.
	Exec []string          `json:"exec,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
	// Bind mounts strings (src:dest:options).
	Binds []string `json:"binds,omitempty"`
	// PortBindings define the bindings between the container ports and host ports
	PortBindings nat.PortMap `json:"portbindings,omitempty"`
	// PortSet define the ports that should be exposed on a container
	PortSet nat.PortSet `json:"portset,omitempty"`
	// NetworkMode defines container networking mode.
	// If set to `host` the host networking will be used for this node, else bridged network
	NetworkMode string `json:"networkmode,omitempty"`
	// MgmtNet is the name of the docker network this node is connected to with its first interface
	MgmtNet string `json:"mgmt-net,omitempty"`
	// MgmtIntf can be used to be rendered by the default node template
	MgmtIntf             string `json:"mgmt-intf,omitempty"`
	MgmtIPv4Address      string `json:"mgmt-ipv4-address,omitempty"`
	MgmtIPv4PrefixLength int    `json:"mgmt-ipv4-prefix-length,omitempty"`
	MgmtIPv6Address      string `json:"mgmt-ipv6-address,omitempty"`
	MgmtIPv6PrefixLength int    `json:"mgmt-ipv6-prefix-length,omitempty"`
	MgmtIPv4Gateway      string `json:"mgmt-ipv4-gateway,omitempty"`
	MgmtIPv6Gateway      string `json:"mgmt-ipv6-gateway,omitempty"`
	MacAddress           string `json:"mac-address,omitempty"`
	ContainerID          string `json:"containerid,omitempty"`
	TLSCert              string `json:"tls-cert,omitempty"`
	TLSKey               string `json:"-"` // Do not marshal into JSON - highly sensitive data
	TLSAnchor            string `json:"tls-anchor,omitempty"`
	NSPath               string `json:"nspath,omitempty"` // network namespace path for this node
	// list of ports to publish with mysocketctl
	Publish []string `json:"publish,omitempty"`
	// Extra /etc/hosts entries for all nodes.
	ExtraHosts []string          `json:"extra-hosts,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"` // container labels
	// Slice of pointers to local endpoints, DO NOT marshal into JSON as it creates a cyclical error
	Endpoints []Endpoint `json:"-"`
	// List of Subject Alternative Names (SAN) to be added to the node's TLS certificate
	SANs []string `json:"SANs,omitempty"`
	// Ignite sandbox and kernel imageNames
	Sandbox string `json:"sandbox,omitempty"`
	Kernel  string `json:"kernel,omitempty"`
	// Configured container runtime
	Runtime string `json:"runtime,omitempty"`
	// Resource limits
	CPU    float64 `json:"cpu,omitempty"`
	CPUSet string  `json:"cpuset,omitempty"`
	Memory string  `json:"memory,omitempty"`

	// status that is set by containerlab to indicate deployment stage
	DeploymentStatus string `json:"deployment-status,omitempty"`
	// Extra node parameters
	Extras  *Extras    `json:"extras,omitempty"`
	WaitFor []string   `json:"wait-for,omitempty"`
	DNS     *DNSConfig `json:"dns,omitempty"`
}

type HostRequirements struct {
	SSSE3        bool `json:"ssse3,omitempty"`         // ssse3 cpu instruction
	VirtRequired bool `json:"virt-required,omitempty"` // indicates that KVM virtualization is required for this node to run
}

// Verify checks if host requirements are met.
func (h *HostRequirements) Verify() error {
	if h.VirtRequired && !virt.VerifyVirtSupport() {
		return fmt.Errorf("the CPU virtualization support is required, but not available")
	}
	if h.SSSE3 && !virt.VerifySSSE3Support() {
		return fmt.Errorf("the SSSE3 CPU feature required, but not available")
	}

	return nil
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

// GenericContainer stores generic container data.
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
	IPv4Gw   string
	IPv6addr string
	IPv6pLen int
	IPv6Gw   string
}

type GenericFilter struct {
	// defined by now "label" / "name" [then only Match is required]
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
// after they started.
type ConfigDispatcher struct {
	Vars map[string]interface{} `yaml:"vars,omitempty"`
}

func (cd *ConfigDispatcher) GetVars() map[string]interface{} {
	if cd == nil {
		return nil
	}
	return cd.Vars
}

// Extras contains extra node parameters which are not entitled to be part of a generic node config.
type Extras struct {
	SRLAgents []string `yaml:"srl-agents,omitempty"`
	// Nokia SR Linux agents. As of now just the agents spec files can be provided here
	MysocketProxy string `yaml:"mysocket-proxy,omitempty"`
	// Proxy address that mysocketctl will use
	CeosCopyToFlash []string `yaml:"ceos-copy-to-flash,omitempty"`
	// paths to files which are to be copied to ceos flash dir
}

// ContainerDetails contains information that is commonly outputted to tables or graphs.
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
	LabName   *string `json:"lab_name,omitempty"`
}

// isClabEntry checks that the entry name contains clab
// then we assume its a clab entry.
func (mse *MySocketIoEntry) isClabEntry() bool {
	splitName := strings.Split(*mse.Name, "-")
	return strings.Contains(*mse.Name, "clab") && len(splitName) >= 4
}

// getContainerName deduce the containername from the name of the mysocketio entry
// precondition is that isClabEntry returned true.
// clab-slr01-srlnode1-tcp-22 -> slr01-srlnode1.
func (mse *MySocketIoEntry) getContainerName() (string, error) {
	splitName := strings.Split(*mse.Name, "-")
	if len(splitName) < 4 {
		return "", fmt.Errorf("issue with entry %s. does not seem to be a clab based mysocketio entry", *mse.Name)
	}
	return strings.Join(splitName[1:len(splitName)-2], "-"), nil
}

type LabData struct {
	Containers []ContainerDetails `json:"containers"`
	MySocketIo []*MySocketIoEntry `json:"mysocketio"`
}

// DNSConfig represents DNS configuration options a node has.
type DNSConfig struct {
	// DNS servers
	Servers []string `yaml:"servers,omitempty"`
	// DNS options
	Options []string `yaml:"options,omitempty"`
	// DNS Search Domains
	Search []string `yaml:"search,omitempty"`
}
