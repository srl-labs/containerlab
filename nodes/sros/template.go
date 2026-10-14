package sros

import (
	_ "embed"
	"text/template"

	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	//go:embed configs/sros_config.go.tpl
	cfgTplSROS string

	//go:embed configs/sros_config_le_26_3.go.tpl
	cfgTplSROSLe26_3 string

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
	// SrosLe25_10 enables the gRPC MD-CLI service (SR OS <= 25.10); it is obsolete in 26.3.
	SrosLe25_10 bool

	// Node identification
	Name string
	// AdminUser and AdminPassword are CLI-quoted (including the surrounding quotes).
	AdminUser     string
	AdminPassword string
	// PromptPassword adds new-password-at-login for the admin user.
	PromptPassword bool

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
// Model-driven SR OS <= 26.3 uses the le-26.3 template, which includes the CCM8 cipher.
// SR OS 26.7, later releases, and development versions reported as 0.0.0 use the current template.
func getTemplateForVariant(v ConfigVariant, swVersion *SrosVersion) (tmpl string, tplName string) {
	// Model-driven (or mixed treated as classic below when family is classic)
	if v.Mode == ConfigModeModelDriven {
		if srosLe26_3(swVersion) {
			return cfgTplSROSLe26_3, "clab-sros-config-le-26.3"
		}
		return cfgTplSROS, "clab-sros-config"
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

func srosTemplateFuncs(version *SrosVersion) template.FuncMap {
	funcs := clabutils.CreateFuncs()
	funcs["srosLe26_3"] = func() bool { return srosLe26_3(version) }
	return funcs
}

// selectConfigTemplate chooses the config template from the config variant (mode + family).
func (n *sros) selectConfigTemplate(tplData *srosTemplateData) (*template.Template, error) {
	v := n.resolveConfigVariant()
	tmpl, tplName := getTemplateForVariant(v, tplData.SwVersion)
	return template.New(tplName).
		Funcs(srosTemplateFuncs(tplData.SwVersion)).
		Parse(tmpl)
}
