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

func TestPrimitiveResourceGroupsCreateLinksBeforeNodes(t *testing.T) {
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

	const definition = `name: lab1
topology:
  defaults:
    kind: linux
  nodes:
    leaf1:
      startup-config: configs/fabric/leaf1.cfg
    client2:
      kind: linux
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

func TestDeployExposesGNMICMetricsPortForClabernetes(t *testing.T) {
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
	if err != nil || !found {
		t.Fatalf("failed to read gnmic ports: found=%t err=%v", found, err)
	}
	if !slices.Contains(ports, "9273/tcp") {
		t.Fatalf("gnmic ports = %v, want 9273/tcp", ports)
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

func TestDeployFailsWhenTopologyAlreadyExists(t *testing.T) {
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
	_, err := r.Deploy(context.Background(), clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyDefinition: []byte(newDefinition),
		Wait:               false,
	})
	if err == nil {
		t.Fatal("expected duplicate topology deploy to fail")
	}
	if !strings.Contains(err.Error(), "already been deployed in namespace 'lab-ns'") {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := getTestTopology(t, r, "lab-ns", "lab1")
	if got := topologyDefinition(t, obj); got != existingDefinition {
		t.Fatalf("topology definition was updated to %q, want %q", got, existingDefinition)
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
			},
			runtimeObjects...,
		),
		kubeClient: kubefake.NewSimpleClientset(),
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
