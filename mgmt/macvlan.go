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
	return parent, nil
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
	for _, family := range []struct {
		name          string
		id            int
		subnet        *string
		gateway, pool string
		otherExplicit bool
	}{
		{"IPv4", netlink.FAMILY_V4, &resolved.IPv4Subnet, resolved.IPv4Gw, resolved.IPv4Range, explicitIPv6},
		{"IPv6", netlink.FAMILY_V6, &resolved.IPv6Subnet, resolved.IPv6Gw, resolved.IPv6Range, explicitIPv4},
	} {
		if *family.subnet != "" {
			continue
		}
		subnet, err := MacvlanParentSubnet(links, parent, family.id)
		if err != nil {
			if family.otherExplicit && family.gateway == "" && family.pool == "" {
				continue
			}
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

// MacvlanParentSubnet returns the parent's single usable subnet for a family.
// No global-unicast address returns an invalid prefix without an error.
func MacvlanParentSubnet(links clabutils.MacvlanNetlink, parent netlink.Link, family int) (netip.Prefix, error) {
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
