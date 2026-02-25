package events

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/cilium/ebpf"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const (
	trafficBPFFSRoot         = "/sys/fs/bpf/containerlab_events_traffic"
	trafficFilterPref        = "42420"
	defaultTrafficTick       = 5 * time.Second
	minTrafficTick           = 1 * time.Second
	bpffsMountPath           = "/sys/fs/bpf"
	bpffsFsType        int64 = 0xCAFE4A11
)

const (
	bucketIPv4    = 1
	bucketIPv6    = 2
	bucketARP     = 3
	bucketICMP    = 4
	bucketICMPv6  = 5
	bucketTCP     = 6
	bucketUDP     = 7
	bucketBGP     = 8
	bucketSTP     = 9
	bucketLLDP    = 10
	bucketOtherL2 = 11
)

//go:embed traffic_proto_top.x86.o
var trafficBPFObjectX86 []byte

//go:embed traffic_proto_top.arm64.o
var trafficBPFObjectARM64 []byte

type trafficBPFObjects struct {
	IngressProg   *ebpf.Program `ebpf:"ingress_prog"`
	EgressProg    *ebpf.Program `ebpf:"egress_prog"`
	ProtoCounters *ebpf.Map     `ebpf:"proto_counters"`
}

type protoCounterKey struct {
	Ifindex   uint32
	Direction uint8
	Bucket    uint8
	Port      uint16
}

type protoCounterValue struct {
	Packets uint64
	Bytes   uint64
}

type protocolRow struct {
	Protocol string
	Packets  uint64
	Bytes    uint64
}

type interfaceProtocolRow struct {
	Interface string
	Protocol  string
	Packets   uint64
	Bytes     uint64
}

type trafficScope struct {
	ID         string
	Label      string
	NSPath     string
	BPFFSPath  string
	MapPinPath string
	IngressPin string
	EgressPin  string
	Attached   map[string]struct{}
	Previous   map[protoCounterKey]protoCounterValue
	IfNames    map[int]string
	Objects    trafficBPFObjects
}

type trafficCollector struct {
	interval time.Duration
	tcpSvc   map[uint16]string
	udpSvc   map[uint16]string
	scopes   map[string]*trafficScope
}

func startTrafficCollector(
	ctx context.Context,
	sink chan<- aggregatedEvent,
	interval time.Duration,
	containers []clabruntime.GenericContainer,
) (<-chan error, error) {
	collector := &trafficCollector{
		interval: normalizeTrafficInterval(interval),
		scopes:   make(map[string]*trafficScope),
	}

	collector.tcpSvc, collector.udpSvc = loadServiceMaps()

	if err := collector.init(containers); err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	go collector.run(ctx, sink, errCh)

	return errCh, nil
}

func normalizeTrafficInterval(interval time.Duration) time.Duration {
	switch {
	case interval <= 0:
		return defaultTrafficTick
	case interval < minTrafficTick:
		return minTrafficTick
	default:
		return interval
	}
}

func (c *trafficCollector) init(containers []clabruntime.GenericContainer) error {
	if _, err := exec.LookPath("tc"); err != nil {
		return fmt.Errorf("traffic monitoring requires %q in PATH: %w", "tc", err)
	}

	if err := ensureBPFFSMounted(); err != nil {
		return err
	}

	scopes, err := discoverTrafficScopes(containers)
	if err != nil {
		return err
	}

	for idx := range scopes {
		scope := scopes[idx]
		if err := c.initScope(scope); err != nil {
			return err
		}
		c.scopes[scope.ID] = scope
	}

	if len(c.scopes) == 0 {
		return fmt.Errorf("no traffic monitoring scopes were discovered")
	}

	return nil
}

