package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// reconcilePrimitiveResources converges the directly managed c9s resources on the compiled
// topology in the same order the c9s Topology controller uses: NodeProfiles carry the policy
// Nodes resolve, and the complete Link set is applied before Nodes so the node controller never
// plans a workload against a partial wiring view.
func (r *Runtime) reconcilePrimitiveResources(
	ctx context.Context,
	namespace,
	topologyName string,
	desired *primitiveResourceSet,
) (
	map[string]*unstructured.Unstructured,
	[]createdPrimitiveResource,
	error,
) {
	existing, err := r.primitiveResourceInventory(ctx, namespace)
	if err != nil {
		return nil, nil, err
	}
	if err := validatePrimitiveResourceOwnership(existing, desired, topologyName, namespace); err != nil {
		return nil, nil, err
	}

	appliedNodes := map[string]*unstructured.Unstructured{}
	created := []createdPrimitiveResource{}

	for _, group := range desired.groups() {
		for _, obj := range group.objects {
			actual, wasCreated, err := r.reconcilePrimitiveResource(
				ctx,
				namespace,
				topologyName,
				group.gvr,
				group.kind,
				obj,
			)
			if err != nil {
				return nil, created, err
			}
			if wasCreated {
				created = append(created, createdPrimitiveResource{
					gvr: group.gvr, name: actual.GetName(),
				})
			}
			if group.gvr == nodeGVR {
				appliedNodes[actual.GetName()] = actual
			}
		}
	}

	if err := r.deleteStalePrimitiveResources(ctx, namespace, topologyName, desired); err != nil {
		return nil, created, err
	}

	return appliedNodes, created, nil
}

func (r *Runtime) primitiveResourceInventory(
	ctx context.Context,
	namespace string,
) (map[schema.GroupVersionResource]map[string]*unstructured.Unstructured, error) {
	result := map[schema.GroupVersionResource]map[string]*unstructured.Unstructured{}
	for _, gvr := range []schema.GroupVersionResource{nodeProfileGVR, linkGVR, nodeGVR} {
		list, err := r.client.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list c9s %s in namespace %s: %w",
				gvr.Resource, namespace, err)
		}

		result[gvr] = make(map[string]*unstructured.Unstructured, len(list.Items))
		for idx := range list.Items {
			obj := &list.Items[idx]
			// Include resources from other labs so ownership validation can reject same-named
			// collisions instead of overwriting them.
			result[gvr][obj.GetName()] = obj
		}
	}

	return result, nil
}

func validatePrimitiveResourceOwnership(
	existing map[schema.GroupVersionResource]map[string]*unstructured.Unstructured,
	desired *primitiveResourceSet,
	topologyName,
	namespace string,
) error {
	for _, group := range desired.groups() {
		for _, obj := range group.objects {
			actual := existing[group.gvr][obj.GetName()]
			if actual == nil || actual.GetLabels()[labelTopologyOwner] == topologyName {
				continue
			}

			return fmt.Errorf(
				"c9s %s %s/%s already exists and belongs to another lab",
				group.kind,
				namespace,
				obj.GetName(),
			)
		}
	}

	return nil
}

func (r *Runtime) reconcilePrimitiveResource(
	ctx context.Context,
	namespace,
	topologyName string,
	gvr schema.GroupVersionResource,
	kind string,
	desired *unstructured.Unstructured,
) (*unstructured.Unstructured, bool, error) {
	resource := r.client.Resource(gvr).Namespace(namespace)
	var actual *unstructured.Unstructured
	created := false

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing, err := resource.Get(ctx, desired.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			actual, err = resource.Create(ctx, desired.DeepCopy(), metav1.CreateOptions{})
			if err == nil {
				created = true
				return nil
			}
			if apierrors.IsAlreadyExists(err) {
				return apierrors.NewConflict(gvr.GroupResource(), desired.GetName(), err)
			}

			return err
		}
		if err != nil {
			return err
		}
		if existing.GetLabels()[labelTopologyOwner] != topologyName {
			return fmt.Errorf("c9s %s %s/%s already exists and belongs to another lab",
				kind, namespace, desired.GetName())
		}

		updated := reconciledPrimitiveObject(existing, desired)
		if primitiveObjectsConform(existing, updated) {
			actual = existing
			return nil
		}

		actual, err = resource.Update(ctx, updated, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to reconcile c9s %s %s/%s: %w",
			kind, namespace, desired.GetName(), err)
	}

	return actual, created, nil
}

func reconciledPrimitiveObject(
	existing,
	desired *unstructured.Unstructured,
) *unstructured.Unstructured {
	updated := existing.DeepCopy()
	if desiredSpec, ok := desired.Object["spec"]; ok {
		updated.Object["spec"] = k8sruntime.DeepCopyJSONValue(desiredSpec)
	} else {
		delete(updated.Object, "spec")
	}

	updated.SetLabels(mergeDesiredMetadata(existing.GetLabels(), desired.GetLabels()))
	updated.SetAnnotations(mergeDesiredMetadata(existing.GetAnnotations(), desired.GetAnnotations()))
	deleteLifecycleLabels(updated)

	return updated
}

