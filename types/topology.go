package types

import (
	"os"
	"path/filepath"

	"github.com/docker/go-connections/nat"
	"github.com/mitchellh/go-homedir"
	"github.com/srl-labs/containerlab/utils"
)

// Topology represents a lab topology
type Topology struct {
	Defaults *NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*NodeDefinition `yaml:"nodes,omitempty"`
	Links    []*LinkConfig              `yaml:"links,omitempty"`
}

func NewTopology() *Topology {
	return &Topology{
		Defaults: new(NodeDefinition),
		Kinds:    make(map[string]*NodeDefinition),
		Nodes:    make(map[string]*NodeDefinition),
		Links:    make([]*LinkConfig, 0),
	}
}

type LinkConfig struct {
	Endpoints []string
	Labels    map[string]string      `yaml:"labels,omitempty"`
	Vars      map[string]interface{} `yaml:"vars,omitempty"`
}

func (t *Topology) GetDefaults() *NodeDefinition {
	if t.Defaults != nil {
		return t.Defaults
	}
	return new(NodeDefinition)
}

func (t *Topology) GetKind(kind string) *NodeDefinition {
	if t.Kinds == nil {
		return new(NodeDefinition)
	}
	if kdef, ok := t.Kinds[kind]; ok {
		return kdef
	}
	return new(NodeDefinition)
}

func (t *Topology) GetKinds() map[string]*NodeDefinition {
	if t.Kinds == nil {
		return make(map[string]*NodeDefinition)
	}
	return t.Kinds
}

func (t *Topology) GetNodeKind(name string) string {
	if t == nil {
		return ""
	}
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetKind() != "" {
			return ndef.GetKind()
		}
		return t.GetDefaults().GetKind()
	}
	return ""
}

func (t *Topology) GetNodeBinds(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		if len(ndef.GetBinds()) > 0 {
			return ndef.GetBinds()
		}
		if len(t.GetKind(t.GetNodeKind(name)).GetBinds()) > 0 {
			return t.GetKind(t.GetNodeKind(name)).GetBinds()
		}
		return t.GetDefaults().GetBinds()
	}
	return nil
}

func (t *Topology) GetNodePorts(name string) (nat.PortSet, nat.PortMap, error) {
	if ndef, ok := t.Nodes[name]; ok {
		// node level ports
		if len(ndef.GetPorts()) != 0 {
			return nat.ParsePortSpecs(ndef.Ports)
		}
		// kind level ports
		if len(t.GetKind(t.GetNodeKind(name)).GetPorts()) > 0 {
			return nat.ParsePortSpecs(t.GetKind(t.GetNodeKind(name)).GetPorts())
		}
		// default level ports
		if len(t.GetDefaults().GetPorts()) > 0 {
			return nat.ParsePortSpecs(t.GetDefaults().GetPorts())
		}
	}
	return nil, nil, nil
}

func (t *Topology) GetNodeEnv(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringMaps(
			utils.MergeStringMaps(t.GetDefaults().GetEnv(),
				t.GetKind(t.GetNodeKind(name)).GetEnv()),
			ndef.GetEnv())
	}
	return nil
}

func (t *Topology) GetNodePublish(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		if len(ndef.GetPublish()) > 0 {
			return ndef.GetPublish()
		}
		if kdef, ok := t.Kinds[ndef.GetKind()]; ok && kdef != nil {
			if len(kdef.GetPublish()) > 0 {
				return kdef.GetPublish()
			}
		}
		return t.Defaults.GetPublish()
	}
	return nil
}

func (t *Topology) GetNodeLabels(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringMaps(t.Defaults.GetLabels(),
			t.GetKind(t.GetNodeKind(name)).GetLabels(),
			ndef.GetLabels())
	}
	return nil
}

func (t *Topology) GetNodeConfigDispatcher(name string) *ConfigDispatcher {
	if ndef, ok := t.Nodes[name]; ok {
		vars := utils.MergeMaps(t.Defaults.GetConfigDispatcher().GetVars(),
			t.GetKind(t.GetNodeKind(name)).GetConfigDispatcher().GetVars(),
			ndef.GetConfigDispatcher().GetVars())

		return &ConfigDispatcher{
			Vars: vars,
		}
	}

	return nil
}

func (t *Topology) GetNodeStartupConfig(name string) (string, error) {
	var cfg string
	if ndef, ok := t.Nodes[name]; ok {
		var err error
		cfg = ndef.GetStartupConfig()
		if t.GetKind(t.GetNodeKind(name)).GetStartupConfig() != "" && cfg == "" {
			cfg = t.GetKind(t.GetNodeKind(name)).GetStartupConfig()
		}
		if cfg == "" {
			cfg = t.GetDefaults().GetStartupConfig()
		}
		if cfg != "" {
			cfg, err = resolvePath(cfg)
			if err != nil {
				return "", err
			}
			_, err = os.Stat(cfg)
			return cfg, err
		}
	}
	return cfg, nil
}

