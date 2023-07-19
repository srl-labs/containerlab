package types

type EndpointRaw struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
	Mac   string `yaml:"mac,omitempty"`
}
