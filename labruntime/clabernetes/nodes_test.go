package clabernetes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
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

func runStateTestNode(name string, extraSpec map[string]any) *unstructured.Unstructured {
	spec := map[string]any{"kind": "linux", "image": "alpine:3", "runState": nodeRunStateRunning}
	for key, value := range extraSpec {
		spec[key] = value
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":       name,
			"namespace":  "lab-ns",
			"generation": int64(1),
			"labels":     map[string]any{labelTopologyOwner: "lab1"},
		},
		"spec": spec,
		"status": map[string]any{
			"readiness": nodeReadinessReady,
			"conditions": []any{map[string]any{
				"type": nodeConditionStopped, "status": "False", "reason": "Running",
				"observedGeneration": int64(1),
			}},
		},
	}}
}

// fakeNodeController plays the clabernetes Node controller in the dynamic fake: a patch bumps
// the Node generation like the API server does for spec changes, and after lag reads of a Node
// the controller reports the requested run state and restart token in its status.
type fakeNodeController struct {
	t       *testing.T
	tracker k8stesting.ObjectTracker
	lag     int

	// peers are the link peers of each node; a running node with a stopped peer stays not
	// ready with ready containers, as clabernetes reports it
	peers map[string][]string

	mu      sync.Mutex
	reads   map[string]int
	patches []k8stesting.PatchAction
}

func newFakeNodeController(t *testing.T, r *Runtime, lag int) *fakeNodeController {
	t.Helper()

	client := r.client.(*dynamicfake.FakeDynamicClient)
	controller := &fakeNodeController{
		t: t, tracker: client.Tracker(), lag: lag, reads: map[string]int{},
	}
	client.PrependReactor("patch", "nodes", controller.patch)
	client.PrependReactor("get", "nodes", controller.get)

	return controller
}

func (c *fakeNodeController) patch(action k8stesting.Action) (bool, k8sruntime.Object, error) {
	patch := action.(k8stesting.PatchAction)

	c.mu.Lock()
	c.patches = append(c.patches, patch)
	c.reads[patch.GetName()] = 0
	c.mu.Unlock()

	var body map[string]any
	if err := json.Unmarshal(patch.GetPatch(), &body); err != nil {
		c.t.Errorf("patch is not JSON: %v", err)
	}
	if _, changesSpec := body["spec"]; changesSpec {
		obj, err := c.tracker.Get(nodeGVR, patch.GetNamespace(), patch.GetName())
		if err != nil {
			return true, nil, err
		}
		node := obj.(*unstructured.Unstructured)
		node.SetGeneration(node.GetGeneration() + 1)
		if err := c.tracker.Update(nodeGVR, node, patch.GetNamespace()); err != nil {
			return true, nil, err
		}
	}

	// the default reaction applies the patch
	return false, nil, nil
}

func (c *fakeNodeController) get(action k8stesting.Action) (bool, k8sruntime.Object, error) {
	get := action.(k8stesting.GetAction)

	c.mu.Lock()
	c.reads[get.GetName()]++
	reconcile := c.reads[get.GetName()] > c.lag
	c.mu.Unlock()
	if !reconcile {
		return false, nil, nil
	}

	obj, err := c.tracker.Get(nodeGVR, get.GetNamespace(), get.GetName())
	if err != nil {
		return false, nil, nil //nolint:nilerr // the default reaction reports it
	}
	node := obj.(*unstructured.Unstructured).DeepCopy()

	status := map[string]any{"readiness": nodeReadinessReady}
	condition := map[string]any{
		"type": nodeConditionStopped, "status": "False", "reason": "Running",
		"observedGeneration": node.GetGeneration(),
	}
	containers := map[string]any{"type": nodeConditionContainersReady, "status": "True"}
	if nodeRunState(node) == nodeRunStateStopped {
		status["readiness"] = "notready"
		condition["status"], condition["reason"] = "True", "Stopped"
		containers["status"] = "Unknown"
	}
	for _, peer := range c.peers[node.GetName()] {
		peerObj, err := c.tracker.Get(nodeGVR, get.GetNamespace(), peer)
		if err == nil && nodeRunState(peerObj.(*unstructured.Unstructured)) == nodeRunStateStopped {
			status["readiness"] = "notready"
		}
	}
	status["conditions"] = []any{condition, containers}
	if token := node.GetAnnotations()[restartRequestedAnnotation]; token != "" &&
		nodeRunState(node) != nodeRunStateStopped {
		status["observedRestartToken"] = token
	}
	node.Object["status"] = status
	if err := c.tracker.Update(nodeGVR, node, get.GetNamespace()); err != nil {
		c.t.Errorf("update status: %v", err)
	}

	return false, nil, nil
}

