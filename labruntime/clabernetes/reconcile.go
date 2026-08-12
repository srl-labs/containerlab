package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"time"

	clabernetesconstants "github.com/clabernetes/clabernetes/constants"
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
// topology. New Nodes are staged until the complete Link set is present, preventing their
// launchers from starting against a partially reconciled wiring set.
func (r *Runtime) reconcilePrimitiveResources(
	ctx context.Context,
	namespace,
	topologyName string,
	desired *primitiveResourceSet,
) (
	map[string]*unstructured.Unstructured,
	map[string]*unstructured.Unstructured,
	[]createdPrimitiveResource,
	error,
) {
	existing, err := r.primitiveResourceInventory(ctx, namespace)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validatePrimitiveResourceOwnership(existing, desired, topologyName, namespace); err != nil {
		return nil, nil, nil, err
	}

	appliedNodes := map[string]*unstructured.Unstructured{}
	createdNodes := map[string]*unstructured.Unstructured{}
	created := []createdPrimitiveResource{}

	// Profiles must exist before Nodes can resolve them.
	for _, profile := range desired.launcherProfiles {
		actual, wasCreated, err := r.reconcilePrimitiveResource(
			ctx,
			namespace,
			topologyName,
			launcherProfileGVR,
			"LauncherProfile",
			profile,
			false,
		)
		if err != nil {
			return nil, nil, created, err
		}
		if wasCreated {
			created = append(created, createdPrimitiveResource{
				gvr: launcherProfileGVR, name: actual.GetName(),
			})
		}
	}

	// Materialize new Node identities first, but keep their launcher deployments disabled until
	// every desired Link has been created or updated.
	for _, node := range desired.nodes {
		if existing[nodeGVR][node.GetName()] != nil {
			continue
		}
		actual, wasCreated, err := r.reconcilePrimitiveResource(
			ctx,
			namespace,
			topologyName,
			nodeGVR,
			"Node",
			node,
			true,
		)
		if err != nil {
			return nil, nil, created, err
		}
		appliedNodes[actual.GetName()] = actual
		if wasCreated {
			createdNodes[actual.GetName()] = actual
			created = append(created, createdPrimitiveResource{gvr: nodeGVR, name: actual.GetName()})
		}
	}

	for _, link := range desired.links {
		actual, wasCreated, err := r.reconcilePrimitiveResource(
			ctx,
			namespace,
			topologyName,
			linkGVR,
			"Link",
			link,
			false,
		)
		if err != nil {
			return nil, nil, created, err
		}
		if wasCreated {
			created = append(created, createdPrimitiveResource{gvr: linkGVR, name: actual.GetName()})
		}
	}

	// Existing Nodes are updated only after the full desired Link set is present. This also
	// clears lifecycle staging/stop labels so deploy converges stopped or interrupted labs.
	for _, node := range desired.nodes {
		if existing[nodeGVR][node.GetName()] == nil {
			continue
		}
		actual, _, err := r.reconcilePrimitiveResource(
			ctx,
			namespace,
			topologyName,
			nodeGVR,
			"Node",
			node,
			false,
		)
		if err != nil {
			return nil, nil, created, err
		}
		appliedNodes[actual.GetName()] = actual
	}

	if err := r.deleteStalePrimitiveResources(ctx, namespace, topologyName, desired); err != nil {
		return nil, nil, created, err
	}

	return appliedNodes, createdNodes, created, nil
}

func (r *Runtime) primitiveResourceInventory(
	ctx context.Context,
	namespace string,
) (map[schema.GroupVersionResource]map[string]*unstructured.Unstructured, error) {
	result := map[schema.GroupVersionResource]map[string]*unstructured.Unstructured{}
	for _, gvr := range []schema.GroupVersionResource{launcherProfileGVR, linkGVR, nodeGVR} {
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
	stageNewNode bool,
) (*unstructured.Unstructured, bool, error) {
	resource := r.client.Resource(gvr).Namespace(namespace)
	var actual *unstructured.Unstructured
	created := false

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing, err := resource.Get(ctx, desired.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			toCreate := desired.DeepCopy()
			if stageNewNode {
				setPrimitiveNodeDeploymentStaged(toCreate, true)
			}
			actual, err = resource.Create(ctx, toCreate, metav1.CreateOptions{})
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

		updated := reconciledPrimitiveObject(existing, desired, stageNewNode)
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
	stageNode bool,
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
	if stageNode {
		setPrimitiveNodeDeploymentStaged(updated, true)
	}

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
	delete(labelsMap, clabernetesconstants.LabelDisableDeployments)
	obj.SetLabels(labelsMap)
}

func setPrimitiveNodeDeploymentStaged(obj *unstructured.Unstructured, staged bool) {
	labelsMap := obj.GetLabels()
	if labelsMap == nil {
		labelsMap = map[string]string{}
	}
	if staged {
		labelsMap[clabernetesconstants.LabelDisableDeployments] = "true"
	} else {
		delete(labelsMap, clabernetesconstants.LabelDisableDeployments)
	}
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
		{gvr: launcherProfileGVR, kind: "LauncherProfile"},
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

func (r *Runtime) reconcileCompatibilityTopology(
	ctx context.Context,
	req clablabruntime.DeployRequest,
	namespace string,
	desired *unstructured.Unstructured,
	desiredPrimitives *primitiveResourceSet,
	stagedConfigMaps []stagedConfigMap,
) (*clablabruntime.LabState, error) {
	if err := r.applyStagedConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
		return nil, err
	}

	resource := r.client.Resource(topologyGVR).Namespace(namespace)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := resource.Get(ctx, req.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updated := reconciledPrimitiveObject(latest, desired, false)
		if primitiveObjectsConform(latest, updated) {
			return nil
		}

		_, err = resource.Update(ctx, updated, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile compatibility clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}
	if err := r.waitCompatibilityPrimitivesReconciled(
		ctx,
		namespace,
		req.Name,
		desiredPrimitives,
		req.Timeout,
	); err != nil {
		return nil, err
	}
	if err := r.waitReady(ctx, req.Name, namespace, req.Timeout); err != nil {
		return nil, err
	}

	nodes, err := r.nodesForTopology(ctx, req.Name, namespace)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := r.deleteStaleConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
		return nil, err
	}

	return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
}

func (r *Runtime) waitCompatibilityPrimitivesReconciled(
	ctx context.Context,
	namespace,
	topologyName string,
	desired *primitiveResourceSet,
	timeout time.Duration,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	err := wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			for _, group := range desired.groups() {
				list, err := r.primitiveResourcesForTopology(
					ctx,
					group.gvr,
					topologyName,
					namespace,
				)
				if err != nil {
					if ctx.Err() != nil || contextDeadlineIsImminent(ctx) {
						return false, nil
					}

					return false, err
				}
				if len(list.Items) != len(group.objects) {
					return false, nil
				}

				actualByName := make(map[string]*unstructured.Unstructured, len(list.Items))
				for idx := range list.Items {
					actualByName[list.Items[idx].GetName()] = &list.Items[idx]
				}
				for _, expected := range group.objects {
					actual := actualByName[expected.GetName()]
					if actual == nil || !apiequality.Semantic.DeepEqual(
						actual.Object["spec"],
						expected.Object["spec"],
					) {
						return false, nil
					}
				}
			}

			return true, nil
		})
	if err == nil {
		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	return fmt.Errorf("timed out after %s waiting for compatibility clabernetes topology %s/%s "+
		"resources to reconcile", r.timeoutFor(timeout), namespace, topologyName)
}