func mergeDesiredMetadata(existing, desired map[string]string) map[string]string {
	result := make(map[string]string, len(existing)+len(desired))
	for key, value := range existing {
		result[key] = value
	}
	for key, value := range desired {
		result[key] = value
	}

	return result
}

func deleteLifecycleLabels(obj *unstructured.Unstructured) {
	labelsMap := obj.GetLabels()
	delete(labelsMap, labelIgnoreReconcile)
	obj.SetLabels(labelsMap)
}

func primitiveObjectsConform(existing, desired *unstructured.Unstructured) bool {
	return apiequality.Semantic.DeepEqual(
		normalizeAPIDefaultedJSON(existing.Object["spec"]),
		normalizeAPIDefaultedJSON(desired.Object["spec"]),
	) &&
		apiequality.Semantic.DeepEqual(existing.GetLabels(), desired.GetLabels()) &&
		apiequality.Semantic.DeepEqual(existing.GetAnnotations(), desired.GetAnnotations())
}

// normalizeAPIDefaultedJSON removes recursively empty/zero JSON values. Kubernetes CRD
// defaulting materializes fields such as false booleans and zero-second probe configuration in
// stored objects even when the renderer omitted them. Those values are declaratively equivalent;
// treating them as drift would make every plan and reconciliation report an update forever.
func normalizeAPIDefaultedJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized := normalizeAPIDefaultedJSON(child)
			if !isZeroJSONValue(normalized) {
				result[key] = normalized
			}
		}

		return result
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			result[idx] = normalizeAPIDefaultedJSON(typed[idx])
		}

		return result
	default:
		return value
	}
}

func isZeroJSONValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case bool:
		return !typed
	case string:
		return typed == ""
	case int:
		return typed == 0
	case int32:
		return typed == 0
	case int64:
		return typed == 0
	case float32:
		return typed == 0
	case float64:
		return typed == 0
	case map[string]any:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func (r *Runtime) deleteStalePrimitiveResources(
	ctx context.Context,
	namespace,
	topologyName string,
	desired *primitiveResourceSet,
) error {
	desiredNames := map[schema.GroupVersionResource]map[string]struct{}{}
	for _, group := range desired.groups() {
		desiredNames[group.gvr] = make(map[string]struct{}, len(group.objects))
		for _, obj := range group.objects {
			desiredNames[group.gvr][obj.GetName()] = struct{}{}
		}
	}

	var deleteErrors []error
	// Remove obsolete wiring before obsolete Nodes, then remove profiles after no Node can refer
	// to them.
	for _, group := range []primitiveResourceGroup{
		{gvr: linkGVR, kind: "Link"},
		{gvr: nodeGVR, kind: "Node"},
		{gvr: nodeProfileGVR, kind: "NodeProfile"},
	} {
		resource := r.client.Resource(group.gvr).Namespace(namespace)
		list, err := resource.List(ctx, metav1.ListOptions{
			LabelSelector: labels.Set{labelTopologyOwner: topologyName}.String(),
		})
		if err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf(
				"failed to list c9s %s resources for lab %s/%s: %w",
				group.kind, namespace, topologyName, err))
			continue
		}

		for idx := range list.Items {
			name := list.Items[idx].GetName()
			if _, keep := desiredNames[group.gvr][name]; keep {
				continue
			}
			if err := resource.Delete(ctx, name, metav1.DeleteOptions{}); err != nil &&
				!apierrors.IsNotFound(err) {
				deleteErrors = append(deleteErrors, fmt.Errorf(
					"failed to delete stale c9s %s %s/%s: %w",
					group.kind, namespace, name, err))
			}
		}
	}

	return errors.Join(deleteErrors...)
}