func (c *fakeNodeController) patchBodies() map[string]map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	bodies := map[string]map[string]any{}
	for _, patch := range c.patches {
		if patch.GetPatchType() != "application/merge-patch+json" {
			c.t.Errorf("patch type = %s, want a merge patch", patch.GetPatchType())
		}
		var body map[string]any
		_ = json.Unmarshal(patch.GetPatch(), &body)
		bodies[patch.GetName()] = body
	}

	return bodies
}

// runStateTestRuntime is lab1 in lab-ns with nodes r1 and r2, and r2b grouped onto r2
// (network-mode container:r2).
func runStateTestRuntime(t *testing.T, lag int) (*Runtime, *fakeNodeController) {
	t.Helper()

	r := newTestRuntime(
		runStateTestNode("r1", nil),
		runStateTestNode("r2", nil),
		runStateTestNode("r2b", map[string]any{"network-mode": "container:r2"}),
	)
	r.labNamespaceOverride = "lab-ns"
	r.pollInterval = time.Millisecond

	return r, newFakeNodeController(t, r, lag)
}

func assertNoWorkloadAccess(t *testing.T, r *Runtime) {
	t.Helper()

	for _, action := range r.kubeClient.(*kubefake.Clientset).Actions() {
		if action.GetResource().Group == "apps" ||
			(action.GetResource().Resource == "pods" && action.GetVerb() != "list" &&
				action.GetVerb() != "get" && action.GetVerb() != "watch") {
			t.Fatalf("node lifecycle touched workloads: %s %s", action.GetVerb(),
				action.GetResource())
		}
	}
	for _, action := range r.client.(*dynamicfake.FakeDynamicClient).Actions() {
		if action.GetResource().Resource == "topologies" && action.GetVerb() != "get" &&
			action.GetVerb() != "list" {
			t.Fatalf("node lifecycle changed the Topology: %s", action.GetVerb())
		}
		if action.GetVerb() == "update" {
			t.Fatalf("node lifecycle updated %s instead of patching it", action.GetResource())
		}
	}
}

func TestStopPatchesThePrimaryRunStateAndWaitsForStopped(t *testing.T) {
	t.Parallel()

	r, controller := runStateTestRuntime(t, 3)

	// r2b shares the pod of r2, so stopping it stops r2, once
	err := r.Stop(context.Background(), clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"R2B", "r2"},
	})
	if err != nil {
		t.Fatal(err)
	}

	bodies := controller.patchBodies()
	if len(bodies) != 1 {
		t.Fatalf("patched nodes = %v, want only the primary r2", bodies)
	}
	want := `{"spec":{"runState":"Stopped"}}`
	if got, _ := json.Marshal(bodies["r2"]); string(got) != want {
		t.Fatalf("stop patch = %s, want %s", got, want)
	}

	node := getTestPrimitive(t, r, nodeGVR, "lab-ns", "r2")
	condition := nodeCondition(node, nodeConditionStopped)
	if condition == nil || condition.status != "True" {
		t.Fatalf("Stop returned before the controller reported Stopped: %+v", node.Object["status"])
	}
	assertNoWorkloadAccess(t, r)

	// the state reports the group as stopped
	state := &clablabruntime.LabState{Name: "lab1", Namespace: "lab-ns"}
	if err := r.enrichState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, n := range state.Nodes {
		states[n.Name] = n.State
	}
	if states["r2"] != "stopped" || states["r2b"] != "stopped" || states["r1"] != "ready" {
		t.Fatalf("node states = %v, want r2 and r2b stopped, r1 ready", states)
	}
}

