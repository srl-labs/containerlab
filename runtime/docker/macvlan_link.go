package docker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// macvlanNetlink is the host networking used by macvlan management networks.
// Keeping it separate from the Docker API also allows lifecycle tests without root.
type macvlanNetlink interface {
	LinkByName(string) (netlink.Link, error)
	LinkAdd(netlink.Link) error
	LinkDel(netlink.Link) error
	LinkSetUp(netlink.Link) error
	LinkSetAlias(netlink.Link, string) error
	AddrList(netlink.Link, int) ([]netlink.Addr, error)
	AddrAdd(netlink.Link, *netlink.Addr) error
	RouteList(netlink.Link, int) ([]netlink.Route, error)
	RouteAdd(*netlink.Route) error
}

type macvlanHost struct {
	links macvlanNetlink
}

func (d *DockerRuntime) macvlanHost() macvlanHost {
	if d.macvlanNetlink != nil {
		return macvlanHost{links: d.macvlanNetlink}
	}
	return macvlanHost{links: &netlink.Handle{}}
}

func macvlanHostName(networkID string) string {
	// Linux interface names are limited to 15 bytes. Hash the complete Docker
	// network ID so names remain stable and independent of topology punctuation.
	hash := sha256.Sum256([]byte(networkID))
	return fmt.Sprintf("cm-%x", hash[:6])
}

func macvlanHostAlias(networkID string) string {
	return "containerlab:macvlan:" + networkID
}

func (h macvlanHost) ensure(
	ctx context.Context,
	networkID string,
	parent netlink.Link,
	ip netip.Addr,
	prefix netip.Prefix,
) error {
	if networkID == "" {
		return fmt.Errorf("Docker returned an empty network ID")
	}
	name := macvlanHostName(networkID)
	link, err := h.links.LinkByName(name)
	if err != nil && !errors.As(err, &netlink.LinkNotFoundError{}) {
		return fmt.Errorf("look up host interface %q: %w", name, err)
	}
	if err != nil {
		link = &netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{
				Name: name, ParentIndex: parent.Attrs().Index,
				MTU: parent.Attrs().MTU,
			},
			Mode: netlink.MACVLAN_MODE_BRIDGE,
		}
		err := h.links.LinkAdd(link)
		if err != nil && !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("create host interface %q: %w", name, err)
		}
		if err == nil {
			// Set the alias explicitly: not all kernels retain IFLA_IFALIAS
			// supplied during macvlan creation. Until this succeeds, other
			// deploys must refuse to adopt the unmarked interface.
			if err := h.links.LinkSetAlias(link, macvlanHostAlias(networkID)); err != nil {
				return errors.Join(
					fmt.Errorf("mark host interface %q: %w", name, err),
					h.links.LinkDel(link),
				)
			}
		}
		// Re-read after creation (including EEXIST from another deploy) to obtain
		// the kernel index and verify ownership before assigning addresses.
		link, err = h.links.LinkByName(name)
		if err != nil {
			return fmt.Errorf("look up created host interface %q: %w", name, err)
		}
	}
	macvlan, ok := link.(*netlink.Macvlan)
	if !ok || link.Attrs().Alias != macvlanHostAlias(networkID) {
		return fmt.Errorf(
			"interface %q (type %s, alias %q) is not owned by this containerlab network",
			name,
			link.Type(),
			link.Attrs().Alias,
		)
	}
	if macvlan.ParentIndex != parent.Attrs().Index || macvlan.Mode != netlink.MACVLAN_MODE_BRIDGE {
		return fmt.Errorf("host interface %q has a different macvlan parent or mode", name)
	}

	// Leave an owned, partially configured interface on failure. A retry can
	// complete it; deleting it could disrupt another lab deploying concurrently.
	if err := h.links.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring host interface %q up: %w", name, err)
	}
	if err := h.ensureAddress(ctx, link, ip); err != nil {
		return err
	}
	return h.ensureRoute(link, ip, prefix)
}

func (h macvlanHost) ensureAddress(ctx context.Context, link netlink.Link, ip netip.Addr) error {
	addrs, err := h.links.AddrList(nil, macvlanFamily(ip))
	if err != nil {
		return fmt.Errorf("list host interface addresses: %w", err)
	}
	want := &netlink.Addr{
		IPNet: &net.IPNet{IP: net.IP(ip.AsSlice()), Mask: net.CIDRMask(ip.BitLen(), ip.BitLen())},
	}
	if ip.Is6() {
		want.Flags = unix.IFA_F_NOPREFIXROUTE
	}
	found := false
	for _, addr := range addrs {
		// IPv6 interfaces acquire a link-local address automatically.
		if ip.Is6() && addr.IP.IsLinkLocalUnicast() {
			continue
		}
		if addr.LinkIndex != link.Attrs().Index {
			if addr.IP.Equal(want.IP) {
				return fmt.Errorf(
					"auxiliary address %s is already assigned to host interface index %d",
					ip,
					addr.LinkIndex,
				)
			}
			continue
		}
		if addr.Equal(*want) {
			found = true
		} else {
			return fmt.Errorf(
				"host interface %q has unexpected address %s",
				link.Attrs().Name,
				addr.String(),
			)
		}
	}
	if found {
		return h.waitAddressReady(ctx, link, ip)
	}
	if err := h.links.AddrAdd(link, want); err != nil {
		if errors.Is(err, unix.EEXIST) {
			// A concurrent deploy may have added it. Inspect once instead of
			// treating any EEXIST (including conflicting addresses) as success.
			addrs, listErr := h.links.AddrList(link, macvlanFamily(ip))
			if listErr != nil {
				return fmt.Errorf("re-inspect host interface addresses: %w", listErr)
			}
			for _, addr := range addrs {
				if addr.Equal(*want) {
					return h.waitAddressReady(ctx, link, ip)
				}
			}
		}
		return fmt.Errorf("assign host address %s: %w", ip, err)
	}
	return h.waitAddressReady(ctx, link, ip)
}

