package clabernetes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

// accessTestRuntime is a lab "lab1" in namespace lab-ns with node r1 (a sanitized "R1") in pod
// lab1-r1-abc, whose device container is node-r1-x next to the clabwire sidecar.
func accessTestRuntime(t *testing.T) *Runtime {
	t.Helper()

	node := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "r1",
			"namespace": "lab-ns",
			"uid":       "uid-r1",
			"labels":    map[string]any{labelTopologyOwner: "lab1"},
		},
		"spec": map[string]any{"kind": "linux"},
		"status": map[string]any{
			"readiness": "ready",
			"directContainers": []any{
				map[string]any{"name": "node-r1-x", "id": "r1"},
			},
			"exposedPorts": map[string]any{
				"ports": []any{map[string]any{
					"exposePort": int64(22), "destinationPort": int64(22), "protocol": "TCP",
				}},
			},
		},
	}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lab1-r1-abc",
			Namespace: "lab-ns",
			Labels: map[string]string{
				labelApp:           clabernetesAppValue,
				labelTopologyOwner: "lab1",
				labelTopologyNode:  "r1",
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	r := newTestRuntimeWithKubeObjects(
		[]*unstructured.Unstructured{node},
		[]k8sruntime.Object{pod},
	)
	r.labNamespaceOverride = "lab-ns"

	return r
}

