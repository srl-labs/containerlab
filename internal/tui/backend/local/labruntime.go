// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// This file holds what the backend does differently for the labs of a lab runtime, which
// run in a Kubernetes cluster. Whatever is read from this host for local labs is asked from
// the runtime here.

const (
	// labNamespacePrefix starts the name of the namespace a lab gets when none is chosen
	// for it: c9s-<lab>. The commands find a lab there on their own.
	labNamespacePrefix = "c9s-"
	flagNamespace      = "--namespace"

	// labRuntimeReadTimeout bounds the requests that read from the cluster. The timeout of
	// the commands is meant for deployments and far too long to wait for a list.
	labRuntimeReadTimeout = 30 * time.Second
	// labRuntimeStatsInterval is the pace the runtime reads the interface counters at.
	// Every reading is a command run in every node, so it is not done every second.
	labRuntimeStatsInterval = 2 * time.Second
	// labRuntimeUsageInterval is the pace the resource usage is asked for. The metrics of a
	// cluster are not refreshed more often than that.
	labRuntimeUsageInterval = 10 * time.Second
	// labRuntimeUsageRetry is the pace a cluster without metrics is asked again at.
	labRuntimeUsageRetry = time.Minute
	// labRuntimeUsageDelay is how long the first reading waits for the UI to come up.
	labRuntimeUsageDelay = time.Second
	// labRuntimeRollbackGrace is how long a deployment that was stopped gets to remove
	// what it created in the cluster before stronger signals are sent.
	labRuntimeRollbackGrace = 2 * time.Minute

	// milliCoresPerPercent converts thousandths of a core to the percentage of a core.
	milliCoresPerPercent = 10

	hostEndpoint = "host"
)

// silenceClusterClient keeps the Kubernetes client from writing to the terminal, which
// belongs to the UI: its own log and the warnings of the API server would be written
// across the screen.
func silenceClusterClient() {
	klog.SetLogger(logr.Discard())
	rest.SetDefaultWarningHandler(rest.NoWarnings{})
}

// labRuntimeName returns the name of the lab runtime as the commands take it.
func labRuntimeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return clablabruntime.ClabernetesRuntimeName
	}

	return name
}

// listed names what the inventory lists.
func (b *Backend) listed() string {
	if b.lrt != nil {
		return "labs of the " + b.rtName + " runtime"
	}

	return "lab containers"
}

// readTimeout returns the timeout of a request that reads from the runtime.
func (b *Backend) readTimeout() time.Duration {
	if b.lrt != nil {
		return min(b.opts.Timeout, labRuntimeReadTimeout)
	}

	return b.opts.Timeout
}

// namespaceArgs returns the arguments that make a command work in the namespace of a lab.
// There are none for a lab in the namespace that is derived from its name, where the
// commands look on their own and which they also create and remove. rec is nil for a lab
// that is not deployed.
func (b *Backend) namespaceArgs(rec *labRecord) []string {
	switch {
	case b.lrt == nil:
		return nil
	case b.opts.Namespace != "":
		return []string{flagNamespace, b.opts.Namespace}
	case rec != nil && rec.namespace != "" && rec.namespace != labNamespacePrefix+rec.name:
		return []string{flagNamespace, rec.namespace}
	default:
		return nil
	}
}

// inNamespace adds the namespace arguments of a lab to the arguments of a command.
func (b *Backend) inNamespace(rec *labRecord, args []string) []string {
	return append(args, b.namespaceArgs(rec)...)
}

// labName returns the name the runtime knows a lab by, which is what the commands take.
func labName(rec *labRecord, listed string) string {
	if rec != nil && rec.name != "" {
		return rec.name
	}

	return listed
}

