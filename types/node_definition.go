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
	// container networking mode. if set to `host` the host networking will be used for this node, else bridged network
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

// UnmarshalYAML is a custom unmarshaler for NodeDefinition type that allows to map old attributes to new ones.
func (n *NodeDefinition) UnmarshalYAML(unmarshal func(any) error) error {
	// define an alias type to avoid recursion during unmarshaling
	type NodeDefinitionAlias NodeDefinition

	// NodeDefinitionWithDeprecatedFields can contain fields that are deprecated
	// but still supported for backward compatibility.
	// see https://github.com/srl-labs/containerlab/blob/6eb44ca5a64cb427a5c43e31c35ac50145a8397f/types/node_definition.go#L96
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

func (n *NodeDefinition) GetKind() string {
	if n == nil {
		return ""
	}
	return n.Kind
}

func (n *NodeDefinition) GetGroup() string {
	if n == nil {
		return ""
	}
	return n.Group
}

func (n *NodeDefinition) GetType() string {
	if n == nil {
		return ""
	}
	return n.Type
}

func (n *NodeDefinition) GetStartupConfig() string {
	if n == nil {
		return ""
	}
	return n.StartupConfig
}

func (n *NodeDefinition) GetStartupDelay() uint {
	if n == nil {
		return 0
	}
	return n.StartupDelay
}

func (n *NodeDefinition) GetEnforceStartupConfig() *bool {
	if n == nil {
		return nil
	}
	return n.EnforceStartupConfig
}

func (n *NodeDefinition) GetSuppressStartupConfig() *bool {
	if n == nil {
		return nil
	}
	return n.SuppressStartupConfig
}

func (n *NodeDefinition) GetAutoRemove() *bool {
	if n == nil {
		return nil
	}
	return n.AutoRemove
}

func (n *NodeDefinition) GetRestartPolicy() string {
	if n == nil {
		return ""
	}
	return n.RestartPolicy
}

func (n *NodeDefinition) GetConfigDispatcher() *ConfigDispatcher {
	if n == nil {
		return nil
	}
	return n.Config
}

func (n *NodeDefinition) GetImage() string {
	if n == nil {
		return ""
	}
	return n.Image
}

func (n *NodeDefinition) GetImagePullPolicy() string {
	if n == nil {
		return ""
	}
	return n.ImagePullPolicy
}

func (n *NodeDefinition) GetLicense() string {
	if n == nil {
		return ""
	}
	return n.License
}

func (n *NodeDefinition) GetPostion() string {
	if n == nil {
		return ""
	}
	return n.Position
}

func (n *NodeDefinition) GetEntrypoint() string {
	if n == nil {
		return ""
	}
	return n.Entrypoint
}

func (n *NodeDefinition) GetCmd() string {
	if n == nil {
		return ""
	}
	return n.Cmd
}

func (n *NodeDefinition) GetBinds() []string {
	if n == nil {
		return nil
	}
	return n.Binds
}

func (n *NodeDefinition) GetDevices() []string {
	if n == nil {
		return nil
	}
	return n.Devices
}

func (n *NodeDefinition) GetCapAdd() []string {
	if n == nil {
		return nil
	}
	return n.CapAdd
}

func (n *NodeDefinition) GetComponents() []*Component {
	if n == nil {
		return nil
	}
	return n.Components
}

func (n *NodeDefinition) GetNodeShmSize() string {
	if n == nil {
		return ""
	}
	return n.ShmSize
}

func (n *NodeDefinition) GetPorts() []string {
	if n == nil {
		return nil
	}
	return n.Ports
}

func (n *NodeDefinition) GetMgmtIPv4() string {
	if n == nil {
		return ""
	}
	return n.MgmtIPv4
}

func (n *NodeDefinition) GetMgmtIPv6() string {
	if n == nil {
		return ""
	}
	return n.MgmtIPv6
}

func (n *NodeDefinition) GetEnv() map[string]string {
	if n == nil {
		return nil
	}
	return n.Env
}

func (n *NodeDefinition) GetEnvFiles() []string {
	if n == nil {
		return nil
	}
	return n.EnvFiles
}

func (n *NodeDefinition) GetUser() string {
	if n == nil {
		return ""
	}
	return n.User
}

func (n *NodeDefinition) GetLabels() map[string]string {
	if n == nil {
		return nil
	}
	return n.Labels
}

func (n *NodeDefinition) GetNetworkMode() string {
	if n == nil {
		return ""
	}
	return n.NetworkMode
}

func (n *NodeDefinition) GetNodeSandbox() string {
	if n == nil {
		return ""
	}
	return n.Sandbox
}

func (n *NodeDefinition) GetNodeKernel() string {
	if n == nil {
		return ""
	}
	return n.Kernel
}

func (n *NodeDefinition) GetNodeRuntime() string {
	if n == nil {
		return ""
	}
	return n.Runtime
}

func (n *NodeDefinition) GetNodeCPU() float64 {
	if n == nil {
		return 0
	}
	return n.CPU
}

func (n *NodeDefinition) GetNodeCPUSet() string {
	if n == nil {
		return ""
	}
	return n.CPUSet
}

func (n *NodeDefinition) GetNodeMemory() string {
	if n == nil {
		return ""
	}
	return n.Memory
}

func (n *NodeDefinition) GetExec() []string {
	if n == nil {
		return nil
	}
	return n.Exec
}

func (n *NodeDefinition) GetSysctls() map[string]string {
	if n == nil || n.Sysctls == nil {
		return map[string]string{}
	}

	return n.Sysctls
}

func (n *NodeDefinition) GetExtras() *Extras {
	if n == nil {
		return nil
	}
	return n.Extras
}

func (n *NodeDefinition) GetStages() *Stages {
	if n == nil {
		return nil
	}
	return n.Stages
}

func (n *NodeDefinition) GetDns() *DNSConfig {
	if n == nil {
		return nil
	}
	return n.DNS
}

func (n *NodeDefinition) GetCertificateConfig() *CertificateConfig {
	if n == nil {
		return nil
	}
	return n.Certificate
}

func (n *NodeDefinition) GetHealthcheckConfig() *HealthcheckConfig {
	if n == nil {
		return nil
	}
	return n.HealthCheck
}

func (n *NodeDefinition) GetAliases() []string {
	if n == nil {
		return nil
	}
	return n.Aliases
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
