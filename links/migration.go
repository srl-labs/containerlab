package links

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// NSMigrationNode is the minimal node surface the bridge-port migration
// works with: exec context plus a resolvable namespace path.
type NSMigrationNode interface {
	ExecFunction(ctx context.Context, f func(ns.NetNS) error) error
	GetNSPath(ctx context.Context) (string, error)
	GetShortName() string
}

// BridgePortRelocation describes a bridge-side veth end parked out of one
// namespace while the in-container bridge hosting it was replaced (a bridge
// parent move). The veth pair stays intact across the namespace hops: the
// client-side end keeps its address and MAC, and the parked half carries its
// device state until it is restored into the replacement namespace.
type BridgePortRelocation struct {
	parkNSPath   string
	KernelName   string // kernel-side name while parked
	OldOwnerName string
	IfaceName    string
	LongName     string // replacement bridge node's long name; exercises the parking netns name
}

// ParkBridgePortByOwnership finds the interface carrying the ownership altname
// of (ownerName, ifaceName) in src's namespace and parks it into the parking
// netns of longName. The pair survives: only the parked half is relocated.
func ParkBridgePortByOwnership(
	ctx context.Context,
	src Node,
	ownerName string,
	ifaceName string,
	longName string,
) (*BridgePortRelocation, error) {
	parkPath, err := clabutils.CreateOrGetNamedNetNS(clabutils.ParkingNetnsName(longName))
	if err != nil {
		return nil, fmt.Errorf("failed to create parking netns for migration: %w", err)
	}

	relocation := &BridgePortRelocation{
		parkNSPath:   parkPath,
		OldOwnerName: ownerName,
		IfaceName:    ifaceName,
		LongName:     longName,
	}

	parkErr := src.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := findLinkByOwnershipAltName(ownerName, ifaceName)
		if err != nil {
			return err
		}

		relocation.KernelName = link.Attrs().Name

		if err := netlink.LinkSetDown(link); err != nil {
			log.Debug("failed to bring down interface while parking",
				"iface", link.Attrs().Name, "err", err)
		}

		parkNs, nsErr := ns.GetNS(parkPath)
		if nsErr != nil {
			return nsErr
		}
		defer parkNs.Close()

		return netlink.LinkSetNsFd(link, int(parkNs.Fd()))
	})
	if parkErr != nil {
		_ = clabutils.UnlinkNamedNetNS(parkPath)
		return nil, parkErr
	}

	return relocation, nil
}

