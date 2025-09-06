package links

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"gopkg.in/yaml.v2"
)

func TestParseLinkType(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    LinkType
		wantErr bool
	}{
		{
			name: "link type host",
			args: args{
				s: string(LinkTypeHost),
			},
			want:    LinkTypeHost,
			wantErr: false,
		},
		{
			name: "link type veth",
			args: args{
				s: string(LinkTypeVEth),
			},
			want:    LinkTypeVEth,
			wantErr: false,
		},
		{
			name: "link type macvlan",
			args: args{
				s: string(LinkTypeMacVLan),
			},
			want:    LinkTypeMacVLan,
			wantErr: false,
		},
		{
			name: "link type mgmt-net",
			args: args{
				s: string(LinkTypeMgmtNet),
			},
			want:    LinkTypeMgmtNet,
			wantErr: false,
		},
		{
			name: "link type deprecate",
			args: args{
				s: string(LinkTypeBrief),
			},
			want:    LinkTypeBrief,
			wantErr: false,
		},
		{
			name: "link type UNKNOWN",
			args: args{
				s: "foobar",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLinkType(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLinkType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLinkType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalRawLinksYaml(t *testing.T) {
	type args struct {
		yaml []byte
	}
	tests := []struct {
		name    string
		args    args
		want    LinkDefinition
		wantErr bool
	}{
		{
			name: "brief link with veth endpoints",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "srl1:e1-5"
                        - "srl2:e1-5"
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkVEthRaw{
					Endpoints: []*EndpointRaw{
						NewEndpointRaw("srl1", "e1-5", ""),
						NewEndpointRaw("srl2", "e1-5", ""),
					},
					LinkCommonParams: LinkCommonParams{
						MTU: clabconstants.DefaultLinkMTU,
					},
				},
			},
		},
		{
			name: "brief link with ip var single side",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv4: [n1:10.10.10.1/24]
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkVEthRaw{
					Endpoints: []*EndpointRaw{
						{Node: "n1", Iface: "e1-1", Vars: &EndpointVars{IPv4: "10.10.10.1/24"}},
						{Node: "n2", Iface: "e1-1", Vars: &EndpointVars{}},
					},
					LinkCommonParams: LinkCommonParams{MTU: DefaultLinkMTU, Vars: &LinkVars{IPv4: []string{"n1:10.10.10.1/24"}}},
				},
			},
		},
		{
			name: "brief link with ip var both sides",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv4: [n1:10.10.10.1/24, n2:10.10.10.2/24]
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkVEthRaw{
					Endpoints: []*EndpointRaw{
						{Node: "n1", Iface: "e1-1", Vars: &EndpointVars{IPv4: "10.10.10.1/24"}},
						{Node: "n2", Iface: "e1-1", Vars: &EndpointVars{IPv4: "10.10.10.2/24"}},
					},
					LinkCommonParams: LinkCommonParams{MTU: DefaultLinkMTU, Vars: &LinkVars{IPv4: []string{"n1:10.10.10.1/24", "n2:10.10.10.2/24"}}},
				},
			},
		},
		{
			name: "brief link with ip var invalid node",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv4: ["n3:10.10.10.3/24"]
                `),
			},
			wantErr: true,
		},
		{
			name: "brief link with ipv4 var duplicate node",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv4: ["n1:10.10.10.1/24", "n1:10.20.30.40/24"]
                `),
			},
			wantErr: true,
		},
		{
			name: "brief link with ipv4 var as invalid address",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv4: ["n1:foo"]
                `),
			},
			wantErr: true,
		},
		{
			name: "brief link with ipv6 var as invalid address",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv6: ["n1:foo"]
                `),
			},
			wantErr: true,
		},
		{
			name: "brief link with ipv4 var non-IPv4 prefix",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv4: ["n1:2001:db8::1/64"]
                `),
			},
			wantErr: true,
		},
		{
			name: "brief link with ipv6 var non-IPv6 prefix",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "n1:e1-1"
                        - "n2:e1-1"
                    vars:
                      ipv6: ["n1:10.10.10.1/24"]
                `),
			},
			wantErr: true,
		},
		{
			name: "brief link with veth endpoints and mtu",
			args: args{
				yaml: []byte(`
                    endpoints:
                        - "srl1:e1-5"
                        - "srl2:e1-5"
                    mtu: 1500
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkVEthRaw{
					Endpoints: []*EndpointRaw{
						NewEndpointRaw("srl1", "e1-5", ""),
						NewEndpointRaw("srl2", "e1-5", ""),
					},
					LinkCommonParams: LinkCommonParams{
						MTU: 1500,
					},
				},
			},
		},
		{
			name: "brief link with macvlan endpoint",
			args: args{
				yaml: []byte(`
                    endpoints: ["srl1:e1-1", "macvlan:eth0"]`,
				),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkMacVlanRaw{
					HostInterface: "eth0",
					Endpoint:      NewEndpointRaw("srl1", "e1-1", ""),
					LinkCommonParams: LinkCommonParams{
						MTU: clabconstants.DefaultLinkMTU,
					},
				},
			},
		},
		{
			name: "brief link with mgmt-net endpoint",
			args: args{
				yaml: []byte(`
                    endpoints: ["srl1:e1-1", "mgmt-net:srl1-e1-1"]`,
				),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkMgmtNetRaw{
					HostInterface: "srl1-e1-1",
					Endpoint:      NewEndpointRaw("srl1", "e1-1", ""),
					LinkCommonParams: LinkCommonParams{
						MTU: clabconstants.DefaultLinkMTU,
					},
				},
			},
		},
		{
			name: "brief link with host endpoint",
			args: args{
				yaml: []byte(`
                    endpoints: ["srl1:e1-1", "host:srl1-e1-1"]`,
				),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeBrief),
				Link: &LinkHostRaw{
					HostInterface: "srl1-e1-1",
					Endpoint:      NewEndpointRaw("srl1", "e1-1", ""),
					LinkCommonParams: LinkCommonParams{
						MTU: clabconstants.DefaultLinkMTU,
					},
				},
			},
		},
		{
			name: "veth link",
			args: args{
				yaml: []byte(`
                    type:              veth
                    mtu:               1400
                    endpoints:
                      - node:          srl1
                        interface:     e1-1
                        mac:           02:00:00:00:00:01
                      - node:          srl2
                        interface:     e1-2
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeVEth),
				Link: &LinkVEthRaw{
					Endpoints: []*EndpointRaw{
						NewEndpointRaw("srl1", "e1-1", "02:00:00:00:00:01"),
						NewEndpointRaw("srl2", "e1-2", ""),
					},
					LinkCommonParams: LinkCommonParams{
						MTU: 1400,
					},
				},
			},
		},
		{
			name: "veth link with endpoint vars",
			args: args{
				yaml: []byte(`
                    type: veth
                    endpoints:
                      - node: n1
                        interface: e1-1
                        vars:
                          ipv4: 10.10.10.1/24
                          ipv6: 2001:db8::1/64
                      - node: n2
                        interface: e1-2
                        vars:
                          ipv4: 10.10.10.2/24
                          ipv6: 2001:db8::2/64
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeVEth),
				Link: &LinkVEthRaw{
					Endpoints: []*EndpointRaw{
						{Node: "n1", Iface: "e1-1", Vars: &EndpointVars{IPv4: "10.10.10.1/24", IPv6: "2001:db8::1/64"}},
						{Node: "n2", Iface: "e1-2", Vars: &EndpointVars{IPv4: "10.10.10.2/24", IPv6: "2001:db8::2/64"}},
					},
				},
			},
		},
		{
			name: "mgmt-net link",
			args: args{
				yaml: []byte(`
                    type:              mgmt-net
                    host-interface:    srl1_e1-5
                    endpoint:
                        node:          srl1
                        interface:     e1-5
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeMgmtNet),
				Link: &LinkMgmtNetRaw{
					HostInterface: "srl1_e1-5",
					Endpoint:      NewEndpointRaw("srl1", "e1-5", ""),
				},
			},
		},
		{
			name: "host link",
			args: args{
				yaml: []byte(`
                    type:              host
                    host-interface:    srl1_e1-5
                    endpoint:
                        node:          srl1
                        interface:     e1-5
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeHost),
				Link: &LinkHostRaw{
					HostInterface: "srl1_e1-5",
					Endpoint:      NewEndpointRaw("srl1", "e1-5", ""),
				},
			},
		},
		{
			name: "macvlan link",
			args: args{
				yaml: []byte(`
                    type:              macvlan
                    host-interface:    srl1_e1-5
                    endpoint:
                        node:          srl1
                        interface:     e1-5
                `),
			},
			wantErr: false,
			want: LinkDefinition{
				Type: string(LinkTypeMacVLan),
				Link: &LinkMacVlanRaw{
					HostInterface: "srl1_e1-5",
					Endpoint:      NewEndpointRaw("srl1", "e1-5", ""),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rl LinkDefinition
			err := yaml.Unmarshal(tt.args.yaml, &rl)
			if (err != nil) != tt.wantErr {
				t.Errorf("RawLinkType Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if diff := cmp.Diff(rl, tt.want); diff != "" {
					t.Errorf("RawLinkType Unmarshal() = %v, want %v, diff:\n%s", rl, tt.want, diff)
					return
				}
			}
		})
	}
}

func Test_extractHostNodeInterfaceData(t *testing.T) {
	type args struct {
		lb             *LinkBriefRaw
		specialEPIndex int
	}
	tests := []struct {
		name       string
		args       args
		wantHost   string
		wantHostIf string
		wantNode   string
		wantNodeIf string
		wantErr    bool
	}{
		{
			name: "Valid input",
			args: args{
				lb: &LinkBriefRaw{
					Endpoints: []string{
						"node1:eth1",
						"node2:eth2",
					},
				},
				specialEPIndex: 0,
			},
			wantHost:   "node1",
			wantHostIf: "eth1",
			wantNode:   "node2",
			wantNodeIf: "eth2",
			wantErr:    false,
		},
		{
			name: "Valid input other specialindex",
			args: args{
				lb: &LinkBriefRaw{
					Endpoints: []string{
						"node1:eth1",
						"node2:eth2",
					},
				},
				specialEPIndex: 1,
			},
			wantHost:   "node2",
			wantHostIf: "eth2",
			wantNode:   "node1",
			wantNodeIf: "eth1",
			wantErr:    false,
		},
		{
			name: "Invalid specialNode",
			args: args{
				lb: &LinkBriefRaw{
					Endpoints: []string{
						"node1:eth1",
						"node2eth2",
					},
				},
				specialEPIndex: 1,
			},
			wantErr: true,
		},
		{
			name: "Invalid Node",
			args: args{
				lb: &LinkBriefRaw{
					Endpoints: []string{
						"node1eth1",
						"node2:eth2",
					},
				},
				specialEPIndex: 1,
			},
			wantErr: true,
		},
		{
			name: "Invalid Node too many colons",
			args: args{
				lb: &LinkBriefRaw{
					Endpoints: []string{
						"node1eth1",
						"node2:et:h2",
					},
				},
				specialEPIndex: 1,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotHostIf, gotNode, gotNodeIf, err :=
				extractHostNodeInterfaceData(tt.args.lb, tt.args.specialEPIndex)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractHostNodeInterfaceData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotHost != tt.wantHost {
				t.Errorf("extractHostNodeInterfaceData() gotHost = %v, want %v", gotHost, tt.wantHost)
			}
			if gotHostIf != tt.wantHostIf {
				t.Errorf("extractHostNodeInterfaceData() gotHostIf = %v, want %v", gotHostIf, tt.wantHostIf)
			}
			if gotNode != tt.wantNode {
				t.Errorf("extractHostNodeInterfaceData() gotNode = %v, want %v", gotNode, tt.wantNode)
			}
			if gotNodeIf != tt.wantNodeIf {
				t.Errorf("extractHostNodeInterfaceData() gotNodeIf = %v, want %v", gotNodeIf, tt.wantNodeIf)
			}
		})
	}
}

func TestSanitizeInterfaceName(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"sanitize-test-original": {
			input: "eth0",
			want:  "eth0",
		},
		"sanitize-test-xrd": {
			input: "Gi0-0-0-0",
			want:  "Gi0-0-0-0",
		},
		"sanitize-test-c8000": {
			input: "Hu0_0_0_1",
			want:  "Hu0_0_0_1",
		},
		"sanitize-test-asa": {
			input: "GigabitEthernet 0/0",
			want:  "GigabitEthernet-0-0",
		},
		"sanitize-test-junos": {
			input: "ge-0/0/0",
			want:  "ge-0-0-0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := SanitizeInterfaceName(tt.input)
			if got != tt.want {
				t.Errorf("got wrong sanitized interface name %q, want %q", got, tt.want)
			}
		})
	}
}

// cover the getter for endpoint IPv4/v6 vars.
func TestEndpointVarIPGetter(t *testing.T) {
	t.Run("nil Vars returns empty strings", func(t *testing.T) {
		eg := &EndpointGeneric{Vars: nil}
		if got := eg.GetIPv4Addr(); got != "" {
			t.Errorf("GetIPv4Addr() = %q; want empty", got)
		}
		if got := eg.GetIPv6Addr(); got != "" {
			t.Errorf("GetIPv6Addr() = %q; want empty", got)
		}
	})

	t.Run("non-nil Vars returns set values", func(t *testing.T) {
		eg := &EndpointGeneric{Vars: &EndpointVars{IPv4: "192.0.2.1/31", IPv6: "2001:db8::1/64"}}
		if got := eg.GetIPv4Addr(); got != "192.0.2.1/31" {
			t.Errorf("GetIPv4Addr() = %q; want %q", got, "192.0.2.1/31")
		}
		if got := eg.GetIPv6Addr(); got != "2001:db8::1/64" {
			t.Errorf("GetIPv6Addr() = %q; want %q", got, "2001:db8::1/64")
		}
	})
}
