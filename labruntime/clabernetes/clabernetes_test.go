package clabernetes

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestParseProcNetDev(t *testing.T) {
	t.Parallel()

	data := []byte(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
 eth0: 1234 12 0 0 0 0 0 0 5678 56 0 0 0 0 0 0
`)

	stats, err := parseProcNetDev(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("len(stats) = %d, want 1", len(stats))
	}

	got := stats[0]
	if got.Name != "eth0" ||
		got.RxBytes != 1234 ||
		got.RxPackets != 12 ||
		got.TxBytes != 5678 ||
		got.TxPackets != 56 {
		t.Fatalf("unexpected stats: %+v", got)
	}
}

func TestParseProcNetDevRejectsMalformedLine(t *testing.T) {
	t.Parallel()

	_, err := parseProcNetDev([]byte("eth0: 1 2 3\n"))
	if err == nil {
		t.Fatal("expected malformed /proc/net/dev line to return an error")
	}
}

func TestPreferredDeviceContainerName(t *testing.T) {
	t.Parallel()

	directContainers := func(entries ...map[string]any) *unstructured.Unstructured {
		containers := make([]any, 0, len(entries))
		for _, entry := range entries {
			containers = append(containers, any(entry))
		}

		return &unstructured.Unstructured{Object: map[string]any{
			"status": map[string]any{"directContainers": containers},
		}}
	}

	tests := []struct {
		name      string
		nodeName  string
		node      *unstructured.Unstructured
		want      string
		wantError string
	}{
		{
			name:     "single-container-node",
			nodeName: "router",
			node: directContainers(
				map[string]any{"name": "node-router-abc"},
			),
			want: "node-router-abc",
		},
		{
			name:     "primary-container-preferred",
			nodeName: "router",
			node: directContainers(
				map[string]any{"name": "node-card-one", "componentID": "1"},
				map[string]any{"name": "node-primary", "componentID": ""},
			),
			want: "node-primary",
		},
		{
			name:     "components-prefer-cpm-a",
			nodeName: "srsim",
			node: directContainers(
				map[string]any{"name": "node-two", "componentID": "2"},
				map[string]any{"name": "node-b", "componentID": "b"},
				map[string]any{"name": "node-one", "componentID": "1"},
				map[string]any{"name": "node-a", "componentID": "a"},
			),
			want: "node-a",
		},
		{
			name:     "components-fall-back-to-cpm-b",
			nodeName: "srsim",
			node: directContainers(
				map[string]any{"name": "node-one", "componentID": "1"},
				map[string]any{"name": "node-b", "componentID": "cpm-b"},
			),
			want: "node-b",
		},
		{
			name:      "no-observed-containers",
			nodeName:  "missing",
			node:      &unstructured.Unstructured{Object: map[string]any{}},
			wantError: "was not observed yet",
		},
		{
			name:     "missing-cpm",
			nodeName: "srsim",
			node: directContainers(
				map[string]any{"name": "node-one", "componentID": "1"},
				map[string]any{"name": "node-two", "componentID": "2"},
			),
			wantError: "CPM component container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := preferredDeviceContainerName(tt.node, tt.nodeName)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf(
						"preferredDeviceContainerName() error = %v, want %q",
						err,
						tt.wantError,
					)
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("preferredDeviceContainerName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanTarPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{
			name: "relative path",
			in:   "./configs/startup.json",
			want: "configs/startup.json",
			ok:   true,
		},
		{name: "current directory", in: ".", want: ".", ok: true},
		{name: "parent path", in: "../secret", ok: false},
		{name: "absolute path", in: "/etc/passwd", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := cleanTarPath(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("cleanTarPath(%q) = %q, %v; want %q, %v",
					tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNamespaceForLab(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		labName            string
		explicit           string
		configuredOverride string
		want               string
		wantError          bool
	}{
		{
			name:    "derived from lab name",
			labName: "clos",
			want:    "c9s-clos",
		},
		{
			name:               "configured namespace override",
			labName:            "clos",
			configuredOverride: "default",
			want:               "default",
		},
		{
			name:               "explicit namespace for discovered lab",
			labName:            "clos",
			explicit:           "legacy-namespace",
			configuredOverride: "default",
			want:               "legacy-namespace",
		},
		{
			name:      "invalid derived namespace",
			labName:   strings.Repeat("a", 60),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &Runtime{labNamespaceOverride: tt.configuredOverride}
			got, err := r.namespaceForLab(tt.labName, tt.explicit)
			if tt.wantError {
				if err == nil {
					t.Fatalf("namespaceForLab(%q, %q) returned no error", tt.labName, tt.explicit)
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("namespaceForLab(%q, %q) = %q, want %q",
					tt.labName, tt.explicit, got, tt.want)
			}
		})
	}
}

func TestConfiguredLabNamespacePrecedence(t *testing.T) {
	t.Setenv(envNamespace, "from-environment")

	if got := configuredLabNamespace("from-flag"); got != "from-flag" {
		t.Fatalf("configured namespace = %q, want flag value", got)
	}
	if got := configuredLabNamespace(""); got != "from-environment" {
		t.Fatalf("configured namespace = %q, want environment value", got)
	}

	t.Setenv(envNamespace, "")
	if got := configuredLabNamespace(""); got != "" {
		t.Fatalf("configured namespace = %q, want automatic namespace selection", got)
	}
}

func TestDeployUsesNamespaceOverrideWithoutManagingIt(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime()
	r.labNamespaceOverride = defaultNamespace
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Namespace != defaultNamespace {
		t.Fatalf("deployed namespace = %q, want %q", state.Namespace, defaultNamespace)
	}
	_ = getTestPrimitive(t, r, nodeGVR, defaultNamespace, "node1")

	if err := r.Destroy(context.Background(), clablabruntime.DestroyRequest{
		Name: "lab1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.kubeClient.CoreV1().Namespaces().Get(
		context.Background(),
		defaultNamespace,
		metav1.GetOptions{},
	); err != nil {
		t.Fatalf("namespace override was deleted: %v", err)
	}
}

func TestDeployCreatesAndDestroyRemovesDedicatedLabNamespace(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime()
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Namespace != "c9s-lab1" {
		t.Fatalf("deployed namespace = %q, want c9s-lab1", state.Namespace)
	}

	namespace, err := r.kubeClient.CoreV1().Namespaces().Get(
		context.Background(),
		"c9s-lab1",
		metav1.GetOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if namespace.Labels[labelRuntime] != clabernetesAppValue ||
		namespace.Labels[labelTopologyOwner] != "lab1" {
		t.Fatalf("unexpected namespace labels: %v", namespace.Labels)
	}
	_ = getTestPrimitive(t, r, nodeGVR, "c9s-lab1", "node1")

	if err := r.Destroy(context.Background(), clablabruntime.DestroyRequest{
		Name: "lab1",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = r.kubeClient.CoreV1().Namespaces().Get(
		context.Background(),
		"c9s-lab1",
		metav1.GetOptions{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected managed lab namespace to be deleted, got %v", err)
	}
}

func TestDestroyPreservesPreexistingLabNamespace(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "node1",
			"namespace": "c9s-lab1",
			"labels": map[string]any{
				labelTopologyOwner: "lab1",
			},
		},
	}}
	r := newTestRuntime(node)
	_, err := r.kubeClient.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c9s-lab1"}},
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Destroy(context.Background(), clablabruntime.DestroyRequest{
		Name: "lab1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.kubeClient.CoreV1().Namespaces().Get(
		context.Background(),
		"c9s-lab1",
		metav1.GetOptions{},
	); err != nil {
		t.Fatalf("pre-existing namespace was deleted: %v", err)
	}
}

func TestSavedFilesFromTarSkipsUnsafeEntries(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	writeTarEntry(t, tw, &tar.Header{
		Name: "config.txt",
		Mode: 0o644,
		Size: int64(len("startup")),
	}, []byte("startup"))
	writeTarEntry(t, tw, &tar.Header{
		Name:     "latest",
		Typeflag: tar.TypeSymlink,
		Mode:     0o777,
		Linkname: "config.txt",
	}, nil)
	writeTarEntry(t, tw, &tar.Header{
		Name: "../secret",
		Mode: 0o644,
		Size: int64(len("secret")),
	}, []byte("secret"))

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	files, err := savedFilesFromTar("node1", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2: %+v", len(files), files)
	}
	if files[0].NodeName != "node1" || files[0].Name != "config.txt" ||
		string(files[0].Data) != "startup" {
		t.Fatalf("unexpected regular file: %+v", files[0])
	}
	if files[1].Name != "latest" || files[1].LinkTarget != "config.txt" {
		t.Fatalf("unexpected symlink: %+v", files[1])
	}
}

func TestStateFromTopology(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"topologyReady": true,
				"topologyState": "running",
			},
			"spec": map[string]any{
				"definition": map[string]any{
					"containerlab": `topology:
  nodes:
    client:
      kind: linux
      image: client:latest
    server:
      kind: srl
      image: server:latest
`,
				},
			},
		},
	}
	obj.SetName("lab1")
	obj.SetNamespace("lab-ns")
	obj.SetLabels(map[string]string{clabconstants.Owner: "alice"})

	state := stateFromTopology(obj, "fallback-ns")
	if state.Name != "lab1" ||
		state.Namespace != "lab-ns" ||
		state.Owner != "alice" ||
		state.TopologyPath != "k8s://lab-ns/topologies/lab1" ||
		!state.Ready ||
		state.State != "running" {
		t.Fatalf("unexpected state metadata: %+v", state)
	}
	if len(state.Nodes) != 2 {
		t.Fatalf("len(state.Nodes) = %d, want 2: %+v", len(state.Nodes), state.Nodes)
	}

	client := state.Nodes[0]
	if client.Name != "client" || client.Kind != "linux" || client.Image != "client:latest" {
		t.Fatalf("unexpected client state: %+v", client)
	}

	server := state.Nodes[1]
	if server.Name != "server" || server.Kind != "srl" || server.Image != "server:latest" {
		t.Fatalf("unexpected server state: %+v", server)
	}
}

func TestPrimitiveResourcesUseCurrentC9sAPI(t *testing.T) {
	t.Parallel()

	for name, gvr := range map[string]schema.GroupVersionResource{
		"topology":     topologyGVR,
		"node":         nodeGVR,
		"link":         linkGVR,
		"node profile": nodeProfileGVR,
	} {
		if gvr.Group != "c9s.run" || gvr.Version != "v1alpha1" {
			t.Fatalf("unexpected %s GVR: %+v", name, gvr)
		}
	}
}

func TestPrimitiveResourceGroupOrder(t *testing.T) {
	t.Parallel()

	set := &primitiveResourceSet{}
	groups := set.groups()
	want := []string{"NodeProfile", "Link", "Node"}
	got := make([]string, 0, len(groups))
	for _, group := range groups {
		got = append(got, group.kind)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("primitive creation order = %v, want %v", got, want)
	}
}

func TestPrimitiveLinkPendingReason(t *testing.T) {
	t.Parallel()

	link := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Accepted", "status": "True"},
			},
			"resolvedEndpoints": map[string]any{
				"endpointA": map[string]any{"nodeName": "node1", "uid": "uid-1"},
				"endpointB": map[string]any{"nodeName": "host"},
			},
		},
	}}
	if reason := primitiveLinkPendingReason(link); reason != "" {
		t.Fatalf("resolved link reported pending: %q", reason)
	}

	if err := unstructured.SetNestedSlice(
		link.Object,
		[]any{map[string]any{
			"type":    "Accepted",
			"status":  "False",
			"reason":  "EndpointsUnresolved",
			"message": "endpoint node missing",
		}},
		"status",
		"conditions",
	); err != nil {
		t.Fatal(err)
	}
	if reason := primitiveLinkPendingReason(link); reason != "endpoint node missing" {
		t.Fatalf("pending reason = %q, want Accepted condition message", reason)
	}

	fresh := &unstructured.Unstructured{Object: map[string]any{}}
	if reason := primitiveLinkPendingReason(fresh); reason != "waiting for Accepted" {
		t.Fatalf("pending reason for fresh link = %q, want waiting for Accepted", reason)
	}
}

func TestEnrichStateUsesNodeResources(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "client",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelTopologyOwner: "lab1",
			},
		},
		"spec": map[string]any{
			"kind":  "linux",
			"image": "client:latest",
		},
		"status": map[string]any{
			"readiness": "ready",
			"exposedPorts": map[string]any{
				"loadBalancerAddress": "192.0.2.10",
			},
		},
	}}

	r := newTestRuntime(node)
	state := &clablabruntime.LabState{Name: "lab1", Namespace: "lab-ns"}
	if err := r.enrichState(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	if len(state.Nodes) != 1 {
		t.Fatalf("len(state.Nodes) = %d, want 1: %+v", len(state.Nodes), state.Nodes)
	}
	got := state.Nodes[0]
	if got.Name != "client" || got.Kind != "linux" || got.Image != "client:latest" ||
		!got.Ready || got.State != "ready" || got.LoadBalancerAddress != "192.0.2.10" {
		t.Fatalf("unexpected Node-derived state: %+v", got)
	}
	if !state.Ready || state.State != "running" {
		t.Fatalf("unexpected aggregate state: %+v", state)
	}
}

func TestWaitReadyTimeoutReportsPendingNodes(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "slow-node",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelTopologyOwner: "lab1",
			},
		},
		"spec": map[string]any{
			"kind":  "arbitrary-kind",
			"image": "example.invalid/arbitrary-kind:latest",
		},
		"status": map[string]any{"readiness": "notready"},
	}}

	r := newTestRuntime(node)
	err := r.waitReady(context.Background(), "lab1", "lab-ns", 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected readiness timeout")
	}
	if !strings.Contains(err.Error(), "timed out after 20ms") ||
		!strings.Contains(err.Error(), "pending nodes: slow-node (notready)") ||
		strings.Contains(err.Error(), "rate limiter") {
		t.Fatalf("unexpected readiness timeout: %v", err)
	}
}

func TestNodeReadinessProgressReportsTransitions(t *testing.T) {
	var output bytes.Buffer
	oldLevel := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	log.SetOutput(&output)
	defer func() {
		log.SetLevel(oldLevel)
		log.SetOutput(os.Stderr)
	}()

	progress := nodeReadinessProgress{}
	progress.report(&clablabruntime.LabState{Nodes: []clablabruntime.NodeState{
		{Name: "node1", State: "notready"},
		{Name: "node2", State: "ready", Ready: true},
	}})

	got := output.String()
	if strings.Contains(got, "node=node1") {
		t.Fatalf("initial non-ready node should not produce a log line:\n%s", got)
	}
	if !strings.Contains(got, "Clabernetes node is ready") ||
		!strings.Contains(got, "node=node2") ||
		!strings.Contains(got, "ready=1") ||
		!strings.Contains(got, "total=2") {
		t.Fatalf("initial ready node progress was not reported:\n%s", got)
	}

	output.Reset()
	progress.report(&clablabruntime.LabState{Nodes: []clablabruntime.NodeState{
		{Name: "node1", State: "ready", Ready: true},
		{Name: "node2", State: "ready", Ready: true},
	}})
	got = output.String()
	if strings.Count(got, "Clabernetes node is ready") != 1 ||
		!strings.Contains(got, "node=node1") ||
		!strings.Contains(got, "ready=2") {
		t.Fatalf("newly ready node progress was not reported exactly once:\n%s", got)
	}

	output.Reset()
	progress.report(&clablabruntime.LabState{Nodes: []clablabruntime.NodeState{
		{Name: "node1", State: "ready", Ready: true},
		{Name: "node2", State: "notready"},
	}})
	got = output.String()
	if !strings.Contains(got, "Clabernetes node is not ready") ||
		!strings.Contains(got, "node=node2") ||
		!strings.Contains(got, "state=notready") ||
		!strings.Contains(got, "ready=1") {
		t.Fatalf("readiness regression was not reported:\n%s", got)
	}

	output.Reset()
	progress.report(&clablabruntime.LabState{Nodes: []clablabruntime.NodeState{
		{Name: "node1", State: "ready", Ready: true},
		{Name: "node2", State: "notready"},
	}})
	if output.Len() != 0 {
		t.Fatalf("unchanged readiness should not produce repeated log lines:\n%s", output.String())
	}
}

func TestImagePullProgressReportsKubeletEvents(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-abc",
			Namespace: "lab-ns",
			Labels: map[string]string{
				labelApp:           clabernetesAppValue,
				labelTopologyOwner: "lab1",
				labelTopologyNode:  "node1",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-a",
			InitContainers: []corev1.Container{
				{Name: "planner", Image: "example/c9s:latest"},
			},
			Containers: []corev1.Container{
				{Name: "node-one", Image: "example/node1:latest"},
			},
		},
	}
	event := func(name, reason, message string) *corev1.Event {
		return &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "lab-ns"},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod", Name: "node1-abc", Namespace: "lab-ns",
			},
			Reason:  reason,
			Message: message,
		}
	}

	r := newTestRuntimeWithKubeObjects(nil, []k8sruntime.Object{
		pod,
		event("pull-start", "Pulling", `Pulling image "example/node1:latest"`),
		event("infra-pull", "Pulling", `Pulling image "example/c9s:latest"`),
	})

	var output bytes.Buffer
	oldLevel := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	log.SetOutput(&output)
	defer func() {
		log.SetLevel(oldLevel)
		log.SetOutput(os.Stderr)
	}()

	state := &clablabruntime.LabState{Name: "lab1"}
	progress := imagePullProgress{}
	progress.observe(context.Background(), r, "lab-ns", state)
	got := output.String()
	if !strings.Contains(got, "Pulling clabernetes node image") ||
		!strings.Contains(got, "node=node1") ||
		!strings.Contains(got, "image=example/node1:latest") ||
		!strings.Contains(got, "kubernetes-node=worker-a") {
		t.Fatalf("image pull start was not reported:\n%s", got)
	}
	if strings.Contains(got, "example/c9s:latest") {
		t.Fatalf("infrastructure image pull must not be reported:\n%s", got)
	}

	output.Reset()
	progress.observe(context.Background(), r, "lab-ns", state)
	if output.Len() != 0 {
		t.Fatalf("unchanged image pull should not produce repeated log lines:\n%s", output.String())
	}

	if _, err := r.kubeClient.CoreV1().Events("lab-ns").Create(
		context.Background(),
		event(
			"pull-done",
			"Pulled",
			`Successfully pulled image "example/node1:latest" in 1.5s`,
		),
		metav1.CreateOptions{},
	); err != nil {
		t.Fatal(err)
	}

	output.Reset()
	progress.observe(context.Background(), r, "lab-ns", state)
	got = output.String()
	if !strings.Contains(got, "Clabernetes node image pull completed") ||
		!strings.Contains(got, "node=node1") {
		t.Fatalf("image pull completion was not reported:\n%s", got)
	}
}

func TestNodePlanFailureClassification(t *testing.T) {
	t.Parallel()

	nodeWithCondition := func(reason, message string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"status": map[string]any{
				"conditions": []any{map[string]any{
					"type":    "PlanApplied",
					"status":  "False",
					"reason":  reason,
					"message": message,
				}},
			},
		}}
	}

	failure, streak := nodePlanFailure(nodeWithCondition("PlanUnsupported", "cannot do that"))
	if failure != "cannot do that" || streak != terminalFailureStreak {
		t.Fatalf("PlanUnsupported = (%q, %d), want terminal", failure, streak)
	}

	failure, streak = nodePlanFailure(
		nodeWithCondition("OCIMetadataResolveManifest", "unauthorized"),
	)
	if failure != "unauthorized" || streak != registryFailureStreak {
		t.Fatalf("OCIMetadata = (%q, %d), want registry debounce", failure, streak)
	}

	failure, _ = nodePlanFailure(nodeWithCondition("PlanMissingInput", "inventory settling"))
	if failure != "" {
		t.Fatalf("PlanMissingInput must not be terminal, got %q", failure)
	}
}

func TestManagePrimitiveOnlyLab(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "node1",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelTopologyOwner:  "primitive-lab",
				clabconstants.Owner: "alice",
			},
		},
		"spec": map[string]any{"kind": "linux", "image": "alpine:3"},
		"status": map[string]any{
			"readiness": "ready",
		},
	}}
	link := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Link",
		"metadata": map[string]any{
			"name":      "node1-eth1-host-eth1",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelTopologyOwner: "primitive-lab",
			},
		},
	}}
	profile := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "NodeProfile",
		"metadata": map[string]any{
			"name":      "primitive-lab",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelTopologyOwner: "primitive-lab",
			},
		},
	}}

	r := newTestRuntime(node, link, profile)
	state, err := r.Inspect(context.Background(), clablabruntime.InspectRequest{
		Name:      "primitive-lab",
		Namespace: "lab-ns",
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "primitive-lab" || state.Owner != "alice" || !state.Ready ||
		len(state.Nodes) != 1 || state.Nodes[0].Name != "node1" {
		t.Fatalf("unexpected primitive-only inspect state: %+v", state)
	}

	states, err := r.List(context.Background(), clablabruntime.ListRequest{Namespace: "lab-ns"})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Name != "primitive-lab" {
		t.Fatalf("unexpected primitive-only list state: %+v", states)
	}

	if err := r.Destroy(context.Background(), clablabruntime.DestroyRequest{
		Name:      "primitive-lab",
		Namespace: "lab-ns",
	}); err != nil {
		t.Fatal(err)
	}
	for _, resource := range []struct {
		gvr  schema.GroupVersionResource
		name string
	}{
		{gvr: nodeGVR, name: "node1"},
		{gvr: linkGVR, name: "node1-eth1-host-eth1"},
		{gvr: nodeProfileGVR, name: "primitive-lab"},
	} {
		_, err := r.client.Resource(resource.gvr).Namespace("lab-ns").
			Get(context.Background(), resource.name, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected %s to be deleted, got %v", resource.name, err)
		}
	}
}

func TestResolvePrimaryNode(t *testing.T) {
	t.Parallel()

	networkModes := map[string]string{
		"primary":   "",
		"secondary": "container:primary",
		"nested":    "container:secondary",
	}

	if got := resolvePrimaryNode("nested", networkModes); got != "primary" {
		t.Fatalf("resolvePrimaryNode(nested) = %q, want primary", got)
	}
	if got := resolvePrimaryNode("standalone", networkModes); got != "standalone" {
		t.Fatalf("resolvePrimaryNode(standalone) = %q, want standalone", got)
	}
}

func TestDeployCreatesPrimitiveResources(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
    node2:
      kind: linux
      image: alpine:3
  links:
    - endpoints: ["node1:eth1", "node2:eth1"]
`

	r := newTestRuntime()
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		Owner:              "alice",
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-ns" || state.Owner != "alice" {
		t.Fatalf("unexpected deploy state: %+v", state)
	}

	assertNoTestTopology(t, r, "lab-ns", "lab1")
	node := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	if node.GetLabels()[labelTopologyOwner] != "lab1" ||
		node.GetLabels()[labelRuntime] != clabernetesAppValue ||
		node.GetLabels()[clabconstants.Owner] != "alice" {
		t.Fatalf("unexpected node labels: %v", node.GetLabels())
	}
	if got, _, _ := unstructured.NestedString(
		node.Object,
		"spec",
		"image",
	); got != "alpine:latest" {
		t.Fatalf("node image = %q, want alpine:latest", got)
	}

	profiles, err := r.client.Resource(nodeProfileGVR).Namespace("lab-ns").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles.Items) != 1 || profiles.Items[0].GetName() != "lab1" {
		t.Fatalf("unexpected launcher profiles: %+v", profiles.Items)
	}

	links, err := r.client.Resource(linkGVR).Namespace("lab-ns").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Items) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links.Items))
	}
	if got, _, _ := unstructured.NestedString(
		links.Items[0].Object,
		"spec",
		"endpointA",
		"nodeName",
	); got != "node1" {
		t.Fatalf("link endpointA nodeName = %q, want node1", got)
	}
}

