package sros

// srosTemplateData holds all data needed for template selection and execution
type srosTemplateData struct {
	// Template selection criteria
	NodeType          string
	ConfigurationMode string
	SwVersion         *SrosVersion
	IsSecureGrpc      bool

	// Node identification
	Name string

	// Certificate data
	TLSKey    string
	TLSCert   string
	TLSAnchor string

	// Banner and SSH
	Banner          string
	SSHPubKeysRSA   []string
	SSHPubKeysECDSA []string

	// Network configuration
	IFaces     map[string]tplIFace
	MgmtMTU    int
	MgmtIPMTU  int
	DNSServers []string

	// Service configurations (populated based on node type and security)
	SystemConfig    string
	SNMPConfig      string
	GRPCConfig      string
	NetconfConfig   string
	LoggingConfig   string
	SSHConfig       string
	ComponentConfig string
}

// tplIFace template interface struct.
type tplIFace struct {
	Slot       string
	Port       string
	BreakoutNo string
	Mtu        int
}
