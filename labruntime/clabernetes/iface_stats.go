package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// interfaceSysfsMarker separates the /proc/net/dev counters from the sysfs interface
	// attributes in the output of interfaceSampleScript.
	interfaceSysfsMarker = "@@clab-sysfs"
	// interfaceSampleScript reads the interface counters and, per interface, the sysfs
	// attributes the interface state events carry, in one exec:
	// name, ifindex, mtu, address, operstate, flags and ifalias, tab separated.
	interfaceSampleScript = `cat /proc/net/dev; echo ` + interfaceSysfsMarker + `; ` +
		`for d in /sys/class/net/*; do [ -e "$d/operstate" ] || continue; ` +
		`printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${d##*/}" "$(cat "$d/ifindex")" ` +
		`"$(cat "$d/mtu")" "$(cat "$d/address")" "$(cat "$d/operstate")" ` +
		`"$(cat "$d/flags")" "$(cat "$d/ifalias")"; done 2>/dev/null`

	// interfaceSampleWorkers bounds the concurrent execs of one sampling round.
	interfaceSampleWorkers = 16

	// managementInterface is a node's management interface in its pod, the interface the
	// clabernetes planner gives the management address by default. It has the name the
	// management interface has in a container of the local runtimes.
	managementInterface = "eth0"
	// loopbackInterface is reported like the local runtimes report it.
	loopbackInterface = "lo"
	// clabernetesAliasPrefix starts the interface aliases clabernetes marks its own interfaces
	// and the lab links it wires with. They are not the topology's names, so they are not
	// reported.
	clabernetesAliasPrefix = "c9s:"
	// zeroMAC is the hardware address sysfs reports for interfaces without one (loopback).
	zeroMAC = "00:00:00:00:00:00"

	// Interface event actions and link types, as the local runtimes report them.
	interfaceActionSnapshot = "snapshot"
	interfaceActionCreate   = "create"
	interfaceActionUpdate   = "update"
	interfaceActionDelete   = "delete"
	interfaceActionStats    = "stats"
	interfaceTypeVeth       = "veth"
	interfaceTypeDevice     = "device"

	// shell exit codes of a command that cannot be found or executed.
	exitCommandNotExecutable = 126
	exitCommandNotFound      = 127
)

type interfacePollOptions struct {
	interval            time.Duration
	includeStats        bool
	includeInitialState bool
}

type c9sIfaceStats struct {
	Name      string
	RxBytes   uint64
	RxPackets uint64
	TxBytes   uint64
	TxPackets uint64
}

type c9sIfaceStatsSample struct {
	Stats     c9sIfaceStats
	Timestamp time.Time
}

// c9sIfaceState is the sysfs view of one interface in a device container.
type c9sIfaceState struct {
	Name      string
	Index     int
	MTU       int
	MAC       string
	OperState string
	Flags     uint64
	Alias     string
}

// trackedNode remembers the interfaces last reported for a node, so their changes and their
// removal can be reported, also when the node stops running.
type trackedNode struct {
	lab    *clablabruntime.LabState
	node   clablabruntime.NodeState
	ifaces map[string]clablabruntime.InterfaceEventData
}

type interfaceSampleTarget struct {
	state *clablabruntime.LabState
	node  clablabruntime.NodeState
	pod   *corev1.Pod
}

type interfaceSample struct {
	target    interfaceSampleTarget
	stats     []c9sIfaceStats
	ifaces    []c9sIfaceState
	hasStates bool
	// links are the node's lab link interfaces as clabwire lists them.
	links     []clabwireInterface
	timestamp time.Time
	err       error
}

// podInterfaceListing is the clabwire interface listing of a pod, per node. It is listed again
// when the pod's interfaces change.
type podInterfaceListing struct {
	names  string
	byNode map[string][]clabwireInterface
}

// interfacePoller samples the interfaces of every running node in an event scope and turns the
// samples into interface events. It reports the interfaces a container of the local runtimes
// has for the same node: the node's lab link interfaces, with the names and aliases clabwire
// lists for them, the management interface and loopback. The pod's own interfaces (the pod
// network, clabernetes' interposition and link transport interfaces) are not reported.
type interfacePoller struct {
	r     *Runtime
	scope eventScope
	opts  interfacePollOptions
	sink  chan<- clablabruntime.Event

	statsSamples map[string]c9sIfaceStatsSample
	nodes        map[string]*trackedNode

	noShellMu sync.Mutex
	// noShell holds the pods whose device container has no shell; they are sampled with a
	// plain cat of /proc/net/dev and report statistics without interface state.
	noShell map[string]bool

	listingsMu sync.Mutex
	// listings holds the clabwire interface listing per pod (namespace/pod).
	listings map[string]*podInterfaceListing

	firstRound bool
}

