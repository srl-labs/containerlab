package links

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/jsimonetti/rtnetlink/rtnl"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// LinkVxlanRaw is the raw (string) representation of a vxlan link as defined in the topology file.
type LinkVxlanRaw struct {
	LinkCommonParams `yaml:",inline"`
	Remote           string      `yaml:"remote"`
	Vni              int         `yaml:"vni"`
	Endpoint         EndpointRaw `yaml:"endpoint"`
	UdpPort          int         `yaml:"udp-port,omitempty"`
	ParentInterface  string      `yaml:"parent-interface,omitempty"`
}

func (lr *LinkVxlanRaw) Resolve(params *ResolveParams) (Link, error) {
	var err error
	link := &LinkVxlan{
		deploymentState:  LinkDeploymentStateNotDeployed,
		LinkCommonParams: lr.LinkCommonParams,
	}

	// resolve local Endpoint
	link.LocalEndpoint, err = lr.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(lr.Remote)
	// if the returned ip is nil, an error occured.
	// we consider, that we maybe have a textual hostname
	// e.g. dns name so we try to resolve the string next
	if ip == nil {
		ips, err := net.LookupIP(lr.Remote)
		if err != nil {
			return nil, err
		}

		// prepare log message
		sb := strings.Builder{}
		for _, ip := range ips {
			sb.WriteString(", ")
			sb.WriteString(ip.String())
		}
		log.Debugf("looked up hostname %s, received IP addresses [%s]", lr.Remote, sb.String()[2:])

		// always use the first address
		if len(ips) <= 0 {
			return nil, fmt.Errorf("unable to resolve %s", lr.Remote)
		}
		ip = ips[0]
	}

	parentIf := lr.ParentInterface

	if parentIf == "" {
		conn, err := rtnl.Dial(nil)
		if err != nil {
			return nil, fmt.Errorf("can't establish netlink connection: %s", err)
		}
		defer conn.Close()
		r, err := conn.RouteGet(ip)
		if err != nil {
			return nil, fmt.Errorf("failed to find a route to VxLAN remote address %s", ip.String())
		}
		parentIf = r.Interface.Name
	}

	// resolve remote endpoint
	link.RemoteEndpoint = NewEndpointVxlan(params.Nodes["host"], link)
	link.RemoteEndpoint.parentIface = parentIf
	link.RemoteEndpoint.udpPort = lr.UdpPort
	link.RemoteEndpoint.remote = ip
	link.RemoteEndpoint.vni = lr.Vni

	// add link to local endpoints node
	link.LocalEndpoint.GetNode().AddLink(link)

	return link, nil
}

func (lr *LinkVxlanRaw) GetType() LinkType {
	return LinkTypeVxlan
}

type LinkVxlan struct {
	LinkCommonParams
	LocalEndpoint  Endpoint
	RemoteEndpoint *EndpointVxlan

	deploymentState LinkDeploymentState
}

func (l *LinkVxlan) Deploy(ctx context.Context) error {

	// retrieve the parent interface netlink handle
	parentIface, err := netlink.LinkByName(l.RemoteEndpoint.parentIface)
	if err != nil {
		return err
	}

	// create the Vxlan struct
	vxlanconf := netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:   l.LocalEndpoint.GetRandIfaceName(),
			TxQLen: 1000,
		},
		VxlanId:      l.RemoteEndpoint.vni,
		VtepDevIndex: parentIface.Attrs().Index,
		Group:        l.RemoteEndpoint.remote,
		Learning:     true,
		L2miss:       true,
		L3miss:       true,
	}
	// set the upd port if defined in the input
	if l.RemoteEndpoint.udpPort != 0 {
		vxlanconf.Port = l.RemoteEndpoint.udpPort
	}
	// define the MTU if defined in the input
	if l.MTU != 0 {
		vxlanconf.LinkAttrs.MTU = l.MTU
	}
	// add the link
	err = netlink.LinkAdd(&vxlanconf)
	if err != nil {
		return nil
	}

	return nil
}

func (l *LinkVxlan) Remove(_ context.Context) error {
	// TODO
	return nil
}

func (l *LinkVxlan) GetEndpoints() []Endpoint {
	return []Endpoint{l.LocalEndpoint, l.RemoteEndpoint}
}

func (l *LinkVxlan) GetType() LinkType {
	return LinkTypeVxlan
}
