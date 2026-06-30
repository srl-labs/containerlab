package links

import (
	"context"
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

func (p *ParkingNode) CaptureFrom(ctx context.Context, src Node) error {
	endpoints, err := p.captureCandidates(ctx, src)
	if err != nil {
		return err
	}

	moved := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if err := ep.MoveTo(ctx, p); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				if err := moved[i].MoveTo(ctx, src); err == nil {
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
	if err := p.DiscoverOwnedEndpoints(ctx, dst); err != nil {
		return nil, err
	}

	endpoints := append([]Endpoint(nil), p.GetEndpoints()...)
	moved := make([]Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		if err := ep.MoveTo(ctx, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moved[i].MoveTo(ctx, p)
			}
			return nil, fmt.Errorf(
				"failed to restore interface %q for node %q: %w",
				ep.GetIfaceName(),
				dst.GetShortName(),
				err,
			)
		}
		if err := ep.Activate(ctx); err != nil {
			_ = ep.MoveTo(ctx, p)
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moved[i].MoveTo(ctx, p)
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
