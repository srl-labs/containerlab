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
// netns keyed by the owner bridge's long name. Keying by the owner (not the
// migration target) keeps a parked port attributable and resolvable even when
// the target bridge later leaves the lab's topologies, and matches the
// destroy-time sweep enumeration. The pair survives: only the parked half is
// relocated. Idempotent: if the port is already parked, returns the existing
// relocation.
func ParkBridgePortByOwnership(
	ctx context.Context,
	src Node,
	ownerName string,
	ifaceName string,
	ownerLongName string,
) (*BridgePortRelocation, error) {
	parkPath, err := clabutils.CreateOrGetNamedNetNS(clabutils.ParkingNetnsName(ownerLongName))
	if err != nil {
		return nil, fmt.Errorf("failed to create parking netns for migration: %w", err)
	}

	relocation := &BridgePortRelocation{
		parkNSPath:   parkPath,
		OldOwnerName: ownerName,
		IfaceName:    ifaceName,
		LongName:     ownerLongName,
	}

	// Idempotency: a previous apply may have parked the port and failed before
	// restoring it. If the parking netns already holds the port, reuse the
	// parked half so an interrupted migration resumes instead of aborting.
	kernelName, parked, err := parkedKernelNameFor(parkPath, ownerName, ifaceName)
	if err != nil {
		return nil, err
	}
	if parked {
		relocation.KernelName = kernelName
		log.Debug("bridge port already parked, resuming migration",
			"owner", ownerName, "iface", ifaceName,
			"kernelName", kernelName, "parkNS", parkPath)
		return relocation, nil
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
		if unlinkErr := clabutils.UnlinkNamedNetNS(parkPath); unlinkErr != nil {
			log.Debug("failed to clean up parking netns after park error",
				"path", parkPath, "err", unlinkErr)
		}
		return nil, parkErr
	}

	return relocation, nil
}

// FindParkedBridgePortByOwnership reports whether a port carrying the
// ownership altname of (ownerName, ifaceName) is currently parked in the
// owner's parking netns and, when found, returns the relocation ready for
// restore. It creates and mutates nothing.
func FindParkedBridgePortByOwnership(
	ownerLongName, ownerName, ifaceName string,
) (*BridgePortRelocation, bool, error) {
	parkPath, err := clabutils.GetNamedNetNS(clabutils.ParkingNetnsName(ownerLongName))
	if err != nil {
		// A missing slot means the port cannot be parked, not an error.
		return nil, false, nil
	}

	kernelName, parked, err := parkedKernelNameFor(parkPath, ownerName, ifaceName)
	if err != nil || !parked {
		return nil, false, err
	}

	return &BridgePortRelocation{
		parkNSPath:   parkPath,
		KernelName:   kernelName,
		OldOwnerName: ownerName,
		IfaceName:    ifaceName,
		LongName:     ownerLongName,
	}, true, nil
}

// RemoveDesertedBridge removes a bridge device from node's namespace when no
// interface is mastered to it anymore. That is the state an in-container
// bridge's device reaches when all of its ports migrated out to a replacement
// bridge under a different parent container that survives the apply.
func RemoveDesertedBridge(
	ctx context.Context,
	node Node,
	bridgeName string,
) error {
	return node.ExecFunction(ctx, func(_ ns.NetNS) error {
		bridgeLink, err := netlink.LinkByName(bridgeName)
		if err != nil {
			// Not present; nothing to remove.
			return nil
		}

		links, linkErr := netlink.LinkList()
		if linkErr != nil {
			return linkErr
		}
		for _, link := range links {
			if link.Attrs().MasterIndex == bridgeLink.Attrs().Index {
				log.Debugf("Skipping removal of deserted bridge %q; it still masters %q",
					bridgeName, link.Attrs().Name)
				return nil
			}
		}

		if err := netlink.LinkDel(bridgeLink); err != nil {
			return fmt.Errorf("failed to remove deserted bridge %q: %w", bridgeName, err)
		}
		log.Info("Removed deserted in-container bridge", "bridge", bridgeName)

		return nil
	})
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
			bridgeLink, bridgeErr := netlink.LinkByName(bridgeName)
			if bridgeErr != nil {
				// Unreachable in the current apply order (the replacement
				// bridge deploys before ports restore); a silent skip would
				// mask an un-bridged restored port if the order changes.
				log.Warnf("replacement bridge %q not found; port %q restored without a master",
					bridgeName, r.KernelName)
			} else if link.Attrs().MasterIndex != bridgeLink.Attrs().Index {
				if err := netlink.LinkSetMaster(link, bridgeLink); err != nil {
					return fmt.Errorf("failed to enslave %q to %q: %w",
						link.Attrs().Name, bridgeName, err)
				}
				log.Info("Re-attached migrated bridge port to replacement bridge",
					"port", link.Attrs().Name, "bridge", bridgeName)
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

	// Note: Parking netns cleanup is handled by the caller after all
	// migrations for a given bridge are complete, to avoid race conditions
	// when multiple ports migrate to the same bridge.

	return nil
}

// ParkNSPath returns the path to the parking netns.
func (r *BridgePortRelocation) ParkNSPath() string {
	return r.parkNSPath
}

func (r *BridgePortRelocation) execInParkingNS(f func(ns.NetNS) error) error {
	parkNs, err := ns.GetNS(r.parkNSPath)
	if err != nil {
		return err
	}
	defer parkNs.Close()

	return parkNs.Do(f)
}

// parkedKernelNameFor reports whether an interface carrying the ownership
// altname of (ownerName, ifaceName) is already parked in the parking netns at
// parkPath, returning its kernel-side name so restore can resume. A missing
// netns means "not parked", not an error.
func parkedKernelNameFor(
	parkPath, ownerName, ifaceName string,
) (string, bool, error) {
	parkNs, err := ns.GetNS(parkPath)
	if err != nil {
		// If the netns doesn't exist, the port can't be parked there
		return "", false, nil
	}
	defer parkNs.Close()

	var kernelName string
	execErr := parkNs.Do(func(_ ns.NetNS) error {
		link, err := findLinkByOwnershipAltName(ownerName, ifaceName)
		if err != nil {
			return nil
		}
		kernelName = link.Attrs().Name
		return nil
	})
	if execErr != nil {
		return "", false, execErr
	}
	return kernelName, kernelName != "", nil
}

// UnlinkEmptyParkingNetNS removes the named parking netns only when it holds no
// interfaces besides the loopback device. A netns that still carries a parked
// port is left in place so an interrupted migration stays recoverable through
// the idempotent park path.
func UnlinkEmptyParkingNetNS(parkPath string) error {
	parkNs, err := ns.GetNS(parkPath)
	if err != nil {
		// The mount is already gone; nothing to clean up.
		return nil
	}
	defer parkNs.Close()

	empty := true
	execErr := parkNs.Do(func(_ ns.NetNS) error {
		links, listErr := netlink.LinkList()
		if listErr != nil {
			return listErr
		}
		for _, link := range links {
			if link.Attrs().Name != "lo" {
				empty = false
				return nil
			}
		}
		return nil
	})
	// Keep the netns on inspection errors and on any retained port.
	if execErr != nil || !empty {
		return nil
	}

	return clabutils.UnlinkNamedNetNS(parkPath)
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