func (r *Runtime) pollInterfaces(
	ctx context.Context,
	scope eventScope,
	opts interfacePollOptions,
	eventSink chan<- clablabruntime.Event,
) {
	if opts.interval <= 0 {
		opts.interval = time.Second
	}

	poller := newInterfacePoller(r, scope, opts, eventSink)
	poller.round(ctx)

	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poller.round(ctx)
		}
	}
}

func newInterfacePoller(
	r *Runtime,
	scope eventScope,
	opts interfacePollOptions,
	eventSink chan<- clablabruntime.Event,
) *interfacePoller {
	return &interfacePoller{
		r:            r,
		scope:        scope,
		opts:         opts,
		sink:         eventSink,
		statsSamples: map[string]c9sIfaceStatsSample{},
		nodes:        map[string]*trackedNode{},
		noShell:      map[string]bool{},
		listings:     map[string]*podInterfaceListing{},
		firstRound:   true,
	}
}

func (p *interfacePoller) round(ctx context.Context) {
	defer func() { p.firstRound = false }()

	states, err := p.r.labStates(ctx, p.scope)
	if err != nil {
		log.Debug("failed to list C9s labs for interface events", "error", err)
		return
	}

	running := map[string]bool{}
	pods := map[string]bool{}
	var targets []interfaceSampleTarget
	for _, state := range states {
		for _, node := range state.Nodes {
			if node.ContainerState != clablabruntime.ContainerStateRunning || node.Pod == "" ||
				node.Container == "" {
				continue
			}

			running[c9sNodeKey(state.Namespace, state.Name, node.Name)] = true
			pods[state.Namespace+"/"+node.Pod] = true
			targets = append(targets, interfaceSampleTarget{
				state: state,
				node:  node,
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name:      node.Pod,
					Namespace: state.Namespace,
				}},
			})
		}
	}

	for _, sample := range p.sampleAll(ctx, targets) {
		if ctx.Err() != nil {
			return
		}
		if sample.err != nil {
			log.Debug("failed to sample C9s interfaces",
				"namespace", sample.target.state.Namespace,
				"lab", sample.target.state.Name,
				"node", sample.target.node.Name,
				"error", sample.err,
			)

			continue
		}

		p.processSample(ctx, sample)
	}

	p.retireStoppedNodes(ctx, running)
	p.forgetListings(pods)
}

// retireStoppedNodes reports the interfaces of nodes that stopped running, or were removed, as
// deleted: they are gone with the node's pod.
func (p *interfacePoller) retireStoppedNodes(ctx context.Context, running map[string]bool) {
	for key, tracked := range p.nodes {
		if running[key] {
			continue
		}

		for _, name := range sortedInterfaceNames(tracked.ifaces) {
			p.emit(ctx, tracked, interfaceActionDelete, tracked.ifaces[name], time.Now())
			p.forgetStats(tracked, name)
		}
		delete(p.nodes, key)
	}
}

// forgetListings drops the interface listings of pods that are gone.
func (p *interfacePoller) forgetListings(pods map[string]bool) {
	p.listingsMu.Lock()
	defer p.listingsMu.Unlock()

	for key := range p.listings {
		if !pods[key] {
			delete(p.listings, key)
		}
	}
}

func (p *interfacePoller) sampleAll(
	ctx context.Context,
	targets []interfaceSampleTarget,
) []interfaceSample {
	samples := make([]interfaceSample, len(targets))

	var wg sync.WaitGroup
	sem := make(chan struct{}, interfaceSampleWorkers)
	for idx := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			samples[idx] = p.sample(ctx, targets[idx])
		}(idx)
	}
	wg.Wait()

	return samples
}

func (p *interfacePoller) sample(ctx context.Context, target interfaceSampleTarget) interfaceSample {
	sample := interfaceSample{target: target}
	podName := target.pod.Name

	p.noShellMu.Lock()
	noShell := p.noShell[podName]
	p.noShellMu.Unlock()

	command := []string{"sh", "-c", interfaceSampleScript}
	if noShell {
		command = []string{"cat", "/proc/net/dev"}
	}

	stdout, stderr, rc, err := p.r.execInPod(ctx, target.pod, target.node.Container, command)
	if !noShell && commandNotExecutable(rc, err) {
		p.noShellMu.Lock()
		p.noShell[podName] = true
		p.noShellMu.Unlock()

		log.Debug("C9s device container has no shell; reporting interface statistics only",
			"namespace", target.state.Namespace,
			"lab", target.state.Name,
			"node", target.node.Name,
		)

		stdout, stderr, rc, err = p.r.execInPod(ctx, target.pod, target.node.Container,
			[]string{"cat", "/proc/net/dev"})
	}

	sample.timestamp = time.Now()
	switch {
	case err != nil:
		sample.err = err
	case rc != 0:
		sample.err = fmt.Errorf("exit code %d: %s", rc, strings.TrimSpace(string(stderr)))
	default:
		sample.stats, sample.ifaces, sample.hasStates, sample.err = parseInterfaceSample(stdout)
	}
	if sample.err != nil {
		return sample
	}

	sample.links, sample.err = p.nodeLinks(ctx, target, sampledInterfaceNames(sample))

	return sample
}

