package clabernetes

import (
	"context"
	"fmt"
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

	// interfaceStateNotPresent is the operational state reported for an interface that
	// disappeared, or whose node stopped running (RFC 2863 "notPresent").
	interfaceStateNotPresent = "notpresent"

	// iffUp is the IFF_UP bit of /sys/class/net/<if>/flags.
	iffUp = 0x1

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

// action maps the interface state onto the up/down action of interface state events. An
// "unknown" operational state (loopback, tunnels) counts as up when the interface is
// administratively up.
func (s c9sIfaceState) action() string {
	if s.OperState == "up" || (s.OperState == "unknown" && s.Flags&iffUp != 0) {
		return "up"
	}

	return "down"
}

// trackedIface is the last reported state of an interface.
type trackedIface struct {
	state  c9sIfaceState
	action string
}

// trackedNode remembers the interfaces last reported for a node, so their removal can be
// reported when the node stops running.
type trackedNode struct {
	namespace string
	lab       string
	node      string
	pod       string
	ifaces    map[string]trackedIface
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
	timestamp time.Time
	err       error
}

// interfacePoller samples the interfaces of every running node in an event scope and turns the
// samples into interface state and statistics events.
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

	poller := &interfacePoller{
		r:            r,
		scope:        scope,
		opts:         opts,
		sink:         eventSink,
		statsSamples: map[string]c9sIfaceStatsSample{},
		nodes:        map[string]*trackedNode{},
		noShell:      map[string]bool{},
		firstRound:   true,
	}

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

func (p *interfacePoller) round(ctx context.Context) {
	defer func() { p.firstRound = false }()

	states, err := p.r.labStates(ctx, p.scope)
	if err != nil {
		log.Debug("failed to list C9s labs for interface events", "error", err)
		return
	}

	running := map[string]bool{}
	var targets []interfaceSampleTarget
	for _, state := range states {
		for _, node := range state.Nodes {
			if !node.Ready || node.Pod == "" || node.Container == "" {
				continue
			}

			running[c9sNodeKey(state.Namespace, state.Name, node.Name)] = true
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
}

// retireStoppedNodes reports the interfaces of nodes that stopped running, or were removed, as
// down: they are gone with the node's pod.
func (p *interfacePoller) retireStoppedNodes(ctx context.Context, running map[string]bool) {
	for key, tracked := range p.nodes {
		if running[key] {
			continue
		}

		for name, iface := range tracked.ifaces {
			iface.state.OperState = interfaceStateNotPresent
			p.emitState(ctx, tracked, iface.state, time.Now())
			p.forgetStats(tracked, name)
		}
		delete(p.nodes, key)
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

	return sample
}

// commandNotExecutable reports an exec that failed because the command does not exist in the
// container.
func commandNotExecutable(rc int, err error) bool {
	if rc == exitCommandNotExecutable || rc == exitCommandNotFound {
		return true
	}

	return err != nil && strings.Contains(err.Error(), "executable file not found")
}

func (p *interfacePoller) processSample(ctx context.Context, sample interfaceSample) {
	state := sample.target.state
	node := sample.target.node
	key := c9sNodeKey(state.Namespace, state.Name, node.Name)

	tracked, known := p.nodes[key]
	if !known {
		tracked = &trackedNode{
			namespace: state.Namespace,
			lab:       state.Name,
			node:      node.Name,
			ifaces:    map[string]trackedIface{},
		}
		p.nodes[key] = tracked
	}
	tracked.pod = node.Pod

	if p.opts.includeStats {
		for _, stat := range sample.stats {
			statsKey := c9sIfaceStatsKey(state.Namespace, state.Name, node.Name, stat.Name)
			current := c9sIfaceStatsSample{Stats: stat, Timestamp: sample.timestamp}

			if previous, ok := p.statsSamples[statsKey]; ok {
				p.r.sendEvent(ctx, p.sink,
					c9sIfaceStatsEvent(state, node, sample.target.pod, stat, previous, current))
			}

			p.statsSamples[statsKey] = current
		}
	}

	if !sample.hasStates {
		return
	}

	present := make(map[string]bool, len(sample.ifaces))
	for _, iface := range sample.ifaces {
		present[iface.Name] = true

		action := iface.action()
		previous, seen := tracked.ifaces[iface.Name]
		tracked.ifaces[iface.Name] = trackedIface{state: iface, action: action}

		switch {
		case seen && previous.action == action && previous.state.OperState == iface.OperState:
			continue
		case !seen && p.firstRound && !p.opts.includeInitialState:
			// Interfaces that exist when the stream starts are only reported with
			// --initial-state; later changes are always reported.
			continue
		}

		p.emitState(ctx, tracked, iface, sample.timestamp)
	}

	for name, iface := range tracked.ifaces {
		if present[name] {
			continue
		}

		iface.state.OperState = interfaceStateNotPresent
		p.emitState(ctx, tracked, iface.state, sample.timestamp)
		p.forgetStats(tracked, name)
		delete(tracked.ifaces, name)
	}
}

func (p *interfacePoller) forgetStats(tracked *trackedNode, ifName string) {
	delete(p.statsSamples, c9sIfaceStatsKey(tracked.namespace, tracked.lab, tracked.node, ifName))
}

func (p *interfacePoller) emitState(
	ctx context.Context,
	tracked *trackedNode,
	iface c9sIfaceState,
	timestamp time.Time,
) {
	p.r.sendEvent(ctx, p.sink, c9sIfaceStateEvent(tracked, iface, timestamp))
}

// c9sIfaceStateEvent reports the operational state of an interface. The actor is the node, as
// for the container events of the lab and the interface events of the local runtimes.
func c9sIfaceStateEvent(
	tracked *trackedNode,
	iface c9sIfaceState,
	timestamp time.Time,
) clablabruntime.Event {
	actorName := fmt.Sprintf("%s-%s", tracked.lab, tracked.node)

	attributes := map[string]string{
		"namespace": tracked.namespace,
		"lab":       tracked.lab,
		"node":      tracked.node,
		"name":      actorName,
		"ifname":    iface.Name,
		"state":     iface.OperState,
		"origin":    "clabernetes",
	}
	if tracked.pod != "" {
		attributes["pod"] = tracked.pod
	}
	if iface.Index > 0 {
		attributes["index"] = strconv.Itoa(iface.Index)
	}
	if iface.MTU > 0 {
		attributes["mtu"] = strconv.Itoa(iface.MTU)
	}
	if iface.MAC != "" {
		attributes["mac"] = iface.MAC
	}
	if iface.Alias != "" {
		attributes["alias"] = iface.Alias
	}

	action := iface.action()
	if iface.OperState == interfaceStateNotPresent {
		action = "down"
	}

	return clablabruntime.Event{
		Timestamp:   timestamp,
		Type:        "interface",
		Action:      action,
		ActorID:     c9sNodeKey(tracked.namespace, tracked.lab, tracked.node),
		ActorName:   actorName,
		ActorFullID: tracked.pod,
		Attributes:  attributes,
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

func c9sIfaceStatsEvent(
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	pod *corev1.Pod,
	stat c9sIfaceStats,
	previous,
	current c9sIfaceStatsSample,
) clablabruntime.Event {
	interval := current.Timestamp.Sub(previous.Timestamp)
	if interval <= 0 {
		interval = time.Second
	}

	seconds := interval.Seconds()
	rxBytesDelta := counterDelta(stat.RxBytes, previous.Stats.RxBytes)
	txBytesDelta := counterDelta(stat.TxBytes, previous.Stats.TxBytes)
	rxPacketsDelta := counterDelta(stat.RxPackets, previous.Stats.RxPackets)
	txPacketsDelta := counterDelta(stat.TxPackets, previous.Stats.TxPackets)

	actorName := fmt.Sprintf("%s-%s", state.Name, node.Name)
	podName := ""
	if pod != nil {
		podName = pod.Name
	}

	return clablabruntime.Event{
		Timestamp:   current.Timestamp,
		Type:        "interface",
		Action:      "stats",
		ActorID:     c9sNodeKey(state.Namespace, state.Name, node.Name),
		ActorName:   actorName,
		ActorFullID: podName,
		Attributes: map[string]string{
			"namespace":        state.Namespace,
			"lab":              state.Name,
			"node":             node.Name,
			"name":             actorName,
			"pod":              podName,
			"ifname":           stat.Name,
			"origin":           "clabernetes",
			"rx_bytes":         strconv.FormatUint(stat.RxBytes, 10),
			"tx_bytes":         strconv.FormatUint(stat.TxBytes, 10),
			"rx_packets":       strconv.FormatUint(stat.RxPackets, 10),
			"tx_packets":       strconv.FormatUint(stat.TxPackets, 10),
			"rx_bps":           strconv.FormatFloat(float64(rxBytesDelta*8)/seconds, 'f', -1, 64),
			"tx_bps":           strconv.FormatFloat(float64(txBytesDelta*8)/seconds, 'f', -1, 64),
			"rx_pps":           strconv.FormatFloat(float64(rxPacketsDelta)/seconds, 'f', -1, 64),
			"tx_pps":           strconv.FormatFloat(float64(txPacketsDelta)/seconds, 'f', -1, 64),
			"interval_seconds": strconv.FormatFloat(seconds, 'f', -1, 64),
		},
	}
}

func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}

	return current - previous
}