// RestoreBridgePort moves the parked half into the replacement bridge's
// namespace, keeps/renames it to the desired kernel name, relabels the
// ownership altname to (newOwnerName, ifaceName) and re-enslaves it to the
// bridge. dst receives a node with both ExecFunction and GetNSPath.
func (r *BridgePortRelocation) RestoreBridgePort(
	ctx context.Context,
	dst NSMigrationNode,
	newOwnerName string,
	bridgeName string,
	desiredKernelName string,
) error {
	parkExecErr := r.execInParkingNS(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(r.KernelName)
		if err != nil {
			return fmt.Errorf("parked interface %q not found in parking ns: %w", r.KernelName, err)
		}

		dstPath, nsErr := dst.GetNSPath(ctx)
		if nsErr != nil {
			return nsErr
		}
		dstNs, nsErr := ns.GetNS(dstPath)
		if nsErr != nil {
			return nsErr
		}
		defer dstNs.Close()

		return netlink.LinkSetNsFd(link, int(dstNs.Fd()))
	})
	if parkExecErr != nil {
		return parkExecErr
	}

	dstErr := dst.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(r.KernelName)
		if err != nil {
			return fmt.Errorf("restored interface %q not found in destination ns: %w",
				r.KernelName, err)
		}

		if desiredKernelName != "" && link.Attrs().Name != desiredKernelName {
			if err := netlink.LinkSetName(link, desiredKernelName); err != nil {
				return err
			}
			link.Attrs().Name = desiredKernelName
		}

		// Strip previous ownership markers and label with the new owner.
		for _, altName := range link.Attrs().AltNames {
			if !strings.HasPrefix(altName, ownershipAltNamePrefix) {
				continue
			}
			if delErr := netlink.LinkDelAltName(link, altName); delErr != nil &&
				!isAltNameNotSupportedErr(delErr) {
				return fmt.Errorf("failed to drop old ownership altname %q: %w", altName, delErr)
			}
		}
		if addErr := linkAddAltName(
			link,
			ownershipAltNameFor(newOwnerName, r.IfaceName),
		); addErr != nil &&
			!isAltNameNotSupportedErr(addErr) {
			return addErr
		}

		// Re-enslave to the replacement bridge, if present.
		if bridgeName != "" {
			if bridgeLink, err := netlink.LinkByName(bridgeName); err == nil {
				if link.Attrs().MasterIndex != bridgeLink.Attrs().Index {
					if err := netlink.LinkSetMaster(link, bridgeLink); err != nil {
						return fmt.Errorf("failed to enslave %q to %q: %w",
							link.Attrs().Name, bridgeName, err)
					}
					log.Info("Re-enslaved migrated bridge port to replacement bridge",
						"port", link.Attrs().Name, "bridge", bridgeName)
				}
			}
		}

		if link.Attrs().OperState != netlink.OperUp {
			if upErr := netlink.LinkSetUp(link); upErr != nil {
				return fmt.Errorf("failed to bring up %q: %w", link.Attrs().Name, upErr)
			}
		}

		return nil
	})
	if dstErr != nil {
		return dstErr
	}

	// Clean up the now empty parking namespace.
	_ = clabutils.UnlinkNamedNetNS(r.parkNSPath)

	return nil
}

func (r *BridgePortRelocation) execInParkingNS(f func(ns.NetNS) error) error {
	parkNs, err := ns.GetNS(r.parkNSPath)
	if err != nil {
		return err
	}
	defer parkNs.Close()

	return parkNs.Do(f)
}

// RelabelOwnershipAltNameFor rewrites the ownership altname of the interface
// identified by (oldOwner, ifaceName) to (newOwner, ifaceName) inside node's
// namespace.
func RelabelOwnershipAltNameFor(
	ctx context.Context,
	node Node,
	oldOwnerName string,
	ifaceName string,
	newOwnerName string,
) error {
	if newOwnerName == oldOwnerName {
		return nil
	}

	return node.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := findLinkByOwnershipAltName(oldOwnerName, ifaceName)
		if err != nil {
			// Fall back to the sanitized kernel name: short names kept their
			// kernel name during relocation.
			link, err = netlink.LinkByName(clabutils.SanitizeInterfaceName(ifaceName))
			if err != nil {
				return fmt.Errorf(
					"interface to relabel ownership for %q (old owner %q) not found: %w",
					ifaceName,
					oldOwnerName,
					err,
				)
			}
		}

		// Drop all previous ownership markers; ownership is single-valued.
		for _, altName := range link.Attrs().AltNames {
			if !strings.HasPrefix(altName, ownershipAltNamePrefix) {
				continue
			}
			if delErr := netlink.LinkDelAltName(link, altName); delErr != nil &&
				!isAltNameNotSupportedErr(delErr) {
				return fmt.Errorf("failed to drop old ownership altname %q: %w", altName, delErr)
			}
		}

		return linkAddAltName(link, ownershipAltNameFor(newOwnerName, ifaceName))
	})
}

// findLinkByOwnershipAltName scans the current namespace for the link carrying
// the ownership altname of (nodeName, ifaceName).
func findLinkByOwnershipAltName(nodeName, ifaceName string) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if HasOwnershipAltNameFor(link, nodeName, ifaceName) {
			return link, nil
		}
	}
	return nil, fmt.Errorf(
		"no interface carries the ownership marker of %q for %q",
		nodeName,
		ifaceName,
	)
}