func (c *trafficCollector) initScope(scope *trafficScope) error {
	if scope == nil {
		return fmt.Errorf("traffic scope is nil")
	}

	if err := os.MkdirAll(scope.BPFFSPath, 0o755); err != nil {
		return fmt.Errorf("failed to create traffic scope bpffs directory %q: %w", scope.BPFFSPath, err)
	}

	if err := c.loadBPFPrograms(scope); err != nil {
		return err
	}

	if err := c.reconcileScopeInterfaces(context.Background(), scope); err != nil {
		return err
	}

	snapshot, err := c.dumpCounterMap(scope)
	if err != nil {
		return err
	}

	scope.Previous = snapshot
	return nil
}

func (c *trafficCollector) run(
	ctx context.Context,
	sink chan<- aggregatedEvent,
	errCh chan<- error,
) {
	defer close(errCh)
	defer c.cleanup()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.sample(ctx, sink); err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}

				return
			}
		}
	}
}

func (c *trafficCollector) sample(ctx context.Context, sink chan<- aggregatedEvent) error {
	globalCounters := make(map[string]protoCounterValue)
	interfaceCounters := make([]interfaceProtocolRow, 0)
	totalAttached := 0

	scopeIDs := make([]string, 0, len(c.scopes))
	for scopeID := range c.scopes {
		scopeIDs = append(scopeIDs, scopeID)
	}
	sort.Strings(scopeIDs)

	for idx := range scopeIDs {
		scope := c.scopes[scopeIDs[idx]]
		if scope == nil {
			continue
		}

		if err := c.reconcileScopeInterfaces(ctx, scope); err != nil {
			return err
		}

		totalAttached += len(scope.Attached)

		current, err := c.dumpCounterMap(scope)
		if err != nil {
			return err
		}

		delta := diffCounters(scope.Previous, current)
		scope.Previous = current
		if len(delta) == 0 {
			continue
		}

		scopeGlobalRows, scopeIfaceRows := c.aggregateScopeDelta(scope, delta)
		for rowIdx := range scopeGlobalRows {
			row := scopeGlobalRows[rowIdx]
			globalCounters[row.Protocol] = addCounter(globalCounters[row.Protocol], protoCounterValue{
				Packets: row.Packets,
				Bytes:   row.Bytes,
			})
		}

		interfaceCounters = append(interfaceCounters, scopeIfaceRows...)
	}

	if len(globalCounters) == 0 && len(interfaceCounters) == 0 {
		return nil
	}

	globalRows := make([]protocolRow, 0, len(globalCounters))
	for protocol, counter := range globalCounters {
		globalRows = append(globalRows, protocolRow{
			Protocol: protocol,
			Packets:  counter.Packets,
			Bytes:    counter.Bytes,
		})
	}

	sort.Slice(globalRows, func(i, j int) bool {
		switch {
		case globalRows[i].Bytes != globalRows[j].Bytes:
			return globalRows[i].Bytes > globalRows[j].Bytes
		case globalRows[i].Packets != globalRows[j].Packets:
			return globalRows[i].Packets > globalRows[j].Packets
		default:
			return globalRows[i].Protocol < globalRows[j].Protocol
		}
	})

	sort.Slice(interfaceCounters, func(i, j int) bool {
		switch {
		case interfaceCounters[i].Bytes != interfaceCounters[j].Bytes:
			return interfaceCounters[i].Bytes > interfaceCounters[j].Bytes
		case interfaceCounters[i].Packets != interfaceCounters[j].Packets:
			return interfaceCounters[i].Packets > interfaceCounters[j].Packets
		case interfaceCounters[i].Interface != interfaceCounters[j].Interface:
			return interfaceCounters[i].Interface < interfaceCounters[j].Interface
		default:
			return interfaceCounters[i].Protocol < interfaceCounters[j].Protocol
		}
	})

	timestamp := time.Now().UTC()
	if !sendTrafficEvent(ctx, sink, aggregatedEvent{
		Timestamp: timestamp,
		Type:      "traffic",
		Action:    "sample",
		ActorName: "tc-ebpf-map",
		Attributes: map[string]string{
			"window":      c.interval.String(),
			"interfaces":  strconv.Itoa(totalAttached),
			"global_rows": strconv.Itoa(len(globalRows)),
			"iface_rows":  strconv.Itoa(len(interfaceCounters)),
			"scopes":      strconv.Itoa(len(c.scopes)),
		},
	}) {
		return nil
	}

	for idx := range globalRows {
		row := globalRows[idx]
		if !sendTrafficEvent(ctx, sink, aggregatedEvent{
			Timestamp: timestamp,
			Type:      "traffic",
			Action:    "global",
			ActorName: row.Protocol,
			Attributes: map[string]string{
				"scope":    "global",
				"protocol": row.Protocol,
				"packets":  strconv.FormatUint(row.Packets, 10),
				"bytes":    strconv.FormatUint(row.Bytes, 10),
				"window":   c.interval.String(),
			},
		}) {
			return nil
		}
	}

	for idx := range interfaceCounters {
		row := interfaceCounters[idx]
		if !sendTrafficEvent(ctx, sink, aggregatedEvent{
			Timestamp: timestamp,
			Type:      "traffic",
			Action:    "interface",
			ActorName: row.Interface,
			Attributes: map[string]string{
				"scope":     "interface",
				"interface": row.Interface,
				"protocol":  row.Protocol,
				"packets":   strconv.FormatUint(row.Packets, 10),
				"bytes":     strconv.FormatUint(row.Bytes, 10),
				"window":    c.interval.String(),
			},
		}) {
			return nil
		}
	}

	return nil
}

