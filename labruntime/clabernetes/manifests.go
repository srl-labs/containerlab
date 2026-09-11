// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clabernetes

import (
	"context"
	"fmt"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// Manifests renders the resources a deploy would create for the topology, in the order the
// runtime applies them, without contacting the cluster. The bundle is the managed lab
// Namespace when containerlab would create one, the staged ConfigMaps every node file mounts
// from, and then either the Topology resource or, for a lab deployed without one, the
// NodeProfile, Link, and Node resources compiled client-side. Every object carries the labels
// the runtime relies on to discover the lab later, so a bundle applied by hand stays
// manageable with inspect, destroy, and the node lifecycle commands.
func (r *Runtime) Manifests(
	_ context.Context,
	req clablabruntime.DeployRequest,
) ([]clablabruntime.Manifest, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}
	if len(req.TopologyDefinition) == 0 {
		return nil, fmt.Errorf("rendered containerlab topology is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareDesiredDeployment(req, namespace)
	if err != nil {
		return nil, err
	}

	manifests := []clablabruntime.Manifest{}

	// A namespace override must already exist, matching the deploy contract; only the
	// canonical per-lab namespace is created by containerlab and therefore emitted.
	if req.Namespace == "" && r.labNamespaceOverride == "" {
		manifest, err := typedManifest(labNamespaceObject(req.Name, namespace))
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}

	for _, staged := range prepared.configMaps {
		manifest, err := typedManifest(stagedConfigMapObject(namespace, req.Name, staged, nil))
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}

	if !req.NoTopologyCR {
		return append(manifests, unstructuredManifest(prepared.topology)), nil
	}

	for _, group := range prepared.primitives.groups() {
		for _, obj := range group.objects {
			manifests = append(manifests, unstructuredManifest(obj))
		}
	}

	return manifests, nil
}

func typedManifest(obj k8sruntime.Object) (clablabruntime.Manifest, error) {
	object, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return clablabruntime.Manifest{}, fmt.Errorf("failed to render manifest: %w", err)
	}

	return unstructuredManifest(&unstructured.Unstructured{Object: object}), nil
}

// unstructuredManifest copies the object and strips the server-populated fields that typed
// conversion materializes as empty values, so the emitted document is a clean input for apply.
func unstructuredManifest(obj *unstructured.Unstructured) clablabruntime.Manifest {
	object := obj.DeepCopy().Object

	unstructured.RemoveNestedField(object, "status")
	if timestamp, found, _ := unstructured.NestedFieldNoCopy(
		object, "metadata", "creationTimestamp",
	); found && timestamp == nil {
		unstructured.RemoveNestedField(object, "metadata", "creationTimestamp")
	}
	if spec, found, _ := unstructured.NestedMap(object, "spec"); found && len(spec) == 0 {
		unstructured.RemoveNestedField(object, "spec")
	}

	copied := &unstructured.Unstructured{Object: object}

	return clablabruntime.Manifest{
		APIVersion: copied.GetAPIVersion(),
		Kind:       copied.GetKind(),
		Namespace:  copied.GetNamespace(),
		Name:       copied.GetName(),
		Object:     object,
	}
}
