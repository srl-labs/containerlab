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
	clabernetesconstants "github.com/clabernetes/clabernetes/constants"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
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

func TestPreferredNestedContainerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		nodeName       string
		exactNames     []string
		componentNames []string
		want           string
		wantError      string
	}{
		{
			name:       "regular-node",
			nodeName:   "router",
			exactNames: []string{"router"},
			want:       "router",
		},
		{
			name:           "components-prefer-cpm-a",
			nodeName:       "srsim",
			componentNames: []string{"srsim-2", "srsim-b", "srsim-1", "srsim-a"},
			want:           "srsim-a",
		},
		{
			name:           "components-fall-back-to-cpm-b",
			nodeName:       "srsim",
			componentNames: []string{"srsim-1", "srsim-b"},
			want:           "srsim-b",
		},
		{
			name:      "missing-node",
			nodeName:  "missing",
			wantError: "was not found",
		},
		{
			name:           "missing-cpm",
			nodeName:       "srsim",
			componentNames: []string{"srsim-1", "srsim-2"},
			wantError:      "CPM component container",
		},
		{
			name:       "ambiguous-exact-node",
			nodeName:   "router",
			exactNames: []string{"router-duplicate", "router"},
			wantError:  "multiple nested containers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := preferredNestedContainerName(
				tt.nodeName,
				tt.exactNames,
				tt.componentNames,
			)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf(
						"preferredNestedContainerName() error = %v, want %q",
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
				t.Fatalf("preferredNestedContainerName() = %q, want %q", got, tt.want)
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
				"topologyState": "ready",
				"nodeReadiness": map[string]any{
					"client": "ready",
					"server": "notready",
				},
				"exposedPorts": map[string]any{
					"client": map[string]any{
						"loadBalancerAddress": "192.0.2.10",
					},
					"server": map[string]any{
						"loadBalancerAddress": "not-an-ip",
					},
				},
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
		state.State != "ready" {
		t.Fatalf("unexpected state metadata: %+v", state)
	}
	if len(state.Nodes) != 2 {
		t.Fatalf("len(state.Nodes) = %d, want 2: %+v", len(state.Nodes), state.Nodes)
	}

	client := state.Nodes[0]
	if client.Name != "client" ||
		client.Kind != "linux" ||
		client.Image != "client:latest" ||
		client.State != "ready" ||
		!client.Ready ||
		client.LoadBalancerAddress != "192.0.2.10" {
		t.Fatalf("unexpected client state: %+v", client)
	}

	server := state.Nodes[1]
	if server.Name != "server" ||
		server.Kind != "srl" ||
		server.Image != "server:latest" ||
		server.Ready ||
		server.LoadBalancerAddress != "" {
		t.Fatalf("unexpected server state: %+v", server)
	}
}

func TestPrimitiveResourcesUseCurrentC9sAPI(t *testing.T) {
	t.Parallel()

	for name, gvr := range map[string]schema.GroupVersionResource{
		"topology":         topologyGVR,
		"node":             nodeGVR,
		"link":             linkGVR,
		"launcher profile": launcherProfileGVR,
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
	want := []string{"LauncherProfile", "Link", "Node"}
	got := make([]string, 0, len(groups))
	for _, group := range groups {
		got = append(got, group.kind)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("primitive creation order = %v, want %v", got, want)
	}
}

func TestStageAndEnablePrimitiveNodeDeployments(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name":      "node1",
			"namespace": "lab-ns",
			"labels": map[string]any{
				"keep": "value",
			},
		},
	}}
	set := &primitiveResourceSet{nodes: []*unstructured.Unstructured{node}}
	stagePrimitiveNodeDeployments(set)

	if node.GetLabels()[clabernetesconstants.LabelDisableDeployments] != "true" {
		t.Fatalf("staged node labels = %v", node.GetLabels())
	}

	r := newTestRuntime()
	created, err := r.client.Resource(nodeGVR).Namespace("lab-ns").Create(
		context.Background(),
		node,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.enablePrimitiveNodeDeployments(
		context.Background(),
		"lab-ns",
		map[string]*unstructured.Unstructured{"node1": created},
	); err != nil {
		t.Fatal(err)
	}

	actual := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	if _, exists := actual.GetLabels()[clabernetesconstants.LabelDisableDeployments]; exists {
		t.Fatalf("enabled node retains staging label: %v", actual.GetLabels())
	}
	if actual.GetLabels()["keep"] != "value" {
		t.Fatalf("enable patch did not preserve labels: %v", actual.GetLabels())
	}
}

