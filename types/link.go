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

type RawLinkType struct {
	Type string `yaml:"type"`
	// Instance interface{}
	Instance *LinkConfig
}

type LinkDefinitionType string

const (
	LinkTypeVEth    LinkDefinitionType = "veth"
	LinkTypeMgmtNet LinkDefinitionType = "mgmt-net"
	LinkTypeMacVLan LinkDefinitionType = "macvlan"
	LinkTypeMacVTap LinkDefinitionType = "macvtap"
	LinkTypeHost    LinkDefinitionType = "host"

	LinkTypeDeprecate LinkDefinitionType = "DEPRECATE"
)

type RawLinkTypeAlias RawLinkType

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
	case string(LinkTypeDeprecate):
		return LinkTypeDeprecate, nil
	default:
		return "", fmt.Errorf("unable to parse %q as LinkType", s)
	}
}

func (rlt *RawLinkTypeAlias) GetType() (LinkDefinitionType, error) {
	return ParseLinkType(rlt.Type)
}

var _ yaml.Unmarshaler = &RawLinkType{}

func (r *RawLinkType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rtAlias RawLinkTypeAlias

	err := unmarshal(&rtAlias)
	// Strict unmarshalling, as we do with containerlab will cause the
	// Links sections to fail. The unmarshal call will throw a yaml.TypeError
	// This section we don't want strict, so if error is not nil but the error type is
	// yaml.TypeError, we will continue
	var e *yaml.TypeError
	if err != nil && errors.As(err, &e) {
		return err
	}

	r.Type = rtAlias.Type

	switch strings.ToLower(rtAlias.Type) {
	case "veth":
		var l RawVEthLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l.ToLinkConfig()
	case "mgmt-net":
		var l RawMgmtNetLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l.ToLinkConfig()
	case "host":
		var l RawHostLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l.ToLinkConfig()
	case "macvlan":
		var l RawMacVLanLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l.ToLinkConfig()
	case "macvtap":
		var l RawMacVTapLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l.ToLinkConfig()
	default:
		// try to parse the depricate format
		var l LinkConfig
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Type = "DEPRECATE"
		r.Instance, err = deprecateLinkConversion(&l)
		if err != nil {
			return err
		}
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