func sendTrafficEvent(ctx context.Context, sink chan<- aggregatedEvent, ev aggregatedEvent) bool {
	select {
	case sink <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *trafficCollector) aggregateScopeDelta(
	scope *trafficScope,
	delta map[protoCounterKey]protoCounterValue,
) ([]protocolRow, []interfaceProtocolRow) {
	global := make(map[string]protoCounterValue)
	perInterface := make(map[string]map[string]protoCounterValue)

	for key, value := range delta {
		protocol := c.protocolLabel(key.Bucket, key.Port)
		if protocol == "" {
			continue
		}

		global[protocol] = addCounter(global[protocol], value)

		ifName := scope.IfNames[int(key.Ifindex)]
		if ifName == "" {
			ifName = fmt.Sprintf("ifindex-%d", key.Ifindex)
		}

		if scope.Label != "host" {
			ifName = scope.Label + ":" + ifName
		}

		protoCounters, ok := perInterface[ifName]
		if !ok {
			protoCounters = make(map[string]protoCounterValue)
			perInterface[ifName] = protoCounters
		}

		protoCounters[protocol] = addCounter(protoCounters[protocol], value)
	}

	globalRows := make([]protocolRow, 0, len(global))
	for protocol, counter := range global {
		globalRows = append(globalRows, protocolRow{
			Protocol: protocol,
			Packets:  counter.Packets,
			Bytes:    counter.Bytes,
		})
	}

	interfaceRows := make([]interfaceProtocolRow, 0)
	for ifName, protocols := range perInterface {
		for protocol, counter := range protocols {
			interfaceRows = append(interfaceRows, interfaceProtocolRow{
				Interface: ifName,
				Protocol:  protocol,
				Packets:   counter.Packets,
				Bytes:     counter.Bytes,
			})
		}
	}

	return globalRows, interfaceRows
}

func addCounter(current, delta protoCounterValue) protoCounterValue {
	current.Packets += delta.Packets
	current.Bytes += delta.Bytes

	return current
}

func (c *trafficCollector) protocolLabel(bucket uint8, port uint16) string {
	switch bucket {
	case bucketIPv4:
		return "ipv4"
	case bucketIPv6:
		return "ipv6"
	case bucketARP:
		return "arp"
	case bucketICMP:
		return "icmp"
	case bucketICMPv6:
		return "icmpv6"
	case bucketBGP:
		return "bgp/tcp"
	case bucketSTP:
		return "stp"
	case bucketLLDP:
		return "lldp"
	case bucketOtherL2:
		return "other_l2"
	case bucketTCP:
		if svc, ok := c.tcpSvc[port]; ok && svc != "" {
			return svc + "/tcp"
		}

		return fmt.Sprintf("tcp/%d/tcp", port)
	case bucketUDP:
		if svc, ok := c.udpSvc[port]; ok && svc != "" {
			return svc + "/udp"
		}

		return fmt.Sprintf("udp/%d/udp", port)
	default:
		return ""
	}
}

func diffCounters(
	previous map[protoCounterKey]protoCounterValue,
	current map[protoCounterKey]protoCounterValue,
) map[protoCounterKey]protoCounterValue {
	delta := make(map[protoCounterKey]protoCounterValue, len(current))

	for key, now := range current {
		then := previous[key]

		var packets uint64
		if now.Packets >= then.Packets {
			packets = now.Packets - then.Packets
		}

		var bytesCount uint64
		if now.Bytes >= then.Bytes {
			bytesCount = now.Bytes - then.Bytes
		}

		if packets == 0 && bytesCount == 0 {
			continue
		}

		delta[key] = protoCounterValue{
			Packets: packets,
			Bytes:   bytesCount,
		}
	}

	return delta
}

func (c *trafficCollector) reconcileScopeInterfaces(ctx context.Context, scope *trafficScope) error {
	desired, ifNames, err := discoverScopeInterfaces(scope.NSPath)
	if err != nil {
		return err
	}

	scope.IfNames = ifNames

	if len(desired) == 0 {
		return fmt.Errorf("no interfaces discovered for tc/eBPF attachment in scope %q", scope.Label)
	}

	desiredSet := make(map[string]struct{}, len(desired))
	var firstAttachErr error

	for idx := range desired {
		ifName := desired[idx]
		desiredSet[ifName] = struct{}{}

		if _, attached := scope.Attached[ifName]; attached {
			continue
		}

		if err := c.attachInterface(ctx, scope, ifName); err != nil {
			if firstAttachErr == nil {
				firstAttachErr = err
			}

			log.Debugf("failed to attach tc/eBPF on %s in scope %s: %v", ifName, scope.Label, err)

			continue
		}

		scope.Attached[ifName] = struct{}{}
	}

	for ifName := range scope.Attached {
		if _, keep := desiredSet[ifName]; keep {
			continue
		}

		c.detachInterface(scope, ifName)
		delete(scope.Attached, ifName)
	}

	if len(scope.Attached) == 0 && firstAttachErr != nil {
		return firstAttachErr
	}

	return nil
}

func discoverScopeInterfaces(nsPath string) ([]string, map[int]string, error) {
	links, err := linksInNamespace(nsPath)
	if err != nil {
		return nil, nil, err
	}

	filtered := make([]string, 0, len(links))
	fallback := make([]string, 0, len(links))
	ifNames := make(map[int]string, len(links))

	for idx := range links {
		link := links[idx]
		attrs := link.Attrs()
		if attrs == nil || attrs.Name == "" || attrs.Name == "lo" {
			continue
		}

		ifNames[attrs.Index] = attrs.Name
		fallback = append(fallback, attrs.Name)
		if shouldAttachTraffic(link) {
			filtered = append(filtered, attrs.Name)
		}
	}

	sort.Strings(filtered)
	sort.Strings(fallback)

	if len(filtered) > 0 {
		return filtered, ifNames, nil
	}

	return fallback, ifNames, nil
}

func linksInNamespace(nsPath string) ([]netlink.Link, error) {
	if nsPath == "" {
		links, err := netlink.LinkList()
		if err != nil {
			return nil, fmt.Errorf("failed to list host links: %w", err)
		}

		return links, nil
	}

	nsHandle, err := netns.GetFromPath(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open namespace path %q: %w", nsPath, err)
	}
	defer nsHandle.Close()

	handle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to open netlink handle for namespace %q: %w", nsPath, err)
	}
	defer handle.Close()

	links, err := handle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links for namespace %q: %w", nsPath, err)
	}

	return links, nil
}

