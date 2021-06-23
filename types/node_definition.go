package types

// NodeDefinition represents a configuration a given node can have in the lab definition file
type NodeDefinition struct {
	Kind     string `yaml:"kind,omitempty"`
	Group    string `yaml:"group,omitempty"`
	Type     string `yaml:"type,omitempty"`
	Config   string `yaml:"config,omitempty"`
	Image    string `yaml:"image,omitempty"`
	License  string `yaml:"license,omitempty"`
	Position string `yaml:"position,omitempty"`
	Cmd      string `yaml:"cmd,omitempty"`
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

func (n *NodeDefinition) GetConfig() string {
	if n == nil {
		return ""
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
