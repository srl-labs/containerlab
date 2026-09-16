package utils

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type macvlanNetlink interface {
	LinkByName(string) (netlink.Link, error)
	LinkAdd(netlink.Link) error
}

// MacvlanHostNetlink provides address, route, and lifecycle operations for host macvlans.
type MacvlanHostNetlink interface {
	macvlanNetlink
	LinkList() ([]netlink.Link, error)
	LinkDel(netlink.Link) error
	LinkSetUp(netlink.Link) error
	LinkSetAlias(netlink.Link, string) error
	AddrList(netlink.Link, int) ([]netlink.Addr, error)
	AddrAdd(netlink.Link, *netlink.Addr) error
	RouteList(netlink.Link, int) ([]netlink.Route, error)
	RouteAdd(*netlink.Route) error
	RouteDel(*netlink.Route) error
}

// MacvlanHost manages an owned auxiliary interface and its host routes.
type MacvlanHost struct {
	Links MacvlanHostNetlink
	// TCPChecksumFill installs or removes software TCP checksum fill on the aux iface.
	// Nil skips the rule (tests).
	TCPChecksumFill func(name string, family int, enable bool) error
}

func addMacvlan(links macvlanNetlink, attrs netlink.LinkAttrs, mode netlink.MacvlanMode) error {
	return links.LinkAdd(&netlink.Macvlan{LinkAttrs: attrs, Mode: mode})
}

func macvlanHostName(networkID string) string {
	// Hashing the network ID produces stable names within Linux's 15-byte limit.
	hash := sha256.Sum256([]byte(networkID))
	return fmt.Sprintf("cm-%x", hash[:6])
}

const macvlanHostAliasPrefix = "containerlab:macvlan:"

func macvlanHostAlias(networkID string) string {
	return macvlanHostAliasPrefix + networkID
}

func macvlanHostMAC(networkID string) net.HardwareAddr {
	hash := sha256.Sum256([]byte(networkID))
	mac := net.HardwareAddr(hash[:6])
	mac[0] = (mac[0] & 0xfe) | 0x02
	return mac
}

func macvlanHostOwned(link netlink.Link, networkID string) bool {
	if _, ok := link.(*netlink.Macvlan); !ok {
		return false
	}
	if link.Attrs().Alias == macvlanHostAlias(networkID) {
		return true
	}
	// Name, type, and MAC are derived from the network ID, so a same-name
	// macvlan is this network's even if LinkSetAlias never ran.
	return bytes.Equal(link.Attrs().HardwareAddr, macvlanHostMAC(networkID))
}

func macvlanHostRequireOwned(link netlink.Link, networkID string) error {
	if macvlanHostOwned(link, networkID) {
		return nil
	}
	return fmt.Errorf(
		"interface %q (type %s, alias %q) is not owned by this containerlab network",
		link.Attrs().Name,
		link.Type(),
		link.Attrs().Alias,
	)
}

func disableMacvlanIPv6Autoconf(name string) error {
	for _, key := range []string{"accept_ra", "autoconf"} {
		path := "/proc/sys/net/ipv6/conf/" + name + "/" + key
		if err := os.WriteFile(path, []byte("0"), 0o600); err != nil &&
			!errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("disable IPv6 %s on %q: %w", key, name, err)
		}
	}
	return nil
}

var macvlanIPv6Sysctl = disableMacvlanIPv6Autoconf

