package clabernetes

import (
	"fmt"

	"github.com/charmbracelet/log"
	clabernetesapisv1alpha1 "github.com/clabernetes/clabernetes/apis/v1alpha1"
	clabernetescompiler "github.com/clabernetes/clabernetes/compiler"
	clabernetesconfig "github.com/clabernetes/clabernetes/config"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type primitiveResourceSet struct {
	nodeProfiles []*unstructured.Unstructured
	links        []*unstructured.Unstructured
	nodes        []*unstructured.Unstructured
}

type primitiveResourceGroup struct {
	gvr     schema.GroupVersionResource
	kind    string
	objects []*unstructured.Unstructured
}

// groups returns the resource groups in the same order the c9s Topology controller emits them:
// profiles carry the policy Nodes resolve, and Links must be complete before their Nodes so the
// node controller never plans against a partial wiring view.
func (s *primitiveResourceSet) groups() []primitiveResourceGroup {
	return []primitiveResourceGroup{
		{gvr: nodeProfileGVR, kind: "NodeProfile", objects: s.nodeProfiles},
		{gvr: linkGVR, kind: "Link", objects: s.links},
		{gvr: nodeGVR, kind: "Node", objects: s.nodes},
	}
}

// compilePrimitiveResources runs the same fail-closed compiler and renderer used by c9s' Topology
// controller, but keeps the Topology in memory. Only the resulting O(1)-per-object NodeProfile,
// Link, and Node resources are sent to the Kubernetes API server.
func compilePrimitiveResources(
	desiredTopology *unstructured.Unstructured,
) (*primitiveResourceSet, error) {
	if desiredTopology == nil {
		return nil, fmt.Errorf("clabernetes topology is nil")
	}

	topology := &clabernetesapisv1alpha1.Topology{}
	if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(
		desiredTopology.Object,
		topology,
	); err != nil {
		return nil, fmt.Errorf("failed to prepare c9s primitive resources: %w", err)
	}

	compiled, err := clabernetescompiler.CompileTopologyWithOptions(
		c9sCompileLogger{},
		topology,
		clabernetescompiler.CompileOptions{
			UnsupportedFieldPolicy: clabernetescompiler.UnsupportedFieldPolicyError,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compile containerlab topology for c9s: %w", err)
	}

	set := &primitiveResourceSet{}
	for _, profile := range clabernetescompiler.RenderNodeProfiles(
		topology,
		compiled,
		clabernetesconfig.GetFakeManager,
	) {
		obj, err := primitiveObject(profile, "NodeProfile", desiredTopology)
		if err != nil {
			return nil, err
		}
		set.nodeProfiles = append(set.nodeProfiles, obj)
	}

	for _, link := range clabernetescompiler.RenderLinks(
		topology,
		compiled,
		clabernetesconfig.GetFakeManager,
	) {
		obj, err := primitiveObject(link, "Link", desiredTopology)
		if err != nil {
			return nil, err
		}
		set.links = append(set.links, obj)
	}

	for _, node := range clabernetescompiler.RenderNodes(
		topology,
		compiled,
		clabernetesconfig.GetFakeManager,
	) {
		obj, err := primitiveObject(node, "Node", desiredTopology)
		if err != nil {
			return nil, err
		}
		set.nodes = append(set.nodes, obj)
	}

	return set, nil
}

func primitiveObject(
	typedObject any,
	kind string,
	desiredTopology *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	object, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(typedObject)
	if err != nil {
		return nil, fmt.Errorf("failed to render c9s %s: %w", kind, err)
	}

	obj := &unstructured.Unstructured{Object: object}
	obj.SetAPIVersion(c9sAPIVersion)
	obj.SetKind(kind)

	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[labelRuntime] = clabernetesAppValue
	if owner := desiredTopology.GetLabels()[clabconstants.Owner]; owner != "" {
		labels[clabconstants.Owner] = owner
	}
	obj.SetLabels(labels)

	if owner := desiredTopology.GetAnnotations()[clabconstants.Owner]; owner != "" {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[clabconstants.Owner] = owner
		obj.SetAnnotations(annotations)
	}

	return obj, nil
}

// c9sCompileLogger adapts c9s compiler diagnostics to containerlab's logger.
type c9sCompileLogger struct{}

func (c9sCompileLogger) Debug(message string)                 { log.Debug(message) }
func (c9sCompileLogger) Debugf(format string, args ...any)    { log.Debugf(format, args...) }
func (c9sCompileLogger) Info(message string)                  { log.Info(message) }
func (c9sCompileLogger) Infof(format string, args ...any)     { log.Infof(format, args...) }
func (c9sCompileLogger) Warn(message string)                  { log.Warn(message) }
func (c9sCompileLogger) Warnf(format string, args ...any)     { log.Warnf(format, args...) }
func (c9sCompileLogger) Critical(message string)              { log.Error(message) }
func (c9sCompileLogger) Criticalf(format string, args ...any) { log.Errorf(format, args...) }
func (c9sCompileLogger) Fatal(message string)                 { log.Error(message) }
func (c9sCompileLogger) Fatalf(format string, args ...any)    { log.Errorf(format, args...) }
func (c9sCompileLogger) GetName() string                      { return "containerlab-c9s" }
func (c9sCompileLogger) GetLevel() string                     { return "info" }
func (c9sCompileLogger) Write(data []byte) (int, error) {
	log.Info(string(data))

	return len(data), nil
}
