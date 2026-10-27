package clabernetes

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// podInterfaces is the sysfs view of a c9s pod: the device's interfaces and clabernetes' own.
func podInterfaces(link string, linkState string) []c9sIfaceState {
	return []c9sIfaceState{
		{Name: "lo", Index: 1, MTU: 65536, MAC: zeroMAC, OperState: "unknown"},
		{Name: "c9s0", Index: 2, MTU: 1500, MAC: "6e:43:ce:6b:3f:dc", OperState: "up"},
		{
			Name: "c9sr0", Index: 3, MTU: 1450, MAC: "02:c9:20:eb:f4:ff", OperState: "up",
			Alias: "c9s:interposition:v1",
		},
		{
			Name: "eth0", Index: 4, MTU: 1450, MAC: "3a:3b:86:d8:73:37", OperState: "up",
			Alias: "c9s:interposition:v1",
		},
		{
			Name: "c9sm0", Index: 5, MTU: 1450, MAC: "06:c9:ac:14:14:54", OperState: "unknown",
			Alias: "c9s:interposition:v1",
		},
		{
			Name: "c9ss73388031", Index: 6, MTU: 9500, MAC: "3e:9c:45:56:67:2f", OperState: "up",
			Alias: "c9s:direct:v1:e332:transport:98d2",
		},
		{
			Name: link, Index: 7, MTU: 9500, MAC: "fe:0b:87:7a:1d:5a", OperState: linkState,
			Alias: "c9s:direct:v1:e332:transport:98d2",
		},
	}
}

func interfaceTestSample(
	node string,
	ts time.Time,
	links []clabwireInterface,
	ifaces []c9sIfaceState,
) interfaceSample {
	stats := make([]c9sIfaceStats, 0, len(ifaces))
	for _, iface := range ifaces {
		stats = append(stats, c9sIfaceStats{Name: iface.Name, RxBytes: uint64(ts.Unix())})
	}

	return interfaceSample{
		target: interfaceSampleTarget{
			state: &clablabruntime.LabState{Name: "lab1", Namespace: "c9s-lab1"},
			node: clablabruntime.NodeState{
				Name: node, Pod: "lab1-" + node + "-abc",
				ContainerState: clablabruntime.ContainerStateRunning,
			},
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "lab1-" + node + "-abc"}},
		},
		stats:     stats,
		ifaces:    ifaces,
		hasStates: true,
		links:     links,
		timestamp: ts,
	}
}

func interfacesByName(events []clablabruntime.Event) map[string]clablabruntime.Event {
	byName := map[string]clablabruntime.Event{}
	for _, ev := range events {
		byName[ev.Interface.Name] = ev
	}

	return byName
}

func TestNetlinkOperState(t *testing.T) {
	t.Parallel()

	for sysfs, want := range map[string]string{
		"up":             "up",
		"down":           "down",
		"unknown":        "unknown",
		"dormant":        "dormant",
		"testing":        "testing",
		"notpresent":     "not-present",
		"lowerlayerdown": "lower-layer-down",
		"":               "unknown",
	} {
		if got := netlinkOperState(sysfs); got != want {
			t.Fatalf("netlinkOperState(%q) = %q, want %q", sysfs, got, want)
		}
	}
}

// TestInterfaceEventDataNamesInterfacesLikeDocker checks the names, aliases, types and states a
// Docker container of the same node kind reports: linux eth1 without an alias, SR Linux e1-1
// with the topology's ethernet-1/1, eth0 and loopback, and no clabernetes alias.
func TestInterfaceEventDataNamesInterfacesLikeDocker(t *testing.T) {
	t.Parallel()

	ifaces := map[string]c9sIfaceState{}
	for _, iface := range podInterfaces("e1-1", "lowerlayerdown") {
		ifaces[iface.Name] = iface
	}
	ifaces["eth1"] = c9sIfaceState{
		Name: "eth1", Index: 8, MTU: 9500, OperState: "up", Alias: "c9s:direct:v1:x",
	}

	tests := []struct {
		iface, alias string
		want         clablabruntime.InterfaceEventData
	}{
		{"eth1", "", clablabruntime.InterfaceEventData{
			Name: "eth1", Index: 8, MTU: 9500, OperState: "up", Type: "veth",
		}},
		{"e1-1", "ethernet-1/1", clablabruntime.InterfaceEventData{
			Name: "e1-1", Alias: "ethernet-1/1", Index: 7, MTU: 9500, MAC: "fe:0b:87:7a:1d:5a",
			OperState: "lower-layer-down", Type: "veth",
		}},
		{"eth0", "", clablabruntime.InterfaceEventData{
			Name: "eth0", Index: 4, MTU: 1450, MAC: "3a:3b:86:d8:73:37", OperState: "up",
			Type: "veth",
		}},
		{"lo", "", clablabruntime.InterfaceEventData{
			Name: "lo", Index: 1, MTU: 65536, OperState: "unknown", Type: "device",
		}},
	}
	for _, tt := range tests {
		if got := interfaceEventData(ifaces[tt.iface], tt.alias); got != tt.want {
			t.Fatalf("%s = %+v, want %+v", tt.iface, got, tt.want)
		}
	}

	// an alias set inside the node is the interface's alias, as netlink reports it
	got := interfaceEventData(c9sIfaceState{Name: "eth2", Alias: "uplink"}, "")
	if got.Alias != "uplink" {
		t.Fatalf("own alias = %+v", got)
	}
}

