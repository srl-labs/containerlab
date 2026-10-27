package clabernetes

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

const offlineDefinition = `name: lab1
topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

func TestNewDoesNotLoadKubeconfig(t *testing.T) {
	t.Setenv(envKubeconfig, "/nonexistent/kubeconfig")
	t.Setenv("KUBECONFIG", "/nonexistent/kubeconfig")

	lr, err := New(clablabruntime.Config{})
	if err != nil {
		t.Fatalf("New() error = %v, want the kubeconfig to be loaded lazily", err)
	}

	req := clablabruntime.DeployRequest{
		Name:               "lab1",
		TopologyDefinition: []byte(offlineDefinition),
	}
	if err := lr.(clablabruntime.TopologyValidator).Validate(context.Background(), req); err != nil {
		t.Fatalf("Validate() error = %v, want offline validation", err)
	}
	manifests, err := lr.(clablabruntime.ManifestEmitter).Manifests(context.Background(), req)
	if err != nil || len(manifests) == 0 {
		t.Fatalf("Manifests() = %d manifests, %v; want offline manifests", len(manifests), err)
	}

	_, err = lr.Inspect(context.Background(), clablabruntime.InspectRequest{Name: "lab1"})
	if err == nil || !strings.Contains(err.Error(), "Kubernetes client config") {
		t.Fatalf("Inspect() error = %v, want the kubeconfig load error", err)
	}
}

func TestConnectLoadsClientsOnce(t *testing.T) {
	t.Parallel()

	var loads atomic.Int32
	r := &Runtime{clients: &clientLoader{
		loadConfig: func() (*rest.Config, string, error) {
			loads.Add(1)
			return &rest.Config{Host: "https://cluster.example"}, "", nil
		},
		newClients: func(cfg *rest.Config) (*kubeClients, error) {
			return &kubeClients{kube: kubefake.NewSimpleClientset(), restConfig: cfg}, nil
		},
	}}

	for range 3 {
		view, err := r.connect()
		if err != nil {
			t.Fatal(err)
		}
		if view.kubeClient == nil || view.restConfig == nil {
			t.Fatal("connected view has no clients")
		}
		if view.namespace != defaultNamespace {
			t.Fatalf("namespace = %q, want %q", view.namespace, defaultNamespace)
		}
		again, err := view.connect()
		if err != nil || again != view {
			t.Fatalf("connect() on a connected view = %p, %v; want the view itself", again, err)
		}
	}
	if loads.Load() != 1 {
		t.Fatalf("kubeconfig loaded %d times, want 1", loads.Load())
	}
	if r.kubeClient != nil {
		t.Fatal("connect() changed the shared runtime instead of returning a view")
	}

	failing := &Runtime{clients: &clientLoader{
		loadConfig: func() (*rest.Config, string, error) {
			return nil, "", errors.New("no kubeconfig")
		},
		newClients: newKubeClients,
	}}
	if _, err := failing.connect(); err == nil {
		t.Fatal("connect() error = nil, want the load error")
	}
}

func TestListNamespacePrecedence(t *testing.T) {
	t.Parallel()

	r := &Runtime{namespace: "ctx-ns"}
	if got := r.listNamespace("", true); got != metav1.NamespaceAll {
		t.Fatalf("all namespaces = %q, want all", got)
	}
	if got := r.listNamespace("", false); got != "ctx-ns" {
		t.Fatalf("no namespace = %q, want the context namespace", got)
	}
	if got := r.listNamespace("req-ns", true); got != "req-ns" {
		t.Fatalf("request namespace = %q, want req-ns", got)
	}

	r.labNamespaceOverride = "override-ns"
	if got := r.listNamespace("", true); got != "override-ns" {
		t.Fatalf("override with all namespaces = %q, want override-ns", got)
	}
	if got := r.listNamespace("req-ns", true); got != "req-ns" {
		t.Fatalf("request namespace with override = %q, want req-ns", got)
	}
}

func TestEventScopeFor(t *testing.T) {
	t.Parallel()

	r := &Runtime{namespace: defaultNamespace}

	tests := []struct {
		name          string
		override      string
		req           clablabruntime.EventStreamRequest
		wantNamespace string
		wantLab       string
		wantSelector  string
	}{
		{
			name:          "all namespaces",
			req:           clablabruntime.EventStreamRequest{AllNamespaces: true},
			wantNamespace: metav1.NamespaceAll,
			wantSelector:  labelTopologyOwner,
		},
		{
			name:          "namespace override",
			override:      "team-a",
			req:           clablabruntime.EventStreamRequest{AllNamespaces: true},
			wantNamespace: "team-a",
			wantSelector:  labelTopologyOwner,
		},
		{
			name:          "lab in its canonical namespace",
			req:           clablabruntime.EventStreamRequest{Name: "lab1"},
			wantNamespace: "c9s-lab1",
			wantLab:       "lab1",
			wantSelector:  labelTopologyOwner + "=lab1",
		},
		{
			name:          "lab in the override namespace",
			override:      "team-a",
			req:           clablabruntime.EventStreamRequest{Name: "lab1"},
			wantNamespace: "team-a",
			wantLab:       "lab1",
			wantSelector:  labelTopologyOwner + "=lab1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			view := *r
			view.labNamespaceOverride = tt.override
			scope, err := view.eventScopeFor(tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if scope.namespace != tt.wantNamespace || scope.lab != tt.wantLab {
				t.Fatalf("scope = %+v, want namespace %q lab %q",
					scope, tt.wantNamespace, tt.wantLab)
			}
			if got := scope.labelSelector(); got != tt.wantSelector {
				t.Fatalf("label selector = %q, want %q", got, tt.wantSelector)
			}
			fieldSelector := scope.topologyListOptions().FieldSelector
			if (tt.wantLab != "") != (fieldSelector == "metadata.name="+tt.wantLab) {
				t.Fatalf("topology field selector = %q for lab %q", fieldSelector, tt.wantLab)
			}
		})
	}
}

func TestStreamEventsWatchesOnlyTheScopedNamespace(t *testing.T) {
	t.Parallel()

	r := newTestRuntime()
	r.labNamespaceOverride = "team-a"

	watched := make(chan string, 4)
	r.kubeClient.(*kubefake.Clientset).PrependWatchReactor("pods",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			watched <- action.GetNamespace() + "|" +
				action.(k8stesting.WatchActionImpl).GetWatchRestrictions().Labels.String()
			return true, watch.NewFake(), nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, _, err := r.StreamEvents(ctx, clablabruntime.EventStreamRequest{
		Name:          "lab1",
		AllNamespaces: false,
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-watched:
		if got != "team-a|"+labelTopologyOwner+"=lab1" {
			t.Fatalf("pod watch = %q, want the lab in namespace team-a", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pods were not watched")
	}
}

func TestNodeStateFromResourceLocatesWorkload(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "srl1", "uid": "uid-1"},
		"status": map[string]any{
			"readiness": "ready",
			"exposedPorts": map[string]any{
				"loadBalancerAddress": "192.0.2.10",
				"ports": []any{
					map[string]any{
						"exposePort": int64(57400), "destinationPort": int64(57400),
						"protocol": "TCP",
					},
					map[string]any{
						"exposePort": int64(22), "destinationPort": int64(22),
						"protocol": "TCP",
					},
					map[string]any{
						"exposePort": int64(161), "destinationPort": int64(161),
						"protocol": "UDP",
					},
				},
			},
			"directContainers": []any{
				map[string]any{"name": "node-srl1-abc", "id": "srl1"},
			},
		},
	}}

	state := nodeStateFromResource(node)
	if state.NodeUID != "uid-1" || state.Container != "node-srl1-abc" ||
		state.LoadBalancerAddress != "192.0.2.10" {
		t.Fatalf("node state = %+v", state)
	}
	want := []clablabruntime.ExposedPort{
		{Port: 22, ExposePort: 22, Protocol: "tcp"},
		{Port: 161, ExposePort: 161, Protocol: "udp"},
		{Port: 57400, ExposePort: 57400, Protocol: "tcp"},
	}
	if len(state.ExposedPorts) != len(want) {
		t.Fatalf("exposed ports = %+v, want %+v", state.ExposedPorts, want)
	}
	for idx := range want {
		if state.ExposedPorts[idx] != want[idx] {
			t.Fatalf("exposed ports = %+v, want %+v", state.ExposedPorts, want)
		}
	}
}

func TestEnrichStateReportsPodAndContainer(t *testing.T) {
	t.Parallel()

	nodeResource := func(name string, spec map[string]any, status map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       "Node",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "lab-ns",
				"uid":       "uid-" + name,
				"labels":    map[string]any{labelTopologyOwner: "lab1"},
			},
			"spec":   spec,
			"status": status,
		}}
	}
	pod := func(name, node string, created time.Time, phase corev1.PodPhase) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "lab-ns",
				Labels: map[string]string{
					labelApp:           clabernetesAppValue,
					labelTopologyOwner: "lab1",
					labelTopologyNode:  node,
				},
				Annotations:       map[string]string{defaultContainerAnnotation: "node-" + node},
				CreationTimestamp: metav1.NewTime(created),
			},
			Status: corev1.PodStatus{Phase: phase},
		}
	}

	now := time.Now()
	r := newTestRuntimeWithKubeObjects(
		[]*unstructured.Unstructured{
			// observed device container
			nodeResource("r1", map[string]any{"kind": "linux"}, map[string]any{
				"readiness":        "ready",
				"directContainers": []any{map[string]any{"name": "node-r1-x", "id": "r1"}},
			}),
			// not observed yet: the pod annotation names the container
			nodeResource("r2", map[string]any{"kind": "linux"}, map[string]any{}),
			// shares the pod of r1
			nodeResource("r3", map[string]any{"kind": "linux", "network-mode": "container:r1"},
				map[string]any{
					"directContainers": []any{
						map[string]any{"name": "node-r3-y", "id": "r3"},
					},
				}),
		},
		[]k8sruntime.Object{
			pod("lab1-r1-old", "r1", now.Add(-time.Hour), corev1.PodRunning),
			pod("lab1-r1-new", "r1", now, corev1.PodRunning),
			pod("lab1-r1-pending", "r1", now.Add(time.Minute), corev1.PodPending),
			pod("lab1-r2-a", "r2", now, corev1.PodPending),
		},
	)

	state := &clablabruntime.LabState{Name: "lab1", Namespace: "lab-ns"}
	if err := r.enrichState(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	got := map[string]clablabruntime.NodeState{}
	for _, node := range state.Nodes {
		got[node.Name] = node
	}

	if n := got["r1"]; n.Pod != "lab1-r1-new" || n.Container != "node-r1-x" || n.NodeUID != "uid-r1" {
		t.Fatalf("r1 = %+v, want the newest running pod and the observed container", n)
	}
	if n := got["r2"]; n.Pod != "lab1-r2-a" || n.Container != "node-r2" {
		t.Fatalf("r2 = %+v, want the default-container annotation", n)
	}
	if n := got["r3"]; n.Pod != "lab1-r1-new" || n.Container != "node-r3-y" {
		t.Fatalf("r3 = %+v, want the pod of r1 and its own container", n)
	}
}

func TestParseInterfaceSample(t *testing.T) {
	t.Parallel()

	output := "Inter-|   Receive |  Transmit\n" +
		" face |bytes packets errs drop fifo frame compressed multicast|bytes packets\n" +
		"    lo: 10 1 0 0 0 0 0 0 10 1 0 0 0 0 0 0\n" +
		"  eth1: 2000 20 0 0 0 0 0 0 3000 30 0 0 0 0 0 0\n" +
		interfaceSysfsMarker + "\n" +
		"eth1\t3\t9500\taa:c1:ab:00:00:01\tup\t0x1003\tto server\n" +
		"eth2\t4\t1500\taa:c1:ab:00:00:02\tlowerlayerdown\t0x1003\t\n" +
		"lo\t1\t65536\t00:00:00:00:00:00\tunknown\t0x9\t\n" +
		"dummy0\t5\t1500\t02:00:00:00:00:01\tunknown\t0x1002\t\n"

	stats, states, hasStates, err := parseInterfaceSample([]byte(output))
	if err != nil {
		t.Fatal(err)
	}
	if !hasStates || len(stats) != 2 || len(states) != 4 {
		t.Fatalf("stats = %+v, states = %+v, hasStates = %t", stats, states, hasStates)
	}
	if stats[1].Name != "eth1" || stats[1].RxBytes != 2000 || stats[1].TxPackets != 30 {
		t.Fatalf("eth1 stats = %+v", stats[1])
	}

	wantActions := map[string]string{"eth1": "up", "eth2": "down", "lo": "up", "dummy0": "down"}
	for _, state := range states {
		if got := state.action(); got != wantActions[state.Name] {
			t.Fatalf("%s action = %q, want %q (%+v)", state.Name, got, wantActions[state.Name], state)
		}
	}
	if states[0].Index != 3 || states[0].MTU != 9500 || states[0].MAC != "aa:c1:ab:00:00:01" ||
		states[0].Alias != "to server" || states[0].Flags != 0x1003 {
		t.Fatalf("eth1 state = %+v", states[0])
	}

	stats, states, hasStates, err = parseInterfaceSample([]byte(
		"  eth1: 2000 20 0 0 0 0 0 0 3000 30 0 0 0 0 0 0\n"))
	if err != nil || hasStates || len(states) != 0 || len(stats) != 1 {
		t.Fatalf("plain /proc/net/dev = %+v %+v %t %v", stats, states, hasStates, err)
	}
}

func TestCommandNotExecutable(t *testing.T) {
	t.Parallel()

	if !commandNotExecutable(127, nil) || !commandNotExecutable(126, nil) {
		t.Fatal("exit codes 126/127 must mean the command cannot run")
	}
	if !commandNotExecutable(0, errors.New(`exec: "sh": executable file not found in $PATH`)) {
		t.Fatal("a missing executable must mean the command cannot run")
	}
	if commandNotExecutable(1, errors.New("connection refused")) {
		t.Fatal("other failures are not a missing command")
	}
}

func TestInterfacePollerEmitsStateChanges(t *testing.T) {
	t.Parallel()

	newSample := func(ts time.Time, ifaces ...c9sIfaceState) interfaceSample {
		return interfaceSample{
			target: interfaceSampleTarget{
				state: &clablabruntime.LabState{Name: "lab1", Namespace: "c9s-lab1"},
				node:  clablabruntime.NodeState{Name: "r1", Pod: "lab1-r1-abc"},
				pod:   &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "lab1-r1-abc"}},
			},
			stats: []c9sIfaceStats{
				{Name: "eth1", RxBytes: uint64(ts.Unix())},
			},
			ifaces:    ifaces,
			hasStates: true,
			timestamp: ts,
		}
	}
	up := c9sIfaceState{Name: "eth1", Index: 3, MTU: 9500, OperState: "up", Flags: 0x1003}
	down := c9sIfaceState{Name: "eth1", Index: 3, MTU: 9500, OperState: "down", Flags: 0x1002}
	eth2 := c9sIfaceState{Name: "eth2", Index: 4, OperState: "up", Flags: 0x1003}

	for _, initialState := range []bool{false, true} {
		sink := make(chan clablabruntime.Event, 32)
		p := &interfacePoller{
			r:            &Runtime{},
			opts:         interfacePollOptions{includeInitialState: initialState},
			sink:         sink,
			statsSamples: map[string]c9sIfaceStatsSample{},
			nodes:        map[string]*trackedNode{},
			noShell:      map[string]bool{},
			firstRound:   true,
		}
		ctx := context.Background()
		start := time.Now()

		p.processSample(ctx, newSample(start, up))
		if got := drain(sink); initialState != (len(got) == 1) {
			t.Fatalf("initial-state=%t: first round emitted %+v", initialState, got)
		}

		p.firstRound = false
		p.processSample(ctx, newSample(start.Add(time.Second), up))
		if got := drain(sink); len(got) != 0 {
			t.Fatalf("unchanged interface emitted %+v", got)
		}

		p.processSample(ctx, newSample(start.Add(2*time.Second), down, eth2))
		got := drain(sink)
		if len(got) != 2 {
			t.Fatalf("change emitted %+v, want eth1 down and new eth2 up", got)
		}
		byIf := map[string]clablabruntime.Event{}
		for _, ev := range got {
			byIf[ev.Attributes["ifname"]] = ev
		}
		ev := byIf["eth1"]
		if ev.Type != "interface" || ev.Action != "down" || ev.ActorID != "c9s-lab1/lab1/r1" ||
			ev.ActorName != "lab1-r1" || ev.ActorFullID != "lab1-r1-abc" ||
			ev.Attributes["state"] != "down" || ev.Attributes["index"] != "3" ||
			ev.Attributes["mtu"] != "9500" || ev.Attributes["origin"] != "clabernetes" ||
			ev.Attributes["lab"] != "lab1" || ev.Attributes["node"] != "r1" ||
			ev.Attributes["namespace"] != "c9s-lab1" || ev.Attributes["pod"] != "lab1-r1-abc" {
			t.Fatalf("eth1 event = %+v", ev)
		}
		if byIf["eth2"].Action != "up" {
			t.Fatalf("eth2 event = %+v", byIf["eth2"])
		}

		// eth2 disappears
		p.processSample(ctx, newSample(start.Add(3*time.Second), down))
		got = drain(sink)
		if len(got) != 1 || got[0].Action != "down" ||
			got[0].Attributes["state"] != interfaceStateNotPresent {
			t.Fatalf("removed interface emitted %+v", got)
		}

		// the node stops running
		p.retireStoppedNodes(ctx, map[string]bool{})
		got = drain(sink)
		if len(got) != 1 || got[0].Attributes["ifname"] != "eth1" ||
			got[0].Attributes["state"] != interfaceStateNotPresent || len(p.nodes) != 0 {
			t.Fatalf("stopped node emitted %+v, tracked %+v", got, p.nodes)
		}
	}
}

func TestInterfacePollerStatsUseTheNodeAsActor(t *testing.T) {
	t.Parallel()

	sink := make(chan clablabruntime.Event, 8)
	p := &interfacePoller{
		r:            &Runtime{},
		opts:         interfacePollOptions{includeStats: true},
		sink:         sink,
		statsSamples: map[string]c9sIfaceStatsSample{},
		nodes:        map[string]*trackedNode{},
		noShell:      map[string]bool{},
	}
	target := interfaceSampleTarget{
		state: &clablabruntime.LabState{Name: "lab1", Namespace: "c9s-lab1"},
		node:  clablabruntime.NodeState{Name: "r1", Pod: "lab1-r1-abc"},
		pod:   &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "lab1-r1-abc"}},
	}
	start := time.Now()
	p.processSample(context.Background(), interfaceSample{
		target: target, timestamp: start,
		stats: []c9sIfaceStats{{Name: "eth1", RxBytes: 1000}},
	})
	p.processSample(context.Background(), interfaceSample{
		target: target, timestamp: start.Add(time.Second),
		stats: []c9sIfaceStats{{Name: "eth1", RxBytes: 2000}},
	})

	got := drain(sink)
	if len(got) != 1 || got[0].Action != "stats" || got[0].ActorID != "c9s-lab1/lab1/r1" ||
		got[0].Attributes["rx_bps"] != "8000" {
		t.Fatalf("stats events = %+v", got)
	}
}

func drain(sink chan clablabruntime.Event) []clablabruntime.Event {
	var events []clablabruntime.Event
	for {
		select {
		case ev := <-sink:
			events = append(events, ev)
		default:
			return events
		}
	}
}

func TestTopologyPersistenceRetainsClaims(t *testing.T) {
	t.Parallel()

	obj := topologyObject("lab1", "lab-ns", "", "topology:\n", topologyWithPersistence(true))
	reclaim, _, _ := unstructured.NestedString(
		obj.Object, "spec", "deployment", "persistence", "reclaim")
	if reclaim != persistenceReclaimRetain {
		t.Fatalf("persistence reclaim = %q, want %q", reclaim, persistenceReclaimRetain)
	}
}

func TestDestroyKeepsRetainedClaimsUnlessCleanup(t *testing.T) {
	t.Parallel()

	claim := func(name, lab string, owned bool) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "c9s-lab1",
			Labels:    map[string]string{labelTopologyOwner: lab},
		}}
		if owned {
			pvc.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: c9sAPIVersion, Kind: "Node", Name: name, UID: types.UID("uid"),
			}}
		}
		return pvc
	}
	managedNamespace := labNamespaceObject("lab1", "c9s-lab1")

	for _, tt := range []struct {
		name          string
		cleanup       bool
		claims        []k8sruntime.Object
		wantClaims    []string
		wantNamespace bool
	}{
		{
			name:          "plain destroy keeps retained claims and their namespace",
			claims:        []k8sruntime.Object{claim("r1", "lab1", false), claim("x", "other", false)},
			wantClaims:    []string{"r1", "x"},
			wantNamespace: true,
		},
		{
			// A Node-owned claim is garbage-collected with its Node (the fake clientset
			// does not do that); destroy neither deletes it nor keeps the namespace for it.
			name:       "plain destroy without retained claims deletes the namespace",
			claims:     []k8sruntime.Object{claim("r1", "lab1", true)},
			wantClaims: []string{"r1"},
		},
		{
			name:       "cleanup deletes the claims and the namespace",
			cleanup:    true,
			claims:     []k8sruntime.Object{claim("r1", "lab1", false), claim("r2", "lab1", false)},
			wantClaims: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := append([]k8sruntime.Object{managedNamespace.DeepCopy()}, tt.claims...)
			r := newTestRuntimeWithKubeObjects(nil, objects)
			if err := r.Destroy(context.Background(), clablabruntime.DestroyRequest{
				Name:    "lab1",
				Wait:    tt.cleanup,
				Timeout: 10 * time.Second,
				Cleanup: tt.cleanup,
			}); err != nil {
				t.Fatal(err)
			}

			claims, err := r.kubeClient.CoreV1().PersistentVolumeClaims("c9s-lab1").
				List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for idx := range claims.Items {
				names = append(names, claims.Items[idx].Name)
			}
			if strings.Join(names, ",") != strings.Join(tt.wantClaims, ",") {
				t.Fatalf("claims = %v, want %v", names, tt.wantClaims)
			}

			_, err = r.kubeClient.CoreV1().Namespaces().
				Get(context.Background(), "c9s-lab1", metav1.GetOptions{})
			if tt.wantNamespace != (err == nil) {
				t.Fatalf("namespace exists = %t (%v), want %t", err == nil, err, tt.wantNamespace)
			}
			if !tt.wantNamespace && !apierrors.IsNotFound(err) {
				t.Fatalf("namespace get error = %v, want not found", err)
			}
		})
	}
}