func shouldAttachTraffic(link netlink.Link) bool {
	attrs := link.Attrs()
	if attrs == nil {
		return false
	}

	name := attrs.Name
	if attrs.MasterIndex != 0 {
		return true
	}

	if strings.HasPrefix(name, "docker0") || strings.HasPrefix(name, "br-") ||
		strings.HasPrefix(name, "vnet") {
		return true
	}

	switch link.Type() {
	case "veth", "bridge":
		return true
	default:
		return false
	}
}

func (c *trafficCollector) attachInterface(ctx context.Context, scope *trafficScope, ifName string) error {
	return withNetNS(scope.NSPath, func() error {
		_ = exec.CommandContext(ctx, "tc", "qdisc", "add", "dev", ifName, "clsact").Run()

		_, err := runCommand(
			ctx,
			"tc",
			"filter",
			"replace",
			"dev",
			ifName,
			"ingress",
			"pref",
			trafficFilterPref,
			"protocol",
			"all",
			"bpf",
			"da",
			"pinned",
			scope.IngressPin,
		)
		if err != nil {
			return err
		}

		_, err = runCommand(
			ctx,
			"tc",
			"filter",
			"replace",
			"dev",
			ifName,
			"egress",
			"pref",
			trafficFilterPref,
			"protocol",
			"all",
			"bpf",
			"da",
			"pinned",
			scope.EgressPin,
		)

		return err
	})
}