// TestInterfacePollerReportsTheTopologyInterfaces runs a linux and an SR Linux node through the
// poller: only the lab link interfaces (named and aliased as clabwire lists them), eth0 and lo
// are reported, with the actions of the netlink events.
func TestInterfacePollerReportsTheTopologyInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		link  string
		alias string
	}{
		{name: "linux", link: "eth1"},
		{name: "srl", link: "e1-1", alias: "ethernet-1/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			links := []clabwireInterface{{
				Node: "r1", Interface: tt.link, Alias: tt.alias,
				PeerNode: "r2", PeerInterface: tt.link,
			}}
			sink := make(chan clablabruntime.Event, 64)
			p := newInterfacePoller(&Runtime{}, eventScope{},
				interfacePollOptions{includeInitialState: true}, sink)
			ctx := context.Background()
			start := time.Now()

			p.processSample(ctx, interfaceTestSample("r1", start, links, podInterfaces(tt.link, "up")))
			got := interfacesByName(drain(sink))
			if len(got) != 3 {
				t.Fatalf("first round reported %v, want %s, eth0 and lo", got, tt.link)
			}
			link := got[tt.link]
			if link.Action != interfaceActionSnapshot || link.Type != clablabruntime.EventTypeInterface ||
				link.ActorID != "c9s-lab1/lab1/r1" || link.ActorFullID != "c9s-lab1/lab1/r1" ||
				link.ActorName != "lab1-r1" || link.Interface.Alias != tt.alias ||
				link.Interface.OperState != "up" || link.Interface.Type != "veth" ||
				link.Lab == nil || link.Node == nil || link.Node.Name != "r1" ||
				link.Attributes["origin"] != "clabernetes" ||
				link.Attributes["pod"] != "lab1-r1-abc" || link.Attributes["namespace"] != "c9s-lab1" {
				t.Fatalf("%s event = %+v", tt.link, link)
			}
			if got["lo"].Interface.Type != "device" || got["eth0"].Interface.Alias != "" {
				t.Fatalf("eth0/lo = %+v %+v", got["eth0"].Interface, got["lo"].Interface)
			}

			// unchanged: nothing
			p.firstRound = false
			p.processSample(ctx, interfaceTestSample("r1", start.Add(time.Second), links,
				podInterfaces(tt.link, "up")))
			if events := drain(sink); len(events) != 0 {
				t.Fatalf("unchanged interfaces reported %+v", events)
			}

			// the peer goes away: the link loses its carrier
			p.processSample(ctx, interfaceTestSample("r1", start.Add(2*time.Second), links,
				podInterfaces(tt.link, "lowerlayerdown")))
			events := drain(sink)
			if len(events) != 1 || events[0].Action != interfaceActionUpdate ||
				events[0].Interface.Name != tt.link ||
				events[0].Interface.OperState != "lower-layer-down" {
				t.Fatalf("carrier loss reported %+v", events)
			}

			// the link interface disappears, then comes back
			withoutLink := podInterfaces(tt.link, "up")
			withoutLink = withoutLink[:len(withoutLink)-1]
			p.processSample(ctx, interfaceTestSample("r1", start.Add(3*time.Second), links, withoutLink))
			events = drain(sink)
			if len(events) != 1 || events[0].Action != interfaceActionDelete ||
				events[0].Interface.Name != tt.link {
				t.Fatalf("removed link reported %+v", events)
			}
			p.processSample(ctx, interfaceTestSample("r1", start.Add(4*time.Second), links,
				podInterfaces(tt.link, "up")))
			events = drain(sink)
			if len(events) != 1 || events[0].Action != interfaceActionCreate ||
				events[0].Interface.Alias != tt.alias {
				t.Fatalf("new link reported %+v", events)
			}

			// the node stops: its interfaces are deleted
			p.retireStoppedNodes(ctx, map[string]bool{})
			got = interfacesByName(drain(sink))
			if len(got) != 3 || got[tt.link].Action != interfaceActionDelete ||
				got["eth0"].Action != interfaceActionDelete || len(p.nodes) != 0 {
				t.Fatalf("stopped node reported %+v", got)
			}
		})
	}
}

