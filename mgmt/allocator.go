package mgmt

import (
	"context"
	"fmt"
	"net/netip"

	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils/ipam"
)

type managementPool struct {
	subnet     string
	allocation string
	gateway    string
}

type managementAssignment struct {
	node       *clabtypes.NodeConfig
	address    *string
	preference string
}

// AllocateManagementIPs assigns management addresses after the runtime resolves
// the network's subnet and gateway.
func AllocateManagementIPs(ctx context.Context, m *clabtypes.MgmtNet, nodes []*clabtypes.NodeConfig, options clabtypes.AllocationOptions) error {
	if m.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	eligible := make([]*clabtypes.NodeConfig, 0, len(nodes))
	for _, node := range nodes {
		if node.ManagementIPAMEligible() {
			eligible = append(eligible, node)
		}
	}

	var dad *ipam.DADClient
	if m.IPAM.DADEnabled() && m.Driver == clabtypes.MgmtDriverMacvlan &&
		m.EffectiveMacvlanMode() == "bridge" {
		var err error
		dad, err = ipam.NewDADClient(m.MacvlanParent)
		if err != nil {
			return err
		}
		defer dad.Close()
	}

	pools := []managementPool{
		{
			subnet:     m.IPv4Subnet,
			allocation: m.IPv4Range,
			gateway:    m.IPv4Gw,
		},
		{
			subnet:     m.IPv6Subnet,
			allocation: m.IPv6Range,
			gateway:    m.IPv6Gw,
		},
	}

	for _, pool := range pools {
		if pool.subnet == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(pool.subnet)
		if err != nil {
			return fmt.Errorf("invalid management subnet %q", pool.subnet)
		}

		if pool.allocation == "" {
			pool.allocation = pool.subnet
		}

		allocation, err := netip.ParsePrefix(pool.allocation)
		if err != nil {
			return fmt.Errorf("invalid management pool %q", pool.allocation)
		}
		gatewayAddress := prefix.Masked().Addr().Next()
		if pool.gateway != "" {
			gatewayAddress, err = netip.ParseAddr(pool.gateway)
			if err != nil {
				return err
			}
		}

		assignments := make([]managementAssignment, 0, len(eligible))
		switch prefix.Addr().BitLen() {
		case 32:
			for _, node := range eligible {
				node.MgmtIPv4PrefixLength, node.MgmtIPv4Gateway = prefix.Bits(), gatewayAddress.String()
				assignments = append(assignments, managementAssignment{
					node:       node,
					address:    &node.MgmtIPv4Address,
					preference: options.Preferred[node.ShortName].IPv4,
				})
			}
		case 128:
			for _, node := range eligible {
				node.MgmtIPv6PrefixLength, node.MgmtIPv6Gateway = prefix.Bits(), gatewayAddress.String()
				assignments = append(assignments, managementAssignment{
					node:       node,
					address:    &node.MgmtIPv6Address,
					preference: options.Preferred[node.ShortName].IPv6,
				})
			}
		}
		byName := make(map[string]*managementAssignment, len(assignments))
		for i := range assignments {
			byName[assignments[i].node.ShortName] = &assignments[i]
		}

		reserved := []netip.Addr{gatewayAddress}
		for _, address := range options.Reserved {
			if prefix.Contains(address) {
				reserved = append(reserved, address)
			}
		}

		allocator, err := ipam.NewIPAllocator(prefix, allocation, reserved)
		if err != nil {
			return err
		}

		owned := make(map[netip.Addr]string)
		for _, current := range options.Existing {
			if !prefix.Contains(current.Address) {
				continue
			}
			if err := allocator.Reserve(current.Address); err != nil {
				if owner := owned[current.Address]; owner != "" {
					return fmt.Errorf(
						"reserve existing management address for node %q (container %q), already owned by node %q: %w",
						current.NodeName,
						current.ContainerID,
						owner,
						err,
					)
				}
				return fmt.Errorf(
					"reserve existing management address for node %q (container %q): %w",
					current.NodeName, current.ContainerID, err,
				)
			}
			owned[current.Address] = current.NodeName
			assignment := byName[current.NodeName]
			if assignment == nil {
				continue
			}
			if *assignment.address == "" {
				*assignment.address = current.Address.String()
			}
		}
		for _, assignment := range assignments {
			if *assignment.address == "" {
				continue
			}
			address, err := netip.ParseAddr(*assignment.address)
			if err != nil {
				return err
			}
			if owned[address] == assignment.node.ShortName {
				continue
			}
			if err := allocator.Reserve(address); err != nil {
				return fmt.Errorf("node %s: %w", assignment.node.ShortName, err)
			}
		}
		for _, assignment := range assignments {
			if *assignment.address != "" {
				continue
			}
			address, err := netip.ParseAddr(assignment.preference)
			if err != nil || !allocation.Contains(address) {
				continue
			}
			if err := allocator.Reserve(address); err != nil {
				continue
			}
			*assignment.address = address.String()
		}
		pending := make([]managementAssignment, 0, len(assignments))
		for _, assignment := range assignments {
			if *assignment.address != "" {
				continue
			}
			pending = append(pending, assignment)
		}
		var addresses []netip.Addr
		if dad == nil {
			addresses, err = allocator.NextBatch(ctx, len(pending))
		} else {
			addresses, err = allocator.NextBatch(ctx, len(pending), dad)
		}
		if err != nil {
			return err
		}
		for i := range pending {
			*pending[i].address = addresses[i].String()
		}
	}
	return ctx.Err()
}
