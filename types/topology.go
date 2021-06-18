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
	Labels    map[string]string `yaml:"labels,omitempty"`
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
		if len(ndef.Binds) > 0 {
			return ndef.Binds
		}
		if t.Kinds != nil {
			if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
				return t.Kinds[ndef.Kind].Binds
			}
		}
		return t.Defaults.Binds
	}
	return nil
}

func (t *Topology) GetNodePorts(name string) (nat.PortSet, nat.PortMap, error) {
	if ndef, ok := t.Nodes[name]; ok {
		// node level ports
		if len(ndef.Ports) != 0 {
			ps, pb, err := nat.ParsePortSpecs(ndef.Ports)
			if err != nil {
				return nil, nil, err
			}
			return ps, pb, nil
		}
		// kind level ports
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if len(kdef.Ports) > 0 {
				ps, pb, err := nat.ParsePortSpecs(kdef.Ports)
				if err != nil {
					return nil, nil, err
				}
				return ps, pb, nil
			}
		}
		// default ports ?
		return nil, nil, nil
	}
	return nil, nil, nil
}

func (t *Topology) GetNodeEnv(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringMaps(
			utils.MergeStringMaps(t.GetDefaults().GetEnv(),
				t.GetKind(ndef.GetKind()).GetEnv()),
			ndef.Env)
	}
	return nil
}

func (t *Topology) GetNodePublish(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		if len(ndef.Publish) > 0 {
			return ndef.Publish
		}
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if len(kdef.Publish) > 0 {
				return kdef.Publish
			}
		}
		return t.Defaults.Publish
	}
	return nil
}

func (t *Topology) GetNodeLabels(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringMaps(
			utils.MergeStringMaps(t.Defaults.GetLabels(),
				t.GetKind(ndef.GetKind()).GetLabels()),
			ndef.Labels)
	}
	return nil
}

func (t *Topology) GetNodeConfig(name string) (string, error) {
	var cfg string
	if ndef, ok := t.Nodes[name]; ok {
		var err error
		cfg = ndef.Config
		if kdef, ok := t.Kinds[ndef.Kind]; ok && cfg == "" {
			cfg = kdef.Config
		}
		if cfg == "" {
			cfg = t.Defaults.Config
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

func (t *Topology) GetNodeLicense(name string) (string, error) {
	var license string
	if ndef, ok := t.Nodes[name]; ok {
		var err error
		license = ndef.License
		if license == "" {
			license = t.GetKind(ndef.GetKind()).GetLicense()
		}
		if license == "" {
			license = t.Defaults.GetLicense()
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
		if ndef.Image != "" {
			return ndef.Image
		}
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if kdef.Image != "" {
				return kdef.Image
			}
		}
		return t.Defaults.Image
	}
	return ""
}

func (t *Topology) GetNodeGroup(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.Group != "" {
			return ndef.Group
		}
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if kdef.Group != "" {
				return kdef.Group
			}
		}
		return t.Defaults.Group
	}
	return ""
}

func (t *Topology) GetNodeType(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.Type != "" {
			return ndef.Type
		}
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if kdef.Type != "" {
				return kdef.Type
			}
		}
		return t.Defaults.Type
	}
	return ""
}

func (t *Topology) GetNodePosition(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.Position != "" {
			return ndef.Position
		}
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if kdef.Position != "" {
				return kdef.Position
			}
		}
		return t.Defaults.Position
	}
	return ""
}

func (t *Topology) GetNodeCmd(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetCmd() != "" {
			return ndef.GetCmd()
		}
		if t.GetKind(ndef.GetKind()).GetCmd() != "" {
			return t.GetKind(ndef.GetKind()).GetCmd()
		}
		return t.Defaults.GetCmd()
	}
	return ""
}

func (t *Topology) GetNodeUser(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.User != "" {
			return ndef.User
		}
		if kdef, ok := t.Kinds[ndef.Kind]; ok && kdef != nil {
			if kdef.User != "" {
				return kdef.User
			}
		}
		return t.Defaults.User
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
