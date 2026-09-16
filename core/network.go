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

	var opts []clabruntime.NetworkCreateOptions

	if addresses := c.staticManagementAddresses(); len(addresses) != 0 {
		opts = append(opts, clabruntime.NetworkCreateOptions{StaticAddresses: addresses})
	}

	// create docker network or use existing one
	if err := c.globalRuntime().CreateNet(ctx, opts...); err != nil {
		return err
	}

	// save mgmt bridge name as a label
	for _, n := range c.Nodes {
		n.Config().Labels[clabconstants.NodeMgmtNetBr] = c.globalRuntime().Mgmt().Bridge
	}

	return nil
}

// SyncMgmtHostRoutes reconciles host routes to current macvlan management endpoints.
func (c *CLab) SyncMgmtHostRoutes(ctx context.Context) error {
	if c.Config == nil || c.Config.Mgmt == nil ||
		c.Config.Mgmt.Driver != clabtypes.MgmtDriverMacvlan ||
		!c.Config.Mgmt.MacvlanAuxEnabled() {
		return nil
	}
	c.mgmtRouteMu.Lock()
	defer c.mgmtRouteMu.Unlock()
	return c.globalRuntime().SyncMgmtHostRoutes(ctx)
}

func (c *CLab) staticManagementAddresses() []netip.Addr {
	var addresses []netip.Addr
	for _, node := range c.Nodes {
		config := node.Config()
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
			return fmt.Errorf("mgmt-net links require a bridge management network and cannot be used with mgmt.driver %q", c.Config.Mgmt.Driver)
		}
	}
	return nil
}

// AllocateToolManagementIPs assigns addresses when the topology uses containerlab IPAM.
func (c *CLab) AllocateToolManagementIPs(ctx context.Context, cfg *clabtypes.NodeConfig) error {
	if c.Config.Mgmt.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nil
	}
	if err := c.CreateNetwork(ctx); err != nil {
		return err
	}
	reserved, err := c.collectReservedManagementAddresses(ctx, nil)
	if err != nil {
		return err
	}
	return mgmt.AllocateManagementIPs(
		ctx,
		c.Config.Mgmt,
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
