package docker

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/charmbracelet/log"
	cerrdefs "github.com/containerd/errdefs"
	networkapi "github.com/docker/docker/api/types/network"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/srl-labs/containerlab/mgmt"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/srl-labs/containerlab/utils/ipam"
)

func (d *DockerRuntime) managementMacvlanHost() clabutils.MacvlanHost {
	if d.macvlanHost.Links == nil {
		return mgmt.NewMacvlanHost(nil)
	}
	return d.macvlanHost
}

func (d *DockerRuntime) SyncMgmtHostRoutes(ctx context.Context) error {
	if d.mgmt.Driver != clabtypes.MgmtDriverMacvlan || !d.mgmt.MacvlanAuxEnabled() {
		return nil
	}
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()
	network, err := d.Client.NetworkInspect(
		nctx,
		d.mgmt.Network,
		networkapi.InspectOptions{},
	)
	if err != nil {
		return fmt.Errorf("inspect macvlan network %q for host routes: %w", d.mgmt.Network, err)
	}
	return d.syncMacvlanHostRoutes(&network)
}

func (d *DockerRuntime) removeOrphanMacvlanHosts(ctx context.Context) error {
	return d.managementMacvlanHost().RemoveOrphans(func(id string) bool {
		_, err := d.Client.NetworkInspect(ctx, id, networkapi.InspectOptions{})
		return !cerrdefs.IsNotFound(err)
	})
}

func (d *DockerRuntime) syncMacvlanHostRoutes(network *networkapi.Inspect) error {
	sources := make([]netip.Addr, 0, 2)
	for _, label := range []string{clabconstants.MacvlanAuxIPv4, clabconstants.MacvlanAuxIPv6} {
		if value := network.Labels[label]; value != "" {
			address, err := netip.ParseAddr(value)
			if err != nil {
				return fmt.Errorf("invalid macvlan auxiliary address %q: %w", value, err)
			}
			sources = append(sources, address)
		}
	}
	if len(sources) == 0 {
		return nil
	}
	destinations := make([]netip.Addr, 0, len(network.Containers)*2)
	for _, endpoint := range network.Containers {
		for _, value := range []string{endpoint.IPv4Address, endpoint.IPv6Address} {
			if prefix, err := netip.ParsePrefix(value); err == nil &&
				prefix.Addr().IsGlobalUnicast() {
				destinations = append(destinations, prefix.Addr())
			}
		}
	}
	if err := d.managementMacvlanHost().SyncRoutes(network.ID, sources, destinations); err != nil {
		return fmt.Errorf("synchronize macvlan host routes for %q: %w", network.Name, err)
	}
	return nil
}

