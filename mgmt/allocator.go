package mgmt

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"

	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils/ipam"
)

// AllocateManagementIPs assigns management addresses after the runtime resolves
// the network's subnet and gateway.
func AllocateManagementIPs(
	ctx context.Context,
	m *clabtypes.MgmtNet,
	nodes []*clabtypes.NodeConfig,
	options clabtypes.AllocationOptions,
) error {
	if m.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nil
	}
	checker := &dadChecker{}
	drafts, err := allocateManagementIPs(ctx, m, nodes, checker.Check, options)
	if err == nil {
		err = setManagementIPConfig(m, drafts)
	}
	if closeErr := checker.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err != nil {
		return err
	}
	for i, node := range nodes {
		node.MgmtIPv4Address = drafts[i].MgmtIPv4Address
		node.MgmtIPv4PrefixLength = drafts[i].MgmtIPv4PrefixLength
		node.MgmtIPv4Gateway = drafts[i].MgmtIPv4Gateway
		node.MgmtIPv6Address = drafts[i].MgmtIPv6Address
		node.MgmtIPv6PrefixLength = drafts[i].MgmtIPv6PrefixLength
		node.MgmtIPv6Gateway = drafts[i].MgmtIPv6Gateway
	}
	return nil
}

func setManagementIPConfig(m *clabtypes.MgmtNet, nodes []*clabtypes.NodeConfig) error {
	for _, v4 := range []bool{true, false} {
		subnet, configuredGateway := m.IPv6Subnet, m.IPv6Gw
		if v4 {
			subnet, configuredGateway = m.IPv4Subnet, m.IPv4Gw
		}
		if subnet == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(subnet)
		if err != nil {
			return err
		}
		gateway := prefix.Masked().Addr().Next()
		if configuredGateway != "" {
			gateway, err = netip.ParseAddr(configuredGateway)
			if err != nil {
				return err
			}
		}
		for _, node := range nodes {
			if !eligibleForManagementIPAM(node) {
				continue
			}
			if v4 && node.MgmtIPv4Address != "" {
				node.MgmtIPv4PrefixLength, node.MgmtIPv4Gateway = prefix.Bits(), gateway.String()
			}
			if !v4 && node.MgmtIPv6Address != "" {
				node.MgmtIPv6PrefixLength, node.MgmtIPv6Gateway = prefix.Bits(), gateway.String()
			}
		}
	}
	return nil
}

func eligibleForManagementIPAM(n *clabtypes.NodeConfig) bool {
	return !n.IsRootNamespaceBased && !n.SkipUniquenessCheck &&
		n.NetworkMode != "host" && n.NetworkMode != "none" &&
		!strings.HasPrefix(n.NetworkMode, "container:")
}

