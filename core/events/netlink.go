package events

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type netlinkRegistry struct {
	ctx                    context.Context
	mu                     sync.Mutex
	watchers               map[string]*netlinkWatcher
	events                 chan<- aggregatedEvent
	includeInitialSnapshot bool
	includeStats           bool
}

func newNetlinkRegistry(ctx context.Context, events chan<- aggregatedEvent, includeInitialSnapshot, includeStats bool) *netlinkRegistry {
	return &netlinkRegistry{
		ctx:                    ctx,
		watchers:               make(map[string]*netlinkWatcher),
		events:                 events,
		includeInitialSnapshot: includeInitialSnapshot,
		includeStats:           includeStats,
	}
}

func (r *netlinkRegistry) Start(container *clabruntime.GenericContainer) {
	clone := cloneContainer(container)
	if clone == nil {
		return
	}

	id := clone.ID
	if id == "" {
		id = clone.ShortID
	}

	if id == "" {
		return
	}

	r.mu.Lock()
	if _, exists := r.watchers[id]; exists {
		r.mu.Unlock()

		return
	}

	watcherCtx, cancel := context.WithCancel(r.ctx)
	watcher := &netlinkWatcher{
		container:       clone,
		cancel:          cancel,
		done:            make(chan struct{}),
		includeSnapshot: r.includeInitialSnapshot,
		includeStats:    r.includeStats,
	}

	r.watchers[id] = watcher
	r.mu.Unlock()

	go watcher.run(watcherCtx, r)
}

func (r *netlinkRegistry) Stop(id string) {
	if id == "" {
		return
	}

	r.mu.Lock()
	watcher, ok := r.watchers[id]
	if ok {
		delete(r.watchers, id)
	}
	r.mu.Unlock()

	if !ok {
		return
	}

	watcher.cancel()
	<-watcher.done
}

func (r *netlinkRegistry) remove(id string, watcher *netlinkWatcher) {
	if id == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if current, ok := r.watchers[id]; ok && current == watcher {
		delete(r.watchers, id)
	}
}

func (r *netlinkRegistry) HandleContainerEvent(
	runtime clabruntime.ContainerRuntime,
	ev clabruntime.ContainerEvent,
) {
	if !strings.EqualFold(ev.Type, clabruntime.EventTypeContainer) {
		return
	}

	action := strings.ToLower(ev.Action)

	switch action {
	case clabruntime.EventActionStart, clabruntime.EventActionUnpause, clabruntime.EventActionRestart:
		container := containerFromEvent(runtime, ev)
		if container != nil {
			r.Start(container)
		}
	case clabruntime.EventActionDie, clabruntime.EventActionStop, clabruntime.EventActionDestroy, clabruntime.EventActionKill:
		id := ev.ActorFullID
		if id == "" {
			id = ev.ActorID
		}

		r.Stop(id)
	}
}

func cloneContainer(container *clabruntime.GenericContainer) *clabruntime.GenericContainer {
	if container == nil {
		return nil
	}

	clone := &clabruntime.GenericContainer{
		Names:   append([]string{}, container.Names...),
		ID:      container.ID,
		ShortID: container.ShortID,
		Labels:  cloneStringMap(container.Labels),
	}

	if clone.ShortID == "" {
		clone.ShortID = shortID(clone.ID)
	}

	if container.Runtime == nil {
		return nil
	}

	clone.SetRuntime(container.Runtime)

	return clone
}

func containerFromEvent(
	runtime clabruntime.ContainerRuntime,
	ev clabruntime.ContainerEvent,
) *clabruntime.GenericContainer {
	attributes := ev.Attributes

	name := ev.ActorName
	if name == "" && attributes != nil {
		name = attributes["name"]
	}

	id := ev.ActorFullID
	if id == "" {
		id = ev.ActorID
	}

	if id == "" && name == "" {
		return nil
	}

	short := ev.ActorID
	if short == "" {
		short = id
	}

	container := &clabruntime.GenericContainer{
		ID:      id,
		ShortID: shortID(short),
	}

	if name != "" {
		container.Names = []string{name}
	}

	if attributes != nil {
		if lab := attributes[clabconstants.Containerlab]; lab != "" {
			container.Labels = map[string]string{clabconstants.Containerlab: lab}
		}
	}

	if container.ShortID == "" {
		container.ShortID = shortID(container.ID)
	}

	if runtime != nil {
		container.SetRuntime(runtime)
	}

	return container
}

type netlinkWatcher struct {
	container       *clabruntime.GenericContainer
	cancel          context.CancelFunc
	done            chan struct{}
	includeSnapshot bool
	includeStats    bool
}