// streamLabRuntime follows the events of the lab runtime until ctx ends. They tell when
// the inventory changed and carry the state and the counters of the interfaces. A stream
// that ends is opened again, after a growing pause.
func (b *Backend) streamLabRuntime(ctx context.Context) {
	pause := newBackoff(streamBackoffFirst, streamBackoffLimit)

	for ctx.Err() == nil {
		opened := time.Now()
		err := b.runLabRuntimeStream(ctx)

		// Without the stream the states go stale, and a new stream reports them all again.
		forgotStates := b.ifaces.reset()
		forgotCounters := b.stats.forgetInterfaces()

		if forgotStates || forgotCounters {
			b.mark(clabinternaltuibackend.TopicInterfaces | clabinternaltuibackend.TopicStats)
		}

		if ctx.Err() != nil {
			return
		}

		if time.Since(opened) > streamHealthy {
			pause.reset()
		}

		wait := pause.next()
		reason := fmt.Sprintf("the event stream of the %s runtime ended, reconnecting in %s: %v",
			b.rtName, wait, err)

		b.setLinkState(false, reason)
		b.logf(levelWarn, "%s", reason)

		if !sleep(ctx, wait) {
			return
		}
	}
}

// runLabRuntimeStream reads the events of the lab runtime until the stream fails and
// returns why it did.
func (b *Backend) runLabRuntimeStream(ctx context.Context) error {
	// The runtime keeps watching for as long as the context lives.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, errs, err := b.lrt.StreamEvents(ctx, clablabruntime.EventStreamRequest{
		AllNamespaces:         true,
		IncludeInitialState:   true,
		IncludeInterfaceStats: true,
		StatsInterval:         labRuntimeStatsInterval,
	})
	if err != nil {
		return err
	}

	b.setLinkState(true, "")
	// Whatever happened while no stream was open went unnoticed.
	b.requestReload()

	// The counters of the interfaces arrive one event each. The traffic of a node is
	// summed up once per reading of the runtime.
	rollup := time.NewTicker(labRuntimeStatsInterval)
	defer rollup.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return errors.New("stream closed")
			}

			b.applyLabRuntimeEvent(&ev)
		case err, ok := <-errs:
			if !ok {
				return errors.New("stream closed")
			}

			if err != nil {
				return err
			}
		case <-rollup.C:
			if b.stats.rollup() {
				b.mark(clabinternaltuibackend.TopicStats)
			}
		}
	}
}

// applyLabRuntimeEvent takes what an event of the lab runtime tells. An interface event
// updates the state or the counters of the interface, any other event stands for a change
// of the inventory, which is listed anew.
func (b *Backend) applyLabRuntimeEvent(ev *clablabruntime.Event) {
	if ev.Type != clablabruntime.EventTypeInterface {
		b.requestReload()

		return
	}

	key, iface := ev.ActorFullID, ev.Interface
	if key == "" || iface == nil || iface.Name == "" {
		return
	}

	if ev.Action == ifaceActionStats {
		if iface.Stats != nil && b.stats.setInterface(key, iface.Name, iface.Stats) {
			b.mark(clabinternaltuibackend.TopicInterfaces)
		}

		return
	}

	changed := b.ifaces.applyInterface(key, ev.Action, &ifaceState{
		name:  iface.Name,
		alias: iface.Alias,
		mac:   iface.MAC,
		kind:  iface.Type,
		mtu:   iface.MTU,
		index: iface.Index,
		state: operState(iface.OperState),
	})

	if ev.Action == ifaceActionDelete {
		changed = b.stats.forgetInterface(key, iface.Name) || changed
	}

	if changed {
		b.mark(clabinternaltuibackend.TopicInterfaces)
	}
}

