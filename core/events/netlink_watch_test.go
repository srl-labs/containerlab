package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/srl-labs/containerlab/mocks/mockruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go.uber.org/mock/gomock"
)

// eventLog collects the interface events of a registry by interface name.
type eventLog struct {
	mu      sync.Mutex
	actions map[string][]string
}

func collectEvents(ctx context.Context, events <-chan aggregatedEvent) *eventLog {
	l := &eventLog{actions: map[string][]string{}}

	go func() {
		for {
			select {
			case ev := <-events:
				l.mu.Lock()
				name := ev.Attributes["ifname"]
				l.actions[name] = append(l.actions[name], ev.Action)
				l.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	return l
}

func (l *eventLog) get(name string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return slices.Clone(l.actions[name])
}

// waitFor waits until every name has an event with one of the actions.
func (l *eventLog) waitFor(t *testing.T, names []string, actions ...string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		var missing []string
		for _, name := range names {
			if !slices.ContainsFunc(l.get(name), func(a string) bool {
				return slices.Contains(actions, a)
			}) {
				missing = append(missing, name)
			}
		}

		if len(missing) == 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("no %v event for %v", actions, missing)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// newTestNetns creates a named network namespace without leaving the
// current thread in it.
func newTestNetns(t *testing.T) (string, *netlink.Handle) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("requires root to create network namespaces")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	orig, err := netns.Get()
	if err != nil {
		t.Fatalf("failed to get current netns: %v", err)
	}
	defer orig.Close()

	name := fmt.Sprintf("clabtst-ev-%d-%d", os.Getpid(), time.Now().UnixNano())

	ns, err := netns.NewNamed(name)
	if err != nil {
		t.Fatalf("failed to create netns: %v", err)
	}

	if err := netns.Set(orig); err != nil {
		t.Fatalf("failed to return to the original netns: %v", err)
	}

	handle, err := netlink.NewHandleAt(ns)
	if err != nil {
		t.Fatalf("failed to create netlink handle: %v", err)
	}

	t.Cleanup(func() {
		handle.Close()
		ns.Close()
		_ = netns.DeleteNamed(name)
	})

	return "/run/netns/" + name, handle
}

func addDummy(t *testing.T, handle *netlink.Handle, name string) {
	t.Helper()

	err := handle.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}})
	if err != nil {
		t.Errorf("failed to add %s: %v", name, err)
	}
}

func newTestContainer(
	t *testing.T,
	getNSPath func() (string, error),
) *clabruntime.GenericContainer {
	t.Helper()

	rt := mockruntime.NewMockContainerRuntime(gomock.NewController(t))
	rt.EXPECT().GetNSPath(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, string) (string, error) { return getNSPath() },
	).AnyTimes()

	container := &clabruntime.GenericContainer{
		ID:      "0123456789abcdef",
		ShortID: "0123456789ab",
		Names:   []string{"clab-test-n1"},
	}
	container.SetRuntime(rt)

	return container
}

func startRegistry(
	t *testing.T,
	container *clabruntime.GenericContainer,
) (*netlinkRegistry, *eventLog) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan aggregatedEvent, 1024)
	registry := newNetlinkRegistry(ctx, events, true, false, 0)
	collected := collectEvents(ctx, events)

	registry.Start(container)
	t.Cleanup(func() {
		registry.Stop(container.ID)
		cancel()
	})

	return registry, collected
}

// TestNetlinkWatcherReportsExistingAndNewLinks checks that links present at
// subscription are reported as snapshots and later ones as created or deleted.
func TestNetlinkWatcherReportsExistingAndNewLinks(t *testing.T) {
	nsPath, handle := newTestNetns(t)
	addDummy(t, handle, "early0")

	container := newTestContainer(t, func() (string, error) { return nsPath, nil })
	_, events := startRegistry(t, container)

	events.waitFor(t, []string{"lo", "early0"}, "snapshot")

	addDummy(t, handle, "late0")
	events.waitFor(t, []string{"late0"}, "create")

	link, err := handle.LinkByName("late0")
	if err != nil {
		t.Fatalf("failed to find late0: %v", err)
	}

	if err := handle.LinkDel(link); err != nil {
		t.Fatalf("failed to delete late0: %v", err)
	}

	events.waitFor(t, []string{"late0"}, "delete")

	if got := events.get("early0"); slices.Contains(got, "create") {
		t.Fatalf("early0 was reported as created: %v", got)
	}
}

