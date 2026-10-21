package sros

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	clabutils "github.com/srl-labs/containerlab/utils"

	"github.com/beevik/etree"
	"github.com/charmbracelet/log"
	"golang.org/x/mod/semver"
)

const (
	grpcMDCLIConfig    = `/configure system grpc md-cli admin-state enable`
	profileMDCLIConfig = `/configure system security aaa local-profiles profile "administrative" grpc rpc-authorization md-cli-session permit`
	tls13CipherCCM8    = `/configure system security tls server-cipher-list "clab-all" tls13-cipher 5 name tls-aes128-ccm8-sha256`
)

func (n *sros) setVersionSpecificParams(tplData *srosTemplateData) {
	currVersion := tplData.SwVersion.MajorMinorSemverString()

	if semver.Compare(currVersion, "v26.0") < 0 {
		tplData.GRPCConfig += "\n" + grpcMDCLIConfig
		tplData.SystemConfig += "\n" + profileMDCLIConfig

		if tplData.IsSecureGrpc {
			tplData.GRPCConfig += "\n" + tls13CipherCCM8
		}
	}
}

// buildPKIImportXML builds the NETCONF action XML that imports a PEM file into the PKI store.
func buildPKIImportXML(inputURL, outputFile, importType string) string {
	action := etree.NewElement("action")
	action.CreateAttr("xmlns", "urn:ietf:params:xml:ns:yang:1")

	admin := action.CreateElement("admin")
	admin.CreateAttr("xmlns", "urn:nokia.com:sros:ns:yang:sr:oper-admin")

	importElem := admin.CreateElement("system").
		CreateElement("security").
		CreateElement("pki").
		CreateElement("import")

	// Add import parameters
	importElem.CreateElement("input-url").SetText(inputURL)
	importElem.CreateElement("output-file").SetText(outputFile)
	importElem.CreateElement("type").SetText(importType)
	importElem.CreateElement("format").SetText("pem")
	var buf bytes.Buffer
	action.WriteTo(&buf, &etree.WriteSettings{
		CanonicalText:    false,
		CanonicalAttrVal: false,
	})
	return buf.String()
}

// buildTLSProfileXML builds the TLS profile configuration XML.
func buildTLSProfileXML() string {
	config := etree.NewElement("config")
	configure := config.CreateElement("configure")
	configure.CreateAttr("xmlns", "urn:nokia.com:sros:ns:yang:sr:conf")
	configure.CreateAttr("xmlns:nc", "urn:ietf:params:xml:ns:netconf:base:1.0")

	certProfile := configure.CreateElement("system").
		CreateElement("security").
		CreateElement("tls").
		CreateElement("cert-profile")

	// Set operation attribute
	certProfile.CreateAttr("nc:operation", "merge")

	// Add profile configuration
	certProfile.CreateElement("cert-profile-name").SetText(tlsCertProfileName)
	certProfile.CreateElement("admin-state").SetText("enable")

	var buf bytes.Buffer
	config.WriteTo(&buf, &etree.WriteSettings{
		CanonicalText:    false,
		CanonicalAttrVal: false,
	})
	return buf.String()
}

// createSROSConfigFiles handles config generation for the SR-SIM kind.
// Flow: version detection → buildStartupConfig (default + partial) → GenerateConfig(dst, config).
func (n *sros) createSROSConfigFiles() error {
	// Get version from image before generating config
	if n.swVersion == nil {
		ctx := context.Background()
		version, err := n.srosVersionFromImage(ctx)
		if err != nil {
			n.swVersion = n.parseVersionString(srosDefaultVersion)
			log.Warn("Failed to get SR OS version from image",
				"node", n.Cfg.ShortName, "version", n.swVersion, "error", err)
		} else {
			n.swVersion = version
			log.Info("Retrieved SR OS version from image",
				"node", n.Cfg.ShortName,
				"version", fmt.Sprintf("%s.%s.%s", version.Major, version.Minor, version.Build))
		}
	}

	// Path pointing to the target config file under configCf3 dir
	cf3CfgFile := filepath.Join(
		n.Cfg.LabDir,
		n.Cfg.Env[envNokiaSrosSlot],
		configCf3,
		startupCfgName,
	)
	isPartial := clabutils.IsPartialConfigFile(n.Cfg.StartupConfig)

	// generate config and use that to boot node
	log.Debug("Reading startup-config", "node", n.Cfg.ShortName, "startup-config",
		n.Cfg.StartupConfig, "isPartial", isPartial)

	startupConfig, err := n.buildStartupConfig(isPartial)
	if err != nil {
		return err
	}

	if startupConfig == "" {
		log.Debug(
			"startup config is empty, skipping startup config file generation",
			"node",
			n.Cfg.ShortName,
		)
		return nil
	}

	return n.GenerateConfig(cf3CfgFile, startupConfig)
}

// buildStartupConfig returns the full startup config string: either from user file (full config)
// or from default + partial config generation. It does not return a template; the name is
// historical.
func (n *sros) buildStartupConfig(isPartial bool) (string, error) {
	// User provides full startup config
	if n.Cfg.StartupConfig != "" && !isPartial {
		c, err := os.ReadFile(n.Cfg.StartupConfig)
		if err != nil {
			return "", err
		}

		cBuf, err := clabutils.SubstituteEnvsAndTemplate(bytes.NewReader(c), n.Cfg)
		if err != nil {
			return "", err
		}
		return cBuf.String(), nil
	}

	// Generate default config and optionally add partial config
	if err := n.addDefaultConfig(); err != nil {
		return "", err
	}
	if err := n.addPartialConfig(); err != nil {
		return "", err
	}

	return string(n.startupCliCfg), nil
}

