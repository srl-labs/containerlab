package links

import (
	"context"
	"errors"
	"fmt"
	"slices"

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

func (p *ParkingNode) CaptureFrom(ctx context.Context, src EndpointOwner) error {
	endpoints, err := p.captureCandidates(ctx, src)
	if err != nil {
		return err
	}

	moved := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if err := ep.MoveTo(ctx, p, false); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moved[i].MoveTo(ctx, src, true)
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

func (p *ParkingNode) RestoreTo(ctx context.Context, dst EndpointOwner) error {
	if err := p.DiscoverOwnedEndpoints(ctx, dst); err != nil {
		return err
	}

	endpoints := append([]Endpoint(nil), p.GetEndpoints()...)
	moved := make([]Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		if err := ep.MoveTo(ctx, dst, true); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moved[i].MoveTo(ctx, p, false)
			}
			return fmt.Errorf(
				"failed to restore interface %q for node %q: %w",
				ep.GetIfaceName(),
				dst.GetShortName(),
				err,
			)
		}
		moved = append(moved, ep)
	}

	return nil
}

func (p *ParkingNode) DiscoverOwnedEndpoints(ctx context.Context, original EndpointOwner) error {
	ifaceNames, err := listOwnedInterfaceNames(ctx, p, trackedIfaceNames(original.GetEndpoints(), p.GetEndpoints()))
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

			currentOwner, ok := ep.GetNode().(EndpointOwner)
			if !ok {
				return fmt.Errorf(
					"node %q does not support endpoint ownership moves",
					ep.GetNode().GetShortName(),
				)
			}

			if err := currentOwner.ReleaseEndpoint(ep); err != nil {
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

func (p *ParkingNode) captureCandidates(ctx context.Context, src EndpointOwner) ([]Endpoint, error) {
	tracked := append([]Endpoint(nil), src.GetEndpoints()...)
	endpoints := make([]Endpoint, 0, len(tracked))
	knownIfaceNames := make(map[string]struct{}, len(tracked))

	for _, ep := range tracked {
		if ep.IsRuntimeDiscovered() {
			endpoints = append(endpoints, ep)
			knownIfaceNames[ep.GetIfaceName()] = struct{}{}
			continue
		}

		link := ep.GetLink()
		if link == nil || link.GetType() != LinkTypeVEth {
			linkType := "runtime-unknown"
			if link != nil {
				linkType = string(link.GetType())
			}

			return nil, fmt.Errorf(
				"node %q endpoint %q is linked via %q, but lifecycle stop/start supports only veth dataplane links",
				src.GetShortName(),
				ep.GetIfaceName(),
				linkType,
			)
		}

		endpoints = append(endpoints, ep)
		knownIfaceNames[ep.GetIfaceName()] = struct{}{}
	}

	runtimeIfaceNames, err := listOwnedInterfaceNames(ctx, src, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover runtime interfaces for node %q: %w", src.GetShortName(), err)
	}

	for _, ifaceName := range runtimeIfaceNames {
		if _, known := knownIfaceNames[ifaceName]; known {
			continue
		}

		runtimeEp := NewRuntimeEndpoint(src, ifaceName)
		if err := src.AdoptEndpoint(runtimeEp); err != nil {
			return nil, err
		}

		endpoints = append(endpoints, runtimeEp)
		knownIfaceNames[ifaceName] = struct{}{}
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

func listOwnedInterfaceNames(ctx context.Context, node Node, knownIfaceNames map[string]struct{}) ([]string, error) {
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

	if link.Type() != "veth" {
		return false
	}

	if _, known := knownIfaceNames[name]; !known && !hasOwnershipAltName(link) {
		return false
	}

	veth, ok := link.(*netlink.Veth)
	if !ok {
		return false
	}

	peerIndex, err := netlink.VethPeerIndex(veth)
	if err != nil {
		return false
	}

	_, err = netlink.LinkByIndex(peerIndex)
	if err == nil {
		return false
	}

	var notFoundErr netlink.LinkNotFoundError
	return errors.As(err, &notFoundErr)
}

func (*ParkingNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}