func (w *netlinkWatcher) run(ctx context.Context, registry *netlinkRegistry) {
	defer close(w.done)
	if w.container == nil {
		return
	}

	defer registry.remove(w.container.ID, w)

	containerName := firstContainerName(w.container)
	if w.container.Runtime == nil {
		log.Debugf("container %s has no runtime, skipping netlink watcher", containerName)

		return
	}

	nsPath, err := waitForNamespacePath(ctx, w.container.Runtime, w.container.ID)
	if err != nil || nsPath == "" {
		log.Debugf("failed to resolve netns for container %s: %v", containerName, err)

		return
	}

	nsHandle, err := netns.GetFromPath(nsPath)
	if err != nil {
		log.Debugf("failed to open netns for container %s: %v", containerName, err)

		return
	}
	defer nsHandle.Close()

	netHandle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		log.Debugf("failed to create netlink handle for container %s: %v", containerName, err)

		return
	}
	defer netHandle.Close()

	states, err := snapshotInterfaces(netHandle)
	if err != nil {
		log.Debugf("failed to snapshot interfaces for container %s: %v", containerName, err)
		states = make(map[int]ifaceSnapshot)
	}

	var statsSamples map[int]ifaceStatsSample
	if w.includeStats {
		statsSamples = make(map[int]ifaceStatsSample, len(states))
		now := time.Now()
		for idx, snapshot := range states {
			if sample, ok := newStatsSample(snapshot, now); ok {
				statsSamples[idx] = sample
			}
		}
	}

	if w.includeSnapshot {
		for _, snapshot := range states {
			registry.emitInterfaceEvent(w.container, "snapshot", snapshot)
		}
	}

	updates := make(chan netlink.LinkUpdate, 32)
	done := make(chan struct{})
	opts := netlink.LinkSubscribeOptions{Namespace: &nsHandle}

	if err := netlink.LinkSubscribeWithOptions(updates, done, opts); err != nil {
		log.Debugf("failed to subscribe to netlink updates for container %s: %v", containerName, err)

		return
	}

	var (
		ticker  *time.Ticker
		tickerC <-chan time.Time
	)
	if w.includeStats {
		ticker = time.NewTicker(time.Second)
		tickerC = ticker.C
		defer ticker.Stop()
	}

	for {
		select {
		case <-tickerC:
			w.collectAndEmitStats(netHandle, states, statsSamples, registry)
		case <-ctx.Done():
			close(done)

			return
		case update, ok := <-updates:
			if !ok {
				return
			}

			w.processUpdate(states, statsSamples, update, registry)
		}
	}
}

func (w *netlinkWatcher) processUpdate(
	states map[int]ifaceSnapshot,
	statsSamples map[int]ifaceStatsSample,
	update netlink.LinkUpdate,
	registry *netlinkRegistry,
) {
	if update.Link == nil {
		return
	}

	attrs := update.Link.Attrs()
	if attrs == nil {
		return
	}

	snapshot := snapshotFromLink(update.Link)
	previous, exists := states[snapshot.Index]

	switch update.Header.Type {
	case unix.RTM_DELLINK:
		if exists {
			snapshot = previous
		}

		delete(states, snapshot.Index)
		delete(statsSamples, snapshot.Index)
		registry.emitInterfaceEvent(w.container, "delete", snapshot)
	case unix.RTM_NEWLINK:
		if exists && snapshot.equal(previous) {
			return
		}

		action := "create"
		if exists {
			action = "update"
		}

		states[snapshot.Index] = snapshot
		if w.includeStats && statsSamples != nil {
			if sample, ok := newStatsSample(snapshot, time.Now()); ok {
				statsSamples[snapshot.Index] = sample
			}
		}
		registry.emitInterfaceEvent(w.container, action, snapshot)
	}
}

func firstContainerName(container *clabruntime.GenericContainer) string {
	if container == nil || len(container.Names) == 0 {
		return ""
	}

	return container.Names[0]
}

func containerLabel(container *clabruntime.GenericContainer) string {
	if container == nil || container.Labels == nil {
		return ""
	}

	return container.Labels[clabconstants.Containerlab]
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}

	return id
}

func interfaceAttributes(
	container *clabruntime.GenericContainer,
	snapshot ifaceSnapshot,
) map[string]string {
	attributes := map[string]string{
		"ifname": snapshot.Name,
		"index":  strconv.Itoa(snapshot.Index),
		"mtu":    strconv.Itoa(snapshot.MTU),
		"state":  snapshot.OperState,
		"type":   snapshot.Type,
		"origin": "netlink",
	}

	if snapshot.Alias != "" {
		attributes["alias"] = snapshot.Alias
	}

	if snapshot.MAC != "" {
		attributes["mac"] = snapshot.MAC
	}

	if lab := containerLabel(container); lab != "" {
		attributes["lab"] = lab
	}

	if name := firstContainerName(container); name != "" {
		attributes["name"] = name
	}

	return attributes
}