func TestDeployCreatesTopologyResource(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime()
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		Owner:              "alice",
		TopologyDefinition: []byte(definition),
		Wait:               false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-ns" || state.Owner != "alice" {
		t.Fatalf("unexpected deploy state: %+v", state)
	}

	obj := getTestTopology(t, r, "lab-ns", "lab1")
	if got := topologyDefinition(t, obj); got != definition {
		t.Fatalf("topology definition = %q, want %q", got, definition)
	}
	if obj.GetLabels()[clabconstants.Owner] != "alice" {
		t.Fatalf("unexpected topology labels: %v", obj.GetLabels())
	}
	// The controller compiles the Topology into primitive resources; containerlab must not
	// create them itself on this path.
	assertNoTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	assertNoTestPrimitive(t, r, nodeProfileGVR, "lab-ns", "lab1")
}

func TestDeployNoTopologyCRRejectsTopologyOwnedLab(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime(topologyObject("lab1", "lab-ns", "", definition))
	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "owned by a Topology resource") {
		t.Fatalf("unexpected topology ownership error: %v", err)
	}
	_ = getTestTopology(t, r, "lab-ns", "lab1")
}

func TestDeployAdoptsPrimitiveOnlyLabIntoTopology(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	existingNode := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "node1",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelApp:           clabernetesAppValue,
				labelTopologyOwner: "lab1",
			},
		},
	}}

	r := newTestRuntime(existingNode)
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(definition),
		Wait:               false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-ns" {
		t.Fatalf("unexpected deploy state: %+v", state)
	}

	// The controller adopts label-matched primitive resources once the Topology exists; the
	// pre-existing Node must survive the deployment.
	_ = getTestTopology(t, r, "lab-ns", "lab1")
	_ = getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
}

