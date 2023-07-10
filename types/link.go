package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

type LinkCommonParams struct {
	Mtu    int                    `yaml:"mtu,omitempty"`
	Labels map[string]string      `yaml:"labels,omitempty"`
	Vars   map[string]interface{} `yaml:"vars,omitempty"`
}

// LinkDefinition represents a link definition in the topology file.
type LinkDefinition struct {
	Type string `yaml:"type"`
	// Instance interface{}
	LinkConfig
}

// LinkDefinitionType represents the type of a link definition.
type LinkDefinitionType string

const (
	LinkTypeVEth    LinkDefinitionType = "veth"
	LinkTypeMgmtNet LinkDefinitionType = "mgmt-net"
	LinkTypeMacVLan LinkDefinitionType = "macvlan"
	LinkTypeMacVTap LinkDefinitionType = "macvtap"
	LinkTypeHost    LinkDefinitionType = "host"

	// LinkTypeLegacy is a link definition where link types
	// are encoded in the endpoint definition as string.
	LinkTypeLegacy LinkDefinitionType = "legacy"
)

func ParseLinkType(s string) (LinkDefinitionType, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case string(LinkTypeMacVLan):
		return LinkTypeMacVLan, nil
	case string(LinkTypeMacVTap):
		return LinkTypeMacVTap, nil
	case string(LinkTypeVEth):
		return LinkTypeVEth, nil
	case string(LinkTypeMgmtNet):
		return LinkTypeMgmtNet, nil
	case string(LinkTypeHost):
		return LinkTypeHost, nil
	case string(LinkTypeLegacy):
		return LinkTypeLegacy, nil
	default:
		return "", fmt.Errorf("unable to parse %q as LinkType", s)
	}
}

var _ yaml.Unmarshaler = &LinkDefinition{}

// MarshalYAML used when writing topology files via the generate command.
// for now this falls back to convertig the LinkConfig into a
// RawVEthLink. Such that the generated LinkConfigs adhere to the new LinkDefinition
// format instead of the depricated one.
func (r *LinkDefinition) MarshalYAML() (interface{}, error) {
	rawVEth, err := vEthFromLinkConfig(&r.LinkConfig)
	if err != nil {
		return nil, err
	}

	x := struct {
		RawVEthLink `yaml:",inline"`
		Type        string `yaml:"type"`
	}{
		RawVEthLink: *rawVEth,
		Type:        string(LinkTypeVEth),
	}

	return x, nil
}

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

	if a.Type == "" {
		lt = LinkTypeLegacy
		r.Type = string(LinkTypeLegacy)
	} else {
		r.Type = a.Type

		lt, err = ParseLinkType(a.Type)
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
			RawVEthLink `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.RawVEthLink.ToLinkConfig()
	case LinkTypeMgmtNet:
		var l struct {
			Type           string `yaml:"type"`
			RawMgmtNetLink `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.RawMgmtNetLink.ToLinkConfig()
	case LinkTypeHost:
		var l struct {
			Type        string `yaml:"type"`
			RawHostLink `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.RawHostLink.ToLinkConfig()
	case LinkTypeMacVLan:
		var l struct {
			Type           string `yaml:"type"`
			RawMacVLanLink `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.RawMacVLanLink.ToLinkConfig()
	case LinkTypeMacVTap:
		var l struct {
			Type           string `yaml:"type"`
			RawMacVTapLink `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.LinkConfig = *l.RawMacVTapLink.ToLinkConfig()
	case LinkTypeLegacy:
		// try to parse the depricate format
		var l struct {
			Type       string `yaml:"type"`
			LinkConfig `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Type = string(LinkTypeLegacy)
		lc, err := deprecateLinkConversion(&l.LinkConfig)
		if err != nil {
			return err
		}
		r.LinkConfig = *lc
	default:
		return fmt.Errorf("unknown link type %q", lt)
	}

	return nil
}

func deprecateLinkConversion(lc *LinkConfig) (*LinkConfig, error) {
	// check two endpoints defined
	if len(lc.Endpoints) != 2 {
		return nil, fmt.Errorf("endpoint definition should consist of exactly 2 entries. %d provided", len(lc.Endpoints))
	}
	for _, v := range lc.Endpoints {
		parts := strings.SplitN(v, ":", 2)
		node := parts[0]

		lt, err := ParseLinkType(node)
		if err != nil {
			// if the link type parsing from the node name did fail
			// we continue, since the node name is not like veth or macvlan or the like
			continue
		}

		// if the node name is equal to a LinkType, we check the Type and only allow the depricated format
		// for old types. New once we force to use the new link format.
		switch lt {
		case LinkTypeMgmtNet:
			continue
		case LinkTypeHost:
			continue
		case LinkTypeMacVLan, LinkTypeMacVTap:
			return nil, fmt.Errorf("Link type %q needs to be defined in new link format", string(lt))
		}
	}
	return lc, nil
}
