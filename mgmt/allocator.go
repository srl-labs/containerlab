package mgmt

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
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
			if !node.ManagementIPAMEligible() {
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
	run := newAllocationRun(m, nodes, check, options)
	if err := run.allocate(ctx); err != nil {
		return nil, err
	}
	return run.drafts, nil
}

type allocationRun struct {
	network *clabtypes.MgmtNet
	nodes   []*clabtypes.NodeConfig
	drafts  []*clabtypes.NodeConfig
	byName  map[string]*clabtypes.NodeConfig
	check   func(context.Context, *clabtypes.MgmtNet, netip.Addr) error
	options clabtypes.AllocationOptions
}

type familyAllocation struct {
	ctx             context.Context
	cancel          context.CancelCauseFunc
	allocator       *ipam.IPAllocator
	prefix          netip.Prefix
	allocation      netip.Prefix
	v4              bool
	concurrency     int
	runtimeReserved map[netip.Addr]bool
	owned           map[netip.Addr]string
	probeErrors     sync.Map
	accept          func(netip.Addr) bool
}

func newAllocationRun(
	m *clabtypes.MgmtNet,
	nodes []*clabtypes.NodeConfig,
	check func(context.Context, *clabtypes.MgmtNet, netip.Addr) error,
	options clabtypes.AllocationOptions,
) *allocationRun {
	run := &allocationRun{
		network: m,
		drafts:  make([]*clabtypes.NodeConfig, len(nodes)),
		byName:  make(map[string]*clabtypes.NodeConfig, len(nodes)),
		check:   check,
		options: options,
	}
	for i, node := range nodes {
		copy := *node
		run.drafts[i] = &copy
		run.byName[copy.ShortName] = &copy
	}
	run.nodes = append([]*clabtypes.NodeConfig(nil), run.drafts...)
	sort.Slice(run.nodes, func(i, j int) bool {
		return run.nodes[i].ShortName < run.nodes[j].ShortName
	})
	return run
}

