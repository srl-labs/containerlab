package links

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
				},
			},
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
