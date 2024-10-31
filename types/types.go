// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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

// String returns a string representation of the endpoint.
func (e *Endpoint) String() string {
	return fmt.Sprintf("%s:%s", e.Node.ShortName, e.EndpointName)
}

// MgmtNet struct defines the management network options.
type MgmtNet struct {
	Network string `yaml:"network,omitempty" json:"network,omitempty"` // container runtime network name
	Bridge  string `yaml:"bridge,omitempty" json:"bridge,omitempty"`
	// linux bridge backing the runtime network
	IPv4Subnet     string `yaml:"ipv4-subnet,omitempty" json:"ipv4-subnet,omitempty"`
	IPv4Gw         string `yaml:"ipv4-gw,omitempty" json:"ipv4-gw,omitempty"`
	IPv4Range      string `yaml:"ipv4-range,omitempty" json:"ipv4-range,omitempty"`
	IPv6Subnet     string `yaml:"ipv6-subnet,omitempty" json:"ipv6-subnet,omitempty"`
	IPv6Gw         string `yaml:"ipv6-gw,omitempty" json:"ipv6-gw,omitempty"`
	IPv6Range      string `yaml:"ipv6-range,omitempty" json:"ipv6-range,omitempty"`
	MTU            int    `yaml:"mtu,omitempty" json:"mtu,omitempty"`
	ExternalAccess *bool  `yaml:"external-access,omitempty" json:"external-access,omitempty"`
}

// Interface compliance.
var _ yaml.Unmarshaler = &MgmtNet{}

// UnmarshalYAML is a custom unmarshaller for MgmtNet that allows to map old attributes to new ones.
func (m *MgmtNet) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// define an alias type to avoid recursion during unmarshalling
	type MgmtNetAlias MgmtNet

	type MgmtNetWithDeprecatedFields struct {
		MgmtNetAlias         `yaml:",inline"`
		DeprecatedIPv4Subnet string `yaml:"ipv4_subnet,omitempty" json:"ipv4_subnet,omitempty"`
		DeprecatedIPv6Subnet string `yaml:"ipv6_subnet,omitempty" json:"ipv6_subnet,omitempty"`
	}
	mn := &MgmtNetWithDeprecatedFields{}

	mn.MgmtNetAlias = (MgmtNetAlias)(*m)
	if err := unmarshal(mn); err != nil {
		return err
	}

	// process deprecated fields and use their values for new fields if new fields are not set
	if len(mn.DeprecatedIPv4Subnet) > 0 && len(mn.IPv4Subnet) == 0 {
		log.Warnf("Attribute \"ipv4_subnet\" is deprecated and will be removed in the future. Change it to \"ipv4-subnet\"")
		mn.IPv4Subnet = mn.DeprecatedIPv4Subnet
	}
	// map old to new if old defined but new not
	if len(mn.DeprecatedIPv6Subnet) > 0 && len(mn.IPv6Subnet) == 0 {
		log.Warnf("Attribute \"ipv6_subnet\" is deprecated and will be removed in the future. Change it to \"ipv6-subnet\"")
		mn.IPv6Subnet = mn.DeprecatedIPv6Subnet
	}

	*m = (MgmtNet)(mn.MgmtNetAlias)

	return nil
}

