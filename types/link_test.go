package types

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
		want    LinkDefinitionType
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
			name: "legacy link",
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
				LinkConfig: LinkConfig{
					Endpoints: []string{
						"srl1:e1-5",
						"srl2:e1-5",
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
				LinkConfig: LinkConfig{
					Endpoints: []string{
						"srl1:e1-5",
						"mgmt-net:srl1_e1-5",
					},
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
				LinkConfig: LinkConfig{
					Endpoints: []string{
						"srl1:e1-5",
						"host:srl1_e1-5",
					},
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
				LinkConfig: LinkConfig{
					Endpoints: []string{
						"srl1:e1-5",
						"macvlan:srl1_e1-5",
					},
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
