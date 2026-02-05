// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import "strings"

// ConfigMode is the configuration mode (model-driven or classic).
type ConfigMode string

const (
	ConfigModeModelDriven ConfigMode = "model-driven"
	ConfigModeClassic     ConfigMode = "classic"
	ConfigModeMixed       ConfigMode = "mixed"
)

// ConfigFamily is the node family (SR, IXR, SAR).
type ConfigFamily string

const (
	ConfigFamilySR  ConfigFamily = "sr"
	ConfigFamilyIXR ConfigFamily = "ixr"
	ConfigFamilySAR ConfigFamily = "sar"
)

// ConfigVariant encodes mode, node family, and security for template and snippet selection.
// Single source of truth for "what config do we use for this node."
type ConfigVariant struct {
	Mode         ConfigMode   // model-driven, classic, or mixed
	Family       ConfigFamily // sr, ixr, sar
	SecureGrpc   bool
	ForceClassic bool // true when SAR-Hm forces classic (MD disabled)
}

// snippetSet holds the system and gRPC config snippets for a variant.
// Other snippets (SNMP, logging, netconf, SSH) are shared across variants.
type snippetSet struct {
	SystemConfig string
	GRPCConfig   string
}

// FullSnippetSet holds all config snippet strings for a variant (system, grpc, snmp, netconf, logging, ssh).
// Single table-driven result for "which snippets apply to this variant."
type FullSnippetSet struct {
	SystemConfig    string
	GRPCConfig      string
	SNMPConfig      string
	NetconfConfig   string
	LoggingConfig   string
	SSHConfig       string
}

// getFullSnippetSet returns the full set of snippet strings for the given variant.
// Shared snippets (SNMP, logging, netconf, SSH) are the same for all variants.
func getFullSnippetSet(v ConfigVariant) FullSnippetSet {
	s := getSnippetSet(v)
	return FullSnippetSet{
		SystemConfig:  s.SystemConfig,
		GRPCConfig:    s.GRPCConfig,
		SNMPConfig:    snmpv2Config,
		NetconfConfig: netconfConfig,
		LoggingConfig: loggingConfig,
		SSHConfig:     sshConfig,
	}
}

// resolveConfigVariant returns the config variant for node n from NodeType,
// Env[envSrosConfigMode], and Certificate.Issue. SAR-Hm forces classic mode;
// the caller should apply that to tplData and n.Cfg.Env when ForceClassic is true.
func (n *sros) resolveConfigVariant() ConfigVariant {
	mode := ConfigMode(strings.ToLower(n.Cfg.Env[envSrosConfigMode]))
	if mode == "" {
		mode = ConfigModeModelDriven
	}
	secureGrpc := n.Cfg.Certificate.Issue != nil && *n.Cfg.Certificate.Issue

	var family ConfigFamily
	switch {
	case n.isIXRNode():
		family = ConfigFamilyIXR
	case n.isSARNode():
		family = ConfigFamilySAR
	default:
		family = ConfigFamilySR
	}

	forceClassic := false
	if family == ConfigFamilySAR && n.isSARHmNode() {
		forceClassic = true
		if mode != ConfigModeClassic && mode != ConfigModeMixed {
			mode = ConfigModeClassic
		}
	}

	return ConfigVariant{
		Mode:         mode,
		Family:       family,
		SecureGrpc:   secureGrpc,
		ForceClassic: forceClassic,
	}
}

// getSnippetSet returns the system and gRPC snippet set for the given variant.
func getSnippetSet(v ConfigVariant) snippetSet {
	sys := systemCfg
	grpc := grpcConfig
	if !v.SecureGrpc {
		grpc = grpcConfigInsecure
	}
	switch v.Family {
	case ConfigFamilyIXR:
		sys = systemCfgIXR
		grpc = grpcConfigIXR
		if !v.SecureGrpc {
			grpc = grpcConfigIXRInsecure
		}
	case ConfigFamilySAR:
		sys = systemCfgSAR
		grpc = grpcConfigSAR
		if !v.SecureGrpc {
			grpc = grpcConfigSARInsecure
		}
	}
	return snippetSet{SystemConfig: sys, GRPCConfig: grpc}
}
