package types

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/dependency_manager"
	"golang.org/x/sync/semaphore"
	"gopkg.in/yaml.v2"
)

type NodeResolver interface {
	ResolveNode(nodeName string) (LinkNode, error)
}

type RawLinkType struct {
	Type     string                 `yaml:"type"`
	Labels   map[string]string      `yaml:"labels,omitempty"`
	Vars     map[string]interface{} `yaml:"vars,omitempty"`
	Instance interface{}
}

type RawLinkTypeAlias RawLinkType

func ParseLinkType(s string) (LinkType, error) {
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

func (rlt *RawLinkTypeAlias) GetType() (LinkType, error) {
	return ParseLinkType(rlt.Type)
}

var _ yaml.Unmarshaler = &RawLinkType{}

func (r *RawLinkType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rtAlias RawLinkTypeAlias

	err := unmarshal(&rtAlias)
	if err != nil {
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
		r.Instance = &l
	case "mgmt-net":
		var l RawMgmtNetLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = &l
	case "host":
		var l RawHostLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = &l
	case "macvlan":
		var l RawMacVLanLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = &l
	case "macvtap":
		var l RawMacVTapLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = &l
	default:
		// try to parse the depricate format
		var l LinkConfig
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Type = "DEPRECATE"
		r.Instance, err = deprecateLinkConversion(l)
		if err != nil {
			return err
		}
	}

	return nil
}

func deprecateLinkConversion(lc LinkConfig) (RawLink, error) {
	// check two endpoints defined
	if len(lc.Endpoints) != 2 {
		return nil, fmt.Errorf("endpoint definition should consist of exactly 2 entries. %d provided", len(lc.Endpoints))
	}
	for x, v := range lc.Endpoints {
		parts := strings.SplitN(v, ":", 2)
		node := parts[0]

		lt, err := ParseLinkType(node)
		if err == nil {
			continue
		}

		switch lt {
		case LinkTypeMacVLan:
			return macVlanFromLinkConfig(lc, x)
		case LinkTypeMacVTap:
			return macVTapFromLinkConfig(lc, x)
		case LinkTypeMgmtNet:
			return mgmtNetFromLinkConfig(lc, x)
		case LinkTypeHost:
			return hostFromLinkConfig(lc, x)
		}
	}
	return vEthFromLinkConfig(lc)
}

type Link interface {
	Deploy(context.Context) error
	Remove(context.Context) error
	GetType() (LinkType, error)
	GetEndpoints() []*Endpoint
}

type RawLink interface {
	Resolve(r NodeResolver) (Link, error)
}

type LinkGenericAttrs struct {
	Labels map[string]string
	Vars   map[string]interface{}
	LinkStatus
}

type LinkState int

const (
	UnDeployed LinkState = iota
	Deployed   LinkState = iota
	Up         LinkState = iota
)

type LinkType string

const (
	LinkTypeVEth    LinkType = "veth"
	LinkTypeMgmtNet LinkType = "mgmt-net"
	LinkTypeMacVLan LinkType = "macvlan"
	LinkTypeMacVTap LinkType = "macvtap"
	LinkTypeHost    LinkType = "host"

	LinkTypeDeprecate LinkType = "DEPRECATE"
)

type LinkStatus struct {
	state LinkState
}

func (l *LinkStatus) GetState() LinkState {
	return l.state
}

type DependencyManagerLink struct {
	Link
	dm  dependency_manager.DependencyManager
	sem *semaphore.Weighted
}

func NewDependencyManagerLink(l Link, dm dependency_manager.DependencyManager, sem *semaphore.Weighted) *DependencyManagerLink {
	return &DependencyManagerLink{
		Link: l,
		dm:   dm,
		sem:  sem,
	}
}

func (d *DependencyManagerLink) Deploy(ctx context.Context) error {

	waitNodes := []string{}
	for _, e := range d.Link.GetEndpoints() {
		waitNodes = append(waitNodes, e.Node.GetNodeName())
	}

	err := d.dm.WaitForNodes(waitNodes, dependency_manager.NodeStateCreated)
	if err != nil {
		log.Error(err)
	}

	// acquire Sem
	err = d.sem.Acquire(ctx, 1)
	if err != nil {
		log.Error(err)
	}
	defer d.sem.Release(1)

	return d.Link.Deploy(ctx)
}