func TestInterfacePollerFirstRoundWithoutInitialState(t *testing.T) {
	t.Parallel()

	sink := make(chan clablabruntime.Event, 16)
	p := newInterfacePoller(&Runtime{}, eventScope{}, interfacePollOptions{}, sink)
	links := []clabwireInterface{{Node: "r1", Interface: "eth1"}}
	p.processSample(context.Background(),
		interfaceTestSample("r1", time.Now(), links, podInterfaces("eth1", "up")))
	if events := drain(sink); len(events) != 0 {
		t.Fatalf("first round without --initial-state reported %+v", events)
	}
}

func TestInterfacePollerStatsCoverTheReportedInterfaces(t *testing.T) {
	t.Parallel()

	sink := make(chan clablabruntime.Event, 32)
	p := newInterfacePoller(&Runtime{}, eventScope{}, interfacePollOptions{includeStats: true}, sink)
	links := []clabwireInterface{{Node: "r1", Interface: "e1-1", Alias: "ethernet-1/1"}}
	start := time.Now()
	first := interfaceTestSample("r1", start, links, podInterfaces("e1-1", "up"))
	second := interfaceTestSample("r1", start.Add(time.Second), links, podInterfaces("e1-1", "up"))
	for idx := range second.stats {
		second.stats[idx].RxBytes = first.stats[idx].RxBytes + 1000
	}

	p.processSample(context.Background(), first)
	_ = drain(sink)
	p.processSample(context.Background(), second)
	got := interfacesByName(drain(sink))
	if len(got) != 3 {
		t.Fatalf("stats reported for %v, want e1-1, eth0 and lo", got)
	}
	stats := got["e1-1"]
	if stats.Action != interfaceActionStats || stats.Interface.Alias != "ethernet-1/1" ||
		stats.Interface.Index != 7 || stats.Interface.OperState != "up" ||
		stats.Interface.Stats == nil || stats.Interface.Stats.RxBps != 8000 ||
		stats.Interface.Stats.Interval != time.Second {
		t.Fatalf("e1-1 stats = %+v %+v", stats, stats.Interface)
	}
}

// listingExec answers clabwire --list-interfaces with the listing of the pod and counts the
// listings.
type listingExec struct {
	mu       sync.Mutex
	listings map[string][]clabwireInterface
	calls    int
}

func (l *listingExec) exec(
	_ context.Context,
	_, pod, container string,
	command []string,
	_ io.Reader,
	stdout, _ io.Writer,
) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if container != clabwireContainer || !slices.Contains(command, "--list-interfaces") {
		return nil
	}
	l.calls++

	return json.NewEncoder(stdout).Encode(l.listings[pod])
}

func TestInterfacePollerListsAPodOncePerInterfaceSet(t *testing.T) {
	t.Parallel()

	listing := &listingExec{listings: map[string][]clabwireInterface{
		"lab1-r1-abc": {
			{Node: "r1", Interface: "e1-1", Alias: "ethernet-1/1"},
			{Node: "r1b", Interface: "e1-2"},
		},
	}}
	r := &Runtime{podExec: listing.exec}
	p := newInterfacePoller(r, eventScope{}, interfacePollOptions{}, make(chan clablabruntime.Event))
	target := interfaceTestSample("r1", time.Now(), nil, nil).target

	for range 3 {
		links, err := p.nodeLinks(context.Background(), target, "e1-1,eth0,lo")
		if err != nil {
			t.Fatal(err)
		}
		if len(links) != 1 || links[0].Alias != "ethernet-1/1" {
			t.Fatalf("links = %+v, want the node's own link", links)
		}
	}
	if listing.calls != 1 {
		t.Fatalf("listed %d times, want once while the interfaces stay the same", listing.calls)
	}

	if _, err := p.nodeLinks(context.Background(), target, "e1-1,e1-2,eth0,lo"); err != nil {
		t.Fatal(err)
	}
	if listing.calls != 2 {
		t.Fatalf("listed %d times, want again after the interfaces changed", listing.calls)
	}

	p.forgetListings(map[string]bool{})
	if len(p.listings) != 0 {
		t.Fatalf("listings of gone pods kept: %+v", p.listings)
	}
}

