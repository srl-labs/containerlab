package docker

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/charmbracelet/log"
	cerrdefs "github.com/containerd/errdefs"
	networkapi "github.com/docker/docker/api/types/network"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
)

const macvlanAuxLabel = "containerlab-macvlan-aux"

func (d *DockerRuntime) createMacvlanNetwork(ctx context.Context) error {
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	opts, err := macvlanNetworkOptions(d.mgmt)
	if err != nil {
		return err
	}
	host := d.macvlanHost()
	parent, err := host.links.LinkByName(d.mgmt.MacvlanParent)
	if err != nil {
		return fmt.Errorf("macvlan parent %q: %w", d.mgmt.MacvlanParent, err)
	}
	if parent.Attrs().Flags&net.FlagUp == 0 {
		log.Warn("Macvlan parent interface is down", "interface", d.mgmt.MacvlanParent)
	}

	nres, err := d.Client.NetworkInspect(nctx, d.mgmt.Network, networkapi.InspectOptions{})
	if cerrdefs.IsNotFound(err) {
		log.Info("Creating docker network", "name", d.mgmt.Network, "driver", "macvlan",
			"parent", d.mgmt.MacvlanParent)
		_, err = d.Client.NetworkCreate(nctx, d.mgmt.Network, opts)
		if err != nil && !cerrdefs.IsConflict(err) {
			return fmt.Errorf("create macvlan network %q: %w", d.mgmt.Network, err)
		}
		// Both a successful create and a concurrent create must be inspected and
		// validated before any host configuration is changed.
		nres, err = d.Client.NetworkInspect(nctx, d.mgmt.Network, networkapi.InspectOptions{})
	}
	if err != nil {
		return fmt.Errorf("inspect macvlan network %q: %w", d.mgmt.Network, err)
	}
	if err := validateMacvlanNetwork(&nres, opts); err != nil {
		return fmt.Errorf("cannot reuse network %q: %w", d.mgmt.Network, err)
	}

	if d.mgmt.MacvlanAux != "" {
		ip, route, err := d.mgmt.MacvlanHostAddress()
		if err != nil {
			return err
		}
		if err := host.ensure(nctx, nres.ID, parent, ip, route); err != nil {
			return fmt.Errorf("configure macvlan host connectivity for %q: %w", d.mgmt.Network, err)
		}
	}
	d.mgmt.Bridge = ""
	for _, pool := range nres.IPAM.Config {
		gateway, err := netip.ParseAddr(pool.Gateway)
		if err != nil {
			continue
		}
		if gateway.Is4() {
			d.mgmt.IPv4Gw = gateway.String()
		} else {
			d.mgmt.IPv6Gw = gateway.String()
		}
	}
	return nil
}

func macvlanNetworkOptions(m *clabtypes.MgmtNet) (networkapi.CreateOptions, error) {
	opts := networkapi.CreateOptions{
		Driver:     "macvlan",
		EnableIPv6: new(m.IPv6Subnet != ""),
		IPAM:       &networkapi.IPAM{Driver: "default"},
		Labels:     map[string]string{clabconstants.Containerlab: ""},
		Options: map[string]string{
			"parent": m.MacvlanParent, "macvlan_mode": m.EffectiveMacvlanMode(),
		},
	}
	for key, value := range m.DriverOpts {
		opts.Options[key] = value
	}
	if m.IPv4Subnet != "" {
		pool := networkapi.IPAMConfig{
			Subnet: m.IPv4Subnet, Gateway: m.IPv4Gw, IPRange: m.IPv4Range,
		}

		opts.IPAM.Config = append(opts.IPAM.Config, pool)
	}
	if m.IPv6Subnet != "" {
		opts.IPAM.Config = append(opts.IPAM.Config, networkapi.IPAMConfig{
			Subnet: m.IPv6Subnet, Gateway: m.IPv6Gw, IPRange: m.IPv6Range,
		})
	}
	if m.MacvlanAux != "" {
		ip, route, err := m.MacvlanHostAddress()
		if err != nil {
			return networkapi.CreateOptions{}, err
		}
		for i := range opts.IPAM.Config {
			pool := &opts.IPAM.Config[i]
			prefix, err := netip.ParsePrefix(pool.Subnet)
			if err == nil && prefix.Contains(ip) {
				pool.AuxAddress = map[string]string{"host": ip.String()}
			}
		}
		// Include the route prefix so sharing labs agree on host connectivity.
		opts.Labels[macvlanAuxLabel] = netip.PrefixFrom(ip, route.Bits()).String()
	}
	return opts, nil
}

func validateMacvlanNetwork(n *networkapi.Inspect, want networkapi.CreateOptions) error {
	if n.Driver != "macvlan" {
		return fmt.Errorf("driver is %q, requested macvlan", n.Driver)
	}
	for key, value := range want.Options {
		actual := n.Options[key]
		if key == "macvlan_mode" && actual == "" {
			actual = "bridge"
		}
		if actual != value {
			return fmt.Errorf("driver option %q is %q, requested %q", key, actual, value)
		}
	}
	for _, pool := range want.IPAM.Config {
		if err := validateMacvlanPool(n.IPAM.Config, pool); err != nil {
			return err
		}
	}
	if aux := want.Labels[macvlanAuxLabel]; aux != "" {
		if _, owned := n.Labels[clabconstants.Containerlab]; !owned {
			return fmt.Errorf("macvlan-aux requires a network created by containerlab")
		}
		if n.Labels[macvlanAuxLabel] != aux {
			return fmt.Errorf(
				"macvlan-aux differs from the network's host configuration; use the same address and prefix",
			)
		}
	}
	return nil
}

func validateMacvlanPool(pools []networkapi.IPAMConfig, want networkapi.IPAMConfig) error {
	for _, pool := range pools {
		if canonicalPrefix(pool.Subnet) != canonicalPrefix(want.Subnet) {
			continue
		}
		if want.Gateway != "" && canonicalIP(pool.Gateway) != canonicalIP(want.Gateway) {
			return fmt.Errorf(
				"subnet %s gateway is %q, requested %q",
				want.Subnet,
				pool.Gateway,
				want.Gateway,
			)
		}
		if canonicalPrefix(pool.IPRange) != canonicalPrefix(want.IPRange) {
			return fmt.Errorf(
				"subnet %s IP range is %q, requested %q",
				want.Subnet,
				pool.IPRange,
				want.IPRange,
			)
		}
		if aux := want.AuxAddress["host"]; aux != "" {
			if canonicalIP(pool.AuxAddress["host"]) != aux || canonicalIP(pool.Gateway) == aux {
				return fmt.Errorf(
					"subnet %s does not reserve host address %s as requested",
					want.Subnet,
					aux,
				)
			}
		}
		return nil
	}
	return fmt.Errorf("subnet %s is missing from the existing network", want.Subnet)
}

func canonicalPrefix(value string) string {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String()
	}
	return value
}

func canonicalIP(value string) string {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr().String()
	}
	if ip, err := netip.ParseAddr(value); err == nil {
		return ip.String()
	}
	return value
}
