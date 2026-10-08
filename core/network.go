package core

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/mgmt"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func (c *CLab) CreateNetwork(ctx context.Context) error {
	defined := c.allMgmtNetworks()
	networks := make(clabtypes.MgmtNetworks, 0, len(defined))
	for _, m := range defined {
		if c.skipMgmtNetwork(m) {
			log.Debug("Skipping management network, no node uses it", "network", m.Network)
			continue
		}
		networks = append(networks, m)
	}
	return c.createNetworks(ctx, networks)
}

// createNetworks creates or inspects the given management networks
// and labels the nodes with their management bridge.
func (c *CLab) createNetworks(ctx context.Context, networks clabtypes.MgmtNetworks) error {
	for _, m := range networks {
		var opts []clabruntime.NetworkCreateOptions
		if addresses := c.staticManagementAddresses(m); len(addresses) != 0 {
			opts = append(opts, clabruntime.NetworkCreateOptions{StaticAddresses: addresses})
		}

		rt, err := c.mgmtRuntime(c.globalRuntimeName, m)
		if err != nil {
			return err
		}
		// create docker network or use existing one
		if err := rt.CreateNet(ctx, opts...); err != nil {
			return err
		}

		// VM kinds read the subnets at init, before the runtime resolved them
		for _, cfg := range c.mgmtNetNodes(m) {
			for key, subnet := range map[string]string{
				"DOCKER_NET_V4_ADDR": m.IPv4Subnet,
				"DOCKER_NET_V6_ADDR": m.IPv6Subnet,
			} {
				if value, ok := cfg.Env[key]; ok && value == "" {
					cfg.Env[key] = subnet
				}
			}
		}
	}

	// save mgmt bridge name as a label
	for _, n := range c.Nodes {
		bridge := c.globalRuntime().Mgmt().Bridge
		if m := c.mgmtNetByNetwork(n.Config().MgmtNet); m != c.Config.Mgmt {
			bridge = m.Bridge
		}
		n.Config().Labels[clabconstants.NodeMgmtNetBr] = bridge
	}

	return nil
}

// mgmtNetNodes returns the configs of the nodes attached to the given management network.
func (c *CLab) mgmtNetNodes(m *clabtypes.MgmtNet) []*clabtypes.NodeConfig {
	var configs []*clabtypes.NodeConfig
	for _, node := range c.Nodes {
		if c.mgmtNetByNetwork(node.Config().MgmtNet) == m {
			configs = append(configs, node.Config())
		}
	}
	return configs
}

// SyncMgmtHostRoutes reconciles host routes to current macvlan management endpoints.
func (c *CLab) SyncMgmtHostRoutes(ctx context.Context) error {
	if c.Config == nil || c.Config.Mgmt == nil {
		return nil
	}
	c.mgmtRouteMu.Lock()
	defer c.mgmtRouteMu.Unlock()
	for _, m := range c.allMgmtNetworks() {
		if m.Driver != clabtypes.MgmtDriverMacvlan || !m.MacvlanAuxEnabled() {
			continue
		}
		rt, err := c.mgmtRuntime(c.globalRuntimeName, m)
		if err != nil {
			return err
		}
		if err := rt.SyncMgmtHostRoutes(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLab) staticManagementAddresses(m *clabtypes.MgmtNet) []netip.Addr {
	var addresses []netip.Addr
	for _, config := range c.mgmtNetNodes(m) {
		if address, err := netip.ParseAddr(config.MgmtIPv4Address); err == nil {
			addresses = append(addresses, address)
		}
		if address, err := netip.ParseAddr(config.MgmtIPv6Address); err == nil {
			addresses = append(addresses, address)
		}
	}
	return addresses
}

func (c *CLab) validateManagementLinks() error {
	if c.Config.Mgmt.Driver != clabtypes.MgmtDriverMacvlan || c.Config.Topology == nil {
		return nil
	}
	for _, link := range c.Config.Topology.Links {
		if link.Link.GetType() == clablinks.LinkTypeMgmtNet {
			return fmt.Errorf(
				"mgmt-net links require a bridge management network and cannot be used with mgmt.driver %q",
				c.Config.Mgmt.Driver,
			)
		}
	}
	return nil
}

// AllocateToolManagementIPs assigns addresses when the topology uses containerlab IPAM.
func (c *CLab) AllocateToolManagementIPs(ctx context.Context, cfg *clabtypes.NodeConfig) error {
	m := c.mgmtNetByNetwork(cfg.MgmtNet)
	if m.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nil
	}
	// tools attach to a network on demand, so it is created even when it is
	// flagged as skipped-when-unused by the topology
	if err := c.createNetworks(ctx, clabtypes.MgmtNetworks{m}); err != nil {
		return err
	}
	reserved, err := c.collectReservedManagementAddresses(ctx, m, nil)
	if err != nil {
		return err
	}
	return mgmt.AllocateManagementIPs(
		ctx,
		m,
		[]*clabtypes.NodeConfig{cfg},
		clabtypes.AllocationOptions{Reserved: reserved},
	)
}

// skipMgmtNetwork reports whether the management network m must not be created
// because it is not used: skip-when-unused is set on it and no node attached to
// it resolves to a network mode that attaches to the management network.
func (c *CLab) skipMgmtNetwork(m *clabtypes.MgmtNet) bool {
	if m == nil || !m.SkipWhenUnused {
		return false
	}

	topo := c.Config.Topology
	if topo == nil || len(topo.Nodes) == 0 {
		return false
	}

	for name := range topo.Nodes {
		if topo.GetNodeNetworkMode(name) == "none" {
			continue
		}
		if c.mgmtNetByNetwork(topo.GetNodeMgmtNet(name)) == m {
			return false
		}
	}

	return true
}

// mgmtNetworksSkipped reports whether every defined management network is
// skipped because it is unused.
func (c *CLab) mgmtNetworksSkipped() bool {
	for _, m := range c.allMgmtNetworks() {
		if !c.skipMgmtNetwork(m) {
			return false
		}
	}
	return true
}