func TestContainerTransitionActions(t *testing.T) {
	t.Parallel()

	const (
		created = clablabruntime.ContainerStateCreated
		running = clablabruntime.ContainerStateRunning
		exited  = clablabruntime.ContainerStateExited
		healthy = clablabruntime.ContainerHealthHealthy
		start   = clablabruntime.ContainerHealthStarting
		sick    = clablabruntime.ContainerHealthUnhealthy
	)
	tracked := func(state, health, pod string) *trackedContainer {
		return &trackedContainer{state: state, health: health, pod: pod}
	}

	tests := []struct {
		name     string
		previous *trackedContainer
		next     *trackedContainer
		want     string
	}{
		{"new pending node", nil, tracked(created, "", ""), "create"},
		{"new running node", nil, tracked(running, start, "p1"),
			"create,start,health_status: starting"},
		{"new ready node", nil, tracked(running, healthy, "p1"),
			"create,start,health_status: healthy"},
		{"new stopped node", nil, tracked(exited, "", ""), "create,stop"},
		{"pod starts", tracked(created, "", "p1"), tracked(running, start, "p1"),
			"start,health_status: starting"},
		{"device ready", tracked(running, start, "p1"), tracked(running, healthy, "p1"),
			"health_status: healthy"},
		{"still booting", tracked(running, start, "p1"), tracked(running, start, "p1"), ""},
		{"no change", tracked(running, healthy, "p1"), tracked(running, healthy, "p1"), ""},
		{"device not ready again", tracked(running, healthy, "p1"), tracked(running, sick, "p1"),
			"health_status: unhealthy"},
		{"stop", tracked(running, healthy, "p1"), tracked(exited, "", ""), "kill,stop,die"},
		{"start", tracked(exited, "", ""), tracked(running, healthy, "p2"),
			"start,health_status: healthy"},
		{"restart, new pod pending", tracked(running, healthy, "p1"), tracked(created, "", "p2"),
			"kill,stop,die"},
		{"restart, new pod running", tracked(running, healthy, "p1"),
			tracked(running, start, "p2"), "kill,stop,die,start,health_status: starting"},
		{"stopped before it ran", tracked(created, "", "p1"), tracked(exited, "", ""), "stop"},
		{"stopped node rescheduled", tracked(exited, "", ""), tracked(created, "", "p2"), ""},
	}
	for _, tt := range tests {
		got := strings.Join(containerTransitionActions(tt.previous, tt.next), ",")
		if got != tt.want {
			t.Fatalf("%s: actions = %q, want %q", tt.name, got, tt.want)
		}
	}

	if got := strings.Join(containerRemovalActions(tracked(running, healthy, "p1")), ","); got !=
		"kill,die,destroy" {
		t.Fatalf("removal of a running node = %q", got)
	}
	if got := strings.Join(containerRemovalActions(tracked(exited, "", "")), ","); got != "destroy" {
		t.Fatalf("removal of a stopped node = %q", got)
	}

	// health: a running node that was healthy and is not ready anymore in the same pod is
	// unhealthy; in a new pod it starts again
	previous := tracked(running, healthy, "p1")
	if got := containerHealth(previous, tracked(running, "", "p1"), start); got != sick {
		t.Fatalf("health in the same pod = %q", got)
	}
	if got := containerHealth(previous, tracked(running, "", "p2"), start); got != start {
		t.Fatalf("health in a new pod = %q", got)
	}
}

// trackerTestNode is a Node of lab1 in lab-ns.
func trackerTestNode(name string, runState string, conditions ...map[string]any) *unstructured.Unstructured {
	status := map[string]any{
		"readiness":        "notready",
		"directContainers": []any{map[string]any{"name": "node-" + name, "id": name}},
	}
	rawConditions := []any{}
	for _, condition := range conditions {
		rawConditions = append(rawConditions, condition)
		if condition["type"] == "Ready" && condition["status"] == "True" {
			status["readiness"] = "ready"
		}
	}
	status["conditions"] = rawConditions

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name": name, "namespace": "lab-ns", "uid": "uid-" + name,
			"labels": map[string]any{
				labelTopologyOwner: "lab1", labelOriginalNodeName: strings.ToUpper(name),
			},
		},
		"spec":   map[string]any{"kind": "linux", "image": "alpine", "runState": runState},
		"status": status,
	}}
}

func condition(conditionType, status string) map[string]any {
	return map[string]any{"type": conditionType, "status": status}
}

