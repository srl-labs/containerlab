package types

type rawMacVXType struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
	NodeInterfaceMAC string `yaml:"mac"`
}
