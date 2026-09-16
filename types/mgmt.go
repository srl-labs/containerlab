package types

import (
	"fmt"
	"net/netip"
	"strings"
)

// Validate checks management driver options without accessing the runtime or host.
func (m *MgmtNet) Validate() error {
	if !m.IPAM.Provider.IsValid() {
		return fmt.Errorf("unsupported mgmt.ipam.provider %q", m.IPAM.Provider)
	}
	if !m.Driver.IsValid() {
		return fmt.Errorf("unsupported management network driver %q", m.Driver)
	}
	switch m.Driver {
	case "", MgmtDriverBridge:
		if m.MacvlanParent != "" || m.MacvlanMode != "" || m.MacvlanAux != nil {
			return fmt.Errorf("macvlan options require mgmt.driver: macvlan")
		}
		return nil
	case MgmtDriverMacvlan:
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
	switch m.EffectiveMacvlanMode() {
	case MacvlanModeBridge, MacvlanModePrivate, MacvlanModeVEPA, MacvlanModePassthru:
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
	if m.IPv4Subnet != "" {
		if err := validateMacvlanSubnet(m.IPv4Subnet, m.IPv4Gw, m.IPv4Range, true); err != nil {
			return fmt.Errorf("macvlan IPv4 configuration: %w", err)
		}
	}
	if m.IPv6Subnet != "" {
		if err := validateMacvlanSubnet(m.IPv6Subnet, m.IPv6Gw, m.IPv6Range, false); err != nil {
			return fmt.Errorf("macvlan IPv6 configuration: %w", err)
		}
	}
	return nil
}

// MacvlanAuxEnabled reports whether auxiliary host connectivity is enabled.
func (m *MgmtNet) MacvlanAuxEnabled() bool {
	return m.EffectiveMacvlanMode() == MacvlanModeBridge && (m.MacvlanAux == nil || *m.MacvlanAux)
}

// EffectiveMacvlanMode returns Docker's default mode when no mode was specified.
func (m *MgmtNet) EffectiveMacvlanMode() string {
	if m.MacvlanMode == "" {
		return MacvlanModeBridge
	}
	return m.MacvlanMode
}

func validateMacvlanSubnet(subnet, gateway, ipRange string, ipv4 bool) error {
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
