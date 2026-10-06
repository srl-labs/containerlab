package sros

import (
	_ "embed"
	"text/template"

	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	//go:embed configs/sros_config_sros25.go.tpl
	cfgTplSROS25 string

	//go:embed configs/sros_config_classic.go.tpl
	cfgTplClassic string

	//go:embed configs/ixr/ixr_config_classic.go.tpl
	cfgTplClassicIxr string

	//go:embed configs/sar/sar_config_classic.go.tpl
	cfgTplClassicSar string
)

// srosTemplateData holds all data needed for template selection and execution.
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
	Banner            string
	SSHPubKeysRSA     []string
	SSHPubKeysECDSA   []string
	SSHPubKeysED25519 []string

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

// getTemplateForVariant returns the template string and name for the given config variant.
// swVersion is for future SR OS 26+ template selection (e.g. cfgTplSROS26 when Major >= "26").
func getTemplateForVariant(v ConfigVariant, swVersion *SrosVersion) (tmpl string, tplName string) {
	// Model-driven (or mixed treated as classic below when family is classic)
	if v.Mode == ConfigModeModelDriven {
		// Placeholder for version-specific template: if swVersion != nil && swVersion.Major >= "26"
		// { return cfgTplSROS26, "clab-sros-config-sros26" }
		return cfgTplSROS25, "clab-sros-config-sros25"
	}
	// Classic (or mixed)
	tmpl = cfgTplClassic
	tplName = "clab-sros-config-classic"
	switch v.Family {
	case ConfigFamilyIXR:
		return cfgTplClassicIxr, "clab-sros-config-classic-ixr"
	case ConfigFamilySAR:
		return cfgTplClassicSar, "clab-sros-config-classic-sar"
	default:
		return tmpl, tplName
	}
}

// selectConfigTemplate chooses the config template from the config variant (mode + family).
func (n *sros) selectConfigTemplate(tplData *srosTemplateData) (*template.Template, error) {
	v := n.resolveConfigVariant()
	tmpl, tplName := getTemplateForVariant(v, tplData.SwVersion)
	return template.New(tplName).
		Funcs(clabutils.CreateFuncs()).
		Parse(tmpl)
}
