package links

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
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

	// we use the same struct for vxlan and vxlan stitch, so we need to differentiate them in the raw format
	LinkType LinkType
}

func (lr *LinkVxlanRaw) Resolve(params *ResolveParams) (Link, error) {
	switch lr.LinkType {
	case LinkTypeVxlan:
		return lr.resolveVxlan(params, false)

	case LinkTypeVxlanStitch:
		return lr.resolveStitchedVxlan(params)

	default:
		return nil, fmt.Errorf("unexpected LinkType %s for Vxlan based link", lr.LinkType)
	}
}

// resolveStitchedVEthComponent creates the veth link and returns it, the endpoint that is
// supposed to be stitched is returned separately for further processing.
func (lr *LinkVxlanRaw) resolveStitchedVEthComponent(params *ResolveParams) (*LinkVEth, Endpoint, error) {
	var err error

	// hostIface is the name of the host interface that will be created
	hostIface := fmt.Sprintf("ve-%s_%s", lr.Endpoint.Node, lr.Endpoint.Iface)

	// when tools vxlan create command is used, the hostIface is provided
	// by the user, otherwise it is generated
	if params.VxlanIfaceNameOverwrite != "" {
		hostIface = params.VxlanIfaceNameOverwrite
	}

	lhr := &LinkHostRaw{
		LinkCommonParams: lr.LinkCommonParams,
		HostInterface:    hostIface,
		Endpoint: &EndpointRaw{
			Node:  lr.Endpoint.Node,
			Iface: lr.Endpoint.Iface,
		},
	}

	hl, err := lhr.Resolve(params)
	if err != nil {
		return nil, nil, err
	}

	vethLink := hl.(*LinkVEth)

	// host endpoint is always the 2nd element in the Endpoints slice
	return vethLink, vethLink.Endpoints[1], nil
}

// resolveStitchedVxlan resolves the stitched raw vxlan link.
func (lr *LinkVxlanRaw) resolveStitchedVxlan(params *ResolveParams) (Link, error) {
	// prepare the vxlan struct
	vxlanLink, err := lr.resolveVxlan(params, true)
	if err != nil {
		return nil, err
	}

	// prepare the veth struct
	vethLink, stitchEp, err := lr.resolveStitchedVEthComponent(params)
	if err != nil {
		return nil, err
	}

	// return the stitched vxlan link
	vxlanStitchedLink := NewVxlanStitched(vxlanLink, vethLink, stitchEp)

	return vxlanStitchedLink, nil
}

func (lr *LinkVxlanRaw) resolveVxlan(params *ResolveParams, stitched bool) (*LinkVxlan, error) {
	var err error
	link := &LinkVxlan{
		LinkCommonParams: lr.LinkCommonParams,
	}

	link.localEndpoint, err = lr.resolveLocalEndpoint(stitched, params, link)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(lr.Remote)
	// if the returned ip is nil, an error occurred.
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
		if len(ips) == 0 {
			return nil, fmt.Errorf("unable to resolve %s", lr.Remote)
		}
		ip = ips[0]
	}

	parentIf := lr.ParentInterface

	if parentIf == "" {
		r, err := clabutils.GetRouteForIP(ip)
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
	// check if MAC-Addr is set in the raw vxlan link
	if lr.Endpoint.MAC == "" {
		// if it is not set generate a MAC
		link.remoteEndpoint.MAC, err = clabutils.GenMac(clabconstants.ClabOUI)
		if err != nil {
			return nil, err
		}
	} else {
		// if a MAC is set, parse and use it
		hwaddr, err := net.ParseMAC(lr.Endpoint.MAC)
		if err != nil {
			return nil, err
		}
		link.remoteEndpoint.MAC = hwaddr
	}

	return link, nil
}

func (lr *LinkVxlanRaw) resolveLocalEndpoint(stitched bool, params *ResolveParams, link *LinkVxlan) (Endpoint, error) {
	if stitched {
		// point the vxlan endpoint to the host system
		vxlanRawEp := lr.Endpoint
		vxlanRawEp.Iface = fmt.Sprintf("vx-%s_%s", lr.Endpoint.Node, lr.Endpoint.Iface)

		if params.VxlanIfaceNameOverwrite != "" {
			vxlanRawEp.Iface = fmt.Sprintf("vx-%s", params.VxlanIfaceNameOverwrite)
		}

		// in the stitched vxlan mode we create vxlan interface in the host node namespace
		vxlanRawEp.Node = "host"
		vxlanRawEp.MAC = ""

		// resolve local Endpoint
		return vxlanRawEp.Resolve(params, link)
	} else {
		// resolve local Endpoint
		return lr.Endpoint.Resolve(params, link)
	}
}

func (*LinkVxlanRaw) GetType() LinkType {
	return LinkTypeVxlan
}

type LinkVxlan struct {
	LinkCommonParams
	localEndpoint  Endpoint
	remoteEndpoint *EndpointVxlan
}

func (l *LinkVxlan) Deploy(ctx context.Context, _ Endpoint) error {
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
	err = l.localEndpoint.GetNode().AddLinkToContainer(ctx, mvInterface,
		SetNameMACAndUpInterface(mvInterface, l.localEndpoint))
	return err
}

// deployVxlanInterface internal function to create the vxlan interface in the host namespace.
func (l *LinkVxlan) deployVxlanInterface() error {
	// retrieve the parent interface netlink handle
	parentIface, err := netlink.LinkByName(l.remoteEndpoint.parentIface)
	if err != nil {
		return fmt.Errorf("error looking up vxlan parent interface %s: %w", l.remoteEndpoint.parentIface, err)
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
		Learning:     true,
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
		return fmt.Errorf("error adding vxlan link %s: %w", l.localEndpoint.String(), err)
	}

	// fetch the mtu from the actual state for templated config generation
	if l.MTU == 0 {
		interf, err := netlink.LinkByName(l.localEndpoint.GetRandIfaceName())
		if err != nil {
			return fmt.Errorf("error looking up local vxlan endpoint of %s : %w", l.localEndpoint.String(), err)
		}
		l.MTU = interf.Attrs().MTU
	}

	return nil
}

func (l *LinkVxlan) Remove(ctx context.Context) error {
	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}
	err := l.localEndpoint.Remove(ctx)
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