// setInterface stores the counters and rates a lab runtime reported for an interface of a
// node. It reports whether the interface is a new one.
func (s *sampler) setInterface(key, name string, stats *clablabruntime.InterfaceStats) bool {
	if name == loopbackInterface {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.remoteNode(key)
	iface, known := node.ifaces[name]

	if !known {
		iface = &ifaceSample{}
		node.ifaces[name] = iface
	}

	iface.counters = netCounters{
		rxBytes:   stats.RxBytes,
		txBytes:   stats.TxBytes,
		rxPackets: stats.RxPackets,
		txPackets: stats.TxPackets,
	}
	iface.rxBps, iface.txBps = stats.RxBps, stats.TxBps
	iface.rxPps, iface.txPps = stats.RxPps, stats.TxPps
	iface.hasRates = true
	iface.rxSeries = pushSample(iface.rxSeries, iface.rxBps)
	iface.txSeries = pushSample(iface.txSeries, iface.txBps)
	node.fresh = true

	return !known
}

// remoteNode returns the sample of a node that a lab runtime runs, creating it when it is
// the first value for the node. The lock must be held.
func (s *sampler) remoteNode(key string) *nodeSample {
	node := s.nodes[key]
	if node == nil {
		node = &nodeSample{ifaces: map[string]*ifaceSample{}}
		s.nodes[key] = node
	}

	return node
}

// forgetInterface drops the counters of an interface that is gone and reports whether
// there were some.
func (s *sampler) forgetInterface(key, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.nodes[key]
	if node == nil {
		return false
	}

	if _, ok := node.ifaces[name]; !ok {
		return false
	}

	delete(node.ifaces, name)

	return true
}

// forgetInterfaces drops the counters of every interface, which nothing keeps current any
// longer. It reports whether there were some.
func (s *sampler) forgetInterfaces() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	forgot := false

	for _, node := range s.nodes {
		forgot = forgot || len(node.ifaces) > 0
		node.ifaces = map[string]*ifaceSample{}
		node.rxBps, node.txBps = 0, 0
		node.fresh = false
	}

	return forgot
}

// rollup sums the traffic of the interfaces of every node that got new counters since the
// last call, and reports whether there was one.
func (s *sampler) rollup() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	any := false

	for _, node := range s.nodes {
		if !node.fresh {
			continue
		}

		node.fresh = false
		node.rxBps, node.txBps = 0, 0

		for _, iface := range node.ifaces {
			node.rxBps += iface.rxBps
			node.txBps += iface.txBps
		}

		node.rxSeries = pushSample(node.rxSeries, node.rxBps)
		node.txSeries = pushSample(node.txSeries, node.txBps)
		any = true
	}

	return any
}

// setUsage stores the resource usage a lab runtime reported for a node.
func (s *sampler) setUsage(key string, cpuPercent float64, memBytes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.remoteNode(key)
	node.hasStats = true
	node.cpuPercent = cpuPercent
	node.memBytes = memBytes
	node.cpuSeries = pushSample(node.cpuSeries, cpuPercent)
}

// sampleLabRuntime asks the lab runtime for the resource usage of the nodes until ctx ends.
func (b *Backend) sampleLabRuntime(ctx context.Context) {
	sampler, ok := b.lrt.(clablabruntime.NodeStatsSampler)
	if !ok {
		return
	}

	wait := labRuntimeUsageDelay
	available := true

	for sleep(ctx, wait) {
		err := b.sampleUsage(ctx, sampler)

		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, clablabruntime.ErrMetricsUnavailable):
			if available {
				b.logf(levelInfo, "the cluster serves no resource metrics, the CPU and "+
					"memory usage of the nodes is not shown: %v", err)
			}

			available, wait = false, labRuntimeUsageRetry
		default:
			available, wait = true, labRuntimeUsageInterval
		}
	}
}

// sampleUsage reads the resource usage of the nodes of every lab once. It only fails for
// what concerns every lab: a lab that cannot be read, ie. because it is being removed, is
// skipped.
func (b *Backend) sampleUsage(ctx context.Context, sampler clablabruntime.NodeStatsSampler) error {
	sampled := false

	for _, rec := range b.inv.records() {
		if rec.namespace == "" || !rec.hasRunningNode() {
			continue
		}

		ctx, cancel := context.WithTimeout(ctx, labRuntimeReadTimeout)
		stats, err := sampler.SampleStats(ctx, clablabruntime.InspectRequest{
			Name:      rec.name,
			Namespace: rec.namespace,
		})

		cancel()

		if errors.Is(err, clablabruntime.ErrMetricsUnavailable) {
			return err
		}

		for i := range stats {
			st := &stats[i]

			b.stats.setUsage(
				labRuntimeNodeKey(rec.namespace, rec.name, st.NodeName),
				float64(st.CPUMilliCores)/milliCoresPerPercent,
				uint64(max(st.MemoryBytes, 0)),
			)

			sampled = true
		}
	}

	if sampled {
		b.mark(clabinternaltuibackend.TopicStats)
	}

	return nil
}

// labRuntimeNodeKey is the ID a lab runtime gives a node: <namespace>/<lab>/<node>.
func labRuntimeNodeKey(namespace, lab, node string) string {
	return namespace + "/" + lab + "/" + node
}