func (c *trafficCollector) detachInterface(scope *trafficScope, ifName string) {
	_ = withNetNS(scope.NSPath, func() error {
		_ = exec.Command("tc", "filter", "del", "dev", ifName, "ingress", "pref", trafficFilterPref,
			"protocol", "all").Run()
		_ = exec.Command("tc", "filter", "del", "dev", ifName, "egress", "pref", trafficFilterPref,
			"protocol", "all").Run()

		return nil
	})
}

func withNetNS(nsPath string, fn func() error) error {
	if nsPath == "" {
		return fn()
	}

	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	currentNS, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return fmt.Errorf("failed to open current netns: %w", err)
	}
	defer currentNS.Close()

	targetNS, err := os.Open(nsPath)
	if err != nil {
		return fmt.Errorf("failed to open target netns %q: %w", nsPath, err)
	}
	defer targetNS.Close()

	if err := unix.Setns(int(targetNS.Fd()), unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("failed to switch to netns %q: %w", nsPath, err)
	}

	defer func() {
		if resetErr := unix.Setns(int(currentNS.Fd()), unix.CLONE_NEWNET); resetErr != nil {
			log.Errorf("failed to restore original netns after %q: %v", nsPath, resetErr)
		}
	}()

	return fn()
}

func (c *trafficCollector) dumpCounterMap(scope *trafficScope) (map[protoCounterKey]protoCounterValue, error) {
	if scope.Objects.ProtoCounters == nil {
		return nil, fmt.Errorf("traffic eBPF map handle is not initialized for scope %q", scope.Label)
	}

	counters := make(map[protoCounterKey]protoCounterValue)
	iter := scope.Objects.ProtoCounters.Iterate()
	var key protoCounterKey
	var value protoCounterValue
	for iter.Next(&key, &value) {
		counters[key] = value
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate traffic eBPF counters in scope %q: %w", scope.Label, err)
	}

	return counters, nil
}