func TestFreshTopologyDeployTimeoutRollsBackTopology(t *testing.T) {
	t.Parallel()

	r := newTestRuntime()
	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:      "timeout-lab",
		Namespace: "lab-ns",
		TopologyDefinition: []byte(`topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
`),
		Wait:    true,
		Timeout: 20 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected readiness timeout")
	}

	assertNoTestTopology(t, r, "lab-ns", "timeout-lab")
}

func TestWaitTopologyGenerationObserved(t *testing.T) {
	t.Parallel()

	topology := func(generation, observed int64, topologyError string) *unstructured.Unstructured {
		status := map[string]any{}
		if observed != 0 {
			status["observedGeneration"] = observed
		}
		if topologyError != "" {
			status["error"] = topologyError
		}

		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       "Topology",
			"metadata": map[string]any{
				"name":       "lab1",
				"namespace":  "lab-ns",
				"generation": generation,
			},
			"status": status,
		}}
	}

	r := newTestRuntime(topology(2, 2, ""))
	if err := r.waitTopologyGenerationObserved(
		context.Background(), "lab-ns", "lab1", 50*time.Millisecond,
	); err != nil {
		t.Fatalf("observed current generation reported an error: %v", err)
	}

	// A controller that predates observedGeneration never reports it; readiness is the only
	// signal then and the wait must not run out the clock.
	r = newTestRuntime(topology(2, 0, ""))
	if err := r.waitTopologyGenerationObserved(
		context.Background(), "lab-ns", "lab1", 50*time.Millisecond,
	); err != nil {
		t.Fatalf("missing observedGeneration reported an error: %v", err)
	}

	r = newTestRuntime(topology(2, 1, ""))
	err := r.waitTopologyGenerationObserved(
		context.Background(), "lab-ns", "lab1", 30*time.Millisecond,
	)
	if err == nil || !strings.Contains(err.Error(), "observe topology") {
		t.Fatalf("stale observedGeneration error = %v, want observation timeout", err)
	}

	r = newTestRuntime(topology(2, 1, "duplicate resources found in the lab-ns namespace"))
	err = r.waitTopologyGenerationObserved(
		context.Background(), "lab-ns", "lab1", 30*time.Millisecond,
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate resources") {
		t.Fatalf("controller error = %v, want fail-fast with the reported message", err)
	}
}

