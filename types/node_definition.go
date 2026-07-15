package types

import (
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	importEnvsKey = "__IMPORT_ENVS"
)

// LinkApplyMode describes how apply should handle dataplane link changes for a
// node that already exists in the running lab.
type LinkApplyMode string

const (
	// LinkApplyModeRecreate deletes and creates the node so generated runtime
	// metadata, container env, binds, and startup files are rebuilt.
	LinkApplyModeRecreate LinkApplyMode = "recreate"
	// LinkApplyModeRestart adds/removes links first and then restarts the same
	// container object.
	LinkApplyModeRestart LinkApplyMode = "restart"
	// LinkApplyModeLive applies link changes directly without node lifecycle
	// actions.
	LinkApplyModeLive LinkApplyMode = "live"
)

// IsValid reports whether m is a supported explicit link apply mode. Empty is
// not a valid explicit mode; it means no override was configured.
func (m LinkApplyMode) IsValid() bool {
	switch m {
	case LinkApplyModeLive, LinkApplyModeRestart, LinkApplyModeRecreate:
		return true
	default:
		return false
	}
}

// NodeCredentials holds login material for SSH/NETCONF/GNMI/etc. (topology
// defaults/kinds/groups/nodes).
type NodeCredentials struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"-" yaml:"password,omitempty"`
	// IdentityFile is the path to the SSH private key used to authenticate against the node.
	// When set it is rendered as an IdentityFile directive in the generated ssh_config. Relative
	// paths are resolved against the topology directory and a leading ~ is expanded; the value is
	// double-quoted on render so spaces are handled.
	IdentityFile string `json:"identity-file,omitempty" yaml:"identity-file,omitempty"`
}

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
	// Hostname overrides the container hostname. When unset, the topology node name is used.
	Hostname   string `yaml:"hostname,omitempty"`
	Entrypoint string `yaml:"entrypoint,omitempty"`
	Cmd        string `yaml:"cmd,omitempty"`
	// list of commands to run in container
	Exec []string `yaml:"exec,omitempty"`
	// list of bind mount compatible strings
	Binds []string `yaml:"binds,omitempty"`
	// list of devices to map in the container
	Devices []string `yaml:"devices,omitempty"`
	// List of capabilities to add for the container
	CapAdd []string `yaml:"cap-add,omitempty"`
	// Run the container in privileged mode.
	Privileged *bool `yaml:"privileged,omitempty"`
	// Cgroup namespace mode for the container.
	CgroupnsMode string `yaml:"cgroupns-mode,omitempty"`
	// PID namespace mode for the container.
	PidMode string `yaml:"pid-mode,omitempty"`
	// Tmpfs mounts to add to the container, keyed by destination path.
	Tmpfs map[string]string `yaml:"tmpfs,omitempty"`
	// Security options to apply to the container runtime.
	SecurityOpts []string `yaml:"security-opts,omitempty"`
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
	// Credentials for SSH/NETCONF/GNMI/etc. (overrides kind default when set).
	Credentials NodeCredentials `yaml:"credentials,omitempty"`
	// Network aliases
	Aliases    []string     `yaml:"aliases,omitempty"`
	Components []*Component `yaml:"components,omitempty"`
	// how `containerlab apply` handles dataplane link changes for this node:
	// live, restart or recreate. Overrides the kind's own declaration.
	LinkApplyMode LinkApplyMode `yaml:"link-apply-mode,omitempty"`
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
	type NodeDefinitionWithDeprecatedFields struct {
		NodeDefinitionAlias `yaml:",inline"`
		LegacyUsername      string `yaml:"username,omitempty"`
		LegacyPassword      string `yaml:"password,omitempty"`
	}

	nd := &NodeDefinitionWithDeprecatedFields{}

	nd.NodeDefinitionAlias = NodeDefinitionAlias(*n)
	if err := unmarshal(nd); err != nil {
		return err
	}

	*n = NodeDefinition(nd.NodeDefinitionAlias)

	if nd.LegacyUsername != "" && n.Credentials.Username == "" {
		n.Credentials.Username = nd.LegacyUsername
	}
	if nd.LegacyPassword != "" && n.Credentials.Password == "" {
		n.Credentials.Password = nd.LegacyPassword
	}

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
		kv := strings.SplitN(e, "=", 2)
		if len(kv) < 2 {
			continue
		}
		if _, exists := n.Env[kv[0]]; exists {
			continue
		}

		n.Env[kv[0]] = kv[1]
	}
}