func trackerTestPod(name string, phase corev1.PodPhase) *corev1.Pod {
	pod := captureTestPod(name, "r1")
	pod.Status.Phase = phase

	return pod
}

type trackerHarness struct {
	t       *testing.T
	r       *Runtime
	tracker *containerTracker
	sink    chan clablabruntime.Event
}

func newTrackerHarness(t *testing.T, node *unstructured.Unstructured, pods ...*corev1.Pod) *trackerHarness {
	t.Helper()

	kubeObjects := make([]k8sruntime.Object, 0, len(pods))
	for _, pod := range pods {
		kubeObjects = append(kubeObjects, pod)
	}
	r := newTestRuntimeWithKubeObjects([]*unstructured.Unstructured{node}, kubeObjects)
	r.labNamespaceOverride = "lab-ns"
	sink := make(chan clablabruntime.Event, 64)

	return &trackerHarness{
		t: t, r: r, sink: sink,
		tracker: &containerTracker{r: r, sink: sink, nodes: map[string]*trackedContainer{}},
	}
}

func (h *trackerHarness) seed(snapshot bool) {
	h.t.Helper()

	state, err := h.r.Inspect(context.Background(), clablabruntime.InspectRequest{Name: "lab1"})
	if err != nil {
		h.t.Fatal(err)
	}
	h.tracker.observeLab(context.Background(), labRef{namespace: "lab-ns", lab: "lab1"}, state,
		&observeSeed{snapshot: snapshot})
}

func (h *trackerHarness) setNode(node *unstructured.Unstructured) {
	h.t.Helper()

	if _, err := h.r.client.Resource(nodeGVR).Namespace("lab-ns").
		Update(context.Background(), node, metav1.UpdateOptions{}); err != nil {
		h.t.Fatal(err)
	}
}

func (h *trackerHarness) deletePod(name string) {
	h.t.Helper()

	if err := h.r.kubeClient.CoreV1().Pods("lab-ns").
		Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		h.t.Fatal(err)
	}
}

func (h *trackerHarness) addPod(pod *corev1.Pod) {
	h.t.Helper()

	if _, err := h.r.kubeClient.CoreV1().Pods("lab-ns").
		Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		h.t.Fatal(err)
	}
}

// refresh reads lab1 again and returns the reported actions with their state attribute.
func (h *trackerHarness) refresh() string {
	h.t.Helper()

	if !h.tracker.refresh(context.Background(), labRef{namespace: "lab-ns", lab: "lab1"}) {
		h.t.Fatal("refresh failed")
	}

	var actions []string
	for _, ev := range drain(h.sink) {
		if ev.Type != clablabruntime.EventTypeContainer || ev.ActorID != "lab-ns/lab1/r1" ||
			ev.ActorFullID != "lab-ns/lab1/r1" || ev.ActorName != "lab1-r1" ||
			ev.Lab == nil || ev.Node == nil || ev.Node.TopologyName != "R1" ||
			ev.Attributes["lab"] != "lab1" || ev.Attributes["node"] != "r1" ||
			ev.Attributes["namespace"] != "lab-ns" {
			h.t.Fatalf("container event = %+v", ev)
		}
		action := ev.Action + "=" + ev.Attributes["state"]
		if signal := ev.Attributes["signal"]; signal != "" {
			action += "/" + signal
		}
		actions = append(actions, action)
	}

	return strings.Join(actions, ",")
}

