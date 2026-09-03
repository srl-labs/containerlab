package clabernetes

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const manifestsTestDefinition = `name: lab1
topology:
  nodes:
    leaf1:
      kind: linux
      image: alpine:latest
      startup-config: configs/leaf1.cfg
    client1:
      kind: linux
      image: alpine:3
  links:
    - endpoints: ["leaf1:eth1", "client1:eth1"]
`

func manifestsTestRequest(t *testing.T) clablabruntime.DeployRequest {
	t.Helper()

	topologyDir := t.TempDir()
	writeFile(t, filepath.Join(topologyDir, "configs", "leaf1.cfg"), "set / system name leaf1\n", 0o644)
	topologyFile := filepath.Join(topologyDir, "lab.clab.yml")
	writeFile(t, topologyFile, manifestsTestDefinition, 0o644)

	return clablabruntime.DeployRequest{
		Name:               "lab1",
		Owner:              "alice",
		TopologyFile:       topologyFile,
		TopologyLabDir:     filepath.Join(topologyDir, "clab-lab1"),
		TopologyDefinition: []byte(manifestsTestDefinition),
	}
}

func manifestKinds(manifests []clablabruntime.Manifest) []string {
	kinds := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		kinds = append(kinds, manifest.Kind+"/"+manifest.Name)
	}

	return kinds
}

func assertManifestIsApplyInput(t *testing.T, manifest clablabruntime.Manifest) {
	t.Helper()

	if _, found := manifest.Object["status"]; found {
		t.Fatalf("%s %s carries a status block", manifest.Kind, manifest.Name)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(
		manifest.Object, "metadata", "creationTimestamp",
	); found {
		t.Fatalf("%s %s carries a creationTimestamp", manifest.Kind, manifest.Name)
	}
	if manifest.APIVersion == "" || manifest.Kind == "" || manifest.Name == "" {
		t.Fatalf("manifest identity is incomplete: %+v", manifest)
	}
	if got, _, _ := unstructured.NestedString(manifest.Object, "apiVersion"); got != manifest.APIVersion {
		t.Fatalf("%s %s apiVersion = %q, want %q", manifest.Kind, manifest.Name, got, manifest.APIVersion)
	}
}

func TestManifestsTopologyBundle(t *testing.T) {
	t.Parallel()

	r := newTestRuntime()
	manifests, err := r.Manifests(context.Background(), manifestsTestRequest(t))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"Namespace/c9s-lab1",
		"ConfigMap/lab1-leaf1-startup-config",
		"Topology/lab1",
	}
	if got := manifestKinds(manifests); !apiequality.Semantic.DeepEqual(got, want) {
		t.Fatalf("manifests = %v, want %v", got, want)
	}
	for _, manifest := range manifests {
		assertManifestIsApplyInput(t, manifest)
	}

	namespace := manifests[0]
	if namespace.APIVersion != "v1" || namespace.Namespace != "" {
		t.Fatalf("unexpected namespace manifest identity: %+v", namespace)
	}
	namespaceLabels, _, _ := unstructured.NestedStringMap(namespace.Object, "metadata", "labels")
	if namespaceLabels[labelTopologyOwner] != "lab1" ||
		namespaceLabels[labelRuntime] != clabernetesAppValue ||
		namespaceLabels["pod-security.kubernetes.io/enforce"] != "privileged" {
		t.Fatalf("unexpected namespace labels: %v", namespaceLabels)
	}

	configMap := manifests[1]
	if configMap.Namespace != "c9s-lab1" {
		t.Fatalf("configmap namespace = %q, want c9s-lab1", configMap.Namespace)
	}
	configMapLabels, _, _ := unstructured.NestedStringMap(configMap.Object, "metadata", "labels")
	if configMapLabels[labelTopologyOwner] != "lab1" || configMapLabels[labelTopologyNode] != "leaf1" {
		t.Fatalf("unexpected configmap labels: %v", configMapLabels)
	}
	// A text file is emitted as plain data, not base64 binaryData, so the manifest is editable.
	data, _, _ := unstructured.NestedStringMap(configMap.Object, "data")
	if data["startup-config"] != "set / system name leaf1\n" {
		t.Fatalf("configmap data = %v, want plain startup-config", data)
	}
	if _, found := configMap.Object["binaryData"]; found {
		t.Fatalf("text-only configmap carries binaryData: %v", configMap.Object["binaryData"])
	}

	topology := manifests[2]
	if topology.APIVersion != c9sAPIVersion || topology.Namespace != "c9s-lab1" {
		t.Fatalf("unexpected topology manifest identity: %+v", topology)
	}
	topologyLabels, _, _ := unstructured.NestedStringMap(topology.Object, "metadata", "labels")
	if topologyLabels[clabconstants.Owner] != "alice" {
		t.Fatalf("unexpected topology labels: %v", topologyLabels)
	}
	files, _, _ := unstructured.NestedSlice(
		topology.Object, "spec", "deployment", "filesFromConfigMap", "leaf1",
	)
	if len(files) != 1 {
		t.Fatalf("topology leaf1 filesFromConfigMap = %v, want one entry", files)
	}
	file, _ := files[0].(map[string]any)
	if file["configMapName"] != "lab1-leaf1-startup-config" {
		t.Fatalf("topology references ConfigMap %v, want lab1-leaf1-startup-config", file)
	}
}

