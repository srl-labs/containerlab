package types

type LinkConfig struct {
	Endpoints []string               `yaml:"endpoints"`
	Labels    map[string]string      `yaml:"labels,omitempty"`
	Vars      map[string]interface{} `yaml:"vars,omitempty"`
	MTU       int                    `yaml:"mtu,omitempty"`
}
