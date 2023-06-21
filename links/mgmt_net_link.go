package links

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
)

type RawMgmtNetLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
}

func (m *RawMgmtNetLink) UnRaw(res Resolver) (Link, error) {
	return &MgmtNetLink{
		LinkGenericAttrs: LinkGenericAttrs{
			Labels: m.Labels,
			Vars:   m.Vars,
		},
		HostInterface: "",
		Node:          nil,
		NodeInterface: "",
	}, nil
}

func mgmtNetFromLinkConfig(lc LinkConfig, specialEPIndex int) (*RawMgmtNetLink, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &RawMgmtNetLink{
		RawLinkTypeAlias: RawLinkTypeAlias{Type: string(LinkTypeMgmtNet), Labels: lc.Labels, Vars: lc.Vars, Instance: nil},
		HostInterface:    hostIf,
		Node:             node,
		NodeInterface:    nodeIf,
	}
	return result, nil
}

type MgmtNetLink struct {
	LinkGenericAttrs
	HostInterface string
	Node          nodes.Node
	NodeInterface string
}

func (m *MgmtNetLink) Deploy() error {
	// TODO
	return fmt.Errorf("not yet implemented")
}

func (m *MgmtNetLink) GetType() (LinkType, error) {
	return LinkTypeMgmtNet, nil
}

func (m *MgmtNetLink) Remove() error {
	// TODO
	log.Warn("not implemented yet")
	return nil

}
