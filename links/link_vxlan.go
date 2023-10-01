package links

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/jsimonetti/rtnetlink/rtnl"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

const (
	// vxlan port is different from the default port number 4789
	// since 4789 may be filtered by the firewalls or clash with other overlay services.
	VxLANDefaultPort = 14789
)

// LinkVxlanRaw is the raw (string) representation of a vxlan link as defined in the topology file.
type LinkVxlanRaw struct {
	LinkCommonParams `yaml:",inline"`
	Remote           string      `yaml:"remote"`
	VNI              int         `yaml:"vni"`
	Endpoint         EndpointRaw `yaml:"endpoint"`
	UDPPort          int         `yaml:"udp-port,omitempty"`
	ParentInterface  string      `yaml:"parent-interface,omitempty"`
	NoLearning       bool        `yaml:"no-learning,omitempty"`
	NoL2Miss         bool        `yaml:"no-l2miss,omitempty"`
	NoL3Miss         bool        `yaml:"no-l3miss,omitempty"`

	// we use the same struct for vxlan and vxlan stitch, so we need to differentiate them in the raw format
	LinkType LinkType
}

func (lr *LinkVxlanRaw) Resolve(params *ResolveParams) (Link, error) {
	switch lr.LinkType {
	case LinkTypeVxlan:
		return lr.resolveRegular(params)
	case LinkTypeVxlanStitch:
		return lr.resolveStitched(params)
	default:
		return nil, fmt.Errorf("unexpected LinkType %s for Vxlan based link", lr.LinkType)
	}
}

func (lr *LinkVxlanRaw) resolveStitchedVxlan(params *ResolveParams, ifaceNamePost string) (*LinkVxlan, error) {
	var err error
	link := &LinkVxlan{
		LinkCommonParams: lr.LinkCommonParams,
		noLearning:       lr.NoLearning,
		noL2Miss:         lr.NoL2Miss,
		noL3Miss:         lr.NoL3Miss,
	}

	// point the vxlan endpoint to the host system
	vxlanRawEp := lr.Endpoint
	vxlanRawEp.Iface = fmt.Sprintf("vx-%s", ifaceNamePost)
	if params.VxlanIfaceNameOverwrite != "" {
		vxlanRawEp.Iface = fmt.Sprintf("vx-%s", params.VxlanIfaceNameOverwrite)
	}
	vxlanRawEp.Node = "host"
	vxlanRawEp.MAC = ""
	if err != nil {
		return nil, err
	}

	// resolve local Endpoint
	link.localEndpoint, err = vxlanRawEp.Resolve(params, link)
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
	link.remoteEndpoint = NewEndpointVxlan(params.Nodes["host"], link)
	link.remoteEndpoint.parentIface = parentIf
	link.remoteEndpoint.udpPort = lr.UDPPort
	if lr.UDPPort == 0 {
		link.remoteEndpoint.udpPort = VxLANDefaultPort
	}
	link.remoteEndpoint.remote = ip
	link.remoteEndpoint.vni = lr.VNI
	link.remoteEndpoint.MAC, err = utils.GenMac(ClabOUI)
	if err != nil {
		return nil, err
	}

	// add link to local endpoints node
	link.localEndpoint.GetNode().AddLink(link)

	return link, nil
}

// resolveStitchedVEth creates the veth link and return it, the endpoint that is
// supposed to be stitched is returned seperately for further processing
func (lr *LinkVxlanRaw) resolveStitchedVEth(params *ResolveParams, ifaceNamePost string) (*LinkVEth, Endpoint, error) {
	var err error

	veth := NewLinkVEth()
	veth.LinkCommonParams = lr.LinkCommonParams

	hostEpRaw := &EndpointRaw{
		Node:  "host",
		Iface: fmt.Sprintf("ve-%s", ifaceNamePost),
	}

	// overwrite the host side veth name. Used with the tools command
	if params.VxlanIfaceNameOverwrite != "" {
		hostEpRaw.Iface = params.VxlanIfaceNameOverwrite
	}

	hostEp, err := hostEpRaw.Resolve(params, veth)
	if err != nil {
		return nil, nil, err
	}

	containerEpRaw := lr.Endpoint

	containerEp, err := containerEpRaw.Resolve(params, veth)
	if err != nil {
		return nil, nil, err
	}

	veth.Endpoints = append(veth.Endpoints, hostEp, containerEp)

	return veth, hostEp, nil
}