func TestPlanReportsTopologyDiffWithoutMutation(t *testing.T) {
	t.Parallel()

	const initial = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
`
	const changed = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime()
	freshPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(freshPlan.Changes) != 1 ||
		freshPlan.Changes[0].Action != clablabruntime.ChangeCreate ||
		freshPlan.Changes[0].Kind != "Topology" {
		t.Fatalf("fresh plan changes = %+v, want one Topology create", freshPlan.Changes)
	}
	assertNoTestTopology(t, r, "lab-ns", "plan-lab")

	namespacedPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", TopologyDefinition: []byte(initial),
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCreates := map[string]bool{
		"Namespace/c9s-plan-lab": false,
		"Topology/plan-lab":      false,
	}
	for _, change := range namespacedPlan.Changes {
		key := change.Kind + "/" + change.Name
		if _, ok := wantCreates[key]; ok && change.Action == clablabruntime.ChangeCreate {
			wantCreates[key] = true
		}
	}
	for change, found := range wantCreates {
		if !found {
			t.Fatalf("plan for missing namespace = %+v, missing create %s",
				namespacedPlan.Changes, change)
		}
	}

	if _, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial), Wait: false,
	}); err != nil {
		t.Fatal(err)
	}

	noOpPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(noOpPlan.Changes) != 0 {
		t.Fatalf("no-op plan changes = %+v, want none", noOpPlan.Changes)
	}

	changedPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(changed),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changedPlan.Changes) != 1 ||
		changedPlan.Changes[0].Action != clablabruntime.ChangeUpdate ||
		changedPlan.Changes[0].Kind != "Topology" {
		t.Fatalf("changed plan = %+v, want one Topology update", changedPlan.Changes)
	}

	if _, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial),
		NoTopologyCR: true,
	}); err == nil || !strings.Contains(err.Error(), "owned by a Topology resource") {
		t.Fatalf("unexpected --no-topology-cr plan error: %v", err)
	}
}

func TestDeployStagesLocalFilesFromTopology(t *testing.T) {
	t.Parallel()

	topologyDir := t.TempDir()
	writeFile(t, filepath.Join(topologyDir, "configs", "client2", "iperf.sh"), "#!/bin/sh\n", 0o755)
	writeFile(
		t,
		filepath.Join(topologyDir, "configs", "prometheus", "prometheus.yml"),
		"global: {}\n",
		0o644,
	)
	writeFile(
		t,
		filepath.Join(topologyDir, "configs", "fabric", "leaf1.cfg"),
		"set / system name leaf1\n",
		0o644,
	)
	writeFile(t, filepath.Join(topologyDir, "configs", "client2.env"), "MODE=test\n", 0o644)
	writeFile(t, filepath.Join(topologyDir, "configs", "agent.yml"), "name: agent\n", 0o644)
	writeFile(t, filepath.Join(topologyDir, "configs", "flash.cfg"), "hostname ceos\n", 0o644)

	const definition = `name: lab1
topology:
  defaults:
    kind: linux
  nodes:
    leaf1:
      startup-config: configs/fabric/leaf1.cfg
    client2:
      kind: linux
      env-files:
        - configs/client2.env
      extras:
        srl-agents:
          - configs/agent.yml
        ceos-copy-to-flash:
          - configs/flash.cfg
      binds:
        - configs/client2:/config
    prometheus:
      kind: linux
      binds:
        - configs/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
`

	topologyFile := filepath.Join(topologyDir, "lab.clab.yml")
	writeFile(t, topologyFile, definition, 0o644)

	r := newTestRuntime()
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyFile:       topologyFile,
		TopologyLabDir:     filepath.Join(topologyDir, "clab-lab1"),
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-ns" {
		t.Fatalf("unexpected deploy state: %+v", state)
	}

	clientConfigMap := getTestConfigMap(t, r, "lab-ns", "lab1-client2-files")
	if got := string(clientConfigMap.BinaryData["configs-client2-iperf-sh"]); got != "#!/bin/sh\n" {
		t.Fatalf("unexpected client2 staged file content: %q", got)
	}
	for key, want := range map[string]string{
		"configs-client2-env": "MODE=test\n",
		"configs-agent-yml":   "name: agent\n",
		"configs-flash-cfg":   "hostname ceos\n",
	} {
		if got := string(clientConfigMap.BinaryData[key]); got != want {
			t.Fatalf("staged %s content = %q, want %q", key, got, want)
		}
	}

	prometheusConfigMap := getTestConfigMap(t, r, "lab-ns", "lab1-prometheus-files")
	if got := string(
		prometheusConfigMap.BinaryData["configs-prometheus-prometheus-yml"],
	); got != "global: {}\n" {
		t.Fatalf("unexpected prometheus staged file content: %q", got)
	}

	startupConfigMap := getTestConfigMap(t, r, "lab-ns", "lab1-leaf1-startup-config")
	if got := string(
		startupConfigMap.BinaryData["startup-config"],
	); got != "set / system name leaf1\n" {
		t.Fatalf("unexpected startup config content: %q", got)
	}

	assertFileMount(
		t,
		getTestPrimitive(t, r, nodeGVR, "lab-ns", "client2"),
		"configs/client2/iperf.sh",
		"lab1-client2-files",
		"configs-client2-iperf-sh",
		"execute",
	)
	for _, filePath := range []string{
		"configs/client2.env",
		"configs/agent.yml",
		"configs/flash.cfg",
	} {
		assertFileMount(
			t,
			getTestPrimitive(t, r, nodeGVR, "lab-ns", "client2"),
			filePath,
			"lab1-client2-files",
			safeConfigMapKey(filePath),
			"read",
		)
	}
	assertFileMount(
		t,
		getTestPrimitive(t, r, nodeGVR, "lab-ns", "prometheus"),
		"configs/prometheus/prometheus.yml",
		"lab1-prometheus-files",
		"configs-prometheus-prometheus-yml",
		"read",
	)
	assertFileMount(
		t,
		getTestPrimitive(t, r, nodeGVR, "lab-ns", "leaf1"),
		"configs/fabric/leaf1.cfg",
		"lab1-leaf1-startup-config",
		"startup-config",
		"read",
	)

	for _, configMap := range []*corev1.ConfigMap{
		clientConfigMap,
		prometheusConfigMap,
		startupConfigMap,
	} {
		if len(configMap.OwnerReferences) != 1 || configMap.OwnerReferences[0].Kind != "Node" {
			t.Fatalf("ConfigMap %s owner references = %+v, want one Node owner",
				configMap.Name, configMap.OwnerReferences)
		}
	}
}

func TestValidateAcceptsLossyContainerlabSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition string
	}{
		{
			name: "management network",
			definition: `mgmt:
  network: shared
topology:
  nodes:
    node1:
      kind: linux
      image: alpine
`,
		},
		{
			name: "group and pinned host port",
			definition: `topology:
  nodes:
    node1:
      kind: linux
      image: alpine
      group: clients
      ports:
        - 8080:80
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newTestRuntime()
			if err := r.Validate(context.Background(), clablabruntime.DeployRequest{
				Name:               "lossy",
				Namespace:          "lab-ns",
				TopologyDefinition: []byte(tt.definition),
			}); err != nil {
				t.Fatalf("Validate() error = %v, want lossy topology to be accepted", err)
			}
		})
	}
}