func (c *trafficCollector) loadBPFPrograms(scope *trafficScope) error {
	_ = os.Remove(scope.MapPinPath)
	_ = os.Remove(scope.IngressPin)
	_ = os.Remove(scope.EgressPin)

	objData, err := trafficBPFObjectForArch()
	if err != nil {
		return err
	}

	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(objData))
	if err != nil {
		return fmt.Errorf("failed to read embedded traffic eBPF object: %w", err)
	}

	if m, ok := spec.Maps["proto_counters"]; ok {
		m.Pinning = ebpf.PinByName
	}

	var objs trafficBPFObjects
	err = spec.LoadAndAssign(&objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{PinPath: scope.BPFFSPath},
	})
	if err != nil {
		return fmt.Errorf("failed to load traffic eBPF objects for scope %q: %w", scope.Label, err)
	}

	if objs.IngressProg == nil || objs.EgressProg == nil || objs.ProtoCounters == nil {
		objsClose(&objs)
		return fmt.Errorf("loaded traffic eBPF object is missing required programs or maps for scope %q", scope.Label)
	}

	if err := objs.IngressProg.Pin(scope.IngressPin); err != nil {
		objsClose(&objs)
		return fmt.Errorf("failed to pin ingress traffic eBPF program for scope %q: %w", scope.Label, err)
	}

	if err := objs.EgressProg.Pin(scope.EgressPin); err != nil {
		objsClose(&objs)
		return fmt.Errorf("failed to pin egress traffic eBPF program for scope %q: %w", scope.Label, err)
	}

	scope.Objects = objs
	return nil
}

func objsClose(objs *trafficBPFObjects) {
	if objs == nil {
		return
	}

	if objs.IngressProg != nil {
		objs.IngressProg.Close()
	}

	if objs.EgressProg != nil {
		objs.EgressProg.Close()
	}

	if objs.ProtoCounters != nil {
		objs.ProtoCounters.Close()
	}
}

func trafficBPFObjectForArch() ([]byte, error) {
	switch goruntime.GOARCH {
	case "amd64":
		if len(trafficBPFObjectX86) == 0 {
			return nil, fmt.Errorf("embedded x86 traffic eBPF object is empty")
		}

		return trafficBPFObjectX86, nil
	case "arm64":
		if len(trafficBPFObjectARM64) == 0 {
			return nil, fmt.Errorf("embedded arm64 traffic eBPF object is empty")
		}

		return trafficBPFObjectARM64, nil
	default:
		return nil, fmt.Errorf("unsupported architecture %q for traffic eBPF object", goruntime.GOARCH)
	}
}

func ensureBPFFSMounted() error {
	if err := os.MkdirAll(bpffsMountPath, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", bpffsMountPath, err)
	}

	var statfs unix.Statfs_t
	if err := unix.Statfs(bpffsMountPath, &statfs); err != nil {
		return fmt.Errorf("failed to statfs %s: %w", bpffsMountPath, err)
	}

	if int64(statfs.Type) == bpffsFsType {
		return nil
	}

	if err := unix.Mount("bpf", bpffsMountPath, "bpf", 0, ""); err != nil &&
		!errors.Is(err, unix.EBUSY) {
		return fmt.Errorf("failed to mount bpffs on %s: %w", bpffsMountPath, err)
	}

	return nil
}

func (c *trafficCollector) cleanup() {
	scopeIDs := make([]string, 0, len(c.scopes))
	for scopeID := range c.scopes {
		scopeIDs = append(scopeIDs, scopeID)
	}
	sort.Strings(scopeIDs)

	for idx := range scopeIDs {
		scope := c.scopes[scopeIDs[idx]]
		if scope == nil {
			continue
		}

		for ifName := range scope.Attached {
			c.detachInterface(scope, ifName)
		}

		objsClose(&scope.Objects)
		_ = os.RemoveAll(scope.BPFFSPath)
	}

	_ = os.RemoveAll(trafficBPFFSRoot)
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	stderrText := strings.TrimSpace(stderr.String())
	if stderrText == "" {
		return nil, fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}

	return nil, fmt.Errorf("%s %s failed: %w (%s)", name, strings.Join(args, " "), err, stderrText)
}

