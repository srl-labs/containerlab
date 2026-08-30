package clabernetes

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

type topologyObjectOption func(map[string]any)

func topologyWithNaming(naming string) topologyObjectOption {
	return func(spec map[string]any) {
		if naming != "" {
			spec["naming"] = naming
		}
	}
}

// topologyWithImagePullSecret sets the image pull secret the kubelet uses when pulling device
// images. An empty name falls back to the runtime default so a Topology always carries a pull
// secret reference unless the deploy deliberately named a different one.
func topologyWithImagePullSecret(secret string) topologyObjectOption {
	return func(spec map[string]any) {
		if secret == "" {
			secret = clablabruntime.DefaultImagePullSecret
		}
		spec["imagePull"] = map[string]any{
			"pullSecrets": []any{secret},
		}
	}
}

func topologyObject(
	name,
	namespace,
	owner,
	definition string,
	opts ...topologyObjectOption,
) *unstructured.Unstructured {
	topologyLabels := map[string]any{
		"containerlab.dev/runtime": clablabruntime.ClabernetesRuntimeName,
	}
	topologyAnnotations := map[string]any{}
	if owner != "" {
		topologyAnnotations[clabconstants.Owner] = owner
		if len(validation.IsValidLabelValue(owner)) == 0 {
			topologyLabels[clabconstants.Owner] = owner
		}
	}

	metadata := map[string]any{
		"name":      name,
		"namespace": namespace,
		"labels":    topologyLabels,
	}
	if len(topologyAnnotations) != 0 {
		metadata["annotations"] = topologyAnnotations
	}

	spec := map[string]any{
		"definition": map[string]any{
			"containerlab": definition,
		},
		// The runtime's readiness contract requires c9s status probes. Persist the setting
		// explicitly: CRD defaulting cannot materialize it in an absent statusProbes object, and
		// the controller's typed writes would then pin the zero value (disabled) instead. This
		// mirrors configurePrimitiveReadiness on the client-side compile.
		"statusProbes": map[string]any{
			"enabled": true,
		},
	}
	for _, opt := range opts {
		opt(spec)
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       "Topology",
			"metadata":   metadata,
			"spec":       spec,
		},
	}
}

// stateFromTopology builds the lab-state skeleton from a Topology CR. The direct-runtime
// Topology status intentionally carries only aggregate readiness -- per-node detail is always
// filled in from the Node objects and workloads by enrichState.
func stateFromTopology(obj *unstructured.Unstructured, namespace string) *clablabruntime.LabState {
	if obj.GetNamespace() != "" {
		namespace = obj.GetNamespace()
	}

	ready, _, _ := unstructured.NestedBool(obj.Object, "status", "topologyReady")
	state, _, _ := unstructured.NestedString(obj.Object, "status", "topologyState")
	owner := obj.GetLabels()[clabconstants.Owner]
	if owner == "" {
		owner = obj.GetAnnotations()[clabconstants.Owner]
	}
	nodeSpecs := nodeSpecsFromTopology(obj)

	nodeNames := make([]string, 0, len(nodeSpecs))
	for nodeName := range nodeSpecs {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	nodes := make([]clablabruntime.NodeState, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		spec := nodeSpecs[nodeName]
		nodes = append(nodes, clablabruntime.NodeState{
			Name:  nodeName,
			Kind:  spec.Kind,
			Image: spec.Image,
		})
	}

	return &clablabruntime.LabState{
		Name:         obj.GetName(),
		Namespace:    namespace,
		Owner:        owner,
		TopologyPath: fmt.Sprintf("k8s://%s/topologies/%s", namespace, obj.GetName()),
		State:        state,
		Ready:        ready,
		Nodes:        nodes,
	}
}

type nodeSpec struct {
	Kind  string `yaml:"kind"`
	Image string `yaml:"image"`
}

type containerlabDefinition struct {
	Topology struct {
		Nodes map[string]nodeSpec `yaml:"nodes"`
	} `yaml:"topology"`
}

func nodeSpecsFromTopology(obj *unstructured.Unstructured) map[string]nodeSpec {
	specs := map[string]nodeSpec{}

	definition, _, _ := unstructured.NestedString(
		obj.Object,
		"spec",
		"definition",
		"containerlab",
	)
	mergeNodeSpecs(specs, definition)

	return specs
}

func mergeNodeSpecs(specs map[string]nodeSpec, definition string) {
	if definition == "" {
		return
	}

	var parsed containerlabDefinition
	if err := yaml.Unmarshal([]byte(definition), &parsed); err != nil {
		log.Debug("failed to parse clabernetes topology definition", "error", err)
		return
	}

	for nodeName, spec := range parsed.Topology.Nodes {
		specs[nodeName] = spec
	}
}