func (h MacvlanHost) Ensure(
	ctx context.Context,
	networkID string,
	parent netlink.Link,
	addresses []netip.Addr,
) error {
	if networkID == "" {
		return fmt.Errorf("macvlan host interface requires a network ID")
	}
	name := macvlanHostName(networkID)
	link, err := h.Links.LinkByName(name)
	if err != nil && !errors.As(err, &netlink.LinkNotFoundError{}) {
		return fmt.Errorf("look up host interface %q: %w", name, err)
	}
	if err != nil {
		err = addMacvlan(h.Links, netlink.LinkAttrs{
			Name: name, ParentIndex: parent.Attrs().Index,
			MTU: parent.Attrs().MTU, HardwareAddr: macvlanHostMAC(networkID),
		}, netlink.MACVLAN_MODE_BRIDGE)
		if err != nil && !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("create host interface %q: %w", name, err)
		}
		// Re-read after creation (including EEXIST from another deploy) to obtain
		// the kernel index and verify ownership before assigning addresses.
		link, err = h.Links.LinkByName(name)
		if err != nil {
			return fmt.Errorf("look up created host interface %q: %w", name, err)
		}
	}
	if err := macvlanHostRequireOwned(link, networkID); err != nil {
		return err
	}
	macvlan := link.(*netlink.Macvlan)
	if macvlan.ParentIndex != parent.Attrs().Index || macvlan.Mode != netlink.MACVLAN_MODE_BRIDGE {
		return fmt.Errorf("host interface %q has a different macvlan parent or mode", name)
	}
	if link.Attrs().Alias != macvlanHostAlias(networkID) {
		// Not all kernels retain IFLA_IFALIAS supplied during macvlan creation.
		if err := h.Links.LinkSetAlias(link, macvlanHostAlias(networkID)); err != nil {
			return fmt.Errorf("mark host interface %q: %w", name, err)
		}
	}

	// Leave an owned, partially configured interface on failure. A retry can
	// complete it; deleting it could disrupt another lab deploying concurrently.
	if err := macvlanIPv6Sysctl(name); err != nil {
		return err
	}
	if err := h.Links.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring host interface %q up: %w", name, err)
	}
	for _, address := range addresses {
		if err := h.ensureAddress(ctx, link, address); err != nil {
			return err
		}
	}
	if h.TCPChecksumFill != nil {
		for _, address := range addresses {
			if err := h.TCPChecksumFill(name, netlinkFamily(address), true); err != nil {
				return fmt.Errorf("install TCP checksum fill on %q: %w", name, err)
			}
		}
	}
	return nil
}

