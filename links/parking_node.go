package links

import (
	"context"
	"fmt"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type ParkingNode struct {
	GenericLinkNode
	containerName string
}

func NewParkingNode(containerName, nsPath string) *ParkingNode {
	return &ParkingNode{
		GenericLinkNode: GenericLinkNode{
			shortname: clabutils.ParkingNetnsName(containerName),
			endpoints: []Endpoint{},
			nspath:    nsPath,
		},
		containerName: containerName,
	}
}

func (p *ParkingNode) RepointSymlink() error {
	return clabutils.LinkContainerNS(p.nspath, p.containerName)
}

func (p *ParkingNode) CaptureFrom(ctx context.Context, src Node) error {
	endpoints, err := p.captureCandidates(ctx, src)
	if err != nil {
		return err
	}

	moved := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if err := moveEndpoint(ctx, ep, p); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				if err := moveEndpoint(ctx, moved[i], src); err == nil {
					_ = moved[i].Activate(ctx)
				}
			}
			return fmt.Errorf(
				"failed to park interface %q for node %q: %w",
				ep.GetIfaceName(),
				src.GetShortName(),
				err,
			)
		}
		moved = append(moved, ep)
	}

	return nil
}

func (p *ParkingNode) RestoreTo(ctx context.Context, dst Node) ([]Endpoint, error) {
	if err := p.renamePairedEndpoints(ctx, dst); err != nil {
		return nil, err
	}
	if err := p.DiscoverOwnedEndpoints(ctx, dst); err != nil {
		return nil, err
	}

	endpoints := append([]Endpoint(nil), p.GetEndpoints()...)
	moved := make([]Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		if err := moveEndpoint(ctx, ep, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moveEndpoint(ctx, moved[i], p)
			}
			return nil, fmt.Errorf(
				"failed to restore interface %q for node %q: %w",
				ep.GetIfaceName(),
				dst.GetShortName(),
				err,
			)
		}
		if err := ep.Activate(ctx); err != nil {
			_ = moveEndpoint(ctx, ep, p)
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moveEndpoint(ctx, moved[i], p)
			}
			return nil, fmt.Errorf(
				"failed to activate interface %q for node %q: %w",
				ep.GetIfaceName(),
				dst.GetShortName(),
				err,
			)
		}
		moved = append(moved, ep)
	}

	return moved, nil
}

