package clab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type EventsOptions struct {
	Format      string
	Runtime     string
	ClabOptions []clabcore.ClabOption
}

func Events(ctx context.Context, opts EventsOptions) error { //nolint:cyclop,funlen
	if err := clabutils.CheckAndGetRootPrivs(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, err := clabcore.NewContainerLab(opts.ClabOptions...)
	if err != nil {
		return err
	}

	runtime, ok := c.Runtimes[opts.Runtime]
	if !ok {
		return fmt.Errorf("runtime %q is not initialized", opts.Runtime)
	}

	format := strings.TrimSpace(strings.ToLower(opts.Format))
	if format == "" {
		format = "plain"
	}

	var printer func(aggregatedEvent)
	switch format {
	case "plain":
		printer = printAggregatedEvent
	case "json":
		printer = printAggregatedEventJSON
	default:
		return fmt.Errorf("output format %q is not supported, use 'plain' or 'json'", opts.Format)
	}

	eventCh := make(chan aggregatedEvent, 128)
	errCh := make(chan error, 1)
	registry := newNetlinkRegistry(eventCh)

	containers, err := c.ListContainers(ctx, clabcore.WithListclabLabelExists())
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for idx := range containers {
		container := containers[idx]
		if !isRunningContainer(&container) {
			continue
		}

		registry.Start(ctx, &container)
	}

	streamOpts := clabruntime.EventStreamOptions{
		Labels: map[string]string{
			clabconstants.Containerlab: "",
		},
	}

	runtimeEvents, runtimeErrs, err := runtime.StreamEvents(ctx, streamOpts)
	if err != nil {
		return fmt.Errorf("failed to stream events for runtime %q: %w", opts.Runtime, err)
	}

	go forwardRuntimeEvents(ctx, runtime, registry, runtimeEvents, runtimeErrs, eventCh, errCh)

	for {
		select {
		case ev := <-eventCh:
			printer(ev)
		case err := <-errCh:
			if err == nil || errors.Is(err, context.Canceled) {
				return nil
			}

			return err
		case <-ctx.Done():
			return nil
		}
	}
}

type aggregatedEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	Type        string            `json:"type"`
	Action      string            `json:"action"`
	ActorID     string            `json:"actor_id"`
	ActorName   string            `json:"actor_name"`
	ActorFullID string            `json:"actor_full_id"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

func aggregatedEventFromContainerEvent(ev clabruntime.ContainerEvent) aggregatedEvent {
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	attributes := copyStringMap(ev.Attributes)

	actorFullID := ev.ActorFullID
	if actorFullID == "" {
		actorFullID = ev.ActorID
	}

	actorName := ev.ActorName
	if actorName == "" && attributes != nil {
		actorName = attributes["name"]
	}

	short := ev.ActorID
	if short == "" {
		short = actorFullID
	}

	action := strings.ToLower(ev.Action)
	if action == "" {
		action = ev.Action
	}

	eventType := strings.ToLower(ev.Type)
	if eventType == "" {
		eventType = ev.Type
	}

	return aggregatedEvent{
		Timestamp:   ts,
		Type:        eventType,
		Action:      action,
		ActorID:     shortID(short),
		ActorName:   actorName,
		ActorFullID: actorFullID,
		Attributes:  attributes,
	}
}

func printAggregatedEvent(ev aggregatedEvent) {
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	ts = ts.UTC()

	actor := ev.ActorID
	if actor == "" {
		actor = ev.ActorName
	}
	if actor == "" {
		actor = "-"
	}

	attrs := mergedEventAttributes(ev)
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	attrParts := make([]string, 0, len(keys))
	for _, k := range keys {
		attrParts = append(attrParts, fmt.Sprintf("%s=%s", k, attrs[k]))
	}

	suffix := ""
	if len(attrParts) > 0 {
		suffix = " (" + strings.Join(attrParts, ", ") + ")"
	}

	fmt.Printf("%s %s %s %s%s\n", ts.Format(time.RFC3339Nano), ev.Type, ev.Action, actor, suffix)
}

func printAggregatedEventJSON(ev aggregatedEvent) {
	evCopy := ev
	evCopy.Attributes = mergedEventAttributes(ev)

	b, err := json.Marshal(evCopy)
	if err != nil {
		log.Debugf("failed to marshal event to json: %v", err)

		return
	}

	fmt.Println(string(b))
}

func mergedEventAttributes(ev aggregatedEvent) map[string]string {
	if len(ev.Attributes) == 0 && ev.ActorName == "" && ev.ActorFullID == "" {
		return nil
	}

	attrs := make(map[string]string, len(ev.Attributes)+2)
	for k, v := range ev.Attributes {
		if v == "" {
			continue
		}

		attrs[k] = v
	}

	if ev.ActorName != "" {
		attrs["name"] = ev.ActorName
	}

	if ev.ActorFullID != "" {
		attrs["id"] = ev.ActorFullID
	}

	if len(attrs) == 0 {
		return nil
	}

	return attrs
}

func forwardRuntimeEvents(
	ctx context.Context,
	runtime clabruntime.ContainerRuntime,
	registry *netlinkRegistry,
	runtimeEvents <-chan clabruntime.ContainerEvent,
	runtimeErrs <-chan error,
	eventSink chan<- aggregatedEvent,
	errSink chan<- error,
) {
	for {
		select {
		case <-ctx.Done():
			errSink <- nil

			return
		case err, ok := <-runtimeErrs:
			if !ok {
				errSink <- nil

				return
			}

			if err != nil && !errors.Is(err, context.Canceled) {
				errSink <- err

				return
			}
		case ev, ok := <-runtimeEvents:
			if !ok {
				errSink <- nil

				return
			}

			registry.HandleContainerEvent(ctx, runtime, ev)

			aggregated := aggregatedEventFromContainerEvent(ev)

			select {
			case eventSink <- aggregated:
			case <-ctx.Done():
				errSink <- nil

				return
			}
		}
	}
}

type netlinkRegistry struct {
	mu       sync.Mutex
	watchers map[string]*netlinkWatcher
	events   chan<- aggregatedEvent
}

func newNetlinkRegistry(events chan<- aggregatedEvent) *netlinkRegistry {
	return &netlinkRegistry{
		watchers: make(map[string]*netlinkWatcher),
		events:   events,
	}
}

func (r *netlinkRegistry) Start(ctx context.Context, container *clabruntime.GenericContainer) {
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

	watcherCtx, cancel := context.WithCancel(ctx)
	watcher := &netlinkWatcher{
		container: clone,
		cancel:    cancel,
		done:      make(chan struct{}),
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
	ctx context.Context,
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
			r.Start(ctx, container)
		}
	case clabruntime.EventActionDie, clabruntime.EventActionStop, clabruntime.EventActionDestroy, clabruntime.EventActionKill:
		id := ev.ActorFullID
		if id == "" {
			id = ev.ActorID
		}

		r.Stop(id)
	}
}

type netlinkWatcher struct {
	container *clabruntime.GenericContainer
	cancel    context.CancelFunc
	done      chan struct{}
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

	states, err := snapshotInterfaces(nsHandle)
	if err != nil {
		log.Debugf("failed to snapshot interfaces for container %s: %v", containerName, err)
		states = make(map[int]ifaceSnapshot)
	}

	updates := make(chan netlink.LinkUpdate, 32)
	done := make(chan struct{})
	opts := netlink.LinkSubscribeOptions{Namespace: &nsHandle}

	if err := netlink.LinkSubscribeWithOptions(updates, done, opts); err != nil {
		log.Debugf("failed to subscribe to netlink updates for container %s: %v", containerName, err)

		return
	}

	for {
		select {
		case <-ctx.Done():
			close(done)

			return
		case update, ok := <-updates:
			if !ok {
				return
			}

			w.processUpdate(states, update, registry)
		}
	}
}

func (w *netlinkWatcher) processUpdate(
	states map[int]ifaceSnapshot,
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
		registry.emitInterfaceEvent(w.container, action, snapshot)
	}
}

func (r *netlinkRegistry) emitInterfaceEvent(
	container *clabruntime.GenericContainer,
	action string,
	snapshot ifaceSnapshot,
) {
	if container == nil {
		return
	}

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

	r.events <- aggregatedEvent{
		Timestamp:   time.Now(),
		Type:        "interface",
		Action:      action,
		ActorID:     container.ShortID,
		ActorName:   firstContainerName(container),
		ActorFullID: container.ID,
		Attributes:  attributes,
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

func snapshotInterfaces(nsHandle netns.NsHandle) (map[int]ifaceSnapshot, error) {
	netHandle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("unable to enter namespace: %w", err)
	}
	defer netHandle.Close()

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

func isRunningContainer(container *clabruntime.GenericContainer) bool {
	if container == nil {
		return false
	}

	return strings.EqualFold(container.State, "running")
}

func cloneContainer(container *clabruntime.GenericContainer) *clabruntime.GenericContainer {
	if container == nil {
		return nil
	}

	clone := &clabruntime.GenericContainer{
		Names:   append([]string{}, container.Names...),
		ID:      container.ID,
		ShortID: container.ShortID,
		Labels:  copyStringMap(container.Labels),
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

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}

	result := make(map[string]string, len(input))
	for k, v := range input {
		result[k] = v
	}

	return result
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