func (lr *LinkVxlanRaw) resolveStitched(params *ResolveParams) (Link, error) {

	ifaceNamePost := fmt.Sprintf("%s-%s", lr.Endpoint.Node, lr.Endpoint.Iface)

	// if the resulting interface name is too long, we generate a random name
	// this will be used for the vxlan and and veth endpoint on the host side
	// but with different prefixes
	if len(ifaceNamePost) > 14 {
		oldName := ifaceNamePost
		ifaceNamePost = stableHashedInterfacename(ifaceNamePost, 8)
		log.Debugf("can't use %s as interface name postfix, falling back to %s", oldName, ifaceNamePost)
	}

	// prepare the vxlan struct
	vxlanLink, err := lr.resolveStitchedVxlan(params, ifaceNamePost)
	if err != nil {
		return nil, err
	}

	// prepare the veth struct
	vethLink, stitchEp, err := lr.resolveStitchedVEth(params, ifaceNamePost)
	if err != nil {
		return nil, err
	}

	// return the stitched vxlan link
	stitchedLink := NewVxlanStitched(vxlanLink, vethLink, stitchEp)

	// add stitched link to node
	params.Nodes[lr.Endpoint.Node].AddLink(stitchedLink)

	return stitchedLink, nil
}

func (lr *LinkVxlanRaw) resolveRegular(params *ResolveParams) (Link, error) {
	var err error
	link := &LinkVxlan{
		LinkCommonParams: lr.LinkCommonParams,
		noLearning:       lr.NoLearning,
		noL2Miss:         lr.NoL2Miss,
		noL3Miss:         lr.NoL3Miss,
	}

	// resolve local Endpoint
	link.localEndpoint, err = lr.Endpoint.Resolve(params, link)
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
	link.remoteEndpoint = NewEndpointVxlan(params.Nodes["host"], link)
	link.remoteEndpoint.parentIface = parentIf
	link.remoteEndpoint.udpPort = lr.UDPPort
	link.remoteEndpoint.remote = ip
	link.remoteEndpoint.vni = lr.VNI

	// add link to local endpoints node
	link.localEndpoint.GetNode().AddLink(link)

	return link, nil
}

func (*LinkVxlanRaw) GetType() LinkType {
	return LinkTypeVxlan
}

type LinkVxlan struct {
	LinkCommonParams
	localEndpoint  Endpoint
	remoteEndpoint *EndpointVxlan
	noLearning     bool
	noL2Miss       bool
	noL3Miss       bool
}

func (l *LinkVxlan) Deploy(ctx context.Context) error {
	err := l.deployVxlanInterface()
	if err != nil {
		return err
	}

	// retrieve the Link by name
	mvInterface, err := netlink.LinkByName(l.localEndpoint.GetRandIfaceName())
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", l.localEndpoint.GetRandIfaceName(), err)
	}

	// add the link to the Node Namespace
	err = l.localEndpoint.GetNode().AddLinkToContainer(ctx, mvInterface, SetNameMACAndUpInterface(mvInterface, l.localEndpoint))
	return err
}

// deployVxlanInterface internal function to create the vxlan interface in the host namespace
func (l *LinkVxlan) deployVxlanInterface() error {
	// retrieve the parent interface netlink handle
	parentIface, err := netlink.LinkByName(l.remoteEndpoint.parentIface)
	if err != nil {
		return err
	}

	// create the Vxlan struct
	vxlanconf := netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:         l.localEndpoint.GetRandIfaceName(),
			TxQLen:       1000,
			HardwareAddr: l.remoteEndpoint.MAC,
		},
		VxlanId:      l.remoteEndpoint.vni,
		VtepDevIndex: parentIface.Attrs().Index,
		Group:        l.remoteEndpoint.remote,
		Learning:     !l.noLearning, // invert the value - we make use of the bool default value == false
		L2miss:       !l.noL2Miss,   // invert the value
		L3miss:       !l.noL3Miss,   // invert the value
	}
	// set the upd port if defined in the input
	if l.remoteEndpoint.udpPort != 0 {
		vxlanconf.Port = l.remoteEndpoint.udpPort
	}

	// define the MTU if defined in the input
	if l.MTU != 0 {
		vxlanconf.LinkAttrs.MTU = l.MTU
	}

	// add the link
	err = netlink.LinkAdd(&vxlanconf)
	if err != nil {
		return err
	}

	// fetch the mtu from the actual state for templated config generation
	if l.MTU == 0 {
		interf, err := netlink.LinkByName(l.localEndpoint.GetRandIfaceName())
		if err != nil {
			return err
		}
		l.MTU = interf.Attrs().MTU
	}

	return nil
}

func (l *LinkVxlan) Remove(_ context.Context) error {
	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}
	err := l.localEndpoint.Remove()
	if err != nil {
		log.Debug(err)
	}
	l.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

func (l *LinkVxlan) GetEndpoints() []Endpoint {
	return []Endpoint{l.localEndpoint, l.remoteEndpoint}
}

func (*LinkVxlan) GetType() LinkType {
	return LinkTypeVxlan
}
