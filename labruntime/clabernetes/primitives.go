// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clabernetes

import (
	"fmt"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcompile "github.com/srl-labs/containerlab/labruntime/clabernetes/compile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// primitiveResourceSet holds the primitive resources one compile produces.
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

// compilePrimitiveResources runs the fail-closed compiler and renderer against the desired
// topology and returns the primitive resources it would create. It mirrors the c9s controller,
// but keeps the Topology in memory. Only the resulting O(1)-per-object NodeProfile, Link, and
// Node resources are sent to the Kubernetes API server.
func compilePrimitiveResources(
	desiredTopology *unstructured.Unstructured,
) (*primitiveResourceSet, error) {
	if desiredTopology == nil {
		return nil, fmt.Errorf("clabernetes topology is nil")
	}

	input, err := compileInputFromUnstructured(desiredTopology)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare c9s primitive resources: %w", err)
	}

	compiled, err := clabcompile.CompileWithOptions(
		clabCompileLogger{},
		input,
		clabcompile.Options{
			UnsupportedFieldPolicy: clabcompile.UnsupportedFieldPolicyError,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compile containerlab topology for c9s: %w", err)
	}

	nodes, links, profiles, err := clabcompile.CompileTopology(input, compiled)
	if err != nil {
		return nil, fmt.Errorf("failed to render c9s resources: %w", err)
	}

	set := &primitiveResourceSet{}
	for i := range profiles {
		obj, err := primitiveObject(&profiles[i], "NodeProfile", desiredTopology)
		if err != nil {
			return nil, err
		}
		set.nodeProfiles = append(set.nodeProfiles, obj)
	}

	for i := range links {
		obj, err := primitiveObject(&links[i], "Link", desiredTopology)
		if err != nil {
			return nil, err
		}
		set.links = append(set.links, obj)
	}

	for i := range nodes {
		obj, err := primitiveObject(&nodes[i], "Node", desiredTopology)
		if err != nil {
			return nil, err
		}
		set.nodes = append(set.nodes, obj)
	}

	return set, nil
}

// compileInputFromUnstructured reads the Topology manifest the runtime prepared into the compile
// contract input: the rendered definition plus the deployment policy fields the compiler
// consumes.
func compileInputFromUnstructured(
	desiredTopology *unstructured.Unstructured,
) (*clabcompile.Input, error) {
	var envelope c9sTopologyEnvelope
	if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(
		desiredTopology.Object,
		&envelope,
	); err != nil {
		return nil, err
	}

	spec := envelope.Spec
	if spec == nil {
		return nil, fmt.Errorf("clabernetes topology %s carries no spec", desiredTopology.GetName())
	}

	input := &clabcompile.Input{
		Name:       desiredTopology.GetName(),
		Namespace:  desiredTopology.GetNamespace(),
		Definition: spec.Definition.Containerlab,
		Deployment: clabcompile.Deployment{
			FilesFromConfigMap: spec.Deployment.FilesFromConfigMap,
			FilesFromSecret:    spec.Deployment.FilesFromSecret,
			FilesFromURL:       spec.Deployment.FilesFromURL,
			Resources:          spec.Deployment.Resources,
			Scheduling:         spec.Deployment.Scheduling,
			Persistence:        spec.Deployment.Persistence,
		},
		DisableManagement: spec.DisableManagement,
		Expose: clabcompile.Expose{
			DisableAutoExpose:      spec.Expose.DisableAutoExpose,
			ExposeType:             spec.Expose.ExposeType,
			UseNodeMgmtIpv4Address: spec.Expose.UseNodeMgmtIpv4Address,
			UseNodeMgmtIpv6Address: spec.Expose.UseNodeMgmtIpv6Address,
		},
		ImagePull: clabcompile.ImagePull{
			Policy:      spec.ImagePull.Policy,
			PullSecrets: spec.ImagePull.PullSecrets,
		},
		StatusProbes: clabcompile.StatusProbes{
			Enabled:                 spec.StatusProbes.Enabled,
			ExcludedNodes:           spec.StatusProbes.ExcludedNodes,
			NodeProbeConfigurations: spec.StatusProbes.NodeProbeConfigurations,
			ProbeConfiguration:      spec.StatusProbes.ProbeConfiguration,
		},
		Annotations: desiredTopology.GetAnnotations(),
		Labels:      desiredTopology.GetLabels(),
	}

	return input, nil
}

// c9sTopologyEnvelope mirrors the clabernetes Topology envelope: metadata is read straight off
// the unstructured object, the spec is converted as a whole.
type c9sTopologyEnvelope struct {
	Spec *c9sTopologySpec `json:"spec"`
}

// c9sTopologySpec mirrors the clabernetes Topology spec fields the compiler consumes. The
// remaining spec fields are irrelevant to the compile and are dropped by the conversion.
type c9sTopologySpec struct {
	Definition struct {
		Containerlab string `json:"containerlab"`
	} `json:"definition"`
	Deployment struct {
		FilesFromConfigMap map[string][]clabcompile.FileFromConfigMap `json:"filesFromConfigMap"`
		FilesFromSecret    map[string][]clabcompile.FileFromSecret    `json:"filesFromSecret"`
		FilesFromURL       map[string][]clabcompile.FileFromURL       `json:"filesFromURL"`
		Resources          map[string]*corev1.ResourceRequirements    `json:"resources"`
		Scheduling         *clabcompile.Scheduling                    `json:"scheduling"`
		Persistence        *clabcompile.Persistence                   `json:"persistence"`
	} `json:"deployment"`
	DisableManagement bool `json:"disableManagement"`
	Expose            struct {
		DisableAutoExpose      bool   `json:"disableAutoExpose"`
		ExposeType             string `json:"exposeType"`
		UseNodeMgmtIpv4Address bool   `json:"useNodeMgmtIpv4Address"`
		UseNodeMgmtIpv6Address bool   `json:"useNodeMgmtIpv6Address"`
	} `json:"expose"`
	ImagePull struct {
		Policy      string   `json:"policy"`
		PullSecrets []string `json:"pullSecrets"`
	} `json:"imagePull"`
	StatusProbes struct {
		Enabled                 bool                                      `json:"enabled"`
		ExcludedNodes           []string                                  `json:"excludedNodes"`
		NodeProbeConfigurations map[string]clabcompile.ProbeConfiguration `json:"nodeProbeConfigurations"`
		ProbeConfiguration      clabcompile.ProbeConfiguration            `json:"probeConfiguration"`
	} `json:"statusProbes"`
}

func primitiveObject(
	rendered *unstructured.Unstructured,
	kind string,
	desiredTopology *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	obj := rendered.DeepCopy()
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

// clabCompileLogger adapts compiler diagnostics to containerlab's logger.
type clabCompileLogger struct{}

func (clabCompileLogger) Debugf(format string, args ...any) { log.Debugf(format, args...) }
func (clabCompileLogger) Warnf(format string, args ...any)  { log.Warnf(format, args...) }
func (clabCompileLogger) Errorf(format string, args ...any) { log.Errorf(format, args...) }