func TestStartWaitsForTheObservedGenerationAndReadiness(t *testing.T) {
	t.Parallel()

	r, controller := runStateTestRuntime(t, 2)
	ctx := context.Background()

	if err := r.Stop(ctx, clablabruntime.NodeRequest{Name: "lab1"}); err != nil {
		t.Fatal(err)
	}
	if bodies := controller.patchBodies(); len(bodies) != 2 || bodies["r2b"] != nil {
		t.Fatalf("stopping the lab patched %v, want the primaries r1 and r2", bodies)
	}

	// The status still says Running from before the stop at the old generation, which must
	// not count as started.
	stale := getTestPrimitive(t, r, nodeGVR, "lab-ns", "r1")
	if ok, _ := nodeReachedRunState(stale, nodeRunStateRunning, stale.GetGeneration()+1); ok {
		t.Fatal("a condition of an older generation counted as started")
	}

	if err := r.Start(ctx, clablabruntime.NodeRequest{Name: "lab1", Nodes: []string{"r1"}}); err != nil {
		t.Fatal(err)
	}
	node := getTestPrimitive(t, r, nodeGVR, "lab-ns", "r1")
	if nodeRunState(node) != nodeRunStateRunning || nodeReadiness(node) != nodeReadinessReady {
		t.Fatalf("Start returned before r1 was ready: %+v", node.Object)
	}
	if got := nodeRunState(getTestPrimitive(t, r, nodeGVR, "lab-ns", "r2")); got != nodeRunStateStopped {
		t.Fatalf("starting r1 changed r2 to %s", got)
	}
	assertNoWorkloadAccess(t, r)
}

func TestRestartSetsANewTokenAndWaitsForIt(t *testing.T) {
	t.Parallel()

	r, controller := runStateTestRuntime(t, 2)
	ctx := context.Background()

	if err := r.Restart(ctx, clablabruntime.NodeRequest{Name: "lab1", Nodes: []string{"r1"}}); err != nil {
		t.Fatal(err)
	}
	first := controller.patchBodies()["r1"]
	metadata, _ := first["metadata"].(map[string]any)
	annotations, _ := metadata["annotations"].(map[string]any)
	token, _ := annotations[restartRequestedAnnotation].(string)
	if token == "" || first["spec"] != nil {
		t.Fatalf("restart patch = %v, want only the restart-requested annotation", first)
	}
	node := getTestPrimitive(t, r, nodeGVR, "lab-ns", "r1")
	if observed, _, _ := unstructured.NestedString(
		node.Object, "status", "observedRestartToken",
	); observed != token {
		t.Fatalf("Restart returned before the token was observed: %q, want %q", observed, token)
	}
	if strings.Contains(string(mustJSON(t, first)), "restart-tokens") {
		t.Fatal("the restart patch set the controller-owned restart-tokens annotation")
	}

	// a stopped node is started by the restart patch, with a different token
	if err := r.Stop(ctx, clablabruntime.NodeRequest{Name: "lab1", Nodes: []string{"r2"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Restart(ctx, clablabruntime.NodeRequest{Name: "lab1", Nodes: []string{"r2b"}}); err != nil {
		t.Fatal(err)
	}
	second := controller.patchBodies()["r2"]
	spec, _ := second["spec"].(map[string]any)
	if spec["runState"] != nodeRunStateRunning {
		t.Fatalf("restart of a stopped node = %v, want runState Running", second)
	}
	secondToken := second["metadata"].(map[string]any)["annotations"].(map[string]any)[restartRequestedAnnotation]
	if secondToken == token {
		t.Fatal("two restarts used the same token")
	}
	assertNoWorkloadAccess(t, r)
}

func TestRunStateWaitTimeoutNamesThePendingNodes(t *testing.T) {
	t.Parallel()

	r, _ := runStateTestRuntime(t, 1_000_000)

	err := r.Stop(context.Background(), clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"r1"}, Timeout: 30 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Stop() error = nil, want a timeout")
	}
	for _, want := range []string{"stop", "r1", "condition Stopped=False (Running)"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("timeout error %q does not mention %q", err, want)
		}
	}
}

func TestNodeLifecycleRejectsUnknownNodes(t *testing.T) {
	t.Parallel()

	r, _ := runStateTestRuntime(t, 0)
	err := r.Stop(context.Background(), clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"missing"},
	})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("Stop() error = %v, want the unknown node named", err)
	}

	empty := newTestRuntime()
	empty.labNamespaceOverride = "lab-ns"
	if err := empty.Start(context.Background(), clablabruntime.NodeRequest{Name: "lab1"}); err == nil ||
		!strings.Contains(err.Error(), "no Node resources") {
		t.Fatalf("Start() on a lab without Nodes = %v", err)
	}
}