// sampledInterfaceNames lists the interface names a sample saw, sorted and comma-joined.
func sampledInterfaceNames(sample interfaceSample) string {
	names := make([]string, 0, len(sample.stats)+len(sample.ifaces))
	seen := map[string]bool{}
	for _, iface := range sample.ifaces {
		if !seen[iface.Name] {
			seen[iface.Name] = true
			names = append(names, iface.Name)
		}
	}
	for _, stat := range sample.stats {
		if !seen[stat.Name] {
			seen[stat.Name] = true
			names = append(names, stat.Name)
		}
	}
	sort.Strings(names)

	return strings.Join(names, ",")
}

// nodeLinks returns the lab link interfaces of a node from its pod's clabwire listing. A pod is
// listed once, and again when its interfaces change (a pod gets its link interfaces as clabwire
// wires them).
func (p *interfacePoller) nodeLinks(
	ctx context.Context,
	target interfaceSampleTarget,
	names string,
) ([]clabwireInterface, error) {
	key := target.pod.Namespace + "/" + target.pod.Name

	p.listingsMu.Lock()
	listing := p.listings[key]
	p.listingsMu.Unlock()

	if listing == nil || listing.names != names {
		entries, err := p.r.listPodInterfaces(ctx, target.pod.Namespace, target.pod.Name)
		if err != nil {
			return nil, err
		}

		listing = &podInterfaceListing{names: names, byNode: map[string][]clabwireInterface{}}
		for _, entry := range entries {
			listing.byNode[entry.Node] = append(listing.byNode[entry.Node], entry)
		}

		p.listingsMu.Lock()
		p.listings[key] = listing
		p.listingsMu.Unlock()
	}

	return listing.byNode[target.node.Name], nil
}

// commandNotExecutable reports an exec that failed because the command does not exist in the
// container.
func commandNotExecutable(rc int, err error) bool {
	if rc == exitCommandNotExecutable || rc == exitCommandNotFound {
		return true
	}

	return err != nil && strings.Contains(err.Error(), "executable file not found")
}

// reportedInterfaces are the interfaces reported for a node, by name, with the alias the
// topology gave them: its lab link interfaces, the management interface and loopback.
func reportedInterfaces(links []clabwireInterface) map[string]string {
	reported := map[string]string{
		loopbackInterface:   "",
		managementInterface: "",
	}
	for _, link := range links {
		reported[link.Interface] = link.Alias
	}

	return reported
}

// interfaceEventData describes an interface the way the local runtimes' netlink events do: the
// alias is the topology's name for the interface (an alias clabernetes set for its own use is
// not reported), the operational state uses netlink's names, lab link and management
// interfaces are veth pairs, loopback a device, and loopback has no hardware address.
func interfaceEventData(iface c9sIfaceState, alias string) clablabruntime.InterfaceEventData {
	if alias == "" && !strings.HasPrefix(iface.Alias, clabernetesAliasPrefix) {
		alias = iface.Alias
	}

	mac := iface.MAC
	if mac == zeroMAC {
		mac = ""
	}

	linkType := interfaceTypeVeth
	if iface.Name == loopbackInterface {
		linkType = interfaceTypeDevice
	}

	return clablabruntime.InterfaceEventData{
		Name:      iface.Name,
		Alias:     alias,
		Index:     iface.Index,
		MTU:       iface.MTU,
		MAC:       mac,
		OperState: netlinkOperState(iface.OperState),
		Type:      linkType,
	}
}

// netlinkOperState names an operational state from sysfs (RFC 2863, as in
// /sys/class/net/<if>/operstate) the way netlink, and so the local runtimes' events, name it.
func netlinkOperState(operState string) string {
	switch operState {
	case "notpresent":
		return "not-present"
	case "lowerlayerdown":
		return "lower-layer-down"
	case "":
		return "unknown"
	default:
		return operState
	}
}