func TestManifestsPrimitiveBundle(t *testing.T) {
	t.Parallel()

	req := manifestsTestRequest(t)
	req.Namespace = "lab-ns"
	req.NoTopologyCR = true

	r := newTestRuntime()
	manifests, err := r.Manifests(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// A namespace override must pre-exist, so the bundle does not create it.
	want := []string{
		"ConfigMap/lab1-leaf1-startup-config",
		"NodeProfile/lab1",
		"Link/lab1-leaf1-eth1-client1-eth1",
		"Node/client1",
		"Node/leaf1",
	}
	got := manifestKinds(manifests)
	if len(got) != len(want) {
		t.Fatalf("manifests = %v, want %v", got, want)
	}
	for idx, manifest := range manifests {
		assertManifestIsApplyInput(t, manifest)
		if manifest.Namespace != "lab-ns" {
			t.Fatalf("%s namespace = %q, want lab-ns", got[idx], manifest.Namespace)
		}
		if manifest.Kind == "Link" {
			// Link names are derived by the compiler; only the kind and position matter here.
			continue
		}
		if got[idx] != want[idx] {
			t.Fatalf("manifests = %v, want %v", got, want)
		}
	}

	for _, manifest := range manifests[1:] {
		labels, _, _ := unstructured.NestedStringMap(manifest.Object, "metadata", "labels")
		if labels[labelTopologyOwner] != "lab1" || labels[labelRuntime] != clabernetesAppValue {
			t.Fatalf("%s %s labels = %v, want lab ownership labels",
				manifest.Kind, manifest.Name, labels)
		}
	}
}

func TestManifestsRejectUnsupportedTopology(t *testing.T) {
	t.Parallel()

	req := manifestsTestRequest(t)
	req.TopologyDefinition = []byte(`name: lab1
topology:
  nodes:
    br0:
      kind: bridge
`)

	r := newTestRuntime()
	if _, err := r.Manifests(context.Background(), req); err == nil {
		t.Fatal("expected manifests to fail for a topology c9s cannot represent")
	}
}

func TestManifestsMatchDeployedResources(t *testing.T) {
	t.Parallel()

	req := manifestsTestRequest(t)
	req.Namespace = "lab-ns"

	r := newTestRuntime()
	manifests, err := r.Manifests(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Deploy(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	deployedTopology := getTestTopology(t, r, "lab-ns", "lab1")
	emittedTopology := manifests[len(manifests)-1]
	if !apiequality.Semantic.DeepEqual(emittedTopology.Object["spec"], deployedTopology.Object["spec"]) {
		t.Fatalf("emitted topology spec differs from the deployed one:\n%v\n%v",
			emittedTopology.Object["spec"], deployedTopology.Object["spec"])
	}
	emittedLabels, _, _ := unstructured.NestedStringMap(emittedTopology.Object, "metadata", "labels")
	if !apiequality.Semantic.DeepEqual(emittedLabels, deployedTopology.GetLabels()) {
		t.Fatalf("emitted topology labels = %v, deployed %v",
			emittedLabels, deployedTopology.GetLabels())
	}

	deployedConfigMap := getTestConfigMap(t, r, "lab-ns", "lab1-leaf1-startup-config")
	emittedConfigMap := manifests[0]
	emittedData, _, _ := unstructured.NestedStringMap(emittedConfigMap.Object, "data")
	if !apiequality.Semantic.DeepEqual(emittedData, deployedConfigMap.Data) {
		t.Fatalf("emitted configmap data = %v, deployed %v", emittedData, deployedConfigMap.Data)
	}
	if _, found := emittedConfigMap.Object["binaryData"]; found ||
		len(deployedConfigMap.BinaryData) != 0 {
		t.Fatalf("text-only configmap carries binaryData: emitted %v, deployed %v",
			emittedConfigMap.Object["binaryData"], deployedConfigMap.BinaryData)
	}
	emittedConfigMapLabels, _, _ := unstructured.NestedStringMap(
		emittedConfigMap.Object, "metadata", "labels",
	)
	if !apiequality.Semantic.DeepEqual(emittedConfigMapLabels, deployedConfigMap.Labels) {
		t.Fatalf("emitted configmap labels = %v, deployed %v",
			emittedConfigMapLabels, deployedConfigMap.Labels)
	}
}

func TestManifestsMatchDeployedPrimitiveResources(t *testing.T) {
	t.Parallel()

	req := manifestsTestRequest(t)
	req.Namespace = "lab-ns"
	req.NoTopologyCR = true

	r := newTestRuntime()
	manifests, err := r.Manifests(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Deploy(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	for _, manifest := range manifests {
		var deployed *unstructured.Unstructured
		switch manifest.Kind {
		case "Node":
			deployed = getTestPrimitive(t, r, nodeGVR, "lab-ns", manifest.Name)
		case "NodeProfile":
			deployed = getTestPrimitive(t, r, nodeProfileGVR, "lab-ns", manifest.Name)
		case "Link":
			deployed = getTestPrimitive(t, r, linkGVR, "lab-ns", manifest.Name)
		default:
			continue
		}
		if !apiequality.Semantic.DeepEqual(manifest.Object["spec"], deployed.Object["spec"]) {
			t.Fatalf("emitted %s %s spec differs from the deployed one:\n%v\n%v",
				manifest.Kind, manifest.Name, manifest.Object["spec"], deployed.Object["spec"])
		}
		labels, _, _ := unstructured.NestedStringMap(manifest.Object, "metadata", "labels")
		if !apiequality.Semantic.DeepEqual(labels, deployed.GetLabels()) {
			t.Fatalf("emitted %s %s labels = %v, deployed %v",
				manifest.Kind, manifest.Name, labels, deployed.GetLabels())
		}
	}
}

func TestManifestsKeepBinaryFilesAsBinaryData(t *testing.T) {
	t.Parallel()

	topologyDir := t.TempDir()
	binaryContent := "\x00\xff\xfe\x01binary"
	writeFile(t, filepath.Join(topologyDir, "configs", "blob.bin"), binaryContent, 0o644)
	writeFile(t, filepath.Join(topologyDir, "configs", "hello.txt"), "hello\tworld \n", 0o644)

	const definition = `name: lab1
topology:
  nodes:
    client1:
      kind: linux
      image: alpine:3
      binds:
        - configs/blob.bin:/config/blob.bin
        - configs/hello.txt:/config/hello.txt
`
	topologyFile := filepath.Join(topologyDir, "lab.clab.yml")
	writeFile(t, topologyFile, definition, 0o644)

	req := clablabruntime.DeployRequest{
		Name:               "lab1",
		Namespace:          "lab-ns",
		TopologyFile:       topologyFile,
		TopologyLabDir:     filepath.Join(topologyDir, "clab-lab1"),
		TopologyDefinition: []byte(definition),
	}

	r := newTestRuntime()
	manifests, err := r.Manifests(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 2 || manifests[0].Kind != "ConfigMap" {
		t.Fatalf("manifests = %v, want the files ConfigMap first", manifestKinds(manifests))
	}

	configMap := manifests[0]
	data, _, _ := unstructured.NestedStringMap(configMap.Object, "data")
	if data["configs-hello-txt"] != "hello\tworld \n" {
		t.Fatalf("text file data = %v, want it staged verbatim", data)
	}
	if _, found := data["configs-blob-bin"]; found {
		t.Fatalf("binary file was staged as text data: %v", data)
	}
	binaryData, _, _ := unstructured.NestedStringMap(configMap.Object, "binaryData")
	if binaryData["configs-blob-bin"] != base64.StdEncoding.EncodeToString([]byte(binaryContent)) {
		t.Fatalf("binary file binaryData = %v, want base64 of the file", binaryData)
	}

	// The applied object splits the keys the same way.
	if _, err = r.Deploy(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	deployed := getTestConfigMap(t, r, "lab-ns", "lab1-client1-files")
	if deployed.Data["configs-hello-txt"] != "hello\tworld \n" ||
		string(deployed.BinaryData["configs-blob-bin"]) != binaryContent {
		t.Fatalf("deployed configmap data = %v binaryData = %v",
			deployed.Data, deployed.BinaryData)
	}
}