// TestContainerTrackerReportsTheNodeLifecycle drives one node through boot, readiness, stop,
// start, restart and removal on the fake cluster and checks the Docker container events.
func TestContainerTrackerReportsTheNodeLifecycle(t *testing.T) {
	t.Parallel()

	node := trackerTestNode("r1", nodeRunStateRunning)
	h := newTrackerHarness(t, node, trackerTestPod("lab1-r1-a", corev1.PodPending))
	h.seed(true)
	snapshot := drain(h.sink)
	if len(snapshot) != 1 || snapshot[0].Action != "created" ||
		snapshot[0].Attributes["origin"] != clablabruntime.EventOriginSnapshot ||
		snapshot[0].Attributes["state"] != "created" {
		t.Fatalf("snapshot = %+v", snapshot)
	}

	// the pod runs, the device boots
	h.deletePod("lab1-r1-a")
	h.addPod(trackerTestPod("lab1-r1-a", corev1.PodRunning))
	if got := h.refresh(); got != "start=running,health_status: starting=running" {
		t.Fatalf("pod running: %s", got)
	}

	// the device is ready
	h.setNode(trackerTestNode("r1", nodeRunStateRunning,
		condition("Ready", "True"), condition(nodeConditionContainersReady, "True")))
	if got := h.refresh(); got != "health_status: healthy=running" {
		t.Fatalf("ready: %s", got)
	}
	if got := h.refresh(); got != "" {
		t.Fatalf("unchanged: %s", got)
	}

	// stop: the Node is stopped and its pod goes
	h.setNode(trackerTestNode("r1", nodeRunStateStopped, condition(nodeConditionStopped, "True")))
	h.deletePod("lab1-r1-a")
	if got := h.refresh(); got != "kill=running/15,stop=exited,die=exited" {
		t.Fatalf("stop: %s", got)
	}

	// start: a new pod
	h.setNode(trackerTestNode("r1", nodeRunStateRunning))
	h.addPod(trackerTestPod("lab1-r1-b", corev1.PodPending))
	if got := h.refresh(); got != "" {
		t.Fatalf("start requested: %s", got)
	}
	h.deletePod("lab1-r1-b")
	h.addPod(trackerTestPod("lab1-r1-b", corev1.PodRunning))
	h.setNode(trackerTestNode("r1", nodeRunStateRunning, condition(nodeConditionContainersReady, "True")))
	if got := h.refresh(); got != "start=running,health_status: healthy=running" {
		t.Fatalf("started: %s", got)
	}

	// restart: the pod is replaced
	h.deletePod("lab1-r1-b")
	h.addPod(trackerTestPod("lab1-r1-c", corev1.PodRunning))
	if got := h.refresh(); got !=
		"kill=running/15,stop=exited,die=exited,start=running,health_status: healthy=running" {
		t.Fatalf("restart: %s", got)
	}

	// destroy
	if err := h.r.client.Resource(nodeGVR).Namespace("lab-ns").
		Delete(context.Background(), "r1", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := h.refresh(); got != "kill=running/15,die=exited,destroy=exited" {
		t.Fatalf("destroy: %s", got)
	}
	if len(h.tracker.nodes) != 0 {
		t.Fatalf("destroyed node still tracked: %+v", h.tracker.nodes)
	}
}

func TestContainerTrackerSeedsWithoutSnapshot(t *testing.T) {
	t.Parallel()

	h := newTrackerHarness(t, trackerTestNode("r1", nodeRunStateRunning,
		condition("Ready", "True")), trackerTestPod("lab1-r1-a", corev1.PodRunning))
	h.seed(false)
	if events := drain(h.sink); len(events) != 0 {
		t.Fatalf("seed without --initial-state reported %+v", events)
	}
	if got := h.refresh(); got != "" {
		t.Fatalf("unchanged after seed: %s", got)
	}
}

func TestNodeContainerState(t *testing.T) {
	t.Parallel()

	deleting := metav1.NewTime(time.Now())
	pod := func(phase corev1.PodPhase, deleted bool) *corev1.Pod {
		p := &corev1.Pod{Status: corev1.PodStatus{Phase: phase}}
		if deleted {
			p.DeletionTimestamp = &deleting
		}

		return p
	}

	tests := []struct {
		name          string
		stopRequested bool
		pod           *corev1.Pod
		want          string
	}{
		{"no pod yet", false, nil, "created"},
		{"pod scheduled, init containers", false, pod(corev1.PodPending, false), "created"},
		{"device containers run", false, pod(corev1.PodRunning, false), "running"},
		{"pod being deleted (restart)", false, pod(corev1.PodRunning, true), "exited"},
		{"pod ended", false, pod(corev1.PodFailed, false), "exited"},
		{"stop requested, pod terminating", true, pod(corev1.PodRunning, true), "exited"},
		{"stopped", true, nil, "exited"},
	}
	for _, tt := range tests {
		if got := nodeContainerState(tt.stopRequested, tt.pod); got != tt.want {
			t.Fatalf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNewestPodPrefersAPodThatIsNotDeleted(t *testing.T) {
	t.Parallel()

	now := time.Now()
	deleting := metav1.NewTime(now)
	old := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "old", CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
			DeletionTimestamp: &deleting,
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	replacement := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "new", CreationTimestamp: metav1.NewTime(now)},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	if got := newestPod(old, replacement); got != replacement {
		t.Fatalf("newestPod = %s, want the replacement", got.Name)
	}
	if got := newestPod(replacement, old); got != replacement {
		t.Fatalf("newestPod = %s, want the replacement", got.Name)
	}
}

func TestEnrichStateReportsContainerStateAndHealth(t *testing.T) {
	t.Parallel()

	r := newTestRuntimeWithKubeObjects(
		[]*unstructured.Unstructured{
			trackerTestNode("r1", nodeRunStateRunning, condition("Ready", "True")),
			// its peer is stopped: not ready, but the device is
			trackerTestNode("r2", nodeRunStateRunning,
				condition(nodeConditionContainersReady, "True")),
			trackerTestNode("r3", nodeRunStateRunning),
			trackerTestNode("r4", nodeRunStateStopped, condition(nodeConditionStopped, "True")),
		},
		[]k8sruntime.Object{
			captureTestPod("lab1-r1-a", "r1"),
			captureTestPod("lab1-r2-a", "r2"),
			captureTestPod("lab1-r3-a", "r3"),
		},
	)

	state := &clablabruntime.LabState{Name: "lab1", Namespace: "lab-ns"}
	if err := r.enrichState(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"r1": "running/healthy", "r2": "running/healthy", "r3": "running/starting", "r4": "exited/",
	}
	for _, node := range state.Nodes {
		if got := node.ContainerState + "/" + node.Health; got != want[node.Name] {
			t.Fatalf("%s = %s, want %s", node.Name, got, want[node.Name])
		}
		if node.TopologyName != strings.ToUpper(node.Name) {
			t.Fatalf("%s topology name = %q", node.Name, node.TopologyName)
		}
	}
}

const sourceDefinition = `name: lab1
topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
    node2:
      kind: linux
      image: alpine:latest
  links:
    - endpoints: ["node1:eth1", "node2:eth1"]
`

func sourceDeployRequest(path string, noTopologyCR bool) clablabruntime.DeployRequest {
	return clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyFile:       path,
		TopologyDefinition: []byte(sourceDefinition),
		NoTopologyCR:       noTopologyCR,
	}
}

// TestDeployRecordsTheTopologyFile checks where a deploy records the topology file and that
// inspect reports it, with and without a Topology resource.
func TestDeployRecordsTheTopologyFile(t *testing.T) {
	t.Parallel()

	// with a Topology resource: on the Topology, updated by every deploy
	r := newTestRuntime()
	if _, err := r.Deploy(context.Background(),
		sourceDeployRequest("/home/alice/labs/lab1.clab.yml", false)); err != nil {
		t.Fatal(err)
	}
	topology := getTestTopology(t, r, "lab-ns", "lab1")
	if got := topology.GetAnnotations()[annotationTopologyFile]; got != "/home/alice/labs/lab1.clab.yml" {
		t.Fatalf("Topology annotation = %q", got)
	}
	state, err := r.Inspect(context.Background(),
		clablabruntime.InspectRequest{Name: "lab1", Namespace: "lab-ns"})
	if err != nil {
		t.Fatal(err)
	}
	if state.TopologyPath != "/home/alice/labs/lab1.clab.yml" {
		t.Fatalf("TopologyPath = %q", state.TopologyPath)
	}

	if _, err := r.Deploy(context.Background(),
		sourceDeployRequest("/srv/labs/lab1.clab.yml", false)); err != nil {
		t.Fatal(err)
	}
	topology = getTestTopology(t, r, "lab-ns", "lab1")
	if got := topology.GetAnnotations()[annotationTopologyFile]; got != "/srv/labs/lab1.clab.yml" {
		t.Fatalf("Topology annotation after a deploy from another file = %q", got)
	}

	// without a Topology resource: on the Nodes only, kept from their creation
	r = newTestRuntime()
	if _, err := r.Deploy(context.Background(),
		sourceDeployRequest("/home/alice/labs/lab1.clab.yml", true)); err != nil {
		t.Fatal(err)
	}
	assertPrimitiveTopologyFile(t, r, "/home/alice/labs/lab1.clab.yml")
	state, err = r.Inspect(context.Background(),
		clablabruntime.InspectRequest{Name: "lab1", Namespace: "lab-ns"})
	if err != nil {
		t.Fatal(err)
	}
	if state.TopologyPath != "/home/alice/labs/lab1.clab.yml" {
		t.Fatalf("TopologyPath without a Topology = %q", state.TopologyPath)
	}

	plan, err := r.Plan(context.Background(), sourceDeployRequest("/srv/labs/lab1.clab.yml", true))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Kind == "Node" {
			t.Fatalf("a deploy from another file would change %+v and replace its pod", change)
		}
	}
	if _, err := r.Deploy(context.Background(),
		sourceDeployRequest("/srv/labs/lab1.clab.yml", true)); err != nil {
		t.Fatal(err)
	}
	assertPrimitiveTopologyFile(t, r, "/home/alice/labs/lab1.clab.yml")
}

func assertPrimitiveTopologyFile(t *testing.T, r *Runtime, want string) {
	t.Helper()

	for _, gvr := range []struct {
		name string
		want string
	}{{"nodes", want}, {"links", ""}, {"nodeprofiles", ""}} {
		resource := nodeGVR
		switch gvr.name {
		case "links":
			resource = linkGVR
		case "nodeprofiles":
			resource = nodeProfileGVR
		}
		list, err := r.client.Resource(resource).Namespace("lab-ns").
			List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for idx := range list.Items {
			if got := list.Items[idx].GetAnnotations()[annotationTopologyFile]; got != gvr.want {
				t.Fatalf("%s %s annotation = %q, want %q", gvr.name, list.Items[idx].GetName(),
					got, gvr.want)
			}
		}
	}
}

func TestCarryOverNodeTopologyFile(t *testing.T) {
	t.Parallel()

	node := func(annotations map[string]any) *unstructured.Unstructured {
		metadata := map[string]any{"name": "node1"}
		if annotations != nil {
			metadata["annotations"] = annotations
		}

		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion, "kind": "Node", "metadata": metadata,
			"spec": map[string]any{"image": "alpine"},
		}}
	}
	desired := node(map[string]any{annotationTopologyFile: "/new.clab.yml"})

	updated := reconciledPrimitiveObject(node(map[string]any{annotationTopologyFile: "/old.clab.yml"}), desired)
	if got := updated.GetAnnotations()[annotationTopologyFile]; got != "/old.clab.yml" {
		t.Fatalf("existing Node file = %q, want it kept", got)
	}

	// a Node deployed before containerlab recorded files does not get one
	existing := node(nil)
	updated = reconciledPrimitiveObject(existing, desired)
	if _, ok := updated.GetAnnotations()[annotationTopologyFile]; ok ||
		!primitiveObjectsConform(existing, updated) {
		t.Fatalf("Node without a file = %+v", updated.GetAnnotations())
	}

	// the Topology always records the latest file
	topology := topologyObject("lab1", "lab-ns", "", "topology:\n")
	setTopologySourceFile(topology, "/old.clab.yml")
	newTopology := topologyObject("lab1", "lab-ns", "", "topology:\n")
	setTopologySourceFile(newTopology, "/new.clab.yml")
	updated = reconciledPrimitiveObject(topology, newTopology)
	if got := updated.GetAnnotations()[annotationTopologyFile]; got != "/new.clab.yml" {
		t.Fatalf("Topology file = %q", got)
	}
}

