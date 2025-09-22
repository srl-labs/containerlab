package types

import (
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	importEnvsKey = "__IMPORT_ENVS"
)

// NodeDefinition represents a configuration a given node can have in the lab definition file.
type NodeDefinition struct {
	Kind                  string            `yaml:"kind,omitempty"`
	Group                 string            `yaml:"group,omitempty"`
	Type                  string            `yaml:"type,omitempty"`
	StartupConfig         string            `yaml:"startup-config,omitempty"`
	StartupDelay          uint              `yaml:"startup-delay,omitempty"`
	EnforceStartupConfig  *bool             `yaml:"enforce-startup-config,omitempty"`
	SuppressStartupConfig *bool             `yaml:"suppress-startup-config,omitempty"`
	AutoRemove            *bool             `yaml:"auto-remove,omitempty"`
	RestartPolicy         string            `yaml:"restart-policy,omitempty"`
	Config                *ConfigDispatcher `yaml:"config,omitempty"`
	Image                 string            `yaml:"image,omitempty"`
	ImagePullPolicy       string            `yaml:"image-pull-policy,omitempty"`
	License               string            `yaml:"license,omitempty"`
	Position              string            `yaml:"position,omitempty"`
	Entrypoint            string            `yaml:"entrypoint,omitempty"`
	Cmd                   string            `yaml:"cmd,omitempty"`
	// list of commands to run in container
	Exec []string `yaml:"exec,omitempty"`
	// list of bind mount compatible strings
	Binds []string `yaml:"binds,omitempty"`
	// list of devices to map in the container
	Devices []string `yaml:"devices,omitempty"`
	// List of capabilities to add for the container
	CapAdd []string `yaml:"cap-add,omitempty"`
	// Set the shared memory size allocated to the container
	ShmSize string `yaml:"shm-size,omitempty"`
	// list of port bindings
	Ports []string `yaml:"ports,omitempty"`
	// user-defined IPv4 address in the management network
	MgmtIPv4 string `yaml:"mgmt-ipv4,omitempty"`
	// user-defined IPv6 address in the management network
	MgmtIPv6 string `yaml:"mgmt-ipv6,omitempty"`
	// environment variables
	Env map[string]string `yaml:"env,omitempty"`
	// external file containing environment variables
	EnvFiles []string `yaml:"env-files,omitempty"`
	// linux user used in a container
	User string `yaml:"user,omitempty"`
	// container labels
	Labels map[string]string `yaml:"labels,omitempty"`
	// container networking mode. if set to `host` the host networking will be used for this node,
	//  else bridged network
	NetworkMode string `yaml:"network-mode,omitempty"`
	// Ignite sandbox and kernel imageNames
	Sandbox string `yaml:"sandbox,omitempty"`
	Kernel  string `yaml:"kernel,omitempty"`
	// Override container runtime
	Runtime string `yaml:"runtime,omitempty"`
	// Set node CPU (cgroup or hypervisor)
	CPU float64 `yaml:"cpu,omitempty"`
	// Set node CPUs to use
	CPUSet string `yaml:"cpu-set,omitempty"`
	// Set node Memory (cgroup or hypervisor)
	Memory string `yaml:"memory,omitempty"`
	// Set the nodes Sysctl
	Sysctls map[string]string `yaml:"sysctls,omitempty"`
	// Extra options, may be kind specific
	Extras *Extras `yaml:"extras,omitempty"`
	// Deployment stages
	Stages *Stages `yaml:"stages,omitempty"`
	// DNS configuration
	DNS *DNSConfig `yaml:"dns,omitempty"`
	// Certificate configuration
	Certificate *CertificateConfig `yaml:"certificate,omitempty"`
	// Healthcheck configuration
	HealthCheck *HealthcheckConfig `yaml:"healthcheck,omitempty"`
	// Network aliases
	Aliases    []string     `yaml:"aliases,omitempty"`
	Components []*Component `yaml:"components,omitempty"`
}

// Interface compliance.
var _ yaml.Unmarshaler = &NodeDefinition{}

// UnmarshalYAML is a custom unmarshaler for NodeDefinition type that allows to map old attributes
// to new ones.
func (n *NodeDefinition) UnmarshalYAML(unmarshal func(any) error) error {
	// define an alias type to avoid recursion during unmarshaling
	type NodeDefinitionAlias NodeDefinition

	// NodeDefinitionWithDeprecatedFields can contain fields that are deprecated
	// but still supported for backward compatibility.
	// see https://github.com/srl-labs/containerlab/blob/6eb44ca5a64cb427a5c43e31c35ac50145a8397f \
	// /types/node_definition.go#L96
	// for how it was used.
	type NodeDefinitionWithDeprecatedFields struct {
		NodeDefinitionAlias `yaml:",inline"`
	}

	nd := &NodeDefinitionWithDeprecatedFields{}

	nd.NodeDefinitionAlias = NodeDefinitionAlias(*n)
	if err := unmarshal(nd); err != nil {
		return err
	}

	*n = NodeDefinition(nd.NodeDefinitionAlias)

	return nil
}

// ImportEnvs imports all environment variables defined in the shell
// if __IMPORT_ENVS is set to true.
func (n *NodeDefinition) ImportEnvs() {
	if n == nil || n.Env == nil {
		return
	}

	var importEnvs bool

	for k, v := range n.Env {
		if k == importEnvsKey && v == "true" {
			importEnvs = true
			break
		}
	}

	if !importEnvs {
		return
	}

	for _, e := range os.Environ() {
		kv := strings.Split(e, "=")
		if _, exists := n.Env[kv[0]]; exists {
			continue
		}

		n.Env[kv[0]] = kv[1]
	}
}