func TestPrimitivePlanAndApplyKeepTheRunState(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3.20
`
	r := newTestRuntime()
	request := clablabruntime.DeployRequest{
		Name: "lab1", Namespace: "lab-ns", TopologyDefinition: []byte(definition),
		NoTopologyCR: true,
	}
	ctx := context.Background()
	if _, err := r.Deploy(ctx, request); err != nil {
		t.Fatal(err)
	}

	node := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	if _, found, _ := unstructured.NestedString(node.Object, "spec", "runState"); found {
		t.Fatal("containerlab sent a runState; it belongs to stop and start")
	}
	// the API server defaults runState; then the node is stopped
	_ = unstructured.SetNestedField(node.Object, nodeRunStateStopped, "spec", "runState")
	if _, err := r.client.Resource(nodeGVR).Namespace("lab-ns").
		Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	plan, err := r.Plan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Kind == "Node" {
			t.Fatalf("plan = %+v, want no Node change for a run state difference", plan.Changes)
		}
	}

	if _, err := r.Deploy(ctx, request); err != nil {
		t.Fatal(err)
	}
	if got := nodeRunState(getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")); got != nodeRunStateStopped {
		t.Fatalf("runState after apply = %s, want Stopped", got)
	}
}

func TestManifestsNeverCarryARunState(t *testing.T) {
	t.Parallel()

	for _, noTopologyCR := range []bool{false, true} {
		req := manifestsTestRequest(t)
		req.Namespace = "lab-ns"
		req.NoTopologyCR = noTopologyCR

		manifests, err := newTestRuntime().Manifests(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		for _, manifest := range manifests {
			if strings.Contains(string(mustJSON(t, manifest.Object)), "runState") {
				t.Fatalf("%s %s carries a runState (noTopologyCR=%v)", manifest.Kind,
					manifest.Name, noTopologyCR)
			}
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func readOpenAPIFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestCheckNodeRunStateSchema(t *testing.T) {
	t.Parallel()

	if err := checkNodeRunStateSchema(
		readOpenAPIFixture(t, "openapi-c9s-v1alpha1-runstate.json"),
	); err != nil {
		t.Fatalf("schema with runState rejected: %v", err)
	}

	err := checkNodeRunStateSchema(readOpenAPIFixture(t, "openapi-c9s-v1alpha1-0.10.json"))
	if !errors.Is(err, clablabruntime.ErrUnsupportedRuntimeVersion) {
		t.Fatalf("clabernetes 0.10 schema: error = %v, want ErrUnsupportedRuntimeVersion", err)
	}
	for _, want := range []string{"spec.runState", "0.11 or newer", "clabwire capture contract"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}

	// a $ref to the spec schema, and an enum without Stopped
	refDoc := `{"components":{"schemas":{
		"node":{"x-kubernetes-group-version-kind":[{"group":"c9s.run","version":"v1alpha1","kind":"Node"}],
			"properties":{"spec":{"allOf":[{"$ref":"#/components/schemas/spec"}]}}},
		"spec":{"properties":{"runState":{"type":"string","enum":["Running"]}}}}}}`
	if err := checkNodeRunStateSchema([]byte(refDoc)); err == nil ||
		!strings.Contains(err.Error(), "does not accept") {
		t.Fatalf("enum without Stopped: error = %v", err)
	}
	if err := checkNodeRunStateSchema([]byte(`{"components":{"schemas":{}}}`)); !errors.Is(
		err, clablabruntime.ErrUnsupportedRuntimeVersion) {
		t.Fatalf("no Node schema: error = %v", err)
	}
	if err := checkNodeRunStateSchema([]byte(`not json`)); err == nil ||
		errors.Is(err, clablabruntime.ErrUnsupportedRuntimeVersion) {
		t.Fatalf("malformed document: error = %v, want a parse error", err)
	}
}

func TestSupportCheckRunsOnceBeforeTheFirstClusterCall(t *testing.T) {
	t.Parallel()

	var fetches atomic.Int32
	fixture := readOpenAPIFixture(t, "openapi-c9s-v1alpha1-0.10.json")
	r := newTestRuntime(runStateTestNode("r1", nil))
	r.labNamespaceOverride = "lab-ns"
	r.support = &supportCheck{fetch: func(context.Context, *Runtime) ([]byte, error) {
		fetches.Add(1)
		return fixture, nil
	}}

	ctx := context.Background()
	for range 2 {
		err := r.Stop(ctx, clablabruntime.NodeRequest{Name: "lab1"})
		if !errors.Is(err, clablabruntime.ErrUnsupportedRuntimeVersion) {
			t.Fatalf("Stop() on clabernetes 0.10 = %v, want ErrUnsupportedRuntimeVersion", err)
		}
	}
	if _, err := r.Inspect(ctx, clablabruntime.InspectRequest{Name: "lab1"}); !errors.Is(
		err, clablabruntime.ErrUnsupportedRuntimeVersion) {
		t.Fatalf("Inspect() on clabernetes 0.10 = %v", err)
	}
	if fetches.Load() != 1 {
		t.Fatalf("the check ran %d times, want once per runtime", fetches.Load())
	}
	if actions := r.client.(*dynamicfake.FakeDynamicClient).Actions(); len(actions) != 0 {
		t.Fatalf("cluster calls before the failed check: %v", actions)
	}

	// offline commands do not check
	req := manifestsTestRequest(t)
	if err := r.Validate(ctx, req); err != nil {
		t.Fatalf("Validate() ran the cluster check: %v", err)
	}
	if _, err := r.Manifests(ctx, req); err != nil {
		t.Fatalf("Manifests() ran the cluster check: %v", err)
	}
	if fetches.Load() != 1 {
		t.Fatal("an offline command ran the cluster check")
	}
}

func TestSupportCheckRetriesFailedRequestsAndRemembersSuccess(t *testing.T) {
	t.Parallel()

	var fetches atomic.Int32
	fixture := readOpenAPIFixture(t, "openapi-c9s-v1alpha1-runstate.json")
	check := &supportCheck{fetch: func(context.Context, *Runtime) ([]byte, error) {
		if fetches.Add(1) == 1 {
			return nil, errors.New("connection refused")
		}
		return fixture, nil
	}}

	ctx := context.Background()
	if err := check.ensure(ctx, nil); err == nil ||
		errors.Is(err, clablabruntime.ErrUnsupportedRuntimeVersion) {
		t.Fatalf("first check = %v, want the request error", err)
	}
	for range 3 {
		if err := check.ensure(ctx, nil); err != nil {
			t.Fatal(err)
		}
	}
	if fetches.Load() != 2 {
		t.Fatalf("fetches = %d, want a retry after the failure and none after success",
			fetches.Load())
	}

	notServed := &supportCheck{fetch: func(context.Context, *Runtime) ([]byte, error) {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, "v1alpha1")
	}}
	if err := notServed.ensure(ctx, nil); !errors.Is(
		err, clablabruntime.ErrUnsupportedRuntimeVersion) ||
		!strings.Contains(err.Error(), "not served") {
		t.Fatalf("missing c9s API = %v", err)
	}
}

func TestSupportCheckReadsTheOpenAPIDocumentAsTheCaller(t *testing.T) {
	t.Parallel()

	fixtures := map[string][]byte{
		"new": readOpenAPIFixture(t, "openapi-c9s-v1alpha1-runstate.json"),
		"old": readOpenAPIFixture(t, "openapi-c9s-v1alpha1-0.10.json"),
	}
	for name, fixture := range fixtures {
		var mu sync.Mutex
		var users []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			users = append(users, req.URL.Path+" as "+req.Header.Get("Impersonate-User"))
			mu.Unlock()
			if req.URL.Path != c9sOpenAPIPath {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		}))

		r := &Runtime{
			clients: &clientLoader{
				loadConfig: func() (*rest.Config, string, error) {
					return &rest.Config{Host: server.URL}, "", nil
				},
				newClients: newKubeClients,
			},
			support: &supportCheck{},
		}
		err := r.CheckSupport(context.Background(), clablabruntime.Identity{User: "clab:alice"})
		server.Close()

		switch name {
		case "new":
			if err != nil {
				t.Fatalf("CheckSupport() against a run state cluster = %v", err)
			}
		case "old":
			if !errors.Is(err, clablabruntime.ErrUnsupportedRuntimeVersion) {
				t.Fatalf("CheckSupport() against clabernetes 0.10 = %v", err)
			}
		}
		if len(users) != 1 || users[0] != c9sOpenAPIPath+" as clab:alice" {
			t.Fatalf("%s: requests = %v, want one OpenAPI read as clab:alice", name, users)
		}
	}
}

func runStateTestLink(a, b string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Link",
		"metadata": map[string]any{
			"name": a + "-" + b, "namespace": "lab-ns",
			"labels": map[string]any{labelTopologyOwner: "lab1"},
		},
		"spec": map[string]any{
			"endpointA": map[string]any{"nodeName": a, "interfaceName": "eth1"},
			"endpointB": map[string]any{"nodeName": b, "interfaceName": "eth1"},
		},
	}}
}

// linkedRunStateTestRuntime is lab1 with client and server linked.
func linkedRunStateTestRuntime(t *testing.T) (*Runtime, *fakeNodeController) {
	t.Helper()

	r := newTestRuntime(
		runStateTestNode("client", nil),
		runStateTestNode("server", nil),
		runStateTestLink("client", "server"),
	)
	r.labNamespaceOverride = "lab-ns"
	r.pollInterval = time.Millisecond
	controller := newFakeNodeController(t, r, 0)
	controller.peers = map[string][]string{"client": {"server"}, "server": {"client"}}

	return r, controller
}

func TestStartAcceptsANodeWhoseLinkPeerIsStopped(t *testing.T) {
	t.Parallel()

	r, _ := linkedRunStateTestRuntime(t)
	ctx := context.Background()

	if err := r.Stop(ctx, clablabruntime.NodeRequest{Name: "lab1"}); err != nil {
		t.Fatal(err)
	}
	// client comes up, but its wire to the stopped server has no far end
	err := r.Start(ctx, clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"client"}, Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start() of a node with a stopped peer = %v", err)
	}
	if got := nodeReadiness(getTestPrimitive(t, r, nodeGVR, "lab-ns", "client")); got == nodeReadinessReady {
		t.Fatal("the fake controller reported client ready despite the stopped peer")
	}
	if err := r.Restart(ctx, clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"client"}, Timeout: 5 * time.Second,
	}); err != nil {
		t.Fatalf("Restart() of a node with a stopped peer = %v", err)
	}
}

func TestLabSettlesWithStoppedNodesAndTheirPeers(t *testing.T) {
	t.Parallel()

	r, _ := linkedRunStateTestRuntime(t)
	ctx := context.Background()
	if err := r.Stop(ctx, clablabruntime.NodeRequest{Name: "lab1", Nodes: []string{"server"}}); err != nil {
		t.Fatal(err)
	}
	// let the fake controller report client as waiting for its peer
	_ = getTestPrimitive(t, r, nodeGVR, "lab-ns", "client")
	if _, err := r.client.Resource(nodeGVR).Namespace("lab-ns").
		Get(ctx, "client", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	state, err := r.Inspect(ctx, clablabruntime.InspectRequest{Name: "lab1"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Ready {
		t.Fatalf("lab with a stopped node reported ready: %+v", state)
	}
	settled, err := r.labSettledWithStoppedNodes(ctx, state)
	if err != nil || !settled {
		t.Fatalf("labSettledWithStoppedNodes() = %v, %v; want settled", settled, err)
	}
	if err := r.waitReady(ctx, "lab1", "lab-ns", time.Second); err != nil {
		t.Fatalf("waitReady() with a stopped node = %v", err)
	}

	// a node whose containers are not ready is still pending
	client := getTestPrimitive(t, r, nodeGVR, "lab-ns", "client")
	states := &labRunStates{
		nodes:        map[string]*unstructured.Unstructured{"client": client},
		networkModes: map[string]string{},
		stoppedPeers: map[string][]string{"client": {"server"}},
	}
	_ = unstructured.SetNestedSlice(client.Object, []any{
		map[string]any{"type": nodeConditionContainersReady, "status": "False"},
	}, "status", "conditions")
	if settled, _ := states.settledNode("client"); settled {
		t.Fatal("a node with containers not ready counted as settled")
	}

	// without stopped nodes nothing settles early
	fresh := newTestRuntime(runStateTestNode("client", nil))
	settled, err = fresh.labSettledWithStoppedNodes(ctx, &clablabruntime.LabState{
		Name: "lab1", Namespace: "lab-ns",
		Nodes: []clablabruntime.NodeState{{Name: "client", State: "notready"}},
	})
	if err != nil || settled {
		t.Fatalf("lab without stopped nodes settled = %v, %v", settled, err)
	}
}

// Objects read back from the API server carry JSON numbers as int64 and float64; the rendered
// ones carry Go ints. A plan against a lab that did not change must report no update.
func TestPrimitiveObjectsConformAfterAnAPIRoundTrip(t *testing.T) {
	t.Parallel()

	req := manifestsTestRequest(t)
	req.NoTopologyCR = true
	req.Namespace = "lab-ns"
	prepared, err := prepareDesiredDeployment(context.Background(), req, "lab-ns")
	if err != nil {
		t.Fatal(err)
	}

	for _, group := range prepared.primitives.groups() {
		for _, obj := range group.objects {
			stored := &unstructured.Unstructured{}
			if err := stored.UnmarshalJSON(mustJSON(t, obj.Object)); err != nil {
				t.Fatal(err)
			}
			if group.kind == "Node" {
				// the API server defaults the run state and prunes fields the schema lacks
				_ = unstructured.SetNestedField(stored.Object, nodeRunStateRunning, "spec", "runState")
				unstructured.RemoveNestedField(stored.Object, "spec", "labels")
				if obj.GetLabels()[labelOriginalNodeName] == "" {
					t.Errorf("Node %s lost its %s label", obj.GetName(), labelOriginalNodeName)
				}
			}
			if !primitiveObjectsConform(stored, reconciledPrimitiveObject(stored, obj)) {
				t.Errorf("%s %s drifts after an API round trip", group.kind, obj.GetName())
			}
		}
	}
}
