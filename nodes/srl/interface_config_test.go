package srl

import (
	"fmt"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
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
				n.Endpoints = append(
					n.Endpoints,
					clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n, name, link)),
				)
			}

			data := srlTemplateData{IFaces: map[string]tplIFace{}, MgmtIPMTU: 1500}
			if err := n.populateInterfaceConfig(&data); err != nil {
				t.Fatal(err)
			}

			if data.MgmtMTU != tc.wantMTU || data.MgmtIPMTU != tc.wantMgmtIPMTU {
				t.Fatalf(
					"management MTUs = %d/%d, want %d/%d",
					data.MgmtMTU,
					data.MgmtIPMTU,
					tc.wantMTU,
					tc.wantMgmtIPMTU,
				)
			}

			if len(data.IFaces) != 2 {
				t.Fatalf("data interfaces = %d, want 2", len(data.IFaces))
			}

			for name, wantFullName := range map[string]string{"e1-1": "ethernet-1/1", "e1-2-1": "ethernet-1/2/1"} {
				iface := data.IFaces[name]
				if iface.FullName != wantFullName || iface.Mtu != tc.wantMTU {
					t.Errorf(
						"interface %s = %+v, want name %s and MTU %d",
						name,
						iface,
						wantFullName,
						tc.wantMTU,
					)
				}
			}
		})
	}
}

func TestSRLInvalidInterfaceNames(t *testing.T) {
	for _, name := range []string{
		"", "eth1", "e1", "e1-", "e-1", "e0-1", "e1-0", "e1-1-0",
		"e1-1-", "e1-1-1-1", "xe1-1", "ee1-1", "e1-1suffix", "e1-x",
		"e1-1\n", "mgmt0suffix",
	} {
		for _, runtimeOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/runtime=%t", name, runtimeOnly), func(t *testing.T) {
				n := &srl{DefaultNode: clabnodes.DefaultNode{
					Cfg: &clabtypes.NodeConfig{NetworkMode: "none"},
				}}

				if runtimeOnly {
					n.Endpoints = append(n.Endpoints, clablinks.NewRuntimeEndpoint(n, name))
				} else {
					n.Endpoints = append(n.Endpoints, clablinks.NewEndpointVeth(
						clablinks.NewEndpointGeneric(n, name, clablinks.NewLinkVEth()),
					))
				}

				data := srlTemplateData{IFaces: map[string]tplIFace{}, MgmtIPMTU: 1500}

				err := n.populateInterfaceConfig(&data)
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("interface %q", name)) {
					t.Fatalf("got error %v, want an error identifying interface %q", err, name)
				}

				if len(data.IFaces) != 0 || data.MgmtMTU != 0 || data.MgmtIPMTU != 1500 {
					t.Fatal("invalid interface changed the configuration")
				}

				if err := n.CheckInterfaceName(); err == nil {
					t.Fatalf("topology validation accepted invalid interface %q", name)
				}
			})
		}
	}
}

func TestPopulateInterfaceConfigAliases(t *testing.T) {
	n := &srl{DefaultNode: clabnodes.DefaultNode{
		Cfg:             &clabtypes.NodeConfig{},
		InterfaceRegexp: InterfaceRegexp,
	}}
	n.OverwriteNode = n

	aliases := map[string]string{"e1-1": "ethernet-1/1", "e12-34-2": "ethernet-12/34/2"}
	for _, alias := range aliases {
		ep := clablinks.NewEndpointVeth(
			clablinks.NewEndpointGeneric(n, alias, clablinks.NewLinkVEth()),
		)
		if err := n.AddEndpoint(ep); err != nil {
			t.Fatal(err)
		}
	}

	if err := n.CheckInterfaceName(); err != nil {
		t.Fatal(err)
	}

	data := srlTemplateData{IFaces: map[string]tplIFace{}}
	if err := n.populateInterfaceConfig(&data); err != nil {
		t.Fatal(err)
	}

	for name, alias := range aliases {
		if got := data.IFaces[name].FullName; got != alias {
			t.Errorf("interface %q full name = %q, want %q", name, got, alias)
		}
	}
}