func (r *netlinkRegistry) emitInterfaceEvent(
	container *clabruntime.GenericContainer,
	action string,
	snapshot ifaceSnapshot,
) {
	if container == nil {
		return
	}

	attributes := interfaceAttributes(container, snapshot)

	event := aggregatedEvent{
		Timestamp:   time.Now(),
		Type:        "interface",
		Action:      action,
		ActorID:     container.ShortID,
		ActorName:   firstContainerName(container),
		ActorFullID: container.ID,
		Attributes:  attributes,
	}

	select {
	case r.events <- event:
	case <-r.ctx.Done():
	}
}

func waitForNamespacePath(
	ctx context.Context,
	runtime clabruntime.ContainerRuntime,
	containerID string,
) (string, error) {
	const (
		attempts   = 5
		retryDelay = 200 * time.Millisecond
	)

	var lastErr error

	for i := 0; i < attempts; i++ {
		nsPath, err := runtime.GetNSPath(ctx, containerID)
		if err == nil && nsPath != "" {
			return nsPath, nil
		}

		if err != nil {
			lastErr = err
		}

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if lastErr != nil {
		return "", lastErr
	}

	return "", fmt.Errorf("namespace path not ready for container %s", containerID)
}

func (r *netlinkRegistry) emitInterfaceStatsEvent(
	container *clabruntime.GenericContainer,
	snapshot ifaceSnapshot,
	metrics ifaceStatsMetrics,
) {
	if !r.includeStats || container == nil || !snapshot.HasStats {
		return
	}

	attributes := interfaceAttributes(container, snapshot)
	attributes["rx_bytes"] = strconv.FormatUint(metrics.RxBytes, 10)
	attributes["tx_bytes"] = strconv.FormatUint(metrics.TxBytes, 10)
	attributes["rx_packets"] = strconv.FormatUint(metrics.RxPackets, 10)
	attributes["tx_packets"] = strconv.FormatUint(metrics.TxPackets, 10)
	attributes["rx_bps"] = strconv.FormatFloat(metrics.RxBps, 'f', -1, 64)
	attributes["tx_bps"] = strconv.FormatFloat(metrics.TxBps, 'f', -1, 64)
	attributes["rx_pps"] = strconv.FormatFloat(metrics.RxPps, 'f', -1, 64)
	attributes["tx_pps"] = strconv.FormatFloat(metrics.TxPps, 'f', -1, 64)
	attributes["interval_seconds"] = strconv.FormatFloat(metrics.Interval.Seconds(), 'f', -1, 64)

	event := aggregatedEvent{
		Timestamp:   metrics.Timestamp,
		Type:        "interface",
		Action:      "stats",
		ActorID:     container.ShortID,
		ActorName:   firstContainerName(container),
		ActorFullID: container.ID,
		Attributes:  attributes,
	}

	select {
	case r.events <- event:
	case <-r.ctx.Done():
	}
}

func snapshotInterfaces(netHandle *netlink.Handle) (map[int]ifaceSnapshot, error) {
	if netHandle == nil {
		return nil, fmt.Errorf("netlink handle is nil")
	}

	links, err := netHandle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("unable to list links: %w", err)
	}

	states := make(map[int]ifaceSnapshot, len(links))
	for _, link := range links {
		snapshot := snapshotFromLink(link)
		states[snapshot.Index] = snapshot
	}

	return states, nil
}

func snapshotFromLink(link netlink.Link) ifaceSnapshot {
	attrs := link.Attrs()

	snapshot := ifaceSnapshot{
		Type: link.Type(),
	}

	if attrs != nil {
		snapshot.Index = attrs.Index
		snapshot.Name = attrs.Name
		snapshot.Alias = attrs.Alias
		snapshot.MTU = attrs.MTU
		if len(attrs.HardwareAddr) > 0 {
			snapshot.MAC = attrs.HardwareAddr.String()
		}
		snapshot.OperState = attrs.OperState.String()
		if stats := attrs.Statistics; stats != nil {
			snapshot.HasStats = true
			snapshot.RxBytes = stats.RxBytes
			snapshot.TxBytes = stats.TxBytes
			snapshot.RxPackets = stats.RxPackets
			snapshot.TxPackets = stats.TxPackets
		}
	}

	return snapshot
}