func TestManifestsCarryNoTopologyFile(t *testing.T) {
	t.Parallel()

	for _, noTopologyCR := range []bool{false, true} {
		manifests, err := (&Runtime{}).Manifests(context.Background(),
			sourceDeployRequest("/home/alice/labs/lab1.clab.yml", noTopologyCR))
		if err != nil {
			t.Fatal(err)
		}
		for _, manifest := range manifests {
			data, _ := json.Marshal(manifest.Object)
			if strings.Contains(string(data), annotationTopologyFile) {
				t.Fatalf("no-topology-cr=%t: %s %s carries the file", noTopologyCR,
					manifest.Kind, manifest.Name)
			}
		}
	}
}

func TestStateReportsTheRecordedTopologyFile(t *testing.T) {
	t.Parallel()

	topology := topologyObject("lab1", "lab-ns", "", "topology:\n")
	if got := stateFromTopology(topology, "lab-ns").TopologyPath; got != "k8s://lab-ns/topologies/lab1" {
		t.Fatalf("lab created by other means = %q", got)
	}
	setTopologySourceFile(topology, "/home/alice/lab1.clab.yml")
	if got := stateFromTopology(topology, "lab-ns").TopologyPath; got != "/home/alice/lab1.clab.yml" {
		t.Fatalf("deployed lab = %q", got)
	}

	nodes := []unstructured.Unstructured{*trackerTestNode("r1", nodeRunStateRunning)}
	if got := stateFromNodeResources("lab1", "lab-ns", nodes).TopologyPath; got !=
		"k8s://lab-ns/labs/lab1" {
		t.Fatalf("Nodes created by other means = %q", got)
	}
	nodes[0].SetAnnotations(map[string]string{annotationTopologyFile: "/home/alice/lab1.clab.yml"})
	if got := stateFromNodeResources("lab1", "lab-ns", nodes).TopologyPath; got !=
		"/home/alice/lab1.clab.yml" {
		t.Fatalf("deployed Nodes = %q", got)
	}
}
