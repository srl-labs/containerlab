package core

import (
	"context"
	"fmt"
	"net/netip"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/mgmt"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func (c *CLab) CreateNetwork(ctx context.Context) error {
	for _, m := range c.usedMgmtNetworks() {
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

// usedMgmtNetworks returns the default management network, the tailscale network
// and the extra management networks at least one node is attached to.
func (c *CLab) usedMgmtNetworks() clabtypes.MgmtNetworks {
	used := clabtypes.MgmtNetworks{c.Config.Mgmt}
	for _, m := range c.extraMgmtNetworks() {
		// the SSO proxy is a tool container outside c.Nodes
		if m.Tailscale != nil {
			used = append(used, m)
			continue
		}
		for _, n := range c.Nodes {
			if n.Config().MgmtNet == m.Network {
				used = append(used, m)
				break
			}
		}
	}
	return used
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
	for _, m := range c.usedMgmtNetworks() {
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
	if err := c.CreateNetwork(ctx); err != nil {
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

func (c *CLab) skipMgmtNetwork() bool {
	if c.Config.Mgmt == nil || !c.Config.Mgmt.SkipWhenUnused {
		return false
	}

	topo := c.Config.Topology
	if topo == nil || len(topo.Nodes) == 0 {
		return false
	}

	for name := range topo.Nodes {
		if topo.GetNodeNetworkMode(name) != "none" {
			return false
		}
	}

	return true
}