func (d *DockerRuntime) createMacvlanNetwork(ctx context.Context, excluded []netip.Addr) error {
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	host := d.managementMacvlanHost()
	nres, inspectErr := d.Client.NetworkInspect(nctx, d.mgmt.Network, networkapi.InspectOptions{})
	if inspectErr != nil && !cerrdefs.IsNotFound(inspectErr) {
		return fmt.Errorf("inspect macvlan network %q: %w", d.mgmt.Network, inspectErr)
	}
	if cerrdefs.IsNotFound(inspectErr) {
		if err := d.removeOrphanMacvlanHosts(nctx); err != nil {
			return err
		}
	}
	parent, err := mgmt.PrepareMacvlanParent(d.mgmt, host.Links)
	if err != nil {
		return err
	}
	var auxiliary []netip.Addr
	if inspectErr == nil {
		if d.mgmt.MacvlanAuxEnabled() {
			if _, owned := nres.Labels[clabconstants.Containerlab]; !owned {
				return fmt.Errorf(
					"cannot reuse network %q: macvlan auxiliary connectivity requires a network created by containerlab",
					d.mgmt.Network,
				)
			}
			auxiliary, err = mgmt.RestoreMacvlanAuxAddresses(
				d.mgmt,
				nres.Labels[clabconstants.MacvlanAuxIPv4],
				nres.Labels[clabconstants.MacvlanAuxIPv6],
			)
			if err != nil {
				return fmt.Errorf("cannot reuse network %q: %w", d.mgmt.Network, err)
			}
		}
	} else if d.mgmt.MacvlanAuxEnabled() {
		parentAddresses, err := mgmt.MacvlanParentAddresses(host.Links, parent)
		if err != nil {
			return err
		}
		excluded = append(excluded, parentAddresses...)
		resolve := mgmt.ResolveMacvlanAuxAddresses
		if d.mgmt.IPAM.DADEnabled() {
			resolve = mgmt.ResolveAndProbeMacvlanAuxAddresses
		}
		auxiliary, err = resolve(nctx, d.mgmt, excluded)
		if err != nil {
			return err
		}
	}
	opts, err := macvlanNetworkOptions(d.mgmt, auxiliary)
	if err != nil {
		return err
	}

	if cerrdefs.IsNotFound(inspectErr) {
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

	if err := mgmt.EnsureMacvlanHost(nctx, d.mgmt, host, parent, nres.ID, auxiliary); err != nil {
		return err
	}
	d.mgmt.Bridge = ""
	setMgmtIPAMFromDockerPools(d.mgmt, nres.IPAM.Config, true)

	return nil
}

func macvlanNetworkOptions(
	m *clabtypes.MgmtNet,
	auxiliary []netip.Addr,
) (networkapi.CreateOptions, error) {
	auxiliaryByFamily := map[bool]string{}
	for _, address := range auxiliary {
		if !address.IsValid() || !address.IsGlobalUnicast() || address.Is4In6() ||
			auxiliaryByFamily[address.Is4()] != "" {
			return networkapi.CreateOptions{}, fmt.Errorf(
				"invalid internal macvlan auxiliary address %q",
				address,
			)
		}
		auxiliaryByFamily[address.Is4()] = address.String()
	}
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
		aux := auxiliaryByFamily[true]
		if m.MacvlanAuxEnabled() && aux == "" {
			return networkapi.CreateOptions{}, fmt.Errorf(
				"missing internal macvlan IPv4 auxiliary address",
			)
		}
		pool := networkapi.IPAMConfig{
			Subnet: m.IPv4Subnet, Gateway: m.IPv4Gw, IPRange: m.IPv4Range,
		}
		if m.MacvlanAuxEnabled() {
			pool.AuxAddress = map[string]string{"host": aux}
			opts.Labels[clabconstants.MacvlanAuxIPv4] = aux
		}
		opts.IPAM.Config = append(opts.IPAM.Config, pool)
	}
	if m.IPv6Subnet != "" {
		aux := auxiliaryByFamily[false]
		if m.MacvlanAuxEnabled() && aux == "" {
			return networkapi.CreateOptions{}, fmt.Errorf(
				"missing internal macvlan IPv6 auxiliary address",
			)
		}
		pool := networkapi.IPAMConfig{
			Subnet:  m.IPv6Subnet,
			Gateway: m.IPv6Gw,
			IPRange: m.IPv6Range,
		}
		if m.MacvlanAuxEnabled() {
			pool.AuxAddress = map[string]string{"host": aux}
			opts.Labels[clabconstants.MacvlanAuxIPv6] = aux
		}
		opts.IPAM.Config = append(opts.IPAM.Config, pool)
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
	if len(n.IPAM.Config) != len(want.IPAM.Config) {
		return fmt.Errorf("subnet count differs from the requested network")
	}
	auxEnabled := want.Labels[clabconstants.MacvlanAuxIPv4] != "" ||
		want.Labels[clabconstants.MacvlanAuxIPv6] != ""
	if _, owned := n.Labels[clabconstants.Containerlab]; auxEnabled && !owned {
		return fmt.Errorf(
			"macvlan auxiliary connectivity requires a network created by containerlab",
		)
	}
	for _, pool := range want.IPAM.Config {
		if err := validateMacvlanPool(n.IPAM.Config, pool); err != nil {
			return err
		}
	}
	for _, label := range []string{clabconstants.MacvlanAuxIPv4, clabconstants.MacvlanAuxIPv6} {
		if n.Labels[label] != want.Labels[label] {
			return fmt.Errorf("existing network has a different macvlan auxiliary address")
		}
	}
	return nil
}

func validateMacvlanPool(pools []networkapi.IPAMConfig, want networkapi.IPAMConfig) error {
	for _, pool := range pools {
		if ipam.CanonicalPrefix(pool.Subnet) != ipam.CanonicalPrefix(want.Subnet) {
			continue
		}
		if want.Gateway != "" && ipam.CanonicalIP(pool.Gateway) != ipam.CanonicalIP(want.Gateway) {
			return fmt.Errorf(
				"subnet %s gateway is %q, requested %q",
				want.Subnet,
				pool.Gateway,
				want.Gateway,
			)
		}
		if ipam.CanonicalPrefix(pool.IPRange) != ipam.CanonicalPrefix(want.IPRange) {
			return fmt.Errorf(
				"subnet %s IP range is %q, requested %q",
				want.Subnet,
				pool.IPRange,
				want.IPRange,
			)
		}
		if aux := want.AuxAddress["host"]; aux != "" {
			if ipam.CanonicalIP(pool.AuxAddress["host"]) != aux ||
				ipam.CanonicalIP(pool.Gateway) == aux {
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
