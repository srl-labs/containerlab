package links

import (
	"context"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/go-cmp/cmp"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/vishvananda/netlink"
)

func TestLinkVEthRaw_ToLinkBriefRaw(t *testing.T) {
	type fields struct {
		LinkCommonParams LinkCommonParams
		Endpoints        []*EndpointRaw
	}
	tests := []struct {
		name   string
		fields fields
		want   *LinkBriefRaw
	}{
		{
			name: "test1",
			fields: fields{
				LinkCommonParams: LinkCommonParams{
					MTU:    1500,
					Labels: map[string]string{"foo": "bar"},
					Vars:   map[string]any{"foo": "bar"},
				},
				Endpoints: []*EndpointRaw{
					{
						Node:  "node1",
						Iface: "eth1",
					},
					{
						Node:  "node2",
						Iface: "eth2",
					},
				},
			},
			want: &LinkBriefRaw{
				Endpoints: []string{"node1:eth1", "node2:eth2"},
				LinkCommonParams: LinkCommonParams{
					MTU:    1500,
					Labels: map[string]string{"foo": "bar"},
					Vars:   map[string]any{"foo": "bar"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LinkVEthRaw{
				LinkCommonParams: tt.fields.LinkCommonParams,
				Endpoints:        tt.fields.Endpoints,
			}

			got := r.ToLinkBriefRaw()

			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("LinkVEthRaw.ToLinkBriefRaw() = %s", d)
			}
		})
	}
}

func TestLinkVEthRaw_GetType(t *testing.T) {
	type fields struct {
		LinkCommonParams LinkCommonParams
		Endpoints        []*EndpointRaw
	}
	tests := []struct {
		name   string
		fields fields
		want   LinkType
	}{
		{
			name: "test1",
			fields: fields{
				LinkCommonParams: LinkCommonParams{},
				Endpoints:        []*EndpointRaw{},
			},
			want: LinkTypeVEth,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LinkVEthRaw{
				LinkCommonParams: tt.fields.LinkCommonParams,
				Endpoints:        tt.fields.Endpoints,
			}
			if got := r.GetType(); got != tt.want {
				t.Errorf("LinkVEthRaw.GetType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinkVEthRaw_Resolve(t *testing.T) {
	fn1 := newFakeNode("node1")
	fn2 := newFakeNode("node2")

	type fields struct {
		LinkCommonParams LinkCommonParams
		Endpoints        []*EndpointRaw
	}
	type args struct {
		params *ResolveParams
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *LinkVEth
		wantErr bool
	}{
		{
			name: "test1",
			fields: fields{
				LinkCommonParams: LinkCommonParams{
					MTU:    1500,
					Labels: map[string]string{"foo": "bar"},
					Vars:   map[string]any{"foo": "bar"},
				},
				Endpoints: []*EndpointRaw{
					{
						Node:  "node1",
						Iface: "eth1",
					},
					{
						Node:  "node2",
						Iface: "eth2",
					},
				},
			},
			args: args{
				params: &ResolveParams{
					Nodes: map[string]Node{
						"node1": fn1,
						"node2": fn2,
					},
				},
			},
			want: &LinkVEth{
				LinkCommonParams: LinkCommonParams{
					MTU:    1500,
					Labels: map[string]string{"foo": "bar"},
					Vars:   map[string]any{"foo": "bar"},
				},
				Endpoints: []Endpoint{
					&EndpointVeth{
						EndpointGeneric: EndpointGeneric{
							Node:      fn1,
							IfaceName: "eth1",
						},
					},
					&EndpointVeth{
						EndpointGeneric: EndpointGeneric{
							Node:      fn2,
							IfaceName: "eth2",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LinkVEthRaw{
				LinkCommonParams: tt.fields.LinkCommonParams,
				Endpoints:        tt.fields.Endpoints,
			}
			got, err := r.Resolve(tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("LinkVEthRaw.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			l := got.(*LinkVEth)
			if d := cmp.Diff(l.LinkCommonParams, tt.want.LinkCommonParams); d != "" {
				t.Errorf("LinkVEthRaw.Resolve() LinkCommonParams diff = %s", d)
			}

			for i, e := range l.Endpoints {
				if e.(*EndpointVeth).IfaceName != tt.want.Endpoints[i].(*EndpointVeth).IfaceName {
					t.Errorf("LinkVEthRaw.Resolve() EndpointVeth got %s, want %s",
						e.(*EndpointVeth).IfaceName, tt.want.Endpoints[i].(*EndpointVeth).IfaceName)
				}

				if e.(*EndpointVeth).Node != tt.want.Endpoints[i].(*EndpointVeth).Node {
					t.Errorf("LinkVEthRaw.Resolve() EndpointVeth got %s, want %s",
						e.(*EndpointVeth).Node, tt.want.Endpoints[i].(*EndpointVeth).Node)
				}
			}
		})
	}
}

// fakeNode is a fake implementation of Node for testing.
type fakeNode struct {
	Name      string
	Endpoints []Endpoint
	State     state.NodeState
	Links     []Link
}

func newFakeNode(name string) *fakeNode {
	return &fakeNode{Name: name}
}

func (*fakeNode) AddLinkToContainer(_ context.Context, _ netlink.Link, _ func(ns.NetNS) error) error {
	panic("not implemented")
}

func (f *fakeNode) AddLink(l Link) {
	f.Links = append(f.Links, l)
}

// AddEndpoint adds the Endpoint to the node.
func (f *fakeNode) AddEndpoint(e Endpoint) error {
	f.Endpoints = append(f.Endpoints, e)

	return nil
}

func (*fakeNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}

func (f *fakeNode) GetShortName() string {
	return f.Name
}

func (f *fakeNode) GetEndpoints() []Endpoint {
	return f.Endpoints
}

func (*fakeNode) ExecFunction(_ context.Context, _ func(ns.NetNS) error) error {
	panic("not implemented")
}

func (f *fakeNode) GetState() state.NodeState {
	return f.State
}

func (*fakeNode) Delete(context.Context) error {
	return nil
}