func TestValidateRejectsStructurallyUnsupportedContainerlabSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition string
		wantError  string
	}{
		{
			name: "bridge pseudo-node",
			definition: `topology:
  nodes:
    br0:
      kind: bridge
`,
			wantError: "pseudo-node",
		},
		{
			name: "link labels",
			definition: `topology:
  nodes:
    node1:
      kind: linux
      image: alpine
    node2:
      kind: linux
      image: alpine
  links:
    - endpoints: ["node1:eth1", "node2:eth1"]
      labels:
        purpose: test
`,
			wantError: "unsupported by c9s",
		},
		{
			name: "unsupported node field",
			definition: `topology:
  nodes:
    node1:
      kind: linux
      image: alpine
      cpu-set: 0-3
`,
			wantError: "unsupported by c9s",
		},
		{
			name: "invalid kubernetes node name",
			definition: `topology:
  nodes:
    Node_One:
      kind: linux
      image: alpine
`,
			wantError: "RFC 1035",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newTestRuntime()
			err := r.Validate(context.Background(), clablabruntime.DeployRequest{
				Name:               "structurally-unsupported",
				Namespace:          "lab-ns",
				TopologyDefinition: []byte(tt.definition),
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestPlanReportsPrimitiveDiffWithoutMutation(t *testing.T) {
	t.Parallel()

	const initial = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
    node2:
      kind: linux
      image: alpine:3
  links:
    - endpoints: ["node1:eth1", "node2:eth1"]
`
	const changed = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime()
	freshPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial),
		NoTopologyCR: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(freshPlan.Changes) != 4 {
		t.Fatalf("fresh plan changes = %+v, want profile, two nodes, and link creates", freshPlan.Changes)
	}
	assertNoTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")

	if _, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial), Wait: false,
		NoTopologyCR: true,
	}); err != nil {
		t.Fatal(err)
	}

	noOpPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial),
		NoTopologyCR: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(noOpPlan.Changes) != 0 {
		t.Fatalf("no-op plan changes = %+v, want none", noOpPlan.Changes)
	}

	changedPlan, err := r.Plan(context.Background(), clablabruntime.DeployRequest{
		Name: "plan-lab", Namespace: "lab-ns", TopologyDefinition: []byte(changed),
		NoTopologyCR: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantChanges := map[string]bool{
		"update/Node/node1":                 false,
		"delete/Node/node2":                 false,
		"delete/Link/node1-eth1-node2-eth1": false,
	}
	for _, change := range changedPlan.Changes {
		key := string(change.Action) + "/" + change.Kind + "/" + change.Name
		if _, ok := wantChanges[key]; ok {
			wantChanges[key] = true
		}
	}
	for change, found := range wantChanges {
		if !found {
			t.Fatalf("changed plan = %+v, missing %s", changedPlan.Changes, change)
		}
	}
}

func TestPrimitiveObjectsConformIgnoresMaterializedZeroDefaults(t *testing.T) {
	t.Parallel()

	desired := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"statusProbes": map[string]any{"enabled": true}},
	}}
	existing := desired.DeepCopy()
	existing.Object["spec"] = map[string]any{
		"statusProbes": map[string]any{
			"enabled":            true,
			"probeConfiguration": map[string]any{"startupSeconds": int64(0)},
		},
		"expose": map[string]any{
			"disableAutoExpose":      false,
			"disableExpose":          false,
			"useNodeMgmtIpv4Address": false,
		},
	}

	if !primitiveObjectsConform(existing, desired) {
		t.Fatal("API-materialized zero/default fields were reported as drift")
	}

	existing.Object["spec"].(map[string]any)["expose"].(map[string]any)["disableExpose"] = true
	if primitiveObjectsConform(existing, desired) {
		t.Fatal("nonzero API field was incorrectly ignored")
	}
}

func TestFreshDeployReadinessTimeoutRollsBackCreatedPrimitives(t *testing.T) {
	t.Parallel()

	r := newTestRuntime()
	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:      "timeout-lab",
		Namespace: "lab-ns",
		TopologyDefinition: []byte(`topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
`),
		Wait:         true,
		Timeout:      20 * time.Millisecond,
		NoTopologyCR: true,
	})
	if err == nil {
		t.Fatal("expected readiness timeout")
	}

	assertNoTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	assertNoTestPrimitive(t, r, nodeProfileGVR, "lab-ns", "timeout-lab")
}

func TestReconcileReadinessTimeoutRetainsExistingLab(t *testing.T) {
	t.Parallel()

	const initial = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
`
	const changed = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime()
	if _, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name: "retained-lab", Namespace: "lab-ns", TopologyDefinition: []byte(initial),
		NoTopologyCR: true,
	}); err != nil {
		t.Fatal(err)
	}

	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name: "retained-lab", Namespace: "lab-ns", TopologyDefinition: []byte(changed),
		Wait: true, Timeout: 20 * time.Millisecond,
		NoTopologyCR: true,
	})
	if err == nil {
		t.Fatal("expected reconcile readiness timeout")
	}

	node := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	image, _, nestedErr := unstructured.NestedString(node.Object, "spec", "image")
	if nestedErr != nil {
		t.Fatal(nestedErr)
	}
	if image != "alpine:latest" {
		t.Fatalf("retained node image = %q, want reconciled image", image)
	}
	_ = getTestPrimitive(t, r, nodeProfileGVR, "lab-ns", "retained-lab")
}