func (rec *labRecord) hasRunningNode() bool {
	for i := range rec.lab.Nodes {
		if strings.EqualFold(rec.lab.Nodes[i].State, stateRunning) {
			return true
		}
	}

	return false
}

// writeLabRuntimeLogs writes the log of a node that the lab runtime runs. The runtime has
// one stream for all of it.
func (b *Backend) writeLabRuntimeLogs(
	ctx context.Context,
	rec *labRecord,
	node *clabinternaltuibackend.Node,
	opts *clabruntime.ContainerLogOptions,
	stdout io.Writer,
) error {
	streamer, ok := b.lrt.(clablabruntime.NodeLogStreamer)
	if !ok {
		return fmt.Errorf("%w: the %s runtime cannot read node logs",
			clabinternaltuibackend.ErrUnavailable, b.rtName)
	}

	return streamer.StreamLogs(ctx, clablabruntime.LogRequest{
		Name:       rec.name,
		Namespace:  rec.namespace,
		NodeName:   rec.runtimeNode(node),
		Tail:       opts.Tail,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}, stdout)
}

// execInLabRuntime runs a command in a node that the lab runtime runs.
func (b *Backend) execInLabRuntime(
	ctx context.Context,
	rec *labRecord,
	node *clabinternaltuibackend.Node,
	command []string,
) (*clabexec.ExecResult, error) {
	return b.lrt.Exec(ctx, clablabruntime.ExecRequest{
		Name:      rec.name,
		Namespace: rec.namespace,
		NodeName:  rec.runtimeNode(node),
		Command:   command,
	})
}

// linksEntry holds the links of a lab as its runtime reported them.
type linksEntry struct {
	// shape tells for which nodes the links were read, or are being read.
	shape   string
	loading bool
	links   []clabinternaltuibackend.Link
	// warning explains why there are no links.
	warning string
}

// runtimeLinks caches the links the lab runtime reported for its labs. The links of a lab
// are read again when other nodes of it run than at the time they were read.
type runtimeLinks struct {
	mu      sync.Mutex
	entries map[string]*linksEntry
}

// get returns the links known for a lab, and whether they have to be read for the given
// shape of the lab. A caller that is told to read them must call put.
func (l *runtimeLinks) get(
	key, shape string,
) (links []clabinternaltuibackend.Link, warning string, read bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if entry == nil {
		if l.entries == nil || len(l.entries) >= maxCachedTopologies {
			l.entries = map[string]*linksEntry{}
		}

		entry = &linksEntry{warning: "the links are being read"}
		l.entries[key] = entry
		read = true
	}

	if entry.shape != shape && !entry.loading {
		read = true
	}

	if read {
		entry.shape, entry.loading = shape, true
	}

	return entry.links, entry.warning, read
}

// put stores the links that were read for a lab, or why they could not be.
func (l *runtimeLinks) put(key string, links []clabinternaltuibackend.Link, warning string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if entry == nil {
		return
	}

	entry.loading = false
	entry.links, entry.warning = links, warning
}

// forget drops everything, so that the links of every lab are read again.
func (l *runtimeLinks) forget() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, entry := range l.entries {
		// What is being read is stored when it arrives.
		if !entry.loading {
			delete(l.entries, key)
		}
	}
}

// labShape describes which nodes of a lab run.
func labShape(lab *clabinternaltuibackend.Lab) string {
	var b strings.Builder

	for i := range lab.Nodes {
		if strings.EqualFold(lab.Nodes[i].State, stateRunning) {
			b.WriteString(lab.Nodes[i].Name)
			b.WriteByte(0)
		}
	}

	return b.String()
}

// labRuntimeTopology returns the topology of a lab that the lab runtime runs. With the
// topology file at hand it is read from that file. A lab that was deployed from elsewhere
// has no file here: its nodes are the ones the runtime lists, and its links are the ones
// the runtime reports for the running nodes. The result shares memory with the cache and
// must not be changed.
func (b *Backend) labRuntimeTopology(rec *labRecord) clabinternaltuibackend.Topology {
	lab := &rec.lab

	if !lab.TopoMissing {
		return b.fileTopology(lab.Name, lab.TopoFile)
	}

	topo := topologyFromNodes(lab, nil)

	shape := labShape(lab)
	if shape == "" {
		topo.Warning = "the links are unknown: no node of the lab is running"

		return topo
	}

	key := labRuntimeNodeKey(rec.namespace, rec.name, "")

	links, warning, read := b.links.get(key, shape)
	if read {
		b.spawn("lab links reader", func() { b.readLabRuntimeLinks(key, rec) })
	}

	topo.Links, topo.Warning = links, warning

	return topo
}

