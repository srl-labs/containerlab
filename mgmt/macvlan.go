package mgmt

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/charmbracelet/log"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// NewMacvlanHost configures host connectivity with the management DAD policy.
// A nil links argument uses the host's netlink interface.
func NewMacvlanHost(m *clabtypes.MgmtNet, links clabutils.MacvlanNetlink) clabutils.MacvlanHost {
	host := clabutils.MacvlanHost{Links: links}
	if host.Links == nil {
		host.Links = &netlink.Handle{}
	}
	if m.IPAM.Provider != clabtypes.IPAMProviderRuntime && m.IPAM.DADEnabled() &&
		m.Driver == "macvlan" && m.EffectiveMacvlanMode() == "bridge" {
		host.Probe = ProbeAddress
	}
	return host
}

// PrepareMacvlanParent looks up the parent and resolves management subnets
// without modifying host interfaces or routes.
func PrepareMacvlanParent(m *clabtypes.MgmtNet, links clabutils.MacvlanNetlink) (netlink.Link, error) {
	parent, err := links.LinkByName(m.MacvlanParent)
	if err != nil {
		return nil, fmt.Errorf("macvlan parent %q: %w", m.MacvlanParent, err)
	}
	if parent.Attrs().Flags&net.FlagUp == 0 {
		log.Warn("Macvlan parent interface is down", "interface", m.MacvlanParent)
	}
	if err := ResolveMacvlanSubnets(m, links, parent); err != nil {
		return nil, err
	}
	if err := validateMacvlanAuxParentRoute(m, links, parent); err != nil {
		return nil, err
	}
	return parent, nil
}

func validateMacvlanAuxParentRoute(m *clabtypes.MgmtNet, links clabutils.MacvlanNetlink, parent netlink.Link) error {
	if m.MacvlanAux == "" {
		return nil
	}
	ip, route, err := m.MacvlanHostAddress()
	if err != nil {
		return err
	}
	routes, err := links.RouteList(nil, macvlanFamily(ip))
	if err != nil {
		return fmt.Errorf("list macvlan parent routes: %w", err)
	}
	for _, existing := range routes {
		if existing.Dst == nil || existing.LinkIndex != parent.Attrs().Index {
			continue
		}
		prefix, err := netip.ParsePrefix(existing.Dst.String())
		if err == nil && prefix.Bits() >= route.Bits() && route.Contains(prefix.Addr()) {
			return fmt.Errorf(
				"mgmt.macvlan-aux route %s conflicts with parent interface %q route %s; use a narrower prefix",
				route, parent.Attrs().Name, prefix,
			)
		}
	}
	return nil
}

func macvlanFamily(ip netip.Addr) int {
	if ip.Is4() {
		return netlink.FAMILY_V4
	}
	return netlink.FAMILY_V6
}

// EnsureMacvlanHost configures auxiliary connectivity after network validation.
// It is a no-op when no auxiliary address is configured.
func EnsureMacvlanHost(ctx context.Context, m *clabtypes.MgmtNet, host clabutils.MacvlanHost, parent netlink.Link, networkID string) error {
	if m.MacvlanAux == "" {
		return nil
	}
	ip, route, err := m.MacvlanHostAddress()
	if err != nil {
		return err
	}
	if err := host.Ensure(ctx, networkID, parent, ip, route); err != nil {
		return fmt.Errorf("configure macvlan host connectivity for %q: %w", m.Network, err)
	}
	return nil
}

// ResolveMacvlanSubnets infers omitted subnets from the parent and validates
// subnet-dependent settings before updating the management configuration.
func ResolveMacvlanSubnets(m *clabtypes.MgmtNet, links clabutils.MacvlanNetlink, parent netlink.Link) error {
	resolved := *m
	explicitIPv4, explicitIPv6 := m.IPv4Subnet != "", m.IPv6Subnet != ""
	aux, _ := netip.ParseAddr(m.MacvlanAux)
	if prefix, err := netip.ParsePrefix(m.MacvlanAux); err == nil {
		aux = prefix.Addr()
	}
	for _, family := range []struct {
		name          string
		id            int
		subnet        *string
		gateway, pool string
		otherExplicit bool
		requested     bool
	}{
		{"IPv4", netlink.FAMILY_V4, &resolved.IPv4Subnet, resolved.IPv4Gw, resolved.IPv4Range,
			explicitIPv6, resolved.IPv4Gw != "" || resolved.IPv4Range != "" || aux.Is4()},
		{"IPv6", netlink.FAMILY_V6, &resolved.IPv6Subnet, resolved.IPv6Gw, resolved.IPv6Range,
			explicitIPv4, resolved.IPv6Gw != "" || resolved.IPv6Range != "" || aux.Is6()},
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
			return fmt.Errorf("macvlan parent %q has no usable %s subnet for the configured gateway or range", parent.Attrs().Name, family.name)
		}
	}
	if resolved.IPv4Subnet == "" && resolved.IPv6Subnet == "" {
		return fmt.Errorf("macvlan parent %q has no usable IP subnet; configure mgmt.ipv4-subnet or mgmt.ipv6-subnet explicitly", parent.Attrs().Name)
	}
	if err := resolved.Validate(); err != nil {
		return err
	}
	if resolved.MacvlanAux != "" {
		if _, _, err := resolved.MacvlanHostAddress(); err != nil {
			return err
		}
	}
	m.IPv4Subnet, m.IPv6Subnet = resolved.IPv4Subnet, resolved.IPv6Subnet
	return nil
}

// macvlanParentSubnet returns the parent's single usable subnet for a family.
// No global-unicast address returns an invalid prefix without an error.
func macvlanParentSubnet(links clabutils.MacvlanNetlink, parent netlink.Link, family int) (netip.Prefix, error) {
	addresses, err := links.AddrList(parent, family)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("read macvlan parent %q addresses: %w", parent.Attrs().Name, err)
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
			return netip.Prefix{}, fmt.Errorf("macvlan parent %q address %s cannot supply a management subnet; the prefix must contain at least four addresses", parent.Attrs().Name, address.String())
		}
		candidate := netip.PrefixFrom(ip, bits).Masked()
		if subnet.IsValid() && subnet != candidate {
			return netip.Prefix{}, fmt.Errorf("macvlan parent %q has multiple subnets in the same address family; configure the corresponding mgmt subnet explicitly", parent.Attrs().Name)
		}
		subnet = candidate
	}
	return subnet, nil
}