func allocateManagementIPs(ctx context.Context, m *clabtypes.MgmtNet, nodes []*clabtypes.NodeConfig,
	check func(context.Context, *clabtypes.MgmtNet, netip.Addr) error,
	options clabtypes.AllocationOptions,
) ([]*clabtypes.NodeConfig, error) {

	if m.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nodes, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Commit generated addresses only after every candidate passes validation and DAD.
	drafts := make([]*clabtypes.NodeConfig, len(nodes))
	for i, node := range nodes {
		copy := *node
		drafts[i] = &copy
	}
	nodes = append([]*clabtypes.NodeConfig(nil), drafts...)
	byName := make(map[string]*clabtypes.NodeConfig, len(nodes))
	for _, node := range nodes {
		byName[node.ShortName] = node
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ShortName < nodes[j].ShortName })

	allocateFamily := func(ctx context.Context, v4 bool) error {
		subnet, pool, gateway := m.IPv6Subnet, m.IPv6Range, m.IPv6Gw
		if v4 {
			subnet, pool, gateway = m.IPv4Subnet, m.IPv4Range, m.IPv4Gw
		}
		if subnet == "" {
			return nil
		}
		if subnet == "auto" {
			return fmt.Errorf(
				"management subnet is unresolved; the runtime must resolve it before IP allocation",
			)
		}
		prefix, err := netip.ParsePrefix(subnet)
		if err != nil || prefix.Addr().Is4() != v4 {
			return fmt.Errorf("invalid management subnet %q", subnet)
		}
		if pool == "" {
			pool = subnet
		}
		allocation, err := netip.ParsePrefix(pool)
		if err != nil {
			return err
		}
		gw := prefix.Masked().Addr().Next()
		if gateway != "" {
			gw, err = netip.ParseAddr(gateway)
			if err != nil {
				return err
			}
		}
		reserved := []netip.Addr{gw}
		if m.MacvlanAux != "" {
			aux, _, err := m.MacvlanHostAddress()
			if err != nil {
				return err
			}
			if aux.Is4() == v4 {
				reserved = append(reserved, aux)
			}
		}
		allocator, err := ipam.NewIPAllocator(prefix, allocation, reserved)
		if err != nil {
			return err
		}
		address := func(n *clabtypes.NodeConfig) *string {
			if v4 {
				return &n.MgmtIPv4Address
			}
			return &n.MgmtIPv6Address
		}
		// Runtime reservations are authoritative even when wire/local DAD is disabled.
		runtimeReserved := make(map[netip.Addr]bool, len(options.Reserved))
		for _, ip := range options.Reserved {
			if prefix.Contains(ip) {
				runtimeReserved[ip] = true
				allocator.ReservePrefix(netip.PrefixFrom(ip, ip.BitLen()))
			}
		}
		owned := make(map[netip.Addr]string)
		for _, current := range options.Existing {
			if !prefix.Contains(current.Address) {
				continue
			}
			if err := allocator.Reserve(current.Address); err != nil {
				return err
			}
			owned[current.Address] = current.NodeName
			if n := byName[current.NodeName]; n != nil && eligibleForManagementIPAM(n) && *address(n) == "" {
				*address(n) = current.Address.String()
			}
		}

		// The bool callback rejects occupied candidates. A cancellation cause
		// carries fatal probe errors out of the generic allocator's retry loop.
		checkCtx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		concurrency := 1
		if m.Driver == "macvlan" && m.EffectiveMacvlanMode() == "bridge" {
			concurrency = len(nodes)
		}
		var probeErrors sync.Map
		var accept func(netip.Addr) bool
		if m.IPAM.DADEnabled() {
			accept = func(ip netip.Addr) bool {
				err := check(checkCtx, m, ip)
				if err == nil {
					return true
				}
				probeErrors.Store(ip, err)
				if !errors.Is(err, ErrDuplicateAddress) {
					cancel(fmt.Errorf("management address %s: %w", ip, err))
					return false
				}
				var occupied *occupiedPrefix
				if errors.As(err, &occupied) {
					allocator.ReservePrefix(occupied.prefix)
				}
				return false
			}
		}
		var explicit []netip.Addr
		var explicitNodes []*clabtypes.NodeConfig
		for _, n := range nodes {
			if !eligibleForManagementIPAM(n) || *address(n) == "" {
				continue
			}
			ip, err := netip.ParseAddr(*address(n))
			if err != nil {
				return err
			}
			if owner, ok := owned[ip]; ok && owner == n.ShortName {
				continue
			}
			if err := allocator.Reserve(ip); err != nil {
				if runtimeReserved[ip] && m.IPAM.DADEnabled() {
					log.Warn("Duplicate static management address; retaining configured address",
						"node", n.ShortName, "address", ip, "error", err)
					continue
				}
				return fmt.Errorf("node %s: %w", n.ShortName, err)
			}
			explicit = append(explicit, ip)
			explicitNodes = append(explicitNodes, n)
		}
		// Reserve all retained preferences before allocating for new nodes.
		var preferences []netip.Addr
		var retained []*clabtypes.NodeConfig
		for _, n := range nodes {
			if !eligibleForManagementIPAM(n) || *address(n) != "" {
				continue
			}
			preference := options.Preferred[n.ShortName].IPv6
			if v4 {
				preference = options.Preferred[n.ShortName].IPv4
			}
			ip, err := netip.ParseAddr(preference)
			if err != nil || !allocation.Contains(ip) {
				continue
			}
			if err := allocator.Reserve(ip); err != nil {
				continue
			}
			preferences = append(preferences, ip)
			retained = append(retained, n)
		}
		candidates := append(append([]netip.Addr(nil), explicit...), preferences...)
		accepted, err := ipam.CheckIPAddresses(checkCtx, candidates, concurrency, accept)
		if err != nil {
			return err
		}
		for i, ok := range accepted[:len(explicit)] {
			if !ok {
				err, _ := probeErrors.Load(explicit[i])
				log.Warn("Duplicate static management address; retaining configured address",
					"node", explicitNodes[i].ShortName, "address", explicit[i], "error", err.(error))
			}
		}
		for i, ok := range accepted[len(explicit):] {
			if ok {
				*address(retained[i]) = preferences[i].String()
			}
		}
		var keys []string
		var pending []*clabtypes.NodeConfig
		for _, n := range nodes {
			if !eligibleForManagementIPAM(n) || *address(n) != "" {
				continue
			}
			keys = append(keys, n.ShortName)
			pending = append(pending, n)
		}
		allocated, err := allocator.AllocateBatch(checkCtx, keys, concurrency, accept)
		if err != nil {
			// Preserve duplicate diagnostics on pool exhaustion as well as fatal errors.
			probeErrors.Range(func(_, value any) bool { err = errors.Join(err, value.(error)); return true })
			return err
		}
		for i, ip := range allocated {
			*address(pending[i]) = ip.String()
		}

		return nil
	}
	if m.IPAM.DADEnabled() && m.Driver == "macvlan" && m.EffectiveMacvlanMode() == "bridge" {
		group, familyCtx := errgroup.WithContext(ctx)
		for _, v4 := range []bool{true, false} {
			group.Go(func() error { return allocateFamily(familyCtx, v4) })
		}
		if err := group.Wait(); err != nil {
			return nil, err
		}
	} else {
		for _, v4 := range []bool{true, false} {
			if err := allocateFamily(ctx, v4); err != nil {
				return nil, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return drafts, nil
}
