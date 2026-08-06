package links

import (
	"context"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/go-cmp/cmp"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
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
					IPv4:   []string{"node1:10.10.10.1/24"},
					IPv6:   []string{"node1:2001:db8::1/64"},
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
					IPv4:   []string{"node1:10.10.10.1/24"},
					IPv6:   []string{"node1:2001:db8::1/64"},
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
					IPv4:   []string{"node1:10.10.10.1/24"},
					IPv6:   []string{"node1:2001:db8::1/64"},
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
					IPv4:   []string{"node1:10.10.10.1/24"},
					IPv6:   []string{"node1:2001:db8::1/64"},
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

func TestLinkVEthRaw_InvalidEndpointVarAFParsing(t *testing.T) {
	fn1 := newFakeNode("node1")
	fn2 := newFakeNode("node2")

	params := &ResolveParams{
		Nodes: map[string]Node{
			"node1": fn1,
			"node2": fn2,
		},
	}

	t.Run("IPv4 endpoint var has IPv6 prefix", func(t *testing.T) {
		r := &LinkVEthRaw{
			LinkCommonParams: LinkCommonParams{},
			Endpoints: []*EndpointRaw{
				{
					Node:  "node1",
					Iface: "eth1",
					IPv4:  "2001:db8::1/64",
				},
				{
					Node:  "node2",
					Iface: "eth2",
				},
			},
		}
		if _, err := r.Resolve(params); err == nil {
			t.Fatalf("want error for non-IPv4 prefix in IPv4 var")
		}
	})

	t.Run("IPv6 endpoint var has IPv4 prefix", func(t *testing.T) {
		r := &LinkVEthRaw{
			LinkCommonParams: LinkCommonParams{},
			Endpoints: []*EndpointRaw{
				{
					Node:  "node1",
					Iface: "eth1",
					IPv6:  "10.10.10.1/24",
				},
				{
					Node:  "node2",
					Iface: "eth2",
				},
			},
		}
		if _, err := r.Resolve(params); err == nil {
			t.Fatalf("want error for non-IPv6 prefix in IPv6 var")
		}
	})
}

func TestLinkVEthRaw_BriefDefaultLinkType(t *testing.T) {
	t.Run("nodes without provider use veth", func(t *testing.T) {
		r := &LinkVEthRaw{
			Endpoints: []*EndpointRaw{
				{Node: "node1"},
				{Node: "node2"},
			},
		}
		params := &ResolveParams{
			Nodes: map[string]Node{
				"node1": newFakeNode("node1"),
				"node2": newFakeNode("node2"),
			},
		}

		if got := r.briefDefaultLinkType(params); got != LinkTypeVEth {
			t.Fatalf("briefDefaultLinkType() = %q, want %q", got, LinkTypeVEth)
		}
	})

	t.Run("optional provider overrides veth", func(t *testing.T) {
		r := &LinkVEthRaw{
			Endpoints: []*EndpointRaw{
				{Node: "node1"},
				{Node: "srsim"},
			},
		}
		params := &ResolveParams{
			Nodes: map[string]Node{
				"node1": newFakeNode("node1"),
				"srsim": &fakeDefaultLinkTypeNode{
					fakeNode: newFakeNode("srsim"),
					linkType: LinkTypeVethStitch,
				},
			},
		}

		if got := r.briefDefaultLinkType(params); got != LinkTypeVethStitch {
			t.Fatalf("briefDefaultLinkType() = %q, want %q", got, LinkTypeVethStitch)
		}
	})

	t.Run("addressed brief link is promoted", func(t *testing.T) {
		r := &LinkVEthRaw{
			LinkCommonParams: LinkCommonParams{
				IPv4: []string{"10.0.0.1/31", "10.0.0.0/31"},
				IPv6: []string{"2001:db8::1/127", "2001:db8::/127"},
			},
			Endpoints: []*EndpointRaw{
				{
					Node: "node1", Iface: "eth1",
					IPv4: "10.0.0.1/31", IPv6: "2001:db8::1/127",
				},
				{
					Node: "srsim", Iface: "eth2",
					IPv4: "10.0.0.0/31", IPv6: "2001:db8::/127",
				},
			},
			fromBrief: true,
		}
		params := &ResolveParams{
			LabName: "lab",
			Nodes: map[string]Node{
				"node1": newFakeNode("node1"),
				"srsim": &fakeDefaultLinkTypeNode{
					fakeNode: newFakeNode("srsim"),
					linkType: LinkTypeVethStitch,
				},
			},
		}

		got, err := r.Resolve(params)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		stitched, ok := got.(*LinkVEthStitched)
		if !ok {
			t.Fatalf("Resolve() type = %T, want *LinkVEthStitched", got)
		}
		if got := stitched.GetEndpoints()[0].GetIPv4Addr().String(); got != "10.0.0.1/31" {
			t.Fatalf("node endpoint IPv4 = %q, want 10.0.0.1/31", got)
		}
		if got := stitched.GetEndpoints()[0].GetIPv6Addr().String(); got != "2001:db8::1/127" {
			t.Fatalf("node endpoint IPv6 = %q, want 2001:db8::1/127", got)
		}
	})
}

// fakeNode is a fake implementation of Node for testing.
type fakeNode struct {
	Name      string
	Endpoints []Endpoint
	State     clabnodesstate.NodeState
	Links     []Link
}

type fakeDefaultLinkTypeNode struct {
	*fakeNode
	linkType LinkType
}

func (n *fakeDefaultLinkTypeNode) DefaultLinkType() LinkType {
	return n.linkType
}

func newFakeNode(name string) *fakeNode {
	return &fakeNode{Name: name}
}

func (*fakeNode) AddLinkToContainer(
	_ context.Context,
	_ netlink.Link,
	_ func(ns.NetNS) error,
) error {
	panic("not implemented")
}

func (f *fakeNode) AddLink(l Link) {
	f.Links = append(f.Links, l)
}

// AddEndpoint adds the Endpoint to the node.
func (f *fakeNode) AddEndpoint(e Endpoint) error {
	return f.AdoptEndpoint(e)
}

func (f *fakeNode) AdoptEndpoint(e Endpoint) error {
	f.Endpoints = append(f.Endpoints, e)
	return nil
}

func (f *fakeNode) ReleaseEndpoint(e Endpoint) error {
	for i, ep := range f.Endpoints {
		if ep != e {
			continue
		}

		f.Endpoints = append(f.Endpoints[:i], f.Endpoints[i+1:]...)
		return nil
	}

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

func (f *fakeNode) GetState() clabnodesstate.NodeState {
	return f.State
}

func (*fakeNode) Delete(context.Context) error {
	return nil
}
