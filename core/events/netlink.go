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
	ctx      context.Context
	mu       sync.Mutex
	watchers map[string]*netlinkWatcher
	events   chan<- aggregatedEvent
}

func newNetlinkRegistry(ctx context.Context, events chan<- aggregatedEvent) *netlinkRegistry {
	return &netlinkRegistry{
		ctx:      ctx,
		watchers: make(map[string]*netlinkWatcher),
		events:   events,
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