type ifaceSnapshot struct {
	Index     int
	Name      string
	Alias     string
	MTU       int
	MAC       string
	OperState string
	Type      string
	HasStats  bool
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
}

func (s ifaceSnapshot) equal(other ifaceSnapshot) bool {
	return s.Index == other.Index &&
		s.Name == other.Name &&
		s.Alias == other.Alias &&
		s.MTU == other.MTU &&
		s.MAC == other.MAC &&
		s.OperState == other.OperState &&
		s.Type == other.Type
}

type ifaceStatsSample struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	Timestamp time.Time
}

type ifaceStatsMetrics struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxBps     float64
	TxBps     float64
	RxPps     float64
	TxPps     float64
	Interval  time.Duration
	Timestamp time.Time
}

func newStatsSample(snapshot ifaceSnapshot, timestamp time.Time) (ifaceStatsSample, bool) {
	if !snapshot.HasStats {
		return ifaceStatsSample{}, false
	}

	return ifaceStatsSample{
		RxBytes:   snapshot.RxBytes,
		TxBytes:   snapshot.TxBytes,
		RxPackets: snapshot.RxPackets,
		TxPackets: snapshot.TxPackets,
		Timestamp: timestamp,
	}, true
}

func (w *netlinkWatcher) collectAndEmitStats(
	netHandle *netlink.Handle,
	states map[int]ifaceSnapshot,
	statsSamples map[int]ifaceStatsSample,
	registry *netlinkRegistry,
) {
	if netHandle == nil || statsSamples == nil {
		return
	}

	now := time.Now()

	for idx, state := range states {
		link, err := netHandle.LinkByIndex(idx)
		if err != nil {
			continue
		}

		current := snapshotFromLink(link)
		state.Name = current.Name
		state.Alias = current.Alias
		state.MTU = current.MTU
		state.MAC = current.MAC
		state.OperState = current.OperState
		state.Type = current.Type
		state.HasStats = current.HasStats
		state.RxBytes = current.RxBytes
		state.TxBytes = current.TxBytes
		state.RxPackets = current.RxPackets
		state.TxPackets = current.TxPackets
		states[idx] = state

		if !state.HasStats {
			delete(statsSamples, idx)

			continue
		}

		prev, ok := statsSamples[idx]
		if !ok || now.Sub(prev.Timestamp) <= 0 {
			statsSamples[idx] = ifaceStatsSample{
				RxBytes:   state.RxBytes,
				TxBytes:   state.TxBytes,
				RxPackets: state.RxPackets,
				TxPackets: state.TxPackets,
				Timestamp: now,
			}

			continue
		}

		interval := now.Sub(prev.Timestamp)
		if interval <= 0 {
			statsSamples[idx] = ifaceStatsSample{
				RxBytes:   state.RxBytes,
				TxBytes:   state.TxBytes,
				RxPackets: state.RxPackets,
				TxPackets: state.TxPackets,
				Timestamp: now,
			}

			continue
		}

		rxBytesDelta := deltaCounter(prev.RxBytes, state.RxBytes)
		txBytesDelta := deltaCounter(prev.TxBytes, state.TxBytes)
		rxPacketsDelta := deltaCounter(prev.RxPackets, state.RxPackets)
		txPacketsDelta := deltaCounter(prev.TxPackets, state.TxPackets)

		seconds := interval.Seconds()
		if seconds <= 0 {
			statsSamples[idx] = ifaceStatsSample{
				RxBytes:   state.RxBytes,
				TxBytes:   state.TxBytes,
				RxPackets: state.RxPackets,
				TxPackets: state.TxPackets,
				Timestamp: now,
			}

			continue
		}

		metrics := ifaceStatsMetrics{
			RxBytes:   state.RxBytes,
			TxBytes:   state.TxBytes,
			RxPackets: state.RxPackets,
			TxPackets: state.TxPackets,
			RxBps:     float64(rxBytesDelta) * 8 / seconds,
			TxBps:     float64(txBytesDelta) * 8 / seconds,
			RxPps:     float64(rxPacketsDelta) / seconds,
			TxPps:     float64(txPacketsDelta) / seconds,
			Interval:  interval,
			Timestamp: now,
		}

		registry.emitInterfaceStatsEvent(w.container, state, metrics)

		statsSamples[idx] = ifaceStatsSample{
			RxBytes:   state.RxBytes,
			TxBytes:   state.TxBytes,
			RxPackets: state.RxPackets,
			TxPackets: state.TxPackets,
			Timestamp: now,
		}
	}
}

func deltaCounter(previous, current uint64) uint64 {
	if current >= previous {
		return current - previous
	}

	return current
}
