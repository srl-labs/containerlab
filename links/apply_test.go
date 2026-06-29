package links

import (
	"context"
	"sort"
	"strings"
	"testing"
)

func TestApplyRuntimeEndpointsForMacVlanExcludesParent(t *testing.T) {
	t.Parallel()

	node := newFakeNode("n1")
	host := newFakeNode("host")
	link := &LinkMacVlan{
		HostEndpoint: NewEndpointMacVlan(NewEndpointGeneric(host, "eth0", nil)),
		NodeEndpoint: NewEndpointVeth(NewEndpointGeneric(node, "eth1", nil)),
	}

	endpoints := ApplyRuntimeEndpoints(link)
	if len(endpoints) != 1 {
		t.Fatalf("expected one runtime endpoint, got %d", len(endpoints))
	}
	if got := endpoints[0].GetNode().GetShortName() + ":" + endpoints[0].GetIfaceName(); got != "n1:eth1" {
		t.Fatalf("unexpected runtime endpoint %q", got)
	}
}

func TestApplyRuntimeEndpointsForVxlanStitchedIncludesUnderlyingObjects(t *testing.T) {
	t.Parallel()

	node := newFakeNode("n1")
	host := newFakeNode("host")
	veth := &applyRuntimeFakeLink{}
	nodeEp := NewEndpointVeth(NewEndpointGeneric(node, "eth1", veth))
	hostEp := NewEndpointHost(NewEndpointGeneric(host, "ve-n1_eth1", veth))
	veth.endpoints = []Endpoint{nodeEp, hostEp}

	vxlan := &LinkVxlan{
		localEndpoint:  NewEndpointVeth(NewEndpointGeneric(host, "vx-n1_eth1", nil)),
		remoteEndpoint: NewEndpointVxlan(host, nil),
	}
	link := NewVxlanStitched(vxlan, veth, hostEp)

	if got := endpointTokens(ApplyRuntimeEndpoints(link)); got != "host:ve-n1_eth1,host:vx-n1_eth1,n1:eth1" {
		t.Fatalf("unexpected runtime endpoints %q", got)
	}
}

func TestEndpointByInterfaceName(t *testing.T) {
	t.Parallel()

	node := newFakeNode("n1")
	host := newFakeNode("host")
	nodeEp := NewEndpointVeth(NewEndpointGeneric(node, "eth1", nil))
	hostEp := NewEndpointHost(NewEndpointGeneric(host, "ve-n1_eth1", nil))

	got, ok := endpointByInterfaceName([]Endpoint{nodeEp, nil, hostEp}, "ve-n1_eth1")
	if !ok {
		t.Fatal("expected endpoint to be found")
	}
	if got != hostEp {
		t.Fatalf("unexpected endpoint %v", got)
	}

	if _, ok := endpointByInterfaceName([]Endpoint{nodeEp, hostEp}, "missing"); ok {
		t.Fatal("expected missing endpoint not to be found")
	}
}

type applyRuntimeFakeLink struct {
	endpoints []Endpoint
}

func (*applyRuntimeFakeLink) Deploy(context.Context, Endpoint) error {
	return nil
}

func (*applyRuntimeFakeLink) Remove(context.Context) error {
	return nil
}

func (*applyRuntimeFakeLink) GetType() LinkType {
	return LinkTypeVEth
}

func (l *applyRuntimeFakeLink) GetEndpoints() []Endpoint {
	return l.endpoints
}

func (l *applyRuntimeFakeLink) GetRuntimeEndpoints() []Endpoint {
	return l.endpoints
}

func (*applyRuntimeFakeLink) GetMTU() int {
	return 0
}

func (*applyRuntimeFakeLink) GetVars() map[string]any {
	return nil
}

func endpointTokens(endpoints []Endpoint) string {
	tokens := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		token := endpointToken(ep)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return strings.Join(tokens, ",")
}

func endpointToken(ep Endpoint) string {
	if ep == nil || ep.GetNode() == nil || ep.GetIfaceName() == "" {
		return ""
	}
	nodeName := ep.GetNode().GetShortName()
	if ep.IsNodeless() && ep.GetNode().GetLinkEndpointType() == LinkEndpointTypeBridge {
		nodeName = "mgmt-net"
	}
	return nodeName + ":" + ep.GetIfaceName()
}