// NodeConfig contains information of a container element.
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
	// when set to true will prevent creation of a startup-config, for auto-provisioning testing (ZTP)
	SuppressStartupConfig bool `json:"suppress-startup-config,omitempty"`
	// when set to true will auto-remove a stopped/failed container
	AutoRemove    bool   `json:"auto-remove,omitempty"`
	RestartPolicy string `json:"restart-policy,omitempty"`
	// path to config file that is actually mounted to the container and is a result of templation
	ResStartupConfig string            `json:"startup-config-abs-path,omitempty"`
	Config           *ConfigDispatcher `json:"config,omitempty"`
	// path to config file that is actually mounted to the container and is a result of templation
	ResConfig       string            `json:"config-abs-path,omitempty"`
	NodeType        string            `json:"type,omitempty"`
	Position        string            `json:"position,omitempty"`
	License         string            `json:"license,omitempty"`
	Image           string            `json:"image,omitempty"`
	ImagePullPolicy PullPolicyValue   `json:"image-pull-policy,omitempty"`
	Sysctls         map[string]string `json:"sysctls,omitempty"`
	User            string            `json:"user,omitempty"`
	Entrypoint      string            `json:"entrypoint,omitempty"`
	Cmd             string            `json:"cmd,omitempty"`
	// Exec is a list of commands to execute inside the container backing the node.
	Exec []string          `json:"exec,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
	// Bind mounts strings (src:dest:options).
	Binds []string `json:"binds,omitempty"`
	// PortBindings define the bindings between the container ports and host ports
	PortBindings nat.PortMap `json:"portbindings,omitempty"`
	// ResultingPortBindings is a list of port bindings that are actually applied to the container
	ResultingPortBindings []*GenericPortBinding `json:"port-bindings,omitempty"`
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
	// TLS Certificate configuration
	Certificate *CertificateConfig
	// Healthcheck configuration parameters
	Healthcheck *HealthcheckConfig
	// Network aliases
	Aliases []string `json:"aliases,omitempty"`
	// NSPath      string `json:"nspath,omitempty"` // network namespace path for this node
	// list of ports to publish with mysocketctl
	Publish []string `json:"publish,omitempty"`
	// Extra /etc/hosts entries for all nodes.
	ExtraHosts []string          `json:"extra-hosts,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"` // container labels
	// Ignite sandbox and kernel imageNames
	Sandbox string `json:"sandbox,omitempty"`
	Kernel  string `json:"kernel,omitempty"`
	// Configured container runtime
	Runtime string `json:"runtime,omitempty"`
	// Resource limits
	CPU    float64 `json:"cpu,omitempty"`
	CPUSet string  `json:"cpuset,omitempty"`
	Memory string  `json:"memory,omitempty"`

	// Extra node parameters
	Extras *Extras    `json:"extras,omitempty"`
	Stages *Stages    `json:"stages,omitempty"`
	DNS    *DNSConfig `json:"dns,omitempty"`
	// Kind parameters
	//
	// IsRootNamespaceBased flag indicates that a certain nodes network
	// namespace (usually based on the kind) is the root network namespace
	IsRootNamespaceBased bool
	// SkipUniquenessCheck prevents the pre-deploy uniqueness check, where
	// we check, that the given node name is not already present on the host.
	// Introduced to prevent the check from running with ext-containers, since
	// they should be present by definition.
	SkipUniquenessCheck bool
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

// FilterFromLabelStrings creates a GenericFilter based on the list of label=value pairs or just label entries.
// A filter of type `label` is created.
// For each label=value input label, a filter with the Field matching the label and Match matching the value is created.
// For each standalone label, a filter with Operator=exists and Field matching the label is created.
func FilterFromLabelStrings(labels []string) []*GenericFilter {
	var gfl []*GenericFilter
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
	// Nokia SR Linux agents. As of now just the agents spec files can be provided here
	SRLAgents []string `yaml:"srl-agents,omitempty"`
	// Proxy address that mysocketctl will use
	MysocketProxy string `yaml:"mysocket-proxy,omitempty"`
	// paths to files which are to be copied to ceos flash dir
	CeosCopyToFlash []string `yaml:"ceos-copy-to-flash,omitempty"`
	// k8s-kind node specific options
	K8sKind *K8sKindExtras `yaml:"k8s_kind,omitempty"`
}

// K8sKindExtras represents the k8s-kind-specific extra options.
type K8sKindExtras struct {
	Deploy *K8sKindDeployExtras `yaml:"deploy,omitempty"`
}

// K8sKindDeployExtras represents the options used for the kind cluster creation.
// It is aligned with the `kind create cluster` command options, but exposes
// only the ones that are relevant for containerlab.
type K8sKindDeployExtras struct {
	Wait *string `yaml:"wait,omitempty"`
}

// ContainerDetails contains information that is commonly outputted to tables or graphs.
type ContainerDetails struct {
	LabName     string                `json:"lab_name,omitempty"`
	LabPath     string                `json:"labPath,omitempty"`
	Name        string                `json:"name,omitempty"`
	ContainerID string                `json:"container_id,omitempty"`
	Image       string                `json:"image,omitempty"`
	Kind        string                `json:"kind,omitempty"`
	Group       string                `json:"group,omitempty"`
	State       string                `json:"state,omitempty"`
	IPv4Address string                `json:"ipv4_address,omitempty"`
	IPv6Address string                `json:"ipv6_address,omitempty"`
	Ports       []*GenericPortBinding `json:"ports,omitempty"`
	Owner       string                `json:"owner,omitempty"`
}

// GenericPortBinding represents a port binding.
type GenericPortBinding struct {
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      int    `json:"host_port,omitempty"`
	ContainerPort int    `json:"port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