func (p *interfacePoller) processSample(ctx context.Context, sample interfaceSample) {
	state := sample.target.state
	node := sample.target.node
	key := c9sNodeKey(state.Namespace, state.Name, node.Name)

	tracked, known := p.nodes[key]
	if !known {
		tracked = &trackedNode{ifaces: map[string]clablabruntime.InterfaceEventData{}}
		p.nodes[key] = tracked
	}
	tracked.lab = state
	tracked.node = node

	reported := reportedInterfaces(sample.links)

	if sample.hasStates {
		p.processStates(ctx, tracked, reported, sample)
	}

	if !p.opts.includeStats {
		return
	}
	for _, stat := range sample.stats {
		alias, ok := reported[stat.Name]
		if !ok {
			continue
		}

		statsKey := c9sIfaceStatsKey(state.Namespace, state.Name, node.Name, stat.Name)
		current := c9sIfaceStatsSample{Stats: stat, Timestamp: sample.timestamp}
		if previous, ok := p.statsSamples[statsKey]; ok {
			data, ok := tracked.ifaces[stat.Name]
			if !ok {
				data = interfaceEventData(c9sIfaceState{Name: stat.Name}, alias)
				data.OperState = ""
			}
			data.Stats = c9sIfaceStatsData(previous, current)
			p.emit(ctx, tracked, interfaceActionStats, data, current.Timestamp)
		}

		p.statsSamples[statsKey] = current
	}
}

// processStates reports the interfaces that appeared (snapshot in the first round with
// --initial-state, create later), changed (update) or disappeared (delete) since the last
// sample, like the netlink events of the local runtimes.
func (p *interfacePoller) processStates(
	ctx context.Context,
	tracked *trackedNode,
	reported map[string]string,
	sample interfaceSample,
) {
	present := map[string]bool{}
	for _, iface := range sample.ifaces {
		alias, ok := reported[iface.Name]
		if !ok {
			continue
		}

		data := interfaceEventData(iface, alias)
		present[iface.Name] = true
		previous, seen := tracked.ifaces[iface.Name]
		tracked.ifaces[iface.Name] = data

		switch {
		case seen && previous == data:
		case !seen && p.firstRound:
			// Interfaces that exist when the stream starts are only reported with
			// --initial-state; later changes are always reported.
			if p.opts.includeInitialState {
				p.emit(ctx, tracked, interfaceActionSnapshot, data, sample.timestamp)
			}
		case !seen:
			p.emit(ctx, tracked, interfaceActionCreate, data, sample.timestamp)
		default:
			p.emit(ctx, tracked, interfaceActionUpdate, data, sample.timestamp)
		}
	}

	for _, name := range sortedInterfaceNames(tracked.ifaces) {
		if present[name] {
			continue
		}

		p.emit(ctx, tracked, interfaceActionDelete, tracked.ifaces[name], sample.timestamp)
		p.forgetStats(tracked, name)
		delete(tracked.ifaces, name)
	}
}