func macvlanFamily(ip netip.Addr) int {
	if ip.Is4() {
		return netlink.FAMILY_V4
	}
	return netlink.FAMILY_V6
}

// IPv6 source addresses cannot be used by routes until duplicate address
// detection completes. Preserve DAD rather than bypassing duplicate detection.
func (h macvlanHost) waitAddressReady(ctx context.Context, link netlink.Link, ip netip.Addr) error {
	if ip.Is4() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		addrs, err := h.links.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			return fmt.Errorf("inspect IPv6 host address: %w", err)
		}
		for _, addr := range addrs {
			if !addr.IP.Equal(net.IP(ip.AsSlice())) {
				continue
			}
			if addr.Flags&unix.IFA_F_DADFAILED != 0 {
				return fmt.Errorf(
					"duplicate address detection failed for macvlan host address %s",
					ip,
				)
			}
			if addr.Flags&unix.IFA_F_TENTATIVE == 0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for IPv6 host address %s: %w", ip, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (h macvlanHost) ensureRoute(link netlink.Link, ip netip.Addr, prefix netip.Prefix) error {
	want := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst: &net.IPNet{
			IP:   net.IP(prefix.Addr().AsSlice()),
			Mask: net.CIDRMask(prefix.Bits(), ip.BitLen()),
		},
		Src:      net.IP(ip.AsSlice()),
		Scope:    netlink.SCOPE_LINK,
		Table:    unix.RT_TABLE_MAIN,
		Protocol: unix.RTPROT_STATIC,
		Type:     unix.RTN_UNICAST,
	}
	if ip.Is6() {
		// Linux represents IPv6 on-link routes with universe scope.
		want.Scope = netlink.SCOPE_UNIVERSE
	}
	found, err := h.checkRoute(want)
	if err != nil || found {
		return err
	}
	if err := h.links.RouteAdd(want); err != nil {
		if errors.Is(err, unix.EEXIST) {
			found, checkErr := h.checkRoute(want)
			if checkErr != nil {
				return checkErr
			}
			if found {
				return nil
			}
		}
		return fmt.Errorf("add host route %s: %w", prefix, err)
	}
	return nil
}

func (h macvlanHost) checkRoute(want *netlink.Route) (bool, error) {
	family := netlink.FAMILY_V4
	if want.Dst.IP.To4() == nil {
		family = netlink.FAMILY_V6
	}
	routes, err := h.links.RouteList(nil, family)
	if err != nil {
		return false, fmt.Errorf("list host routes: %w", err)
	}
	found := false
	wantBits, _ := want.Dst.Mask.Size()
	for _, route := range routes {
		if route.Dst == nil {
			continue
		}
		bits, _ := route.Dst.Mask.Size()
		if bits < wantBits || !want.Dst.Contains(route.Dst.IP) {
			continue
		}
		if route.LinkIndex == want.LinkIndex && bits == wantBits &&
			route.Scope == want.Scope && route.Src.Equal(want.Src) &&
			len(route.Gw) == 0 && len(route.MultiPath) == 0 && route.Type == unix.RTN_UNICAST {
			found = true
			continue
		}
		return false, fmt.Errorf("host route %s conflicts with route %s on interface index %d; "+
			"use a narrower macvlan-aux prefix or a separate subnet", want.Dst, route.Dst, route.LinkIndex)
	}
	return found, nil
}

func (h macvlanHost) remove(networkID string) error {
	if networkID == "" {
		return fmt.Errorf("cannot remove macvlan host interface without a network ID")
	}
	name := macvlanHostName(networkID)
	link, err := h.links.LinkByName(name)
	if errors.As(err, &netlink.LinkNotFoundError{}) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up host interface %q for cleanup: %w", name, err)
	}
	if _, ok := link.(*netlink.Macvlan); !ok || link.Attrs().Alias != macvlanHostAlias(networkID) {
		return fmt.Errorf(
			"refusing to remove interface %q: not owned by this containerlab network",
			name,
		)
	}
	// The kernel removes the interface's addresses and routes with the link.
	if err := h.links.LinkDel(link); err != nil && !errors.Is(err, unix.ENODEV) {
		return fmt.Errorf("remove host interface %q: %w", name, err)
	}
	return nil
}