func (p *GenericPortBinding) String() string {
	var result string
	if strings.Contains(p.HostIP, ":") {
		result = fmt.Sprintf("[%s]", p.HostIP)
	} else {
		result = p.HostIP
	}
	result += fmt.Sprintf(":%d/%s -> %d", p.HostPort, p.Protocol, p.ContainerPort)
	return result
}

type LabData struct {
	Containers []ContainerDetails `json:"containers"`
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

// CertificateConfig represents TLS parameters set for a node.
type CertificateConfig struct {
	// default false value indicates that the node does not use TLS
	Issue *bool `yaml:"issue,omitempty"`
	// KeySize is the size of the key in bits
	KeySize int `yaml:"key-size,omitempty"`
	// ValidityDuration is the duration of the certificate validity
	ValidityDuration time.Duration `yaml:"validity-duration"`
	// list of subject Alternative Names (SAN) to be added to the node's certificate
	SANs []string `yaml:"sans,omitempty"`
}

// Merge merges the given CertificateConfig into the current one.
func (c *CertificateConfig) Merge(x *CertificateConfig) *CertificateConfig {
	if x == nil {
		return c
	}

	if x.ValidityDuration > 0 {
		c.ValidityDuration = x.ValidityDuration
	}

	if x.Issue != nil {
		c.Issue = x.Issue
	}

	if x.KeySize > 0 {
		c.KeySize = x.KeySize
	}

	if len(x.SANs) > 0 {
		c.SANs = x.SANs
	}

	return c
}

// PullPolicyValue represents Image pull policy values.
type PullPolicyValue string

const (
	PullPolicyAlways       PullPolicyValue = "Always"
	PullPolicyNever        PullPolicyValue = "Never"
	PullPolicyIfNotPresent PullPolicyValue = "IfNotPresent"
)

// ParsePullPolicyValue parses the given string and tries to map it to
// a valid PullPolicyValue. Defaults to PullPolicyIfNotPresent.
func ParsePullPolicyValue(s string) PullPolicyValue {
	// remove whitespace and convert to lower
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "always":
		return PullPolicyAlways
	case "never":
		return PullPolicyNever
	case "ifnotpresent":
		return PullPolicyIfNotPresent
	}

	// default to IfNotPresent
	return PullPolicyIfNotPresent
}

// HealthcheckConfig represents healthcheck parameters set for a container.
type HealthcheckConfig struct {
	// Test is the command to run to check the health of the container
	Test []string `yaml:"test,omitempty"`
	// Interval is the time to wait between checks in seconds
	Interval int `yaml:"interval,omitempty"`
	// Timeout is the time in seconds to wait before considering the check to have hung
	Timeout int `yaml:"timeout,omitempty"`
	// Retries is the number of consecutive failures needed to consider a container as unhealthy
	Retries int `yaml:"retries,omitempty"`
	// StartPeriod is the time to wait for the container to initialize
	// before starting health-retries countdown in seconds
	StartPeriod int `yaml:"start-period,omitempty"`
}

// GetIntervalDuration returns the interval as time.Duration.
func (h *HealthcheckConfig) GetIntervalDuration() time.Duration {
	return time.Duration(h.Interval) * time.Second
}

// GetTimeoutDuration returns the timeout as time.Duration.
func (h *HealthcheckConfig) GetTimeoutDuration() time.Duration {
	return time.Duration(h.Timeout) * time.Second
}

// GetStartPeriodDuration returns the start period as time.Duration.
func (h *HealthcheckConfig) GetStartPeriodDuration() time.Duration {
	return time.Duration(h.StartPeriod) * time.Second
}
