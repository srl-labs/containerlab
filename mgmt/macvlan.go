package mgmt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/charmbracelet/log"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/srl-labs/containerlab/utils/ipam"
	"github.com/vishvananda/netlink"
)

// NewMacvlanHost configures host connectivity using the supplied netlink backend.
// A nil backend uses the host's netlink interface.
func NewMacvlanHost(
	links clabutils.MacvlanHostNetlink,
) clabutils.MacvlanHost {
	host := clabutils.MacvlanHost{Links: links}
	if host.Links == nil {
		host.Links = &netlink.Handle{}
	}
	return host
}

// PrepareMacvlanParent looks up the parent and resolves management subnets
// without modifying host interfaces or routes.
func PrepareMacvlanParent(
	m *clabtypes.MgmtNet,
	links clabutils.MacvlanHostNetlink,
) (netlink.Link, error) {
	parent, err := links.LinkByName(m.MacvlanParent)
	if err != nil {
		return nil, fmt.Errorf("macvlan parent %q: %w", m.MacvlanParent, err)
	}
	if parent.Attrs().Flags&net.FlagUp == 0 {
		log.Warn("Macvlan parent interface is down", "interface", m.MacvlanParent)
	}
	if err := resolveMacvlanSubnets(m, links, parent); err != nil {
		return nil, err
	}
	return parent, nil
}

// MacvlanParentAddresses returns globally routable addresses assigned to the parent.
func MacvlanParentAddresses(
	links clabutils.MacvlanHostNetlink,
	parent netlink.Link,
) ([]netip.Addr, error) {
	var result []netip.Addr
	for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		addresses, err := links.AddrList(parent, family)
		if err != nil {
			return nil, fmt.Errorf("read macvlan parent %q addresses: %w", parent.Attrs().Name, err)
		}
		for _, address := range addresses {
			if ip, ok := netip.AddrFromSlice(address.IP); ok && ip.Unmap().IsGlobalUnicast() {
				result = append(result, ip.Unmap())
			}
		}
	}
	return result, nil
}

// EnsureMacvlanHost configures the host macvlan and its auxiliary addresses.
func EnsureMacvlanHost(
	ctx context.Context,
	m *clabtypes.MgmtNet,
	host clabutils.MacvlanHost,
	parent netlink.Link,
	networkID string,
	addresses []netip.Addr,
) error {
	if !m.MacvlanAuxEnabled() {
		return nil
	}
	if err := host.Ensure(ctx, networkID, parent, addresses); err != nil {
		return fmt.Errorf("configure macvlan host connectivity for %q: %w", m.Network, err)
	}
	return nil
}

// resolveMacvlanSubnets infers omitted subnets from the parent and validates
// subnet-dependent settings before updating the management configuration.
func resolveMacvlanSubnets(
	m *clabtypes.MgmtNet,
	links clabutils.MacvlanHostNetlink,
	parent netlink.Link,
) error {
	resolved := *m
	explicitIPv4, explicitIPv6 := m.IPv4Subnet != "", m.IPv6Subnet != ""
	for _, family := range []struct {
		name          string
		id            int
		subnet        *string
		gateway, pool string
		otherExplicit bool
		requested     bool
	}{
		{
			"IPv4", netlink.FAMILY_V4, &resolved.IPv4Subnet, resolved.IPv4Gw, resolved.IPv4Range,
			explicitIPv6, resolved.IPv4Gw != "" || resolved.IPv4Range != "",
		},
		{
			"IPv6", netlink.FAMILY_V6, &resolved.IPv6Subnet, resolved.IPv6Gw, resolved.IPv6Range,
			explicitIPv4, resolved.IPv6Gw != "" || resolved.IPv6Range != "",
		},
	} {
		if *family.subnet != "" {
			continue
		}
		if family.otherExplicit && !family.requested {
			continue
		}
		subnet, err := macvlanParentSubnet(links, parent, family.id)
		if err != nil {
			return err
		}
		if subnet.IsValid() {
			*family.subnet = subnet.String()
		} else if family.gateway != "" || family.pool != "" {
			return fmt.Errorf(
				"macvlan parent %q has no usable %s subnet for the configured gateway or range",
				parent.Attrs().Name,
				family.name,
			)
		}
	}
	if resolved.IPv4Subnet == "" && resolved.IPv6Subnet == "" {
		return fmt.Errorf(
			"macvlan parent %q has no usable IP subnet; configure mgmt.ipv4-subnet or mgmt.ipv6-subnet explicitly",
			parent.Attrs().Name,
		)
	}
	if err := resolved.Validate(); err != nil {
		return err
	}
	m.IPv4Subnet, m.IPv6Subnet = resolved.IPv4Subnet, resolved.IPv6Subnet
	return nil
}

// ResolveMacvlanAuxAddresses selects one reserved host address per configured family.
func ResolveMacvlanAuxAddresses(
	ctx context.Context,
	m *clabtypes.MgmtNet,
	excluded []netip.Addr,
) ([]netip.Addr, error) {
	return resolveMacvlanAuxAddresses(ctx, m, excluded, nil)
}

