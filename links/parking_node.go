package links

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// containerNamer is implemented by nodes that expose a runtime container name,
// used to locate the deterministic parking netns for a peer.
type containerNamer interface {
	GetContainerName() string
}

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

type pendingPeerRename struct {
	desiredName string
	peerName    string
}

type indexedIface struct {
	name  string
	index int
}

// renamePairedEndpoints maps parked veths to the destination topology by their
// surviving peer. The parked interface name belongs to the old node kind and
// is not part of the preserved link's identity.
//
// When both ends of a link are parked, the peer's new name is not in the live
// container ns. Resolve that peer from its parking ns (or from leftover old
// names already restored into the live ns) and match by trailing port index.
func (p *ParkingNode) renamePairedEndpoints(ctx context.Context, dst Node) error {
	desiredByPeerIndex := map[int]string{}
	pendingByPeer := map[string][]pendingPeerRename{}
	peerNodes := map[string]Node{}

	for _, desired := range dst.GetEndpoints() {
		peer, ok := otherRuntimeEndpoint(desired)
		if !ok {
			continue
		}

		index, found, err := linkIndexByName(ctx, peer.GetNode(), peer.GetIfaceName())
		if err != nil {
			return fmt.Errorf("failed to inspect peer for %q: %w", desired.GetIfaceName(), err)
		}
		if found {
			desiredByPeerIndex[index] = desired.GetIfaceName()
			continue
		}

		peerNodeName := peer.GetNode().GetShortName()
		pendingByPeer[peerNodeName] = append(pendingByPeer[peerNodeName], pendingPeerRename{
			desiredName: desired.GetIfaceName(),
			peerName:    peer.GetIfaceName(),
		})
		peerNodes[peerNodeName] = peer.GetNode()
	}

	for peerNodeName, pending := range pendingByPeer {
		mapped, err := resolveParkedPeerIndexes(
			ctx,
			peerNodes[peerNodeName],
			pending,
			desiredByPeerIndex,
		)
		if err != nil {
			return err
		}
		for index, name := range mapped {
			desiredByPeerIndex[index] = name
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

func otherRuntimeEndpoint(desired Endpoint) (Endpoint, bool) {
	if desired == nil || desired.GetLink() == nil {
		return nil, false
	}
	linkEndpoints := RuntimeEndpoints(desired.GetLink())
	if len(linkEndpoints) != 2 {
		return nil, false
	}
	if linkEndpoints[0] == desired {
		return linkEndpoints[1], true
	}
	if linkEndpoints[1] == desired {
		return linkEndpoints[0], true
	}
	return nil, false
}

func linkIndexByName(ctx context.Context, node Node, name string) (int, bool, error) {
	var index int
	found := false
	err := node.ExecFunction(ctx, func(_ ns.NetNS) error {
		peerLink, err := netlink.LinkByName(name)
		if _, notFound := err.(netlink.LinkNotFoundError); notFound {
			return nil
		}
		if err != nil {
			return err
		}
		index = peerLink.Attrs().Index
		found = true
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return index, found, nil
}

func parkingNodeFor(node Node) (*ParkingNode, bool) {
	namer, ok := node.(containerNamer)
	if !ok {
		return nil, false
	}
	containerName := namer.GetContainerName()
	if containerName == "" {
		return nil, false
	}
	parkPath, err := clabutils.GetNamedNetNS(clabutils.ParkingNetnsName(containerName))
	if err != nil {
		return nil, false
	}
	return NewParkingNode(containerName, parkPath), true
}

func resolveParkedPeerIndexes(
	ctx context.Context,
	peerNode Node,
	pending []pendingPeerRename,
	usedIndexes map[int]string,
) (map[int]string, error) {
	candidates, err := parkedPeerCandidates(ctx, peerNode, usedIndexes)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to inspect parked peers on %q: %w",
			peerNode.GetShortName(),
			err,
		)
	}
	return mapParkedPeerIndexes(pending, candidates), nil
}

func parkedPeerCandidates(
	ctx context.Context,
	peerNode Node,
	usedIndexes map[int]string,
) ([]indexedIface, error) {
	var candidates []indexedIface
	seen := map[int]struct{}{}

	appendOwned := func(node Node, ownerName string) error {
		ifaces, err := DiscoverOwnedInterfacesFor(ctx, node, ownerName, nil)
		if err != nil {
			return err
		}
		for _, iface := range ifaces {
			if iface.Type != "veth" || iface.Index == 0 {
				continue
			}
			if _, used := usedIndexes[iface.Index]; used {
				continue
			}
			if _, exists := seen[iface.Index]; exists {
				continue
			}
			candidates = append(candidates, indexedIface{
				name:  iface.Name,
				index: iface.Index,
			})
			seen[iface.Index] = struct{}{}
		}
		return nil
	}

	if err := appendOwned(peerNode, peerNode.GetShortName()); err != nil {
		return nil, err
	}
	if parkNode, ok := parkingNodeFor(peerNode); ok {
		if err := appendOwned(parkNode, peerNode.GetShortName()); err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

func mapParkedPeerIndexes(pending []pendingPeerRename, candidates []indexedIface) map[int]string {
	result := map[int]string{}
	used := map[int]struct{}{}
	matched := make([]bool, len(pending))

	tryMatch := func(name string, i int) bool {
		port, ok := ifacePortIndex(name)
		if !ok {
			return false
		}
		cand, ok := uniqueIfaceByPort(candidates, used, port)
		if !ok {
			return false
		}
		result[cand.index] = pending[i].desiredName
		used[cand.index] = struct{}{}
		matched[i] = true
		return true
	}

	for i, item := range pending {
		if tryMatch(item.peerName, i) {
			continue
		}
		tryMatch(item.desiredName, i)
	}

	var leftoverPending []int
	for i, ok := range matched {
		if !ok {
			leftoverPending = append(leftoverPending, i)
		}
	}
	var leftoverCands []indexedIface
	for _, cand := range candidates {
		if _, ok := used[cand.index]; ok {
			continue
		}
		leftoverCands = append(leftoverCands, cand)
	}
	if len(leftoverPending) == 1 && len(leftoverCands) == 1 {
		result[leftoverCands[0].index] = pending[leftoverPending[0]].desiredName
	}

	return result
}

func uniqueIfaceByPort(candidates []indexedIface, used map[int]struct{}, port int) (indexedIface, bool) {
	found := indexedIface{}
	count := 0
	for _, cand := range candidates {
		if _, ok := used[cand.index]; ok {
			continue
		}
		candPort, ok := ifacePortIndex(cand.name)
		if !ok || candPort != port {
			continue
		}
		found = cand
		count++
		if count > 1 {
			return indexedIface{}, false
		}
	}
	if count != 1 {
		return indexedIface{}, false
	}
	return found, true
}

// ifacePortIndex returns the trailing decimal run of a Linux iface name
// (e1-3, et3, eth3).
func ifacePortIndex(name string) (int, bool) {
	end := len(name)
	start := end
	for start > 0 && name[start-1] >= '0' && name[start-1] <= '9' {
		start--
	}
	if start == end {
		return 0, false
	}
	port, err := strconv.Atoi(name[start:])
	if err != nil {
		return 0, false
	}
	return port, true
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