// prepareConfigTemplateData prepares all data needed for template selection and execution.
// Service configs are filled from a single table-driven result: variant → getFullSnippetSet(v).
func (n *sros) prepareConfigTemplateData() (*srosTemplateData, error) {
	b, err := n.banner()
	if err != nil {
		return nil, err
	}

	componentConfig := ""
	if !isFullConfigFile(n.Cfg.StartupConfig) {
		componentConfig = n.generateComponentConfig()
	} else {
		log.Debugf(
			"SR-SIM node %q has non-partial startup-config defined, skipping component config gen",
			n.Cfg.LongName,
		)
	}

	v := n.resolveConfigVariant()
	snippets := getFullSnippetSet(v)
	configMode := string(v.Mode)
	if v.ForceClassic {
		log.Warn(
			"SAR-Hm nodes only support classic configuration mode. Overriding configuration mode to 'classic'",
			"node",
			n.Cfg.LongName,
			"node-type",
			strings.ToLower(n.Cfg.NodeType),
		)
		configMode = string(ConfigModeClassic)
		n.kindSpecificCfg().ConfigMode = ConfigModeClassic
	}

	tplData := &srosTemplateData{
		// Selection criteria
		NodeType:          strings.ToLower(n.Cfg.NodeType),
		ConfigurationMode: configMode,
		SwVersion:         n.swVersion,
		IsSecureGrpc:      *n.Cfg.Certificate.Issue,

		// Node data
		Name:            n.Cfg.ShortName,
		TLSKey:          n.Cfg.TLSKey,
		TLSCert:         n.Cfg.TLSCert,
		TLSAnchor:       n.Cfg.TLSAnchor,
		Banner:          b,
		IFaces:          map[string]tplIFace{},
		MgmtMTU:         0,
		MgmtIPMTU:       n.Runtime.Mgmt().MTU,
		ComponentConfig: componentConfig,

		// Service configs from variant (single source of truth)
		SystemConfig:  snippets.SystemConfig,
		GRPCConfig:    snippets.GRPCConfig,
		SNMPConfig:    snippets.SNMPConfig,
		NetconfConfig: snippets.NetconfConfig,
		LoggingConfig: snippets.LoggingConfig,
		SSHConfig:     snippets.SSHConfig,
	}

	if n.Config().DNS != nil {
		tplData.DNSServers = append(tplData.DNSServers, n.Config().DNS.Servers...)
	}

	n.prepareSSHPubKeys(tplData)

	n.setVersionSpecificParams(tplData)

	return tplData, nil
}

// addDefaultConfig adds sros default configuration such as tls certs, gnmi/json-rpc, login-banner,
// ssh keys.
func (n *sros) addDefaultConfig() error {
	// Prepare all template data
	tplData, err := n.prepareConfigTemplateData()
	if err != nil {
		return err
	}

	// Select appropriate template
	srosCfgTpl, err := n.selectConfigTemplate(tplData)
	if err != nil {
		return fmt.Errorf("failed to select config template: %w", err)
	}
	log.Debug("Prepare SR OS config template", "template", srosCfgTpl.Name(),
		"node", n.Cfg.LongName,
		"configuration-mode", tplData.ConfigurationMode,
		"node-type", tplData.NodeType,
		"secure-grpc", tplData.IsSecureGrpc,
		"sw-version", tplData.SwVersion)

	// Execute template
	buf := new(bytes.Buffer)
	err = srosCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}

	if buf.Len() == 0 {
		log.Warn(
			"Buffer empty, template parsing error",
			"node", n.Cfg.ShortName,
			"template", srosCfgTpl.Name(),
		)
	} else {
		log.Debug("Additional default config parsed",
			"node", n.Cfg.ShortName,
			"template", srosCfgTpl.Name())
		n.startupCliCfg = append(n.startupCliCfg, buf.String()...)
	}

	return nil
}

// applyPartialConfig applies partial configuration to the SR OS.
func (n *sros) addPartialConfig() error {
	if n.Cfg.StartupConfig != "" {
		// b holds the configuration to be applied to the node
		b := &bytes.Buffer{}
		// apply partial configs if partial config is used
		if clabutils.IsPartialConfigFile(n.Cfg.StartupConfig) && n.isCPM("") {
			log.Info("Adding configuration",
				"node", n.Cfg.LongName,
				"type", "partial",
				"source", n.Cfg.StartupConfig)

			r, err := os.Open(n.Cfg.StartupConfig)
			if err != nil {
				return err
			}

			defer r.Close() // skipcq: GO-S2307

			_, err = io.Copy(b, r)
			if err != nil {
				return err
			}

			configContent, err := clabutils.SubstituteEnvsAndTemplate(b, n.Cfg)
			if err != nil {
				return err
			}
			if configContent.Len() == 0 {
				log.Warn(
					"Buffer empty, PARTIAL config template parsing error",
					"node",
					n.Cfg.ShortName,
				)
			} else {
				log.Debug("Additional PARTIAL config parsed", "node",
					n.Cfg.ShortName, "partial-config", configContent.String())
				n.startupCliCfg = append(n.startupCliCfg, configContent.String()...)
			}
		} else {
			log.Warn(
				"Passed startup-config option, but it will not have any effect",
				"node",
				n.Cfg.ShortName,
			)
		}
	}
	return nil
}

// isConfigClassic reports whether the node is in classic or mixed configuration mode.
func (n *sros) isConfigClassic() bool {
	mode := n.kindSpecificCfg().ConfigMode
	return mode == ConfigModeClassic || mode == ConfigModeMixed
}

// isFullConfigFile returns true if the config file doesn't contain .partial substring
// and the config file is NOT nil (ie. startup config IS defined).
// it is intended that the 'c' arg is n.Cfg.StartupConfig.
func isFullConfigFile(c string) bool {
	return c != "" && !clabutils.IsPartialConfigFile(c)
}