func TestDeployReconcileDeletesStaleStagedConfigMaps(t *testing.T) {
	t.Parallel()

	topologyDir := t.TempDir()
	writeFile(t, filepath.Join(topologyDir, "configs", "node1", "startup.sh"), "#!/bin/sh\n", 0o755)

	const initialDefinition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
      binds:
        - configs/node1:/config
`
	const updatedDefinition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`
	topologyFile := filepath.Join(topologyDir, "lab.clab.yml")
	writeFile(t, topologyFile, initialDefinition, 0o644)

	r := newTestRuntime()
	req := clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyFile:       topologyFile,
		TopologyDefinition: []byte(initialDefinition),
		Wait:               false,
		NoTopologyCR:       true,
	}
	if _, err := r.Deploy(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_ = getTestConfigMap(t, r, "lab-ns", "lab1-node1-files")

	req.TopologyDefinition = []byte(updatedDefinition)
	if _, err := r.Deploy(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_, err := r.kubeClient.CoreV1().ConfigMaps("lab-ns").Get(
		context.Background(),
		"lab1-node1-files",
		metav1.GetOptions{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale ConfigMap to be deleted, got %v", err)
	}
	if files, found, err := unstructured.NestedSlice(
		getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1").Object,
		"spec",
		"filesFromConfigMap",
	); err != nil || found || len(files) != 0 {
		t.Fatalf("stale node file mounts remain: found=%t files=%v err=%v", found, files, err)
	}
}

func TestDeployPreservesDockerCompatibleNamesForEmptyPrefixTopology(t *testing.T) {
	t.Parallel()

	const definition = `name: st
prefix: ""
topology:
  nodes:
    leaf1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:24.10.1
    prometheus:
      kind: linux
      image: quay.io/prometheus/prometheus:v2.54.1
`

	r := newTestRuntime()
	if _, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "st",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	}); err != nil {
		t.Fatal(err)
	}

	assertNoTestTopology(t, r, "lab-ns", "st")
	_ = getTestPrimitive(t, r, nodeGVR, "lab-ns", "leaf1")
	_ = getTestPrimitive(t, r, nodeGVR, "lab-ns", "prometheus")
}

