package clabernetes

import (
	"fmt"
	"net"
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
	}
	for _, opt := range opts {
		opt(spec)
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "clabernetes.containerlab.dev/v1alpha1",
			"kind":       "Topology",
			"metadata":   metadata,
			"spec":       spec,
		},
	}
}

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
	nodeReadiness, _, _ := unstructured.NestedStringMap(
		obj.Object,
		"status",
		"nodeReadiness",
	)
	exposedPorts, _, _ := unstructured.NestedMap(obj.Object, "status", "exposedPorts")
	nodeSpecs := nodeSpecsFromTopology(obj)

	nodeNames := make([]string, 0, len(nodeSpecs)+len(nodeReadiness))
	seenNodes := map[string]struct{}{}
	for nodeName := range nodeSpecs {
		nodeNames = append(nodeNames, nodeName)
		seenNodes[nodeName] = struct{}{}
	}
	for nodeName := range nodeReadiness {
		if _, ok := seenNodes[nodeName]; ok {
			continue
		}
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	nodes := make([]clablabruntime.NodeState, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		nodeState := nodeReadiness[nodeName]
		spec := nodeSpecs[nodeName]
		nodes = append(nodes, clablabruntime.NodeState{
			Name:                nodeName,
			Kind:                spec.Kind,
			Image:               spec.Image,
			State:               nodeState,
			Ready:               nodeState == "ready",
			LoadBalancerAddress: loadBalancerAddress(exposedPorts, nodeName),
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

	statusConfigs, _, _ := unstructured.NestedStringMap(obj.Object, "status", "configs")
	for _, config := range statusConfigs {
		mergeNodeSpecs(specs, config)
	}

	if len(specs) != 0 {
		return specs
	}

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

func loadBalancerAddress(exposedPorts map[string]any, nodeName string) string {
	raw, ok := exposedPorts[nodeName]
	if !ok {
		return ""
	}

	nodeExpose, ok := raw.(map[string]any)
	if !ok {
		return ""
	}

	addr, ok := nodeExpose["loadBalancerAddress"].(string)
	if !ok {
		return ""
	}

	if net.ParseIP(addr) == nil {
		return ""
	}

	return addr
}
