package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// LinkCommonParams represents the common parameters for all link types.
type LinkCommonParams struct {
	Mtu    int                    `yaml:"mtu,omitempty"`
	Labels map[string]string      `yaml:"labels,omitempty"`
	Vars   map[string]interface{} `yaml:"vars,omitempty"`
}

// LinkDefinition represents a link definition in the topology file.
type LinkDefinition struct {
	Type       string `yaml:"type,omitempty"`
	LinkConfig `yaml:",inline"`
}

// LinkDefinitionType represents the type of a link definition.
type LinkDefinitionType string

const (
	LinkTypeVEth    LinkDefinitionType = "veth"
	LinkTypeMgmtNet LinkDefinitionType = "mgmt-net"
	LinkTypeMacVLan LinkDefinitionType = "macvlan"
	LinkTypeHost    LinkDefinitionType = "host"

	// LinkTypeBrief is a link definition where link types
	// are encoded in the endpoint definition as string and allow users
	// to quickly type out link endpoints in a yaml file.
	LinkTypeBrief LinkDefinitionType = "brief"
)

// parseLinkType parses a string representation of a link type into a LinkDefinitionType.
func parseLinkType(s string) (LinkDefinitionType, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case string(LinkTypeMacVLan):
		return LinkTypeMacVLan, nil
	case string(LinkTypeVEth):
		return LinkTypeVEth, nil
	case string(LinkTypeMgmtNet):
		return LinkTypeMgmtNet, nil
	case string(LinkTypeHost):
		return LinkTypeHost, nil
	case string(LinkTypeBrief):
		return LinkTypeBrief, nil
	default:
		return "", fmt.Errorf("unable to parse %q as LinkType", s)
	}
}

var _ yaml.Unmarshaler = (*LinkDefinition)(nil)

// UnmarshalYAML deserializes links passed via topology file into LinkDefinition struct.
// It supports both the brief and specific link type notations.
func (r *LinkDefinition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// alias struct to avoid recursion and pass strict yaml unmarshalling
	// we don't care about the embedded LinkConfig, as we only need to unmarshal
	// the type field.
	var a struct {
		Type string `yaml:"type"`
		// Throwaway endpoints field, as we don't care about it.
		Endpoints any `yaml:"endpoints"`
	}
	err := unmarshal(&a)
	if err != nil {
		return err
	}

	var lt LinkDefinitionType

	// if no type is specified, we assume that brief notation of a link definition is used.
	if a.Type == "" {
		lt = LinkTypeBrief
		r.Type = string(LinkTypeBrief)
	} else {
		r.Type = a.Type

		lt, err = parseLinkType(a.Type)
		if err != nil {
			return err
		}
	}

	switch lt {
	case LinkTypeVEth:
		var l struct {
			// the Type field is injected artificially
			// to allow strict yaml parsing to work.
			Type        string `yaml:"type"`
			LinkVEthRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.LinkVEthRaw.ToLinkConfig()
	case LinkTypeMgmtNet:
		var l struct {
			Type           string `yaml:"type"`
			LinkMgmtNetRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.LinkMgmtNetRaw.ToLinkConfig()
	case LinkTypeHost:
		var l struct {
			Type        string `yaml:"type"`
			LinkHostRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.LinkHostRaw.ToLinkConfig()
	case LinkTypeMacVLan:
		var l struct {
			Type           string `yaml:"type"`
			LinkMACVLANRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.LinkMACVLANRaw.ToLinkConfig()
	case LinkTypeBrief:
		// brief link's endpoint format
		var l struct {
			Type       string `yaml:"type"`
			LinkConfig `yaml:",inline"`
		}

		err := unmarshal(&l)
		if err != nil {
			return err
		}

		r.Type = string(LinkTypeBrief)

		r.LinkConfig = l.LinkConfig
	default:
		return fmt.Errorf("unknown link type %q", lt)
	}

	return nil
}