// TestNetlinkWatcherReportsLinksAddedWhileStarting adds links while the
// watcher starts. Each link must be reported, whether the watcher lists it or
// is notified about it.
func TestNetlinkWatcherReportsLinksAddedWhileStarting(t *testing.T) {
	for round := range 20 {
		t.Run(fmt.Sprintf("round %d", round), func(t *testing.T) {
			nsPath, handle := newTestNetns(t)
			container := newTestContainer(t, func() (string, error) { return nsPath, nil })

			names := make([]string, 0, 100)
			for i := range cap(names) {
				names = append(names, fmt.Sprintf("dummy%d", i))
			}

			var wg sync.WaitGroup
			wg.Go(func() {
				for _, name := range names {
					addDummy(t, handle, name)
				}
			})

			_, events := startRegistry(t, container)
			wg.Wait()

			events.waitFor(t, names, "snapshot", "create")
		})
	}
}

// TestNetlinkWatcherRetriesNamespaceLookup checks that the watcher keeps
// trying when the namespace is not found within the first attempt.
func TestNetlinkWatcherRetriesNamespaceLookup(t *testing.T) {
	nsPath, _ := newTestNetns(t)

	origDelay := watchRetryDelay
	watchRetryDelay = 10 * time.Millisecond
	t.Cleanup(func() { watchRetryDelay = origDelay })

	var (
		mu    sync.Mutex
		calls int
	)

	// waitForNamespacePath asks five times per attempt, so the first attempt fails.
	container := newTestContainer(t, func() (string, error) {
		mu.Lock()
		defer mu.Unlock()

		calls++
		if calls <= 6 {
			return "", errors.New("not ready")
		}

		return nsPath, nil
	})

	_, events := startRegistry(t, container)
	events.waitFor(t, []string{"lo"}, "snapshot")
}

// TestNetlinkWatcherStopsAfterRetries checks that the watcher gives up when
// the namespace is never found.
func TestNetlinkWatcherStopsAfterRetries(t *testing.T) {
	origAttempts, origDelay := watchMaxAttempts, watchRetryDelay
	watchMaxAttempts, watchRetryDelay = 2, 10*time.Millisecond
	t.Cleanup(func() { watchMaxAttempts, watchRetryDelay = origAttempts, origDelay })

	container := newTestContainer(t, func() (string, error) { return "", errors.New("gone") })
	registry, _ := startRegistry(t, container)

	registry.mu.Lock()
	watcher := registry.watchers[container.ID]
	registry.mu.Unlock()

	select {
	case <-watcher.done:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop")
	}
}

// TestProcessExistingAfterResubscribe checks how a listed link is reported on
// the first subscription and after a resubscribe.
func TestProcessExistingAfterResubscribe(t *testing.T) {
	link := func(mtu int) netlink.Link {
		return &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Index: 7, Name: "eth1", MTU: mtu}}
	}

	tests := map[string]struct {
		includeSnapshot bool
		initial         bool
		known           netlink.Link
		want            []string
	}{
		"first subscription": {
			includeSnapshot: true,
			initial:         true,
			want:            []string{"snapshot"},
		},
		"first subscription without snapshot": {initial: true},
		"new after resubscribe":               {want: []string{"create"}},
		"changed after resubscribe":           {known: link(1400), want: []string{"update"}},
		"unchanged after resubscribe":         {known: link(1500)},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			events := make(chan aggregatedEvent, 8)
			registry := newNetlinkRegistry(t.Context(), events, tt.includeSnapshot, false, 0)
			watcher := &netlinkWatcher{
				container:       &clabruntime.GenericContainer{Names: []string{"n1"}},
				includeSnapshot: tt.includeSnapshot,
			}

			states := map[int]ifaceSnapshot{}
			if tt.known != nil {
				states[7] = snapshotFromLink(tt.known)
			}

			watcher.processExisting(states, nil, link(1500), nil, registry, tt.initial)
			close(events)

			var got []string
			for ev := range events {
				got = append(got, ev.Action)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("actions = %v, want %v", got, tt.want)
			}

			if states[7].MTU != 1500 {
				t.Fatalf("state MTU = %d, want 1500", states[7].MTU)
			}
		})
	}
}