// readLabRuntimeLinks asks the lab runtime for the links of a lab and caches the answer.
func (b *Backend) readLabRuntimeLinks(key string, rec *labRecord) {
	defer b.mark(clabinternaltuibackend.TopicLinks | clabinternaltuibackend.TopicInterfaces)

	capturer, ok := b.lrt.(clablabruntime.PacketCapturer)
	if !ok {
		b.links.put(key, nil, "the links are unknown: the "+b.rtName+" runtime does not list them")

		return
	}

	ctx, cancel := context.WithTimeout(b.ctx, labRuntimeReadTimeout)
	defer cancel()

	interfaces, err := capturer.CaptureInterfaces(ctx, clablabruntime.NodeRequest{
		Name:      rec.name,
		Namespace: rec.namespace,
	})
	if err != nil {
		b.links.put(key, nil, "the links are unknown: "+oneLine(err.Error()))

		return
	}

	names := map[string]bool{}
	for i := range rec.lab.Nodes {
		names[rec.lab.Nodes[i].Name] = true
	}

	b.links.put(key, linksFromInterfaces(interfaces, names), "")
}

// linksFromInterfaces assembles links from the link interfaces a lab runtime lists, which
// come with the far end of their link. A link between two nodes is listed from both ends
// and kept once. nodes holds the names of the nodes of the lab.
func linksFromInterfaces(
	interfaces []clablabruntime.CaptureInterface,
	nodes map[string]bool,
) []clabinternaltuibackend.Link {
	type end struct{ node, iface string }

	// The topology names an interface by its alias when the kind of the node has one.
	name := func(ci *clablabruntime.CaptureInterface) string {
		if ci.Alias != "" {
			return ci.Alias
		}

		return ci.Interface
	}

	// The far end is named as the topology wrote it, which is how the near end of the
	// same link is listed too, by its name or by its alias.
	listed := map[end]end{}

	for i := range interfaces {
		ci := &interfaces[i]

		near := end{ci.Node, name(ci)}
		listed[near] = near
		listed[end{ci.Node, ci.Interface}] = near
	}

	var links []clabinternaltuibackend.Link

	seen := map[end]bool{}

	for i := range interfaces {
		ci := &interfaces[i]

		near := end{ci.Node, name(ci)}
		if seen[near] {
			continue
		}

		seen[near] = true

		link := clabinternaltuibackend.Link{
			A: clabinternaltuibackend.Endpoint{Node: near.node, Interface: near.iface},
		}

		if ci.PeerNode != "" {
			far := end{ci.PeerNode, ci.PeerInterface}
			if known, ok := listed[far]; ok {
				far = known
			}

			seen[far] = true
			link.Z = &clabinternaltuibackend.Endpoint{
				Node:      far.node,
				Interface: far.iface,
				External:  ci.PeerNode == hostEndpoint || !nodes[ci.PeerNode],
			}
		}

		links = append(links, link)
	}

	slices.SortFunc(links, func(a, b clabinternaltuibackend.Link) int {
		if c := clabinternaltuitermtext.NaturalCompare(a.A.Node, b.A.Node); c != 0 {
			return c
		}

		return clabinternaltuitermtext.NaturalCompare(a.A.Interface, b.A.Interface)
	})

	return links
}

// inspectWorkload adds where in the cluster a node runs.
func inspectWorkload(r *report, ctr *clabruntime.GenericContainer) {
	workload := ctr.Workload
	if workload == nil {
		return
	}

	r.section("Workload")
	r.field("namespace", workload.Namespace)
	r.field("pod", workload.Pod)
	r.field("container", workload.Container)
	r.field("node uid", workload.NodeUID)
	r.field("load balancer", workload.LoadBalancerAddress)
}