func sortedInterfaceNames(ifaces map[string]clablabruntime.InterfaceEventData) []string {
	names := make([]string, 0, len(ifaces))
	for name := range ifaces {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (p *interfacePoller) forgetStats(tracked *trackedNode, ifName string) {
	delete(p.statsSamples,
		c9sIfaceStatsKey(tracked.lab.Namespace, tracked.lab.Name, tracked.node.Name, ifName))
}

func (p *interfacePoller) emit(
	ctx context.Context,
	tracked *trackedNode,
	action string,
	data clablabruntime.InterfaceEventData,
	timestamp time.Time,
) {
	p.r.sendEvent(ctx, p.sink, c9sInterfaceEvent(tracked, action, data, timestamp))
}

// c9sInterfaceEvent is an interface event of a node. The actor is the node, as for its container
// events. The interface itself is in Interface; the events command reports it with the
// attributes of the local runtimes' interface events.
func c9sInterfaceEvent(
	tracked *trackedNode,
	action string,
	data clablabruntime.InterfaceEventData,
	timestamp time.Time,
) clablabruntime.Event {
	key := c9sNodeKey(tracked.lab.Namespace, tracked.lab.Name, tracked.node.Name)

	attributes := map[string]string{
		"namespace": tracked.lab.Namespace,
		"lab":       tracked.lab.Name,
		"node":      tracked.node.Name,
		"origin":    clabernetesAppValue,
	}
	if tracked.node.Pod != "" {
		attributes["pod"] = tracked.node.Pod
	}

	node := tracked.node

	return clablabruntime.Event{
		Timestamp:   timestamp,
		Type:        clablabruntime.EventTypeInterface,
		Action:      action,
		ActorID:     key,
		ActorName:   fmt.Sprintf("%s-%s", tracked.lab.Name, tracked.node.Name),
		ActorFullID: key,
		Attributes:  attributes,
		Lab:         tracked.lab,
		Node:        &node,
		Interface:   &data,
	}
}

// c9sIfaceStatsData computes the counters and rates of a stats event from two samples.
func c9sIfaceStatsData(previous, current c9sIfaceStatsSample) *clablabruntime.InterfaceStats {
	interval := current.Timestamp.Sub(previous.Timestamp)
	if interval <= 0 {
		interval = time.Second
	}

	seconds := interval.Seconds()
	stat := current.Stats

	return &clablabruntime.InterfaceStats{
		RxBytes:   stat.RxBytes,
		TxBytes:   stat.TxBytes,
		RxPackets: stat.RxPackets,
		TxPackets: stat.TxPackets,
		RxBps:     float64(counterDelta(stat.RxBytes, previous.Stats.RxBytes)*8) / seconds,
		TxBps:     float64(counterDelta(stat.TxBytes, previous.Stats.TxBytes)*8) / seconds,
		RxPps:     float64(counterDelta(stat.RxPackets, previous.Stats.RxPackets)) / seconds,
		TxPps:     float64(counterDelta(stat.TxPackets, previous.Stats.TxPackets)) / seconds,
		Interval:  interval,
	}
}

// parseInterfaceSample splits the output of interfaceSampleScript into the /proc/net/dev
// counters and the sysfs interface states. hasStates is false for output without the sysfs
// section, such as a plain cat of /proc/net/dev.
func parseInterfaceSample(data []byte) ([]c9sIfaceStats, []c9sIfaceState, bool, error) {
	text := string(data)

	procPart, sysfsPart, hasStates := strings.Cut(text, interfaceSysfsMarker+"\n")
	if !hasStates {
		procPart, _, hasStates = strings.Cut(text, interfaceSysfsMarker)
	}

	stats, err := parseProcNetDev([]byte(procPart))
	if err != nil {
		return nil, nil, false, err
	}
	if !hasStates {
		return stats, nil, false, nil
	}

	return stats, parseSysfsInterfaces(sysfsPart), true, nil
}

func parseSysfsInterfaces(data string) []c9sIfaceState {
	lines := strings.Split(data, "\n")
	states := make([]c9sIfaceState, 0, len(lines))

	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 7) //nolint:mnd
		if len(fields) < 5 || strings.TrimSpace(fields[0]) == "" {
			continue
		}

		state := c9sIfaceState{
			Name:      strings.TrimSpace(fields[0]),
			MAC:       strings.TrimSpace(fields[3]),
			OperState: strings.ToLower(strings.TrimSpace(fields[4])),
		}
		state.Index, _ = strconv.Atoi(strings.TrimSpace(fields[1]))
		state.MTU, _ = strconv.Atoi(strings.TrimSpace(fields[2]))
		if len(fields) > 5 {
			state.Flags, _ = strconv.ParseUint(
				strings.TrimPrefix(strings.TrimSpace(fields[5]), "0x"), 16, 64)
		}
		if len(fields) > 6 {
			state.Alias = strings.TrimSpace(fields[6])
		}
		if state.OperState == "" {
			state.OperState = "unknown"
		}

		states = append(states, state)
	}

	return states
}

func parseProcNetDev(data []byte) ([]c9sIfaceStats, error) {
	lines := strings.Split(string(data), "\n")
	stats := make([]c9sIfaceStats, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		ifName := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			return nil, fmt.Errorf("unexpected /proc/net/dev line for %q: %q", ifName, line)
		}

		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rx bytes for %q: %w", ifName, err)
		}
		rxPackets, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rx packets for %q: %w", ifName, err)
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx bytes for %q: %w", ifName, err)
		}
		txPackets, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx packets for %q: %w", ifName, err)
		}

		stats = append(stats, c9sIfaceStats{
			Name:      ifName,
			RxBytes:   rxBytes,
			RxPackets: rxPackets,
			TxBytes:   txBytes,
			TxPackets: txPackets,
		})
	}

	return stats, nil
}

// c9sNodeKey is the actor ID of a node: <namespace>/<lab>/<node>, the same value inspect
// reports as container_id.
func c9sNodeKey(namespace, lab, node string) string {
	return namespace + "/" + lab + "/" + node
}

func c9sIfaceStatsKey(namespace, lab, node, ifName string) string {
	return c9sNodeKey(namespace, lab, node) + "/" + ifName
}

func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}

	return current - previous
}
