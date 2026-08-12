package links

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

var ErrInterfaceUnowned = errors.New("interface is not marked as containerlab-owned")

// OwnedInterface describes a containerlab-owned interface discovered in a node's
// network namespace.
type OwnedInterface struct {
	Name        string
	Index       int
	PeerIndex   int
	MasterIndex int
	MasterName  string
	Type        string
}

// DiscoverOwnedInterfaces returns containerlab-owned interfaces and the
// netlink metadata needed to verify veth link pairing.
func DiscoverOwnedInterfaces(
	ctx context.Context,
	node Node,
	knownIfaceNames map[string]struct{},
) ([]OwnedInterface, error) {
	var interfaces []OwnedInterface

	err := node.ExecFunction(ctx, func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			name := link.Attrs().Name
			if name == "lo" || name == "eth0" {
				continue
			}
			if knownIfaceNames != nil {
				if _, known := knownIfaceNames[name]; !known {
					continue
				}
			}
			if !HasOwnershipAltNameFor(link, node.GetShortName(), name) {
				continue
			}

			info := OwnedInterface{
				Name:        name,
				Index:       link.Attrs().Index,
				MasterIndex: link.Attrs().MasterIndex,
				Type:        link.Type(),
			}
			if info.MasterIndex != 0 {
				master, masterErr := netlink.LinkByIndex(info.MasterIndex)
				if masterErr != nil {
					return fmt.Errorf(
						"failed to discover master for %q: %w",
						name,
						masterErr,
					)
				}
				info.MasterName = master.Attrs().Name
			}
			if info.Type == "veth" {
				veth, ok := link.(*netlink.Veth)
				if !ok {
					veth = &netlink.Veth{LinkAttrs: *link.Attrs()}
				}
				peerIndex, peerErr := netlink.VethPeerIndex(veth)
				if peerErr != nil {
					return fmt.Errorf("failed to discover veth peer for %q: %w", name, peerErr)
				}
				info.PeerIndex = peerIndex
			}

			interfaces = append(interfaces, info)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Name < interfaces[j].Name
	})

	return interfaces, nil
}

func DiscoverOwnedInterfaceNames(
	ctx context.Context,
	node Node,
	knownIfaceNames map[string]struct{},
) ([]string, error) {
	interfaces, err := DiscoverOwnedInterfaces(ctx, node, knownIfaceNames)
	if err != nil {
		return nil, err
	}

	ifaceNames := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		ifaceNames = append(ifaceNames, iface.Name)
	}
	sort.Strings(ifaceNames)

	return ifaceNames, nil
}

// ValidateOwnedInterface returns an error when an existing interface is not
// owned by the requested logical endpoint. Missing interfaces are valid.
func ValidateOwnedInterface(ctx context.Context, node Node, ifaceName string) error {
	return node.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifaceName)
		if _, notfound := err.(netlink.LinkNotFoundError); notfound {
			return nil
		}
		if err != nil {
			return err
		}
		if HasOwnershipAltNameFor(link, node.GetShortName(), ifaceName) {
			return nil
		}
		return fmt.Errorf(
			"interface %q on node %q exists but is not containerlab-owned",
			ifaceName,
			node.GetShortName(),
		)
	})
}

func RemoveOwnedInterface(ctx context.Context, node Node, ifaceName string) error {
	if node == nil || ifaceName == "" {
		return nil
	}

	return node.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifaceName)
		if _, notfound := err.(netlink.LinkNotFoundError); notfound {
			return nil
		}
		if err != nil {
			return err
		}
		if !HasOwnershipAltNameFor(link, node.GetShortName(), ifaceName) {
			return fmt.Errorf(
				"%w: interface %q on node %q",
				ErrInterfaceUnowned,
				ifaceName,
				node.GetShortName(),
			)
		}

		return netlink.LinkDel(link)
	})
}