func TestPrimitiveLinkPendingReason(t *testing.T) {
	t.Parallel()

	link := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"resolvedEndpoints": map[string]any{
				"endpointA": map[string]any{"nodeName": "node1", "uid": "uid-1"},
				"endpointB": map[string]any{"nodeName": "host"},
			},
		},
	}}
	if reason := primitiveLinkPendingReason(link); reason != "" {
		t.Fatalf("resolved link reported pending: %q", reason)
	}

	if err := unstructured.SetNestedField(
		link.Object,
		"endpoint node missing",
		"status",
		"error",
	); err != nil {
		t.Fatal(err)
	}
	if reason := primitiveLinkPendingReason(link); reason != "endpoint node missing" {
		t.Fatalf("pending reason = %q, want endpoint status error", reason)
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

func TestImagePullRequestsFilterAndProgress(t *testing.T) {
	request := func(name, node, image, kubernetesNode string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       "ImageRequest",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "lab-ns",
			},
			"spec": map[string]any{
				"topologyNodeName": node,
				"requestedImage":   image,
				"kubernetesNode":   kubernetesNode,
			},
		}}
	}

	r := newTestRuntime(
		request("node1-image", "node1", "example/node1:latest", "worker-a"),
		request("node1-other-image", "node1", "example/other:latest", "worker-a"),
		request("foreign-image", "foreign", "example/foreign:latest", "worker-b"),
	)
	requests, err := r.imagePullRequests(
		context.Background(),
		"lab-ns",
		&clablabruntime.LabState{Nodes: []clablabruntime.NodeState{
			{Name: "node1", Image: "example/node1:latest"},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].name != "node1-image" {
		t.Fatalf("image pull requests = %+v, want only node1-image", requests)
	}

	var output bytes.Buffer
	oldLevel := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	log.SetOutput(&output)
	defer func() {
		log.SetLevel(oldLevel)
		log.SetOutput(os.Stderr)
	}()

	progress := imagePullProgress{}
	progress.report(requests)
	got := output.String()
	if !strings.Contains(got, "Pulling clabernetes node image") ||
		!strings.Contains(got, "node=node1") ||
		!strings.Contains(got, "image=example/node1:latest") ||
		!strings.Contains(got, "kubernetes-node=worker-a") {
		t.Fatalf("image pull start was not reported:\n%s", got)
	}

	output.Reset()
	progress.report(requests)
	if output.Len() != 0 {
		t.Fatalf("unchanged image pull should not produce repeated log lines:\n%s", output.String())
	}

	output.Reset()
	progress.report(nil)
	got = output.String()
	if !strings.Contains(got, "Clabernetes node image pull completed") ||
		!strings.Contains(got, "node=node1") ||
		!strings.Contains(got, "image=example/node1:latest") {
		t.Fatalf("image pull completion was not reported:\n%s", got)
	}

	output.Reset()
	progress.report(nil)
	if output.Len() != 0 {
		t.Fatalf("completed image pull should not produce repeated log lines:\n%s", output.String())
	}

	output.Reset()
	progress.reportLauncherLog(launcherImageLog{
		podName:        "pod1",
		node:           "node1",
		image:          "example/node1:latest",
		kubernetesNode: "worker-a",
		content: "image \"example/node1:latest\" is present, begin copy to docker daemon...\n" +
			"Loaded image: example/node1:latest\n",
	})
	got = output.String()
	if !strings.Contains(got, "already present on Kubernetes node") ||
		strings.Contains(got, "copied to launcher Docker daemon") ||
		strings.Count(got, "node=node1") != 1 {
		t.Fatalf("cached image copy lifecycle was not reported:\n%s", got)
	}

	output.Reset()
	progress.reportLauncherLog(launcherImageLog{
		podName:        "pod1",
		node:           "node1",
		image:          "example/node1:latest",
		kubernetesNode: "worker-a",
		content:        "Loaded image: example/node1:latest\n",
	})
	if output.Len() != 0 {
		t.Fatalf("completed image copy should not produce repeated log lines:\n%s", output.String())
	}

	output.Reset()
	progress.reportLauncherLog(launcherImageLog{
		podName:        "pod2",
		node:           "node2",
		image:          "example/node2:latest",
		kubernetesNode: "worker-b",
		content: "image \"example/node2:latest\" is now available on node, continuing...\n" +
			"Loaded image: example/node2:latest\n",
	})
	got = output.String()
	if !strings.Contains(got, "Copying clabernetes node image") ||
		strings.Contains(got, "already present") ||
		strings.Contains(got, "copied to launcher Docker daemon") {
		t.Fatalf("pulled image copy lifecycle was not reported:\n%s", got)
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
		"kind":       "LauncherProfile",
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
		{gvr: launcherProfileGVR, name: "primitive-lab"},
	} {
		_, err := r.client.Resource(resource.gvr).Namespace("lab-ns").
			Get(context.Background(), resource.name, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected %s to be deleted, got %v", resource.name, err)
		}
	}
}

func TestResolveLauncherNode(t *testing.T) {
	t.Parallel()

	networkModes := map[string]string{
		"primary":   "",
		"secondary": "container:primary",
		"nested":    "container:secondary",
	}

	if got := resolveLauncherNode("nested", networkModes); got != "primary" {
		t.Fatalf("resolveLauncherNode(nested) = %q, want primary", got)
	}
	if got := resolveLauncherNode("standalone", networkModes); got != "standalone" {
		t.Fatalf("resolveLauncherNode(standalone) = %q, want standalone", got)
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

	profiles, err := r.client.Resource(launcherProfileGVR).Namespace("lab-ns").
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
		{
			name: "lossy link metadata",
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

	r := newTestRuntime()
	err := r.Validate(context.Background(), clablabruntime.DeployRequest{
		Name:      "structurally-unsupported",
		Namespace: "lab-ns",
		TopologyDefinition: []byte(`topology:
  nodes:
    br0:
      kind: bridge
`),
	})
	if err == nil || !strings.Contains(err.Error(), "pseudo-node") {
		t.Fatalf("Validate() error = %v, want structurally unsupported pseudo-node error", err)
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
		Wait:    true,
		Timeout: 20 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected readiness timeout")
	}

	assertNoTestPrimitive(t, r, nodeGVR, "lab-ns", "node1")
	assertNoTestPrimitive(t, r, launcherProfileGVR, "lab-ns", "timeout-lab")
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
	}); err != nil {
		t.Fatal(err)
	}

	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name: "retained-lab", Namespace: "lab-ns", TopologyDefinition: []byte(changed),
		Wait: true, Timeout: 20 * time.Millisecond,
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
	_ = getTestPrimitive(t, r, launcherProfileGVR, "lab-ns", "retained-lab")
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

	profile := getTestPrimitive(t, r, launcherProfileGVR, "lab-ns", "st")
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

func TestDeployReconcilesCompatibilityTopology(t *testing.T) {
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

	node2 := getTestPrimitive(t, r, nodeGVR, "lab-ns", "node2")
	if _, exists := node2.GetLabels()[clabernetesconstants.LabelDisableDeployments]; exists {
		t.Fatalf("new node retained deployment staging label: %v", node2.GetLabels())
	}
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
		Wait: false,
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

	_ = getTestTopology(t, r, "lab-a", "lab1")
	assertNoTestTopology(t, r, "lab-b", "lab1")
	_ = getTestPrimitive(t, r, nodeGVR, "lab-b", "node1")
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

func newTestRuntime(objects ...*unstructured.Unstructured) *Runtime {
	runtimeObjects := make([]k8sruntime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}

	return &Runtime{
		client: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			k8sruntime.NewScheme(),
			map[schema.GroupVersionResource]string{
				topologyGVR:        "TopologyList",
				nodeGVR:            "NodeList",
				linkGVR:            "LinkList",
				launcherProfileGVR: "LauncherProfileList",
				imageRequestGVR:    "ImageRequestList",
			},
			runtimeObjects...,
		),
		kubeClient: kubefake.NewSimpleClientset(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: defaultNamespace}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lab-ns"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lab-a"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lab-b"}},
		),
		namespace: defaultNamespace,
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

func topologyNaming(t *testing.T, obj *unstructured.Unstructured) string {
	t.Helper()

	naming, found, err := unstructured.NestedString(obj.Object, "spec", "naming")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return ""
	}

	return naming
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
