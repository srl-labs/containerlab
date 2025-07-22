package sros

// srosTemplateData top level data struct.
type srosTemplateData struct {
	Name            string
	TLSKey          string
	TLSCert         string
	TLSAnchor       string
	Banner          string
	IFaces          map[string]tplIFace
	SSHPubKeysRSA   []string
	SSHPubKeysECDSA []string
	MgmtMTU         int
	MgmtIPMTU       int
	DNSServers      []string
	NodeType        string
	// SNMPConfig is a string containing SNMP configuration
	SNMPConfig string
	// GRPCConfig is a string containing GRPC configuration
	GRPCConfig string
	// ACLConfig is a string containing ACL configuration
	ACLConfig string
	// NetconfConfig is a string containing Netconf server configuration
	NetconfConfig string
	// OCServerConfig is a string containing OpenConfig server configuration
	SystemConfig string
	// LoggingConfig is a string containing Logging configuration
	LoggingConfig string
	// SSHConfig is a string containing SSH configuration
	SSHConfig string
	// PartialConfig
	PartialConfig string
}

// tplIFace template interface struct.
type tplIFace struct {
	Slot       string
	Port       string
	BreakoutNo string
	Mtu        int
}
