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

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
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
		{name: "relative path", in: "./configs/startup.json", want: "configs/startup.json", ok: true},
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

func TestDeployCreatesTopology(t *testing.T) {
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
}

func TestDeployStagesLocalFilesFromTopology(t *testing.T) {
	t.Parallel()

	topologyDir := t.TempDir()
	writeFile(t, filepath.Join(topologyDir, "configs", "client2", "iperf.sh"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(topologyDir, "configs", "prometheus", "prometheus.yml"), "global: {}\n", 0o644)
	writeFile(t, filepath.Join(topologyDir, "configs", "fabric", "leaf1.cfg"), "set / system name leaf1\n", 0o644)

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
	if got := string(prometheusConfigMap.BinaryData["configs-prometheus-prometheus-yml"]); got != "global: {}\n" {
		t.Fatalf("unexpected prometheus staged file content: %q", got)
	}

	startupConfigMap := getTestConfigMap(t, r, "lab-ns", "lab1-leaf1-startup-config")
	if got := string(startupConfigMap.BinaryData["startup-config"]); got != "set / system name leaf1\n" {
		t.Fatalf("unexpected startup config content: %q", got)
	}

	obj := getTestTopology(t, r, "lab-ns", "lab1")
	assertFileMount(
		t,
		obj,
		"client2",
		"configs/client2/iperf.sh",
		"lab1-client2-files",
		"configs-client2-iperf-sh",
		"execute",
	)
	assertFileMount(
		t,
		obj,
		"prometheus",
		"configs/prometheus/prometheus.yml",
		"lab1-prometheus-files",
		"configs-prometheus-prometheus-yml",
		"read",
	)
	assertFileMount(
		t,
		obj,
		"leaf1",
		"configs/fabric/leaf1.cfg",
		"lab1-leaf1-startup-config",
		"startup-config",
		"read",
	)
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

	obj := getTestTopology(t, r, "lab-ns", "st")
	if got := topologyNaming(t, obj); got != "non-prefixed" {
		t.Fatalf("topology naming = %q, want %q", got, "non-prefixed")
	}
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

	definitionAfterDeploy := topologyDefinition(t, getTestTopology(t, r, "lab-ns", "st"))
	if !strings.Contains(definitionAfterDeploy, "mgmt:\n  network: st\n  ipv4-subnet: 172.20.20.0/24") {
		t.Fatalf("topology definition did not preserve mgmt config:\n%s", definitionAfterDeploy)
	}
	if !strings.Contains(definitionAfterDeploy, "- leaf1:e1-1") ||
		!strings.Contains(definitionAfterDeploy, "- prometheus:eth1") {
		t.Fatalf("topology definition did not preserve brief link endpoints:\n%s", definitionAfterDeploy)
	}
	if strings.Contains(definitionAfterDeploy, "node: leaf1") ||
		strings.Contains(definitionAfterDeploy, "interface: e1-1") {
		t.Fatalf("topology definition rendered structured link endpoints:\n%s", definitionAfterDeploy)
	}

	var config clabRuntimeConfig
	if err := yaml.Unmarshal([]byte(definitionAfterDeploy), &config); err != nil {
		t.Fatal(err)
	}

	gnmic := config.Topology.Nodes["gnmic"]
	if gnmic == nil {
		t.Fatal("gnmic node was not found in topology definition")
	}
	if !slices.Contains(gnmic.Ports, "9273:9273/tcp") {
		t.Fatalf("gnmic ports = %v, want 9273:9273/tcp", gnmic.Ports)
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

func newTestRuntime(objects ...*unstructured.Unstructured) *Runtime {
	runtimeObjects := make([]k8sruntime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}

	return &Runtime{
		client:     dynamicfake.NewSimpleDynamicClient(k8sruntime.NewScheme(), runtimeObjects...),
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
	nodeName,
	filePath,
	configMapName,
	configMapPath,
	mode string,
) {
	t.Helper()

	filesFromConfigMap, found, err := unstructured.NestedMap(
		obj.Object,
		"spec",
		"deployment",
		"filesFromConfigMap",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("filesFromConfigMap was not found")
	}

	rawMounts, ok := filesFromConfigMap[nodeName].([]any)
	if !ok {
		t.Fatalf("filesFromConfigMap[%s] has unexpected type %T", nodeName, filesFromConfigMap[nodeName])
	}

	for _, rawMount := range rawMounts {
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
		rawMounts,
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
