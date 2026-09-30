package srl

import (
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
)

func TestPopulateInterfaceConfig(t *testing.T) {
	for _, tc := range []struct {
		name          string
		mtu           int
		runtimeOnly   bool
		wantMTU       int
		wantMgmtIPMTU int
	}{
		{name: "restored endpoints without topology links", runtimeOnly: true, wantMgmtIPMTU: 1500},
		{name: "default link MTU", mtu: clabconstants.DefaultLinkMTU, wantMgmtIPMTU: 1500},
		{name: "explicit link MTU", mtu: 9000, wantMTU: 9000, wantMgmtIPMTU: 8986},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &srl{DefaultNode: clabnodes.DefaultNode{}}
			for _, name := range []string{"mgmt0", "e1-1", "e1-2-1"} {
				if tc.runtimeOnly {
					n.Endpoints = append(n.Endpoints, clablinks.NewRuntimeEndpoint(n, name))
					continue
				}
				link := clablinks.NewLinkVEth()
				link.MTU = tc.mtu
				n.Endpoints = append(n.Endpoints, clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n, name, link)))
			}
			data := srlTemplateData{IFaces: map[string]tplIFace{}, MgmtIPMTU: 1500}
			n.populateInterfaceConfig(&data)
			if data.MgmtMTU != tc.wantMTU || data.MgmtIPMTU != tc.wantMgmtIPMTU {
				t.Fatalf("management MTUs = %d/%d, want %d/%d", data.MgmtMTU, data.MgmtIPMTU, tc.wantMTU, tc.wantMgmtIPMTU)
			}
			if len(data.IFaces) != 2 {
				t.Fatalf("data interfaces = %d, want 2", len(data.IFaces))
			}
			for name, wantFullName := range map[string]string{"e1-1": "ethernet-1/1", "e1-2-1": "ethernet-1/2/1"} {
				iface := data.IFaces[name]
				if iface.FullName != wantFullName || iface.Mtu != tc.wantMTU {
					t.Errorf("interface %s = %+v, want name %s and MTU %d", name, iface, wantFullName, tc.wantMTU)
				}
			}
		})
	}
}