func TestDeployDoesNotInferApplicationPorts(t *testing.T) {
	t.Parallel()

	const definition = `name: st
prefix: ""
mgmt:
  network: st
  ipv4-subnet: 172.20.20.0/24
topology:
  nodes:
    leaf1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:24.10.1
    gnmic:
      kind: linux
      image: ghcr.io/openconfig/gnmic:0.39.1
      cmd: --config /gnmic-config.yml --log subscribe
    prometheus:
      kind: linux
      image: quay.io/prometheus/prometheus:v2.54.1
      ports:
        - 9090:9090
  links:
    - endpoints: ["leaf1:e1-1", "prometheus:eth1"]
`

	r := newTestRuntime()
	if _, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "st",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(definition),
		Wait:               false,
		NoTopologyCR:       true,
	}); err != nil {
		t.Fatal(err)
	}

	assertNoTestTopology(t, r, "lab-ns", "st")
	gnmic := getTestPrimitive(t, r, nodeGVR, "lab-ns", "gnmic")
	ports, found, err := unstructured.NestedStringSlice(gnmic.Object, "spec", "ports")
	if err != nil {
		t.Fatalf("failed to read gnmic ports: %v", err)
	}
	if found || len(ports) != 0 {
		t.Fatalf("gnmic ports = %v, found=%t; want no inferred application ports", ports, found)
	}
	prometheus := getTestPrimitive(t, r, nodeGVR, "lab-ns", "prometheus")
	ports, found, err = unstructured.NestedStringSlice(prometheus.Object, "spec", "ports")
	if err != nil || !found || !slices.Contains(ports, "9090") {
		t.Fatalf("prometheus normalized ports = %v, found=%t, err=%v; want 9090", ports, found, err)
	}

	profile := getTestPrimitive(t, r, nodeProfileGVR, "lab-ns", "st")
	if enabled, found, err := unstructured.NestedBool(
		profile.Object,
		"spec",
		"statusProbes",
		"enabled",
	); err != nil || !found || !enabled {
		t.Fatalf("status probes enabled = %t, found=%t, err=%v; want true", enabled, found, err)
	}
	statusProbes, found, err := unstructured.NestedMap(profile.Object, "spec", "statusProbes")
	if err != nil || !found {
		t.Fatalf("failed to read status probe configuration: found=%t err=%v", found, err)
	}
	if configurations, found := statusProbes["nodeProbeConfigurations"]; found &&
		configurations != nil {
		t.Fatalf("containerlab must not infer node probe configurations: %v", statusProbes)
	}
	if excludedNodes, found := statusProbes["excludedNodes"]; found && excludedNodes != nil {
		t.Fatalf("containerlab must not exclude kinds from generic readiness: %v", statusProbes)
	}
	if got, _, _ := unstructured.NestedString(
		profile.Object,
		"spec",
		"mgmt",
		"ipv4-subnet",
	); got != "172.20.20.0/24" {
		t.Fatalf("launcher profile management subnet = %q, want 172.20.20.0/24", got)
	}
	links, err := r.client.Resource(linkGVR).Namespace("lab-ns").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Items) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links.Items))
	}
}

func TestDeployReconcilesExistingTopology(t *testing.T) {
	t.Parallel()

	const existingDefinition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3.20
`
	const newDefinition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3.21
`

	r := newTestRuntime(topologyObject("lab1", "lab-ns", "", existingDefinition))
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(newDefinition),
		Wait:               false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-ns" {
		t.Fatalf("unexpected reconciled state: %+v", state)
	}

	obj := getTestTopology(t, r, "lab-ns", "lab1")
	if got := topologyDefinition(t, obj); got != newDefinition {
		t.Fatalf("topology definition was updated to %q, want %q", got, newDefinition)
	}
}

func TestDeployReconcilesPrimitiveResources(t *testing.T) {
	t.Parallel()

	const initialDefinition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3.20
    old-node:
      kind: linux
      image: alpine:3.20
  links:
    - endpoints: ["node1:eth1", "old-node:eth1"]
`
	const updatedDefinition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3.21
    node2:
      kind: linux
      image: alpine:3.21
  links:
    - endpoints: ["node1:eth2", "node2:eth1"]
`

	r := newTestRuntime()
	request := clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(initialDefinition),
		Wait:               false,
		NoTopologyCR:       true,
	}
	if _, err := r.Deploy(context.Background(), request); err != nil {
		t.Fatal(err)
	}

	node1 := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	node1.SetLabels(mergeDesiredMetadata(node1.GetLabels(), map[string]string{
		"example.com/preserved": "true",
		labelIgnoreReconcile:    "true",
	}))
	if err := unstructured.SetNestedField(node1.Object, "ready", "status", "readiness"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.client.Resource(nodeGVR).Namespace("lab-ns").Update(
		context.Background(),
		node1,
		metav1.UpdateOptions{},
	); err != nil {
		t.Fatal(err)
	}

	request.TopologyDefinition = []byte(updatedDefinition)
	state, err := r.Deploy(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-ns" || len(state.Nodes) != 2 {
		t.Fatalf("unexpected reconciled state: %+v", state)
	}

	node1 = getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	if got, _, _ := unstructured.NestedString(node1.Object, "spec", "image"); got != "alpine:3.21" {
		t.Fatalf("node1 image = %q, want alpine:3.21", got)
	}
	if got, _, _ := unstructured.NestedString(node1.Object, "status", "readiness"); got != "ready" {
		t.Fatalf("node1 status readiness = %q, want preserved ready", got)
	}
	if node1.GetLabels()["example.com/preserved"] != "true" {
		t.Fatalf("node1 extra labels were not preserved: %v", node1.GetLabels())
	}
	if _, exists := node1.GetLabels()[labelIgnoreReconcile]; exists {
		t.Fatalf("node1 retained stop lifecycle label: %v", node1.GetLabels())
	}

	_ = getTestPrimitive(t, r, nodeGVR, "lab-ns", "node2")
	assertNoTestPrimitive(t, r, nodeGVR, "lab-ns", "old-node")

	links, err := r.client.Resource(linkGVR).Namespace("lab-ns").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Items) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links.Items))
	}
	if got, _, _ := unstructured.NestedString(
		links.Items[0].Object,
		"spec",
		"endpointB",
		"nodeName",
	); got != "node2" {
		t.Fatalf("reconciled link endpointB = %q, want node2", got)
	}
}

func TestDeployRejectsPrimitiveResourceOwnedByAnotherLab(t *testing.T) {
	t.Parallel()

	foreignNode := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "node1",
			"namespace": "lab-ns",
			"labels": map[string]any{
				labelTopologyOwner: "other-lab",
			},
		},
	}}
	r := newTestRuntime(foreignNode)
	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:      "lab1",
		Namespace: "lab-ns",
		TopologyDefinition: []byte(`topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`),
		Wait:         false,
		NoTopologyCR: true,
	})
	if err == nil || !strings.Contains(err.Error(), "belongs to another lab") {
		t.Fatalf("unexpected ownership collision error: %v", err)
	}

	actual := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	if actual.GetLabels()[labelTopologyOwner] != "other-lab" {
		t.Fatalf("foreign node ownership changed: %v", actual.GetLabels())
	}
}

func TestDeployDuplicateCheckIsNamespaceScoped(t *testing.T) {
	t.Parallel()

	const definition = `topology:
  nodes:
    node1:
      kind: linux
      image: alpine:latest
`

	r := newTestRuntime(topologyObject("lab1", "lab-a", "", definition))
	state, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-b",
		TopologyDefinition: []byte(definition),
		Wait:               false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Name != "lab1" || state.Namespace != "lab-b" {
		t.Fatalf("unexpected deploy state: %+v", state)
	}

	// The same-named topology in lab-a must not shadow the fresh deployment into lab-b.
	_ = getTestTopology(t, r, "lab-a", "lab1")
	_ = getTestTopology(t, r, "lab-b", "lab1")
}

func TestForwardPodWatchReconnectsOnClosedChannel(t *testing.T) {
	t.Parallel()

	watcher := watch.NewFake()
	watcher.Stop()

	r := &Runtime{}
	if !r.forwardPodWatch(
		context.Background(),
		watcher,
		make(chan clablabruntime.Event, 1),
		make(chan error, 1),
	) {
		t.Fatal("expected closed pod watch to request reconnect")
	}
}

func TestForwardTopologyWatchReconnectsOnClosedChannel(t *testing.T) {
	t.Parallel()

	watcher := watch.NewFake()
	watcher.Stop()

	r := &Runtime{}
	if !r.forwardTopologyWatch(
		context.Background(),
		"default",
		watcher,
		make(chan clablabruntime.Event, 1),
		make(chan error, 1),
	) {
		t.Fatal("expected closed topology watch to request reconnect")
	}
}