// renamePairedEndpoints maps parked veths to the destination topology by their
// surviving peer. The parked interface name belongs to the old node kind and
// is not part of the preserved link's identity.
func (p *ParkingNode) renamePairedEndpoints(ctx context.Context, dst Node) error {
	desiredByPeerIndex := map[int]string{}
	for _, desired := range dst.GetEndpoints() {
		linkEndpoints := RuntimeEndpoints(desired.GetLink())
		if len(linkEndpoints) != 2 {
			continue
		}
		var peer Endpoint
		if linkEndpoints[0] == desired {
			peer = linkEndpoints[1]
		} else if linkEndpoints[1] == desired {
			peer = linkEndpoints[0]
		} else {
			continue
		}

		err := peer.GetNode().ExecFunction(ctx, func(_ ns.NetNS) error {
			peerLink, err := netlink.LinkByName(peer.GetIfaceName())
			if _, notFound := err.(netlink.LinkNotFoundError); notFound {
				return nil
			}
			if err != nil {
				return err
			}
			desiredByPeerIndex[peerLink.Attrs().Index] = desired.GetIfaceName()
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to inspect peer for %q: %w", desired.GetIfaceName(), err)
		}
	}

	return p.ExecFunction(ctx, func(_ ns.NetNS) error {
		parkedLinks, err := netlink.LinkList()
		if err != nil {
			return err
		}
		for _, parkedLink := range parkedLinks {
			if parkedLink.Type() != "veth" {
				continue
			}
			veth, ok := parkedLink.(*netlink.Veth)
			if !ok {
				veth = &netlink.Veth{LinkAttrs: *parkedLink.Attrs()}
			}
			peerIndex, err := netlink.VethPeerIndex(veth)
			if err != nil {
				return err
			}
			newName, matched := desiredByPeerIndex[peerIndex]
			oldName := parkedLink.Attrs().Name
			if !matched || newName == oldName {
				continue
			}
			if !HasOwnershipAltNameFor(parkedLink, dst.GetShortName(), oldName) {
				continue
			}
			if err := netlink.LinkSetName(parkedLink, newName); err != nil {
				return fmt.Errorf("failed to rename parked interface %q to %q: %w", oldName, newName, err)
			}
			if err := replaceOwnershipAltName(parkedLink, dst.GetShortName(), oldName, newName); err != nil {
				_ = netlink.LinkSetName(parkedLink, oldName)
				return err
			}
			log.Infof("Renamed parked interface node=%s interface=%s new-interface=%s", dst.GetShortName(), oldName, newName)
		}
		return nil
	})
}

func (p *ParkingNode) DiscoverOwnedEndpoints(ctx context.Context, original Node) error {
	ifaceNames, err := listOwnedInterfaceNames(
		ctx,
		p,
		trackedIfaceNames(original.GetEndpoints(), p.GetEndpoints()),
	)
	if err != nil {
		return err
	}

	discovered := make(map[string]struct{}, len(ifaceNames))
	for _, ifaceName := range ifaceNames {
		discovered[ifaceName] = struct{}{}
	}

	for _, ep := range append([]Endpoint(nil), p.GetEndpoints()...) {
		if _, exists := discovered[ep.GetIfaceName()]; exists {
			continue
		}

		if err := p.ReleaseEndpoint(ep); err != nil {
			return err
		}
		if ep.IsRuntimeDiscovered() {
			continue
		}

		ep.SetNode(original)
		if err := original.AdoptEndpoint(ep); err != nil {
			return err
		}
	}

	known := make(map[string]Endpoint, len(original.GetEndpoints())+len(p.GetEndpoints()))
	for _, ep := range append([]Endpoint(nil), original.GetEndpoints()...) {
		known[ep.GetIfaceName()] = ep
	}
	for _, ep := range append([]Endpoint(nil), p.GetEndpoints()...) {
		known[ep.GetIfaceName()] = ep
	}

	for _, ifaceName := range ifaceNames {
		if ep, ok := known[ifaceName]; ok {
			if ep.GetNode() == Node(p) {
				continue
			}

			if err := ep.GetNode().ReleaseEndpoint(ep); err != nil {
				return err
			}
			ep.SetNode(p)
			if err := p.AdoptEndpoint(ep); err != nil {
				return err
			}
			continue
		}

		runtimeEp := NewRuntimeEndpoint(p, ifaceName)
		if err := p.AdoptEndpoint(runtimeEp); err != nil {
			return err
		}
	}

	return nil
}

func (p *ParkingNode) captureCandidates(
	ctx context.Context,
	src Node,
) ([]Endpoint, error) {
	trackedEndpoints := src.GetEndpoints()
	tracked := make(map[string]Endpoint, len(trackedEndpoints))
	for _, ep := range trackedEndpoints {
		tracked[ep.GetIfaceName()] = ep
	}
	presentIfaceNames, err := listOwnedInterfaceNames(ctx, src, trackedIfaceNames(trackedEndpoints))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to discover runtime interfaces for node %q: %w",
			src.GetShortName(),
			err,
		)
	}

	endpoints := make([]Endpoint, 0, len(presentIfaceNames))
	for _, ifaceName := range presentIfaceNames {
		ep, ok := tracked[ifaceName]
		if !ok {
			ep = NewRuntimeEndpoint(src, ifaceName)
			if err := src.AdoptEndpoint(ep); err != nil {
				return nil, err
			}
		}

		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

func trackedIfaceNames(endpointSets ...[]Endpoint) map[string]struct{} {
	ifaceNames := map[string]struct{}{}

	for _, endpoints := range endpointSets {
		for _, ep := range endpoints {
			ifaceNames[ep.GetIfaceName()] = struct{}{}
		}
	}

	return ifaceNames
}

func listOwnedInterfaceNames(
	ctx context.Context,
	node Node,
	knownIfaceNames map[string]struct{},
) ([]string, error) {
	var ifaces []string

	err := node.ExecFunction(ctx, func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			if !isOwnedInterface(link, knownIfaceNames) {
				continue
			}

			ifaces = append(ifaces, link.Attrs().Name)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.Sort(ifaces)

	return ifaces, nil
}

func isOwnedInterface(link netlink.Link, knownIfaceNames map[string]struct{}) bool {
	name := link.Attrs().Name
	switch name {
	case "lo", "eth0":
		return false
	}

	if _, known := knownIfaceNames[name]; !known && !hasOwnershipAltName(link) {
		return false
	}

	return true
}

func (*ParkingNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}
