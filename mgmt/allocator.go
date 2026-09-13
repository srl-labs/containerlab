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
	options ...clabtypes.AllocationOptions,
) error {
	if m.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nil
	}
	checker := &dadChecker{}
	// Keep node configs unchanged if closing the shared probe interface fails.
	drafts := make([]*clabtypes.NodeConfig, len(nodes))
	for i, node := range nodes {
		copy := *node
		drafts[i] = &copy
	}
	err := allocateManagementIPs(ctx, m, drafts, checker.Check, options...)
	if closeErr := checker.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err != nil {
		return err
	}
	for i, node := range nodes {
		node.MgmtIPv4Address = drafts[i].MgmtIPv4Address
		node.MgmtIPv6Address = drafts[i].MgmtIPv6Address
	}
	return nil
}

func allocateManagementIPs(ctx context.Context, m *clabtypes.MgmtNet, nodes []*clabtypes.NodeConfig,
	check func(context.Context, *clabtypes.MgmtNet, netip.Addr) error,
	options ...clabtypes.AllocationOptions,
) error {

	if m.IPAM.Provider == clabtypes.IPAMProviderRuntime {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	var existing []clabtypes.ExistingAddress
	var reservedAddresses []netip.Addr
	preferred := make(map[string]clabtypes.NodeAddresses)
	for _, option := range options {
		existing = append(existing, option.Existing...)
		reservedAddresses = append(reservedAddresses, option.Reserved...)
		for name, addresses := range option.Preferred {
			preferred[name] = addresses
		}
	}
	// Commit generated addresses only after every candidate passes validation and DAD.
	originals := append([]*clabtypes.NodeConfig(nil), nodes...)
	nodes = make([]*clabtypes.NodeConfig, len(originals))
	for i, node := range originals {
		copy := *node
		nodes[i] = &copy
	}
	drafts := append([]*clabtypes.NodeConfig(nil), nodes...)
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
		eligible := func(n *clabtypes.NodeConfig) bool {
			return n.NetworkMode != "host" && n.NetworkMode != "none" &&
				!strings.HasPrefix(n.NetworkMode, "container:")
		}
		// Runtime reservations are authoritative even when wire/local DAD is disabled.
		runtimeReserved := make(map[netip.Addr]bool, len(reservedAddresses))
		for _, ip := range reservedAddresses {
			if prefix.Contains(ip) {
				runtimeReserved[ip] = true
				allocator.ReservePrefix(netip.PrefixFrom(ip, ip.BitLen()))
			}
		}
		owned := make(map[netip.Addr]string)
		for _, current := range existing {
			if !prefix.Contains(current.Address) {
				continue
			}
			if err := allocator.Reserve(current.Address); err != nil {
				return err
			}
			owned[current.Address] = current.NodeName
			if n := byName[current.NodeName]; n != nil && eligible(n) && *address(n) == "" {
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
			if !eligible(n) || *address(n) == "" {
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
					log.Warn("Static management address is already reserved by the runtime",
						"node", n.ShortName, "address", ip)
				}
				return fmt.Errorf("node %s: %w", n.ShortName, err)
			}
			explicit = append(explicit, ip)
			explicitNodes = append(explicitNodes, n)
		}
		accepted, err := ipam.CheckIPAddresses(checkCtx, explicit, concurrency, accept)
		if err != nil {
			return err
		}
		for i, ok := range accepted {
			if !ok {
				err, _ := probeErrors.Load(explicit[i])
				log.Warn("Duplicate static management address; retaining configured address",
					"node", explicitNodes[i].ShortName, "address", explicit[i], "error", err.(error))
			}
		}
		// Reserve all retained preferences before allocating for new nodes.
		var preferences []netip.Addr
		var retained []*clabtypes.NodeConfig
		for _, n := range nodes {
			if !eligible(n) || *address(n) != "" {
				continue
			}
			preference := preferred[n.ShortName].IPv6
			if v4 {
				preference = preferred[n.ShortName].IPv4
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
		accepted, err = ipam.CheckIPAddresses(checkCtx, preferences, concurrency, accept)
		if err != nil {
			return err
		}
		for i, ok := range accepted {
			if ok {
				*address(retained[i]) = preferences[i].String()
			}
		}
		var keys []string
		var pending []*clabtypes.NodeConfig
		for _, n := range nodes {
			if !eligible(n) || *address(n) != "" {
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
			return err
		}
	} else {
		for _, v4 := range []bool{true, false} {
			if err := allocateFamily(ctx, v4); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for i, original := range originals {
		original.MgmtIPv4Address = drafts[i].MgmtIPv4Address
		original.MgmtIPv6Address = drafts[i].MgmtIPv6Address
	}
	return nil
}