func TestForwardPodWatchEmitsPodEvent(t *testing.T) {
	t.Parallel()

	watcher := watch.NewFake()
	events := make(chan clablabruntime.Event, 1)
	errs := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &Runtime{}
	done := make(chan bool, 1)
	go func() {
		done <- r.forwardPodWatch(ctx, watcher, events, errs)
	}()

	watcher.Add(&corev1.Pod{})
	watcher.Add(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "lab-ns",
			Labels: map[string]string{
				labelTopologyOwner: "lab1",
				labelTopologyNode:  "node1",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	})

	got := <-events
	if got.ActorID != "lab-ns/lab1/node1" ||
		got.ActorName != "lab1-node1" ||
		got.ActorFullID != "pod1" ||
		got.Attributes["phase"] != string(corev1.PodRunning) ||
		got.Attributes["pod_ip"] != "10.0.0.1" {
		t.Fatalf("unexpected pod event: %+v", got)
	}

	cancel()
	watcher.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("forwardPodWatch did not exit after cancellation")
	}
}

func TestWaitDeploymentRolloutWaitsForTheOutgoingReplicaToGo(t *testing.T) {
	t.Parallel()

	// Mid rolling update: the Deployment controller has observed the new generation and the
	// replacement pod is up, but the outgoing pod is still counted and still available. Ready
	// replicas alone would call this settled.
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "server",
			Namespace:  defaultNamespace,
			Generation: 2,
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			Replicas:           2,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		},
	}

	r := newTestRuntimeWithKubeObjects(nil, []k8sruntime.Object{deployment})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.waitDeploymentRollout(ctx, defaultNamespace, "server", 2, 1, 50*time.Millisecond)
	if err == nil {
		t.Fatal("rollout wait returned while the previous pod template was still running")
	}
}

func TestWaitDeploymentRolloutWaitsForTheObservedGeneration(t *testing.T) {
	t.Parallel()

	// The write landed but the Deployment controller has not acted on it yet, so the status
	// still describes the previous pod template.
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "server",
			Namespace:  defaultNamespace,
			Generation: 3,
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		},
	}

	r := newTestRuntimeWithKubeObjects(nil, []k8sruntime.Object{deployment})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.waitDeploymentRollout(ctx, defaultNamespace, "server", 3, 1, 50*time.Millisecond)
	if err == nil {
		t.Fatal("rollout wait returned before the deployment controller observed the update")
	}
}

func TestWaitDeploymentRolloutReturnsOnSettledRollout(t *testing.T) {
	t.Parallel()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "server",
			Namespace:  defaultNamespace,
			Generation: 2,
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		},
	}

	r := newTestRuntimeWithKubeObjects(nil, []k8sruntime.Object{deployment})

	if err := r.waitDeploymentRollout(
		context.Background(),
		defaultNamespace,
		"server",
		2,
		1,
		time.Second,
	); err != nil {
		t.Fatalf("rollout wait did not return on a settled rollout: %v", err)
	}
}

func TestWaitDeploymentRolloutReturnsOnScaleToZero(t *testing.T) {
	t.Parallel()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "server",
			Namespace:  defaultNamespace,
			Generation: 4,
		},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 4},
	}

	r := newTestRuntimeWithKubeObjects(nil, []k8sruntime.Object{deployment})

	if err := r.waitDeploymentRollout(
		context.Background(),
		defaultNamespace,
		"server",
		4,
		0,
		time.Second,
	); err != nil {
		t.Fatalf("rollout wait did not return on a stopped deployment: %v", err)
	}
}

func newTestRuntime(objects ...*unstructured.Unstructured) *Runtime {
	return newTestRuntimeWithKubeObjects(objects, nil)
}

func newTestRuntimeWithKubeObjects(
	objects []*unstructured.Unstructured,
	kubeObjects []k8sruntime.Object,
) *Runtime {
	runtimeObjects := make([]k8sruntime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}

	baseKubeObjects := []k8sruntime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: defaultNamespace}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lab-ns"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lab-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lab-b"}},
	}
	baseKubeObjects = append(baseKubeObjects, kubeObjects...)

	return &Runtime{
		client: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			k8sruntime.NewScheme(),
			map[schema.GroupVersionResource]string{
				topologyGVR:    "TopologyList",
				nodeGVR:        "NodeList",
				linkGVR:        "LinkList",
				nodeProfileGVR: "NodeProfileList",
			},
			runtimeObjects...,
		),
		kubeClient: kubefake.NewSimpleClientset(baseKubeObjects...),
		namespace:  defaultNamespace,
	}
}

func getTestTopology(
	t *testing.T,
	r *Runtime,
	namespace string,
	name string,
) *unstructured.Unstructured {
	t.Helper()

	obj, err := r.client.Resource(topologyGVR).Namespace(namespace).
		Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return obj
}

func assertNoTestTopology(
	t *testing.T,
	r *Runtime,
	namespace string,
	name string,
) {
	t.Helper()

	_, err := r.client.Resource(topologyGVR).Namespace(namespace).
		Get(context.Background(), name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected compatibility Topology %s/%s not to exist, got %v",
			namespace, name, err)
	}
}

func getTestPrimitive(
	t *testing.T,
	r *Runtime,
	gvr schema.GroupVersionResource,
	namespace string,
	name string,
) *unstructured.Unstructured {
	t.Helper()

	obj, err := r.client.Resource(gvr).Namespace(namespace).
		Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return obj
}

func assertNoTestPrimitive(
	t *testing.T,
	r *Runtime,
	gvr schema.GroupVersionResource,
	namespace,
	name string,
) {
	t.Helper()

	_, err := r.client.Resource(gvr).Namespace(namespace).
		Get(context.Background(), name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected c9s %s %s/%s not to exist, got %v",
			gvr.Resource, namespace, name, err)
	}
}

func topologyDefinition(t *testing.T, obj *unstructured.Unstructured) string {
	t.Helper()

	definition, found, err := unstructured.NestedString(
		obj.Object,
		"spec",
		"definition",
		"containerlab",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("topology definition was not found")
	}

	return definition
}

func getTestConfigMap(
	t *testing.T,
	r *Runtime,
	namespace string,
	name string,
) *corev1.ConfigMap {
	t.Helper()

	configMap, err := r.kubeClient.CoreV1().ConfigMaps(namespace).
		Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return configMap
}

func assertFileMount(
	t *testing.T,
	obj *unstructured.Unstructured,
	filePath,
	configMapName,
	configMapPath,
	mode string,
) {
	t.Helper()

	filesFromConfigMap, found, err := unstructured.NestedSlice(
		obj.Object,
		"spec",
		"filesFromConfigMap",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("filesFromConfigMap was not found")
	}

	for _, rawMount := range filesFromConfigMap {
		mount, ok := rawMount.(map[string]any)
		if !ok {
			t.Fatalf("mount has unexpected type %T", rawMount)
		}
		if mount["filePath"] == filePath &&
			mount["configMapName"] == configMapName &&
			mount["configMapPath"] == configMapPath &&
			mount["mode"] == mode {
			return
		}
	}

	t.Fatalf(
		"mount %s/%s/%s/%s was not found in %+v",
		filePath,
		configMapName,
		configMapPath,
		mode,
		filesFromConfigMap,
	)
}

func writeFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func writeTarEntry(t *testing.T, tw *tar.Writer, hdr *tar.Header, data []byte) {
	t.Helper()

	if hdr.Typeflag == 0 {
		hdr.Typeflag = tar.TypeReg
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		return
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
}