func (r *allocationRun) allocate(ctx context.Context) error {
	if r.network.IPAM.DADEnabled() && r.network.Driver == "macvlan" &&
		r.network.EffectiveMacvlanMode() == "bridge" {
		group, familyCtx := errgroup.WithContext(ctx)
		for _, v4 := range []bool{true, false} {
			group.Go(func() error { return r.allocateFamily(familyCtx, v4) })
		}
		return group.Wait()
	}
	for _, v4 := range []bool{true, false} {
		if err := r.allocateFamily(ctx, v4); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (r *allocationRun) allocateFamily(ctx context.Context, v4 bool) error {
	subnet, pool, gateway := r.network.IPv6Subnet, r.network.IPv6Range, r.network.IPv6Gw
	if v4 {
		subnet, pool, gateway = r.network.IPv4Subnet, r.network.IPv4Range, r.network.IPv4Gw
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
	gatewayAddress := prefix.Masked().Addr().Next()
	if gateway != "" {
		gatewayAddress, err = netip.ParseAddr(gateway)
		if err != nil {
			return err
		}
	}
	reserved := []netip.Addr{gatewayAddress}
	if r.network.MacvlanAux != "" {
		aux, _, err := r.network.MacvlanHostAddress()
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
	checkCtx, cancel := context.WithCancelCause(ctx)
	family := &familyAllocation{
		ctx:             checkCtx,
		cancel:          cancel,
		allocator:       allocator,
		prefix:          prefix,
		allocation:      allocation,
		v4:              v4,
		concurrency:     1,
		runtimeReserved: make(map[netip.Addr]bool, len(r.options.Reserved)),
		owned:           make(map[netip.Addr]string),
	}
	defer family.cancel(nil)
	if r.network.Driver == "macvlan" && r.network.EffectiveMacvlanMode() == "bridge" {
		family.concurrency = len(r.nodes)
	}
	if r.network.IPAM.DADEnabled() {
		family.accept = func(ip netip.Addr) bool {
			err := r.check(family.ctx, r.network, ip)
			if err == nil {
				return true
			}
			family.probeErrors.Store(ip, err)
			if !errors.Is(err, ErrDuplicateAddress) {
				family.cancel(fmt.Errorf("management address %s: %w", ip, err))
				return false
			}
			var occupied *occupiedPrefix
			if errors.As(err, &occupied) {
				family.allocator.ReservePrefix(occupied.prefix)
			}
			return false
		}
	}
	if err := r.reserveExisting(family); err != nil {
		return err
	}
	if err := r.probeRetained(family); err != nil {
		return err
	}
	return r.allocatePending(family)
}

func (r *allocationRun) reserveExisting(family *familyAllocation) error {
	for _, address := range r.options.Reserved {
		if family.prefix.Contains(address) {
			family.runtimeReserved[address] = true
			family.allocator.ReservePrefix(netip.PrefixFrom(address, address.BitLen()))
		}
	}
	for _, current := range r.options.Existing {
		if !family.prefix.Contains(current.Address) {
			continue
		}
		if err := family.allocator.Reserve(current.Address); err != nil {
			return err
		}
		family.owned[current.Address] = current.NodeName
		node := r.byName[current.NodeName]
		if node != nil && node.ManagementIPAMEligible() && *family.address(node) == "" {
			*family.address(node) = current.Address.String()
		}
	}
	return nil
}

func (r *allocationRun) probeRetained(family *familyAllocation) error {
	var explicit []netip.Addr
	var explicitNodes []*clabtypes.NodeConfig
	for _, node := range r.nodes {
		if !node.ManagementIPAMEligible() || *family.address(node) == "" {
			continue
		}
		address, err := netip.ParseAddr(*family.address(node))
		if err != nil {
			return err
		}
		if owner, ok := family.owned[address]; ok && owner == node.ShortName {
			continue
		}
		if err := family.allocator.Reserve(address); err != nil {
			if family.runtimeReserved[address] && r.network.IPAM.DADEnabled() {
				log.Warn("Duplicate static management address; retaining configured address",
					"node", node.ShortName, "address", address, "error", err)
				continue
			}
			return fmt.Errorf("node %s: %w", node.ShortName, err)
		}
		explicit = append(explicit, address)
		explicitNodes = append(explicitNodes, node)
	}

	var preferences []netip.Addr
	var retained []*clabtypes.NodeConfig
	for _, node := range r.nodes {
		if !node.ManagementIPAMEligible() || *family.address(node) != "" {
			continue
		}
		preference := r.options.Preferred[node.ShortName].IPv6
		if family.v4 {
			preference = r.options.Preferred[node.ShortName].IPv4
		}
		address, err := netip.ParseAddr(preference)
		if err != nil || !family.allocation.Contains(address) {
			continue
		}
		if err := family.allocator.Reserve(address); err != nil {
			continue
		}
		preferences = append(preferences, address)
		retained = append(retained, node)
	}
	candidates := append(append([]netip.Addr(nil), explicit...), preferences...)
	accepted, err := ipam.CheckIPAddresses(
		family.ctx,
		candidates,
		family.concurrency,
		family.accept,
	)
	if err != nil {
		return err
	}
	for i, ok := range accepted[:len(explicit)] {
		if !ok {
			err, _ := family.probeErrors.Load(explicit[i])
			log.Warn("Duplicate static management address; retaining configured address",
				"node", explicitNodes[i].ShortName, "address", explicit[i], "error", err.(error))
		}
	}
	for i, ok := range accepted[len(explicit):] {
		if ok {
			*family.address(retained[i]) = preferences[i].String()
		}
	}
	return nil
}

func (r *allocationRun) allocatePending(family *familyAllocation) error {
	var keys []string
	var pending []*clabtypes.NodeConfig
	for _, node := range r.nodes {
		if !node.ManagementIPAMEligible() || *family.address(node) != "" {
			continue
		}
		keys = append(keys, node.ShortName)
		pending = append(pending, node)
	}
	allocated, err := family.allocator.AllocateBatch(
		family.ctx,
		keys,
		family.concurrency,
		family.accept,
	)
	if err != nil {
		family.probeErrors.Range(func(_, value any) bool {
			err = errors.Join(err, value.(error))
			return true
		})
		return err
	}
	for i, address := range allocated {
		*family.address(pending[i]) = address.String()
	}
	return nil
}

func (f *familyAllocation) address(node *clabtypes.NodeConfig) *string {
	if f.v4 {
		return &node.MgmtIPv4Address
	}
	return &node.MgmtIPv6Address
}