func (h MacvlanHost) ensureAddress(ctx context.Context, link netlink.Link, ip netip.Addr) error {
	family := netlinkFamily(ip)
	addrs, err := h.Links.AddrList(nil, family)
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
	if err := h.Links.AddrAdd(link, want); err != nil {
		if errors.Is(err, unix.EEXIST) {
			// A concurrent deploy may have added it. Inspect once instead of
			// treating any EEXIST (including conflicting addresses) as success.
			addrs, listErr := h.Links.AddrList(link, family)
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

// SyncRoutes reconciles exact host routes for the current network endpoints.
func (h MacvlanHost) SyncRoutes(networkID string, sources, destinations []netip.Addr) error {
	if networkID == "" {
		return errors.New("macvlan network ID is empty")
	}

	link, err := h.Links.LinkByName(macvlanHostName(networkID))
	if err != nil {
		return fmt.Errorf("look up host interface for route synchronization: %w", err)
	}

	if err := macvlanHostRequireOwned(link, networkID); err != nil {
		return err
	}

	var source4, source6 netip.Addr
	for _, source := range sources {
		if source.Is4() {
			source4 = source
		} else {
			source6 = source
		}
	}

	desired := make(map[netip.Addr]netip.Addr, len(destinations))
	for _, destination := range destinations {
		source := source6
		if destination.Is4() {
			source = source4
		}
		if !source.IsValid() {
			return fmt.Errorf("no macvlan auxiliary source for management address %s", destination)
		}
		desired[destination] = source
	}

	var stale []netlink.Route
	tables := map[int][]netlink.Route{}
	for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		routes, err := h.Links.RouteList(nil, family)
		if err != nil {
			return fmt.Errorf("list host routes for synchronization: %w", err)
		}
		kept := make([]netlink.Route, 0, len(routes))
		for i := range routes {
			route := &routes[i]
			if !ownedMacvlanHostRoute(route, link.Attrs().Index) {
				kept = append(kept, *route)
				continue
			}
			destination, ok := netip.AddrFromSlice(route.Dst.IP)
			if !ok {
				kept = append(kept, *route)
				continue
			}
			destination = destination.Unmap()
			source := desired[destination]
			if source.IsValid() && route.Src.Equal(net.IP(source.AsSlice())) {
				delete(desired, destination)
				kept = append(kept, *route)
				continue
			}
			stale = append(stale, *route)
		}
		tables[family] = kept
	}

	for i := range stale {
		if err := h.Links.RouteDel(&stale[i]); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("remove stale macvlan host route %s: %w", stale[i].Dst, err)
		}
	}
	for destination, source := range desired {
		if err := h.ensureRoute(
			link,
			source,
			netip.PrefixFrom(destination, destination.BitLen()),
			tables[netlinkFamily(destination)],
		); err != nil {
			return err
		}
	}
	return nil
}

func ownedMacvlanHostRoute(route *netlink.Route, linkIndex int) bool {
	if route.Dst == nil || route.LinkIndex != linkIndex || route.Protocol != unix.RTPROT_STATIC ||
		route.Table != unix.RT_TABLE_MAIN || route.Type != unix.RTN_UNICAST ||
		len(route.Gw) != 0 || len(route.MultiPath) != 0 {
		return false
	}
	bits, width := route.Dst.Mask.Size()
	return bits == width
}

// IPv6 source addresses cannot be used by routes until kernel DAD completes.
func (h MacvlanHost) waitAddressReady(ctx context.Context, link netlink.Link, ip netip.Addr) error {
	if ip.Is4() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		addrs, err := h.Links.AddrList(link, netlink.FAMILY_V6)
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

func (h MacvlanHost) ensureRoute(
	link netlink.Link,
	ip netip.Addr,
	prefix netip.Prefix,
	routes []netlink.Route,
) error {
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
	found, err := checkRoute(want, routes)
	if err != nil || found {
		return err
	}
	if err := h.Links.RouteAdd(want); err != nil {
		if errors.Is(err, unix.EEXIST) {
			routes, listErr := h.Links.RouteList(nil, netlinkFamily(ip))
			if listErr != nil {
				return fmt.Errorf("list host routes: %w", listErr)
			}
			found, checkErr := checkRoute(want, routes)
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

func checkRoute(want *netlink.Route, routes []netlink.Route) (bool, error) {
	found := false
	wantBits, _ := want.Dst.Mask.Size()
	for _, route := range routes {
		if route.Dst == nil || route.Table != want.Table {
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
			"use a narrower host route prefix or a separate subnet", want.Dst, route.Dst, route.LinkIndex)
	}
	return found, nil
}

func (h MacvlanHost) Remove(networkID string) error {
	if networkID == "" {
		return fmt.Errorf("cannot remove macvlan host interface without a network ID")
	}
	name := macvlanHostName(networkID)
	link, err := h.Links.LinkByName(name)
	if errors.As(err, &netlink.LinkNotFoundError{}) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up host interface %q for cleanup: %w", name, err)
	}
	if err := macvlanHostRequireOwned(link, networkID); err != nil {
		return err
	}
	if h.TCPChecksumFill != nil {
		for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
			if err := h.TCPChecksumFill(name, family, false); err != nil {
				log.Debugf("remove TCP checksum fill on %s: %v", name, err)
			}
		}
	}
	// The kernel removes the interface's addresses and routes with the link.
	if err := h.Links.LinkDel(link); err != nil && !errors.Is(err, unix.ENODEV) {
		return fmt.Errorf("remove host interface %q: %w", name, err)
	}
	return nil
}

func macvlanHostNetworkID(link netlink.Link) (string, bool) {
	if _, ok := link.(*netlink.Macvlan); !ok {
		return "", false
	}
	alias := link.Attrs().Alias
	if !strings.HasPrefix(alias, macvlanHostAliasPrefix) {
		return "", false
	}
	id := alias[len(macvlanHostAliasPrefix):]
	if id == "" || link.Attrs().Name != macvlanHostName(id) {
		return "", false
	}
	return id, true
}

// RemoveOrphans deletes host macvlans whose Docker network no longer exists.
func (h MacvlanHost) RemoveOrphans(alive func(networkID string) bool) error {
	links, err := h.Links.LinkList()
	if err != nil {
		return fmt.Errorf("list host interfaces for macvlan cleanup: %w", err)
	}
	for _, link := range links {
		id, ok := macvlanHostNetworkID(link)
		if !ok || alive(id) {
			continue
		}
		if err := h.Remove(id); err != nil {
			return err
		}
	}
	return nil
}
