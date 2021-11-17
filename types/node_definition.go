package types

import (
	"os"
	"strings"
)

const (
	importEnvsKey = "__IMPORT_ENVS"
)

// NodeDefinition represents a configuration a given node can have in the lab definition file
type NodeDefinition struct {
	Kind                 string            `yaml:"kind,omitempty"`
	Group                string            `yaml:"group,omitempty"`
	Type                 string            `yaml:"type,omitempty"`
	StartupConfig        string            `yaml:"startup-config,omitempty"`
	StartupDelay         uint              `yaml:"startup-delay,omitempty"`
	EnforceStartupConfig bool              `yaml:"enforce-startup-config,omitempty"`
	Config               *ConfigDispatcher `yaml:"config,omitempty"`
	Image                string            `yaml:"image,omitempty"`
	License              string            `yaml:"license,omitempty"`
	Position             string            `yaml:"position,omitempty"`
	Entrypoint           string            `yaml:"entrypoint,omitempty"`
	Cmd                  string            `yaml:"cmd,omitempty"`
	// list of commands to run in container
	Exec []string `yaml:"exec,omitempty"`
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

	// Extra options, may be kind specific
	Extras *Extras `yaml:"extras,omitempty"`
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

func (n *NodeDefinition) GetEnforceStartupConfig() bool {
	if n == nil {
		return false
	}
	return n.EnforceStartupConfig

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

func (n *NodeDefinition) GetPublish() []string {
	if n == nil {
		return nil
	}
	return n.Publish
}

func (n *NodeDefinition) GetEnv() map[string]string {
	if n == nil {
		return nil
	}
	return n.Env
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

func (n *NodeDefinition) GetExtras() *Extras {
	if n == nil {
		return nil
	}
	return n.Extras
}

// ImportEnvs imports all environment variales defined in the shell
// if __IMPORT_ENVS is set to true
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