// deployTopology creates the lab's Topology resource and lets the clabernetes controller compile
// and own the primitive resources. Pre-existing primitive resources that carry the lab's
// ownership labels are adopted by the controller.
func (r *Runtime) deployTopology(
	ctx context.Context,
	req clablabruntime.DeployRequest,
	namespace string,
	prepared *preparedDeployment,
	primitiveExists bool,
) (*clablabruntime.LabState, error) {
	stagedConfigMaps := prepared.configMaps

	managedNamespace := req.Namespace == "" && r.labNamespaceOverride == ""
	namespaceCreated, err := r.ensureLabNamespace(ctx, req.Name, namespace, managedNamespace)
	if err != nil {
		return nil, err
	}
	// A fresh deployment is transactional: a failure removes only what this operation created.
	// Once the controller has adopted pre-existing primitive resources, deleting the Topology
	// would cascade to them, so an existing lab is always retained for diagnosis and recovery.
	cleanupFresh := func(topologyCreated bool) {
		if primitiveExists {
			return
		}
		if topologyCreated {
			_ = r.client.Resource(topologyGVR).Namespace(namespace).
				Delete(ctx, req.Name, metav1.DeleteOptions{})
		}
		r.deleteStagedConfigMaps(ctx, namespace, stagedConfigMaps)
		if namespaceCreated {
			_, _ = r.deleteManagedLabNamespace(ctx, req.Name, namespace)
		}
	}

	if err = r.applyStagedConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
		if namespaceCreated {
			_, _ = r.deleteManagedLabNamespace(ctx, req.Name, namespace)
		}

		return nil, err
	}

	operation := "Creating"
	if primitiveExists {
		operation = "Adopting"
	}
	log.Info(
		operation+" clabernetes lab topology",
		"name", req.Name,
		"namespace", namespace,
		"nodes", len(prepared.primitives.nodes),
		"links", len(prepared.primitives.links),
	)
	_, err = r.client.Resource(topologyGVR).Namespace(namespace).
		Create(ctx, prepared.topology.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		cleanupFresh(false)

		return nil, fmt.Errorf("failed to create clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}

	if err = r.awaitTopologyConverged(ctx, req, namespace, prepared.configMaps); err != nil {
		cleanupFresh(true)

		return nil, err
	}

	return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
}

// reconcileTopology updates the definition of an existing Topology-owned lab in place and waits
// for the controller to converge the compiled primitive resources on it.
func (r *Runtime) reconcileTopology(
	ctx context.Context,
	req clablabruntime.DeployRequest,
	namespace string,
	prepared *preparedDeployment,
) (*clablabruntime.LabState, error) {
	desired := prepared.topology
	stagedConfigMaps := prepared.configMaps

	if err := r.applyStagedConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
		return nil, err
	}

	resource := r.client.Resource(topologyGVR).Namespace(namespace)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := resource.Get(ctx, req.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// The Topology naming field is immutable and required once set; carry the stored value
		// forward so replacing the spec cannot strip it.
		if naming, found, _ := unstructured.NestedString(
			latest.Object, "spec", "naming",
		); found && naming != "" {
			desired = desired.DeepCopy()
			if err := unstructured.SetNestedField(
				desired.Object, naming, "spec", "naming",
			); err != nil {
				return err
			}
		}

		updated := reconciledPrimitiveObject(latest, desired)
		if primitiveObjectsConform(latest, updated) {
			return nil
		}

		_, err = resource.Update(ctx, updated, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}
	if err := r.awaitTopologyConverged(ctx, req, namespace, prepared.configMaps); err != nil {
		return nil, err
	}

	return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
}

// awaitTopologyConverged waits until the controller has observed the Topology's current
// generation and the lab reports ready, then binds the staged ConfigMaps to the
// controller-created Nodes and prunes ConfigMaps no longer referenced.
func (r *Runtime) awaitTopologyConverged(
	ctx context.Context,
	req clablabruntime.DeployRequest,
	namespace string,
	stagedConfigMaps []stagedConfigMap,
) error {
	if err := r.waitTopologyGenerationObserved(ctx, namespace, req.Name, req.Timeout); err != nil {
		return err
	}
	if err := r.waitReady(ctx, req.Name, namespace, req.Timeout); err != nil {
		return err
	}

	nodes, err := r.nodesForTopology(ctx, req.Name, namespace)
	if err != nil {
		return err
	}
	nodesByName := make(map[string]*unstructured.Unstructured, len(nodes.Items))
	for idx := range nodes.Items {
		nodesByName[nodes.Items[idx].GetName()] = &nodes.Items[idx]
	}
	if err := r.setStagedConfigMapNodeOwnerReferences(
		ctx,
		namespace,
		stagedConfigMaps,
		nodesByName,
	); err != nil {
		return err
	}

	return r.deleteStaleConfigMaps(ctx, namespace, req.Name, stagedConfigMaps)
}

// waitTopologyGenerationObserved blocks until the controller reports having compiled the
// Topology's current generation, so the readiness check that follows refers to the definition
// this deployment submitted rather than a previous one. A controller error surfaced in the
// Topology status (for example a child resource conflict) aborts the wait immediately. A
// controller that predates observedGeneration never reports the field; the wait then degrades
// to the readiness check instead of running out the clock.
func (r *Runtime) waitTopologyGenerationObserved(
	ctx context.Context,
	namespace,
	name string,
	timeout time.Duration,
) error {
	effectiveTimeout := r.timeoutFor(timeout)
	waitCtx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	err := wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			topology, err := r.client.Resource(topologyGVR).Namespace(namespace).
				Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if ctx.Err() != nil || contextDeadlineIsImminent(ctx) {
					return false, nil
				}

				return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
					namespace, name, err)
			}

			if topologyError, _, _ := unstructured.NestedString(
				topology.Object, "status", "error",
			); topologyError != "" {
				return false, fmt.Errorf("clabernetes topology %s/%s reported: %s",
					namespace, name, topologyError)
			}

			observed, found, _ := unstructured.NestedInt64(
				topology.Object, "status", "observedGeneration",
			)
			if !found {
				return true, nil
			}

			return observed >= topology.GetGeneration(), nil
		})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	return fmt.Errorf(
		"timed out after %s waiting for the clabernetes controller to observe topology %s/%s",
		effectiveTimeout, namespace, name)
}