func TestLocateNode(t *testing.T) {
	t.Parallel()

	r := accessTestRuntime(t)

	all, err := r.LocateNode(context.Background(), clablabruntime.NodeRequest{Name: "lab1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("locations = %+v, want r1", all)
	}

	// the topology spells the node "R1"; Kubernetes carries it as "r1"
	named, err := r.LocateNode(context.Background(), clablabruntime.NodeRequest{
		Name:  "lab1",
		Nodes: []string{"R1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := named[0]
	if got.NodeName != "r1" || got.Namespace != "lab-ns" || got.Pod != "lab1-r1-abc" ||
		got.Container != "node-r1-x" || got.NodeUID != "uid-r1" ||
		len(got.ExposedPorts) != 1 || got.ExposedPorts[0].Port != 22 {
		t.Fatalf("location = %+v", got)
	}

	if _, err := r.LocateNode(context.Background(), clablabruntime.NodeRequest{
		Name:  "lab1",
		Nodes: []string{"missing"},
	}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("unknown node error = %v", err)
	}
}

func TestStreamLogsTargetsTheDeviceContainer(t *testing.T) {
	t.Parallel()

	r := accessTestRuntime(t)
	tail := int64(25)

	var out strings.Builder
	if err := r.StreamLogs(context.Background(), clablabruntime.LogRequest{
		Name:       "lab1",
		NodeName:   "R1",
		Tail:       &tail,
		Follow:     true,
		Timestamps: true,
	}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "fake logs" {
		t.Fatalf("log output = %q", out.String())
	}

	var opts *corev1.PodLogOptions
	for _, action := range r.kubeClient.(*kubefake.Clientset).Actions() {
		if action.GetSubresource() != "log" {
			continue
		}
		generic, ok := action.(k8stesting.GenericActionImpl)
		if !ok {
			continue
		}
		opts, _ = generic.Value.(*corev1.PodLogOptions)
	}
	if opts == nil || opts.Container != "node-r1-x" || !opts.Follow || !opts.Timestamps ||
		opts.TailLines == nil || *opts.TailLines != 25 {
		t.Fatalf("pod log options = %+v", opts)
	}

	if err := r.StreamLogs(context.Background(), clablabruntime.LogRequest{
		Name:      "lab1",
		NodeName:  "r1",
		Container: "clabwire",
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestDialPortValidatesTheRequest(t *testing.T) {
	t.Parallel()

	r := accessTestRuntime(t)
	for _, tt := range []struct {
		nodes []string
		port  int
	}{
		{nodes: nil, port: 22},
		{nodes: []string{"r1", "r2"}, port: 22},
		{nodes: []string{"r1"}, port: 0},
		{nodes: []string{"r1"}, port: 70000},
	} {
		if _, err := r.DialPort(context.Background(), clablabruntime.NodeRequest{
			Name: "lab1", Nodes: tt.nodes,
		}, tt.port); err == nil {
			t.Fatalf("DialPort(%v, %d) error = nil", tt.nodes, tt.port)
		}
	}
}

func podMetrics(name string, containers map[string][2]string) *unstructured.Unstructured {
	entries := make([]any, 0, len(containers))
	for container, usage := range containers {
		entries = append(entries, map[string]any{
			"name":  container,
			"usage": map[string]any{"cpu": usage[0], "memory": usage[1]},
		})
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1",
		"kind":       "PodMetrics",
		"metadata":   map[string]any{"name": name, "namespace": "lab-ns"},
		"timestamp":  "2026-10-27T10:00:00Z",
		"window":     "15s",
		"containers": entries,
	}}
}

func TestSampleStatsSumsTheDeviceContainers(t *testing.T) {
	t.Parallel()

	r := accessTestRuntime(t)
	// PodMetrics is served as the "pods" resource of metrics.k8s.io
	if err := r.client.(*dynamicfake.FakeDynamicClient).Tracker().Create(
		podMetricsGVR,
		podMetrics("lab1-r1-abc", map[string][2]string{
			"node-r1-x": {"250000000n", "100Mi"},
			"clabwire":  {"500m", "1Gi"},
		}),
		"lab-ns",
	); err != nil {
		t.Fatal(err)
	}

	stats, err := r.SampleStats(context.Background(), clablabruntime.InspectRequest{Name: "lab1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	got := stats[0]
	if got.NodeName != "r1" || got.Pod != "lab1-r1-abc" || got.Namespace != "lab-ns" ||
		got.CPUMilliCores != 250 || got.MemoryBytes != 100*1024*1024 ||
		got.Window != 15*time.Second || got.Timestamp.IsZero() {
		t.Fatalf("stats = %+v, want the device container only", got)
	}
}

func TestSampleStatsReportsAMissingMetricsAPI(t *testing.T) {
	t.Parallel()

	r := accessTestRuntime(t)
	r.client.(*dynamicfake.FakeDynamicClient).PrependReactor("list", "pods",
		func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
			if action.GetResource().Group != podMetricsGVR.Group {
				return false, nil, nil
			}
			return true, nil, apierrors.NewNotFound(
				schema.GroupResource{Group: podMetricsGVR.Group, Resource: "pods"}, "")
		})

	_, err := r.SampleStats(context.Background(), clablabruntime.InspectRequest{Name: "lab1"})
	if !errors.Is(err, clablabruntime.ErrMetricsUnavailable) {
		t.Fatalf("SampleStats() error = %v, want ErrMetricsUnavailable", err)
	}
	if !strings.Contains(err.Error(), "metrics API (metrics.k8s.io) not available") {
		t.Fatalf("error message = %q", err)
	}
}

func TestTerminalSizeQueue(t *testing.T) {
	t.Parallel()

	resize := make(chan clablabruntime.TerminalSize, 1)
	queue := newTerminalSizeQueue(context.Background(), resize)

	resize <- clablabruntime.TerminalSize{Width: 120, Height: 40}
	if size := queue.Next(); size == nil || size.Width != 120 || size.Height != 40 {
		t.Fatalf("Next() = %+v", size)
	}

	close(resize)
	if size := queue.Next(); size != nil {
		t.Fatalf("Next() after close = %+v, want nil", size)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if size := newTerminalSizeQueue(
		ctx,
		make(chan clablabruntime.TerminalSize),
	).Next(); size != nil {
		t.Fatalf("Next() after cancel = %+v, want nil", size)
	}
}

// impersonationRecorder is an API server stub that records the impersonation headers of every
// request and answers with a Status error.
type impersonationRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	method string
	path   string
	user   string
	groups []string
}

func (rec *impersonationRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rec.mu.Lock()
	rec.requests = append(rec.requests, recordedRequest{
		method: req.Method,
		path:   req.URL.Path,
		user:   req.Header.Get("Impersonate-User"),
		groups: req.Header.Values("Impersonate-Group"),
	})
	rec.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = io.WriteString(w,
		`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`)
}

func (rec *impersonationRecorder) take() []recordedRequest {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	requests := rec.requests
	rec.requests = nil

	return requests
}

func TestIdentityImpersonatesOnEveryTransport(t *testing.T) {
	t.Parallel()

	recorder := &impersonationRecorder{}
	server := httptest.NewServer(recorder)
	defer server.Close()

	r := &Runtime{
		identity: clablabruntime.Identity{User: "clab:default"},
		clients: &clientLoader{
			loadConfig: func() (*rest.Config, string, error) {
				return &rest.Config{Host: server.URL}, "", nil
			},
			newClients: newKubeClients,
		},
	}
	alice := clablabruntime.Identity{
		User:   "clab:alice",
		Groups: []string{"clab:users", "clab:admins"},
	}

	view, err := r.connect(alice)
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.connect(alice)
	if err != nil || again.kubeClient != view.kubeClient {
		t.Fatal("clients of one identity were not cached")
	}
	defaultView, err := r.connect(clablabruntime.Identity{})
	if err != nil || defaultView.kubeClient == view.kubeClient {
		t.Fatal("the configured identity shares the clients of another identity")
	}

	ctx := context.Background()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "lab1-r1-abc", Namespace: "lab-ns"}}

	// REST
	_, _ = r.Inspect(ctx, clablabruntime.InspectRequest{Name: "lab1", Identity: alice})
	// pod exec: websocket first, then SPDY
	_, _, _, _ = view.execInPod(ctx, pod, "node-r1-x", []string{"true"})
	// port-forward over SPDY
	if conn, err := view.dialPodPort("lab-ns", "lab1-r1-abc", 22); err == nil {
		_ = conn.Close()
	}
	// a call without identity uses the configured one
	_, _ = r.Inspect(ctx, clablabruntime.InspectRequest{Name: "lab1"})

	requests := recorder.take()
	seen := map[string]bool{}
	for _, req := range requests {
		switch {
		case strings.HasSuffix(req.path, "/exec"):
			seen["exec-"+req.method] = true
		case strings.HasSuffix(req.path, "/portforward"):
			seen["portforward"] = true
		default:
			seen["rest-"+req.user] = true
		}

		if req.user == "clab:default" {
			if len(req.groups) != 0 {
				t.Fatalf("configured identity sent groups: %+v", req)
			}
			continue
		}
		if req.user != alice.User || strings.Join(req.groups, ",") != "clab:users,clab:admins" {
			t.Fatalf("request without alice's impersonation: %+v", req)
		}
	}

	for _, want := range []string{
		"rest-clab:alice", "exec-GET", "exec-POST", "portforward", "rest-clab:default",
	} {
		if !seen[want] {
			t.Fatalf("no %s request recorded: %+v", want, requests)
		}
	}
}

func TestNoIdentityLeavesTheKubeconfigAsItIs(t *testing.T) {
	t.Parallel()

	recorder := &impersonationRecorder{}
	server := httptest.NewServer(recorder)
	defer server.Close()

	r := &Runtime{clients: &clientLoader{
		loadConfig: func() (*rest.Config, string, error) {
			return &rest.Config{Host: server.URL}, "", nil
		},
		newClients: newKubeClients,
	}}

	_, _ = r.Inspect(context.Background(), clablabruntime.InspectRequest{Name: "lab1"})
	for _, req := range recorder.take() {
		if req.user != "" || len(req.groups) != 0 {
			t.Fatalf("request impersonates without an identity: %+v", req)
		}
	}
}