func resolveMacvlanAuxAddresses(
	ctx context.Context,
	m *clabtypes.MgmtNet,
	excluded []netip.Addr,
	dad *ipam.DADClient,
) ([]netip.Addr, error) {
	resolved := make([]netip.Addr, 0, 2)
	for _, family := range []struct {
		name, subnet, gateway string
	}{
		{"IPv4", m.IPv4Subnet, m.IPv4Gw},
		{"IPv6", m.IPv6Subnet, m.IPv6Gw},
	} {
		if family.subnet == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(family.subnet)
		if err != nil {
			return nil, fmt.Errorf("resolve macvlan %s auxiliary address: %w", family.name, err)
		}
		gateway := prefix.Masked().Addr().Next()
		if family.gateway != "" {
			gateway, err = netip.ParseAddr(family.gateway)
			if err != nil {
				return nil, fmt.Errorf("resolve macvlan %s auxiliary gateway: %w", family.name, err)
			}
		}
		reserved := []netip.Addr{gateway}
		for _, address := range excluded {
			if prefix.Contains(address) {
				reserved = append(reserved, address)
			}
		}
		address, err := resolveMacvlanAuxAddress(ctx, prefix, reserved, dad)
		if err != nil {
			return nil, fmt.Errorf("resolve macvlan %s auxiliary address: %w", family.name, err)
		}
		resolved = append(resolved, address)
	}
	return resolved, nil
}

func resolveMacvlanAuxAddress(
	ctx context.Context,
	prefix netip.Prefix,
	reserved []netip.Addr,
	dad *ipam.DADClient,
) (netip.Addr, error) {
	allocator, err := ipam.NewIPAllocator(prefix, prefix, reserved)
	if err != nil {
		return netip.Addr{}, err
	}
	if dad == nil {
		return allocator.Next(ctx)
	}
	return allocator.Next(ctx, dad)
}

// ResolveAndProbeMacvlanAuxAddresses selects addresses after checking the parent segment.
func ResolveAndProbeMacvlanAuxAddresses(
	ctx context.Context,
	m *clabtypes.MgmtNet,
	excluded []netip.Addr,
) ([]netip.Addr, error) {
	dad, err := ipam.NewDADClient(m.MacvlanParent)
	if err != nil {
		return nil, err
	}
	addresses, err := resolveMacvlanAuxAddresses(ctx, m, excluded, dad)
	return addresses, errors.Join(err, dad.Close())
}

// RestoreMacvlanAuxAddresses validates persisted addresses.
func RestoreMacvlanAuxAddresses(m *clabtypes.MgmtNet, ipv4, ipv6 string) ([]netip.Addr, error) {
	addresses := make([]netip.Addr, 0, 2)
	for _, family := range []struct {
		name, subnet, gateway, address string
		ipv4                           bool
	}{
		{"IPv4", m.IPv4Subnet, m.IPv4Gw, ipv4, true},
		{"IPv6", m.IPv6Subnet, m.IPv6Gw, ipv6, false},
	} {
		if family.subnet == "" {
			if family.address != "" {
				return nil, fmt.Errorf(
					"existing network has an unexpected macvlan %s auxiliary address",
					family.name,
				)
			}
			continue
		}
		if family.address == "" {
			return nil, fmt.Errorf(
				"existing network lacks a reserved macvlan %s auxiliary address",
				family.name,
			)
		}
		subnet, err := netip.ParsePrefix(family.subnet)
		if err != nil {
			return nil, err
		}
		address, err := netip.ParseAddr(family.address)
		if err != nil || address.Is4() != family.ipv4 || !address.IsGlobalUnicast() ||
			address.Is4In6() || !subnet.Contains(address) {
			return nil, fmt.Errorf(
				"invalid persisted macvlan %s auxiliary address %q",
				family.name,
				family.address,
			)
		}
		gateway := subnet.Masked().Addr().Next()
		if family.gateway != "" {
			gateway, err = netip.ParseAddr(family.gateway)
			if err != nil {
				return nil, err
			}
		}
		if address == gateway || address == subnet.Masked().Addr() ||
			(address.Is4() && !subnet.Contains(address.Next())) {
			return nil, fmt.Errorf(
				"persisted macvlan %s auxiliary address %s is reserved",
				family.name,
				address,
			)
		}
		addresses = append(addresses, address)
	}
	return addresses, nil
}

// macvlanParentSubnet returns the parent's single usable subnet for a family.
// No global-unicast address returns an invalid prefix without an error.
func macvlanParentSubnet(
	links clabutils.MacvlanHostNetlink,
	parent netlink.Link,
	family int,
) (netip.Prefix, error) {
	addresses, err := links.AddrList(parent, family)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf(
			"read macvlan parent %q addresses: %w",
			parent.Attrs().Name,
			err,
		)
	}
	var subnet netip.Prefix
	for _, address := range addresses {
		ip, ok := netip.AddrFromSlice(address.IP)
		if !ok || !ip.Unmap().IsGlobalUnicast() {
			continue
		}
		ip = ip.Unmap()
		if ip.Is4() != (family == netlink.FAMILY_V4) {
			continue
		}
		bits, width := address.Mask.Size()
		if width != ip.BitLen() || bits > ip.BitLen()-2 {
			return netip.Prefix{}, fmt.Errorf(
				"macvlan parent %q address %s cannot supply a management subnet; the prefix must contain at least four addresses",
				parent.Attrs().Name,
				address.String(),
			)
		}
		candidate := netip.PrefixFrom(ip, bits).Masked()
		if subnet.IsValid() && subnet != candidate {
			return netip.Prefix{}, fmt.Errorf(
				"macvlan parent %q has multiple subnets in the same address family; configure the corresponding mgmt subnet explicitly",
				parent.Attrs().Name,
			)
		}
		subnet = candidate
	}
	return subnet, nil
}
