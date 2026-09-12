package types

import (
	"fmt"
	"net/netip"
	"strings"
)

// Validate checks management driver options without accessing the runtime or host.
func (m *MgmtNet) Validate() error {
	switch m.Driver {
	case "", "bridge":
		if m.MacvlanParent != "" || m.MacvlanMode != "" || m.MacvlanAux != "" {
			return fmt.Errorf("macvlan options require mgmt.driver: macvlan")
		}
		return nil
	case "macvlan":
	default:
		return fmt.Errorf("unsupported management network driver %q", m.Driver)
	}

	if m.MacvlanParent == "" {
		return fmt.Errorf("mgmt.macvlan-parent is required for macvlan networks")
	}
	if len(m.MacvlanParent) > 15 || strings.ContainsAny(m.MacvlanParent, "/:\x00 \t\r\n") {
		return fmt.Errorf("invalid mgmt.macvlan-parent interface name %q", m.MacvlanParent)
	}
	if m.Bridge != "" || m.MTU != 0 {
		return fmt.Errorf(
			"macvlan networks do not support mgmt.bridge or mgmt.mtu; MTU is inherited from the parent",
		)
	}
	if m.ExternalAccess != nil && !*m.ExternalAccess {
		return fmt.Errorf("mgmt.external-access: false is not supported for macvlan networks")
	}
	switch m.MacvlanMode {
	case "", "bridge", "private", "vepa", "passthru":
	default:
		return fmt.Errorf("unsupported mgmt.macvlan-mode %q", m.MacvlanMode)
	}
	for key, value := range map[string]string{
		"parent": m.MacvlanParent, "macvlan_mode": m.EffectiveMacvlanMode(),
	} {
		if option, ok := m.DriverOpts[key]; ok && option != value {
			return fmt.Errorf("mgmt.driver-opts.%s conflicts with the macvlan configuration", key)
		}
	}
	if m.IPv4Subnet == "" && m.IPv6Subnet == "" {
		return fmt.Errorf("macvlan networks require an explicit ipv4-subnet or ipv6-subnet")
	}
	if err := validateMacvlanSubnet(m.IPv4Subnet, m.IPv4Gw, m.IPv4Range, true); err != nil {
		return fmt.Errorf("macvlan IPv4 configuration: %w", err)
	}
	if err := validateMacvlanSubnet(m.IPv6Subnet, m.IPv6Gw, m.IPv6Range, false); err != nil {
		return fmt.Errorf("macvlan IPv6 configuration: %w", err)
	}
	if m.MacvlanAux != "" {
		if m.EffectiveMacvlanMode() != "bridge" {
			return fmt.Errorf(
				"mgmt.macvlan-aux requires macvlan-mode: bridge for host connectivity",
			)
		}
		_, _, err := m.MacvlanHostAddress()
		return err
	}
	return nil
}

// EffectiveMacvlanMode returns Docker's default mode when no mode was specified.
func (m *MgmtNet) EffectiveMacvlanMode() string {
	if m.MacvlanMode == "" {
		return "bridge"
	}
	return m.MacvlanMode
}

func validateMacvlanSubnet(subnet, gateway, ipRange string, ipv4 bool) error {
	if subnet == "" && gateway == "" && ipRange == "" {
		return nil
	}
	prefix, err := netip.ParsePrefix(subnet)
	if err != nil || prefix.Addr().Is4() != ipv4 || prefix.Addr().Is4In6() {
		return fmt.Errorf(
			"invalid subnet %q; an explicit subnet of the correct address family is required",
			subnet,
		)
	}
	if gateway != "" {
		ip, err := netip.ParseAddr(gateway)
		if err != nil || !prefix.Contains(ip) || ip.Is4() != ipv4 || !ip.IsGlobalUnicast() {
			return fmt.Errorf("gateway %q must be a unicast address within %s", gateway, subnet)
		}
		if ipv4 && (ip == prefix.Masked().Addr() || !prefix.Contains(ip.Next())) {
			return fmt.Errorf("gateway %q is a subnet boundary address", gateway)
		}
	}
	if ipRange != "" {
		pool, err := netip.ParsePrefix(ipRange)
		if err != nil || pool.Addr().Is4() != ipv4 ||
			pool.Bits() < prefix.Bits() || !prefix.Contains(pool.Masked().Addr()) {
			return fmt.Errorf("IP range %q must be contained in %s", ipRange, subnet)
		}
	}
	return nil
}

// MacvlanHostAddress returns the reserved IPv4 address and the destination for
// the host route. The interface itself uses /32 to avoid an implicit subnet route.
func (m *MgmtNet) MacvlanHostAddress() (netip.Addr, netip.Prefix, error) {
	subnet, err := netip.ParsePrefix(m.IPv4Subnet)
	if err != nil || !subnet.Addr().Is4() {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf(
			"mgmt.macvlan-aux requires an explicit IPv4 subnet",
		)
	}
	route := subnet.Masked()
	var ip netip.Addr
	if strings.Contains(m.MacvlanAux, "/") {
		var aux netip.Prefix
		aux, err = netip.ParsePrefix(m.MacvlanAux)
		ip, route = aux.Addr(), aux.Masked()
	} else {
		ip, err = netip.ParseAddr(m.MacvlanAux)
	}
	if err != nil || !ip.Is4() || !ip.IsGlobalUnicast() || !subnet.Contains(ip) {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf(
			"mgmt.macvlan-aux %q must be an IPv4 address within %s",
			m.MacvlanAux,
			subnet,
		)
	}
	if route.Bits() < subnet.Bits() {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf(
			"mgmt.macvlan-aux route %s must be contained in %s",
			route,
			subnet,
		)
	}
	if route.Bits() == 32 {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf(
			"mgmt.macvlan-aux /32 would route only the host address, not any containers",
		)
	}
	// Docker reserves the subnet's first usable address as the default gateway.
	gateway := subnet.Masked().Addr().Next()
	if m.IPv4Gw != "" {
		gateway, err = netip.ParseAddr(m.IPv4Gw)
		if err != nil {
			return netip.Addr{}, netip.Prefix{}, fmt.Errorf("invalid IPv4 gateway: %w", err)
		}
	}
	if ip == gateway || ip == subnet.Masked().Addr() || !subnet.Contains(ip.Next()) {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf(
			"mgmt.macvlan-aux %s conflicts with the gateway or subnet boundary",
			ip,
		)
	}
	return ip, route, nil
}