func loadServiceMaps() (map[uint16]string, map[uint16]string) {
	tcp := make(map[uint16]string)
	udp := make(map[uint16]string)

	file, err := os.Open("/etc/services")
	if err != nil {
		return tcp, udp
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if hashIdx := strings.IndexByte(line, '#'); hashIdx >= 0 {
			line = line[:hashIdx]
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		portProto := strings.SplitN(fields[1], "/", 2)
		if len(portProto) != 2 {
			continue
		}

		portValue, err := strconv.ParseUint(portProto[0], 10, 16)
		if err != nil {
			continue
		}

		port := uint16(portValue)
		service := fields[0]

		switch portProto[1] {
		case "tcp":
			if _, exists := tcp[port]; !exists {
				tcp[port] = service
			}
		case "udp":
			if _, exists := udp[port]; !exists {
				udp[port] = service
			}
		}
	}

	return tcp, udp
}

func discoverTrafficScopes(containers []clabruntime.GenericContainer) ([]*trafficScope, error) {
	scopes := make([]*trafficScope, 0, len(containers)+1)
	scopes = append(scopes, newTrafficScope("host", "host", ""))

	seenInodes := make(map[uint64]struct{})

	for idx := range containers {
		container := containers[idx]
		if container.Pid <= 0 {
			continue
		}

		name := container.ShortID
		if len(container.Names) > 0 && container.Names[0] != "" {
			name = container.Names[0]
		}
		if name == "" {
			name = fmt.Sprintf("pid-%d", container.Pid)
		}

		paths := []struct {
			Path  string
			Label string
		}{
			{
				Path:  fmt.Sprintf("/proc/%d/ns/net", container.Pid),
				Label: name + "/root-netns",
			},
		}

		nestedDir := fmt.Sprintf("/proc/%d/root/var/run/netns", container.Pid)
		entries, err := os.ReadDir(nestedDir)
		if err == nil {
			for entryIdx := range entries {
				entry := entries[entryIdx]
				if entry.IsDir() {
					continue
				}

				paths = append(paths, struct {
					Path  string
					Label string
				}{
					Path:  filepath.Join(nestedDir, entry.Name()),
					Label: name + "/" + entry.Name(),
				})
			}
		}

		for pathIdx := range paths {
			path := paths[pathIdx]
			inode, err := netnsInode(path.Path)
			if err != nil {
				continue
			}

			if _, exists := seenInodes[inode]; exists {
				continue
			}
			seenInodes[inode] = struct{}{}

			scopeID := fmt.Sprintf("ns-%d", inode)
			scopes = append(scopes, newTrafficScope(scopeID, path.Label, path.Path))
		}
	}

	return scopes, nil
}

func newTrafficScope(id, label, nsPath string) *trafficScope {
	safeID := sanitizeForPath(id)
	bpffsPath := filepath.Join(trafficBPFFSRoot, safeID)

	return &trafficScope{
		ID:         id,
		Label:      label,
		NSPath:     nsPath,
		BPFFSPath:  bpffsPath,
		MapPinPath: filepath.Join(bpffsPath, "proto_counters"),
		IngressPin: filepath.Join(bpffsPath, "ingress_prog"),
		EgressPin:  filepath.Join(bpffsPath, "egress_prog"),
		Attached:   make(map[string]struct{}),
		Previous:   make(map[protoCounterKey]protoCounterValue),
		IfNames:    make(map[int]string),
		Objects:    trafficBPFObjects{},
	}
}

func sanitizeForPath(value string) string {
	if value == "" {
		return "scope"
	}

	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	result := b.String()
	if result == "" {
		return "scope"
	}

	return result
}

func netnsInode(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unexpected stat type for %q", path)
	}

	return stat.Ino, nil
}
