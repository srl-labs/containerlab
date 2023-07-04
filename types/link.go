package types

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

type LinkCommonParams struct {
	Mtu    int                    `yaml:"mtu,omitempty"`
	Labels map[string]string      `yaml:"labels,omitempty"`
	Vars   map[string]interface{} `yaml:"vars,omitempty"`
}

type LinkDefinition struct {
	Type string `yaml:"type"`
	// Instance interface{}
	LinkConfig
}

type LinkDefinitionType string

const (
	LinkTypeVEth    LinkDefinitionType = "veth"
	LinkTypeMgmtNet LinkDefinitionType = "mgmt-net"
	LinkTypeMacVLan LinkDefinitionType = "macvlan"
	LinkTypeMacVTap LinkDefinitionType = "macvtap"
	LinkTypeHost    LinkDefinitionType = "host"

	// legacy link definifion
	LinkTypeDeprecate LinkDefinitionType = "deprecate"
)

type LinkDefinitionAlias LinkDefinition

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
	case string(LinkTypeDeprecate):
		return LinkTypeDeprecate, nil
	default:
		return "", fmt.Errorf("unable to parse %q as LinkType", s)
	}
}

func (rlt *LinkDefinitionAlias) GetType() (LinkDefinitionType, error) {
	return ParseLinkType(rlt.Type)
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
	var rtAlias LinkDefinitionAlias

	err := unmarshal(&rtAlias)
	// Strict unmarshalling, as we do with containerlab will cause the
	// Links sections to fail. The unmarshal call will throw a yaml.TypeError
	// This section we don't want strict, so if error is not nil but the error type is
	// yaml.TypeError, we will continue
	var e *yaml.TypeError
	if err != nil && !errors.As(err, &e) {
		return err
	}

	r.Type = rtAlias.Type

	lt, err := ParseLinkType(rtAlias.Type)
	if err != nil {
		return err
	}

	switch lt {
	case LinkTypeVEth:
		var l RawVEthLink
		err := unmarshal(&l)
		if err != nil && !errors.As(err, &e) {
			return err
		}
		r.LinkConfig = *l.ToLinkConfig()
	case LinkTypeMgmtNet:
		var l RawMgmtNetLink
		err := unmarshal(&l)
		if err != nil && !errors.As(err, &e) {
			return err
		}
		r.LinkConfig = *l.ToLinkConfig()
	case LinkTypeHost:
		var l RawHostLink
		err := unmarshal(&l)
		if err != nil && !errors.As(err, &e) {
			return err
		}
		r.LinkConfig = *l.ToLinkConfig()
	case LinkTypeMacVLan:
		var l RawMacVLanLink
		err := unmarshal(&l)
		if err != nil && !errors.As(err, &e) {
			return err
		}
		r.LinkConfig = *l.ToLinkConfig()
	case LinkTypeMacVTap:
		var l RawMacVTapLink
		err := unmarshal(&l)
		if err != nil && !errors.As(err, &e) {
			return err
		}
		r.LinkConfig = *l.ToLinkConfig()
	case LinkTypeDeprecate:
		// try to parse the depricate format
		var l LinkConfig
		err := unmarshal(&l)
		if err != nil && !errors.As(err, &e) {
			return err
		}
		r.Type = string(LinkTypeDeprecate)
		lc, err := deprecateLinkConversion(&l)
		r.LinkConfig = *lc
		if err != nil {
			return err
		}
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
