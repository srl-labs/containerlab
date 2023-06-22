package types

import (
	log "github.com/sirupsen/logrus"
)

type RawHostLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
}

func (h *RawHostLink) UnRaw(res NodeResolver) (Link, error) {

	node, err := res.ResolveNode(h.Node)
	if err != nil {
		return nil, err
	}
	cEndpoint := NewEndpoint(node, h.NodeInterface, nil)

	return &HostLink{
		HostInterface:     h.HostInterface,
		ContainerEndpoint: cEndpoint,
		LinkGenericAttrs: LinkGenericAttrs{
			Labels: h.Labels,
			Vars:   h.Vars,
		},
	}, nil
}

func hostFromLinkConfig(lc LinkConfig, specialEPIndex int) (RawLink, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &RawHostLink{
		RawLinkTypeAlias: RawLinkTypeAlias{
			Type:     string(LinkTypeHost),
			Labels:   lc.Labels,
			Vars:     lc.Vars,
			Instance: nil,
		},
		HostInterface: hostIf,
		Node:          node,
		NodeInterface: nodeIf,
	}
	return result, nil
}

type HostLink struct {
	HostInterface     string
	ContainerEndpoint *Endpoint
	LinkGenericAttrs
}

func (m *HostLink) Deploy() error {
	log.Warn("TODO")
	// TODO
	return nil
}

func (m *HostLink) GetType() (LinkType, error) {
	return LinkTypeHost, nil
}

func (m *HostLink) Remove() error {
	// TODO
	log.Warn("not implemented yet")
	return nil
}
