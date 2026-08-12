package vr_ros

import (
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestROSInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*clablinks.EndpointVeth
		node      *vrRos
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ether2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ether4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "ether6",
					},
				},
			},
			node: &vrRos{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "ros",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth3", "eth5",
			},
		},
		"original-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth6",
					},
				},
			},
			node: &vrRos{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "ros",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth2", "eth4", "eth6",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			tc.node.FirstDataIfIndex = 1
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					t.Errorf("got error for endpoint %+v", gotEndpointErr)
				}
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					t.Errorf("got error for check %+v", gotCheckErr)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							t.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}

// TestROSFilterManagementInterfaceConfig verifies that filterManagementInterfaceConfig
// drops only the management interface (ether1) IP address entry and keeps everything
// else. It guards against substring greediness (ether11 must survive) and against
// collateral removal of neighbouring sections or ether1 references outside /ip address.
func TestROSFilterManagementInterfaceConfig(t *testing.T) {

	input := `/interface ethernet
set [ find default-name=ether1 ] advertise="1G-baseT-full,2.5G-baseT,5G-baseT,10G-baseT"
set [ find default-name=ether12 ] comment="uplink-1" l2mtu=9000
set [ find default-name=ether2 ] comment=uplink-2 disabled=yes l2mtu=9000
set [ find default-name=sfp-sfpplus3 ] comment=uplink-3 l2mtu=9000
/ip address
add address=192.0.2.10/24 comment=mgmt interface=ether1 network=192.0.2.0
add address=198.51.100.33/24 comment=data-2 interface=ether2 network=198.51.100.0
add address=203.0.113.36/24 comment=data-11 interface=ether11 network=203.0.113.0
add address=192.0.2.252/24 comment=loopback interface=lo network=192.0.2.252
/ip dhcp-client
add interface=ether1 comment="network=keep-this-dhcp-client"
add interface=VLAN100_LAN name=BR-WAN use-peer-dns=no use-peer-ntp=no
/ip dhcp-server lease
add address=192.0.2.219 client-id=\
    aa:bb:cc:00:00:00:01:00:00:00:2e:00:00:00:88:00:00:00:00:68 mac-address=\
    00:00:00:00:00:01 server=defconf
/ip dhcp-server network
add address=192.0.2.0/24 comment=defconf dns-server=198.51.100.53 gateway=\
    192.0.2.1
/ip dns`

	n := new(vrRos)
	got := n.filterManagementInterfaceConfig(input)

	// The management interface (ether1) IP entry must be filtered out.
	if strings.Contains(got, "interface=ether1 network=") {
		t.Errorf("management ether1 entry was not filtered out:\n%s", got)
	}

	// Other interface named ether11 must NOT be filtered out (no substring greediness).
	if !strings.Contains(got, "interface=ether11 network=") {
		t.Errorf("data interface ether11 entry was incorrectly removed:\n%s", got)
	}

	// Everything else, including other sections and ether1 refs outside /ip address, must be kept.
	for _, want := range []string{
		"/interface ethernet",
		"default-name=ether1",
		"/ip address",
		"interface=ether2 network=198.51.100.0",
		"interface=lo network=192.0.2.252",
		"/ip dhcp-client",
		`interface=ether1 comment="network=keep-this-dhcp-client"`,
		"interface=VLAN100_LAN name=BR-WAN",
		"/ip dhcp-server lease",
		"address=192.0.2.219 client-id=",
		"/ip dhcp-server network",
		"address=192.0.2.0/24 comment=defconf",
		"/ip dns",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected entry %q to be kept, but it is missing:\n%s", want, got)
		}
	}
}
