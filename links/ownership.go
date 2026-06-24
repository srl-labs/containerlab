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

func DiscoverOwnedInterfaceNames(
	ctx context.Context,
	node Node,
	knownIfaceNames map[string]struct{},
) ([]string, error) {
	var ifaceNames []string

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
			if !hasOwnershipAltName(link) {
				continue
			}

			ifaceNames = append(ifaceNames, name)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(ifaceNames)

	return ifaceNames, nil
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
		if !hasOwnershipAltName(link) {
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