func (t *Topology) GetNodeResetConfig(name string) bool {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetResetConfig() != nil {
			return *ndef.GetResetConfig()
		}
		kind_reset_config := t.GetKind(t.GetNodeKind(name)).GetResetConfig()
		if kind_reset_config != nil {
			return *kind_reset_config
		}
		default_reset_config := t.GetDefaults().GetResetConfig()
		if default_reset_config != nil {
			return *default_reset_config
		}
	}
	return false
}

func (t *Topology) GetNodeLicense(name string) (string, error) {
	var license string
	if ndef, ok := t.Nodes[name]; ok {
		var err error
		license = ndef.GetLicense()
		if t.GetKind(t.GetNodeKind(name)).GetLicense() != "" && license == "" {
			license = t.GetKind(t.GetNodeKind(name)).GetLicense()
		}
		if license == "" {
			license = t.GetDefaults().GetLicense()
		}
		if license != "" {
			license, err = resolvePath(license)
			if err != nil {
				return "", err
			}
			_, err = os.Stat(license)
			return license, err
		}
	}
	return license, nil
}

func (t *Topology) GetNodeImage(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetImage() != "" {
			return ndef.GetImage()
		}
		if t.GetKind(t.GetNodeKind(name)).GetImage() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetImage()
		}
		return t.GetDefaults().GetImage()
	}
	return ""
}

func (t *Topology) GetNodeGroup(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetGroup() != "" {
			return ndef.GetGroup()
		}
		if t.GetKind(t.GetNodeKind(name)).GetGroup() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetGroup()
		}
		return t.GetDefaults().GetGroup()
	}
	return ""
}

func (t *Topology) GetNodeType(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetType() != "" {
			return ndef.GetType()
		}
		if t.GetKind(t.GetNodeKind(name)).GetType() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetType()
		}
		return t.GetDefaults().GetType()
	}
	return ""
}

func (t *Topology) GetNodePosition(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetPostion() != "" {
			return ndef.GetPostion()
		}
		if t.GetKind(t.GetNodeKind(name)).GetPostion() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetPostion()
		}
		return t.GetDefaults().GetPostion()
	}
	return ""
}

func (t *Topology) GetNodeCmd(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetCmd() != "" {
			return ndef.GetCmd()
		}
		if t.GetKind(t.GetNodeKind(name)).GetCmd() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetCmd()
		}
		return t.GetDefaults().GetCmd()
	}
	return ""
}

func (t *Topology) GetNodeUser(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetUser() != "" {
			return ndef.GetUser()
		}
		if t.GetKind(t.GetNodeKind(name)).GetUser() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetUser()
		}
		return t.GetDefaults().GetUser()
	}
	return ""
}

func (t *Topology) GetNodeNetworkMode(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNetworkMode() != "" {
			return ndef.GetNetworkMode()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNetworkMode() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNetworkMode()
		}
		return t.GetDefaults().GetNetworkMode()
	}
	return ""
}

func (t *Topology) GetNodeSandbox(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeSandbox() != "" {
			return ndef.GetNodeSandbox()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeSandbox() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeSandbox()
		}
		return t.GetDefaults().GetNodeSandbox()
	}
	return ""
}

func (t *Topology) GetNodeKernel(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeKernel() != "" {
			return ndef.GetNodeKernel()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeKernel() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeKernel()
		}
		return t.GetDefaults().GetNodeKernel()
	}
	return ""
}

func (t *Topology) GetNodeRuntime(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeRuntime() != "" {
			return ndef.GetNodeRuntime()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeRuntime() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeRuntime()
		}
		return t.GetDefaults().GetNodeRuntime()
	}
	return ""
}

func (t *Topology) GetNodeCPU(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeCPU() != "" {
			return ndef.GetNodeCPU()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeCPU() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeCPU()
		}
		return t.GetDefaults().GetNodeCPU()
	}
	return ""
}

func (t *Topology) GetNodeRAM(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeRAM() != "" {
			return ndef.GetNodeRAM()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeRAM() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeRAM()
		}
		return t.GetDefaults().GetNodeRAM()
	}
	return ""
}

//resolvePath resolves a string path by expanding `~` to home dir or getting Abs path for the given path
func resolvePath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	var err error
	switch {
	// resolve ~/ path
	case p[0] == '~':
		p, err = homedir.Expand(p)
		if err != nil {
			return "", err
		}
	default:
		p, err = filepath.Abs(p)
		if err != nil {
			return "", err
		}
	}
	return p, nil
}
