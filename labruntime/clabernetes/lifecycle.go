package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (r *Runtime) Deploy(
	ctx context.Context,
	req clablabruntime.DeployRequest,
) (*clablabruntime.LabState, error) {
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
	log.Info("Preparing clabernetes lab", "name", req.Name, "namespace", namespace)
	topologyResource := r.client.Resource(topologyGVR).Namespace(namespace)

	existingTopology, err := topologyResource.Get(ctx, req.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		existingTopology = nil
		// Expected for a fresh lab and for labs deployed without a Topology resource.
	case err != nil:
		return nil, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	// A Topology-owned lab cannot be reconciled directly: the controller owns the primitive
	// resources and would revert every direct write. Fail closed instead of fighting it.
	if req.NoTopologyCR && existingTopology != nil {
		return nil, fmt.Errorf(
			"clabernetes lab %s/%s is owned by a Topology resource; "+
				"deploy without --no-topology-cr to reconcile it, or destroy the lab first",
			namespace, req.Name)
	}

	primitiveExists, err := r.primitiveLabExists(ctx, req.Name, namespace)
	if err != nil {
		return nil, err
	}

	prepared, err := prepareDesiredDeployment(req, namespace)
	if err != nil {
		return nil, err
	}
	stagedConfigMaps := prepared.configMaps
	primitives := prepared.primitives
	log.Info(
		"Staging clabernetes lab artifacts",
		"name", req.Name,
		"namespace", namespace,
		"config-maps", len(stagedConfigMaps),
	)

	// The Topology resource is the primary deployment path: containerlab persists the rendered
	// definition and the clabernetes controller compiles and owns the Node, Link, and NodeProfile
	// resources. --no-topology-cr keeps that compilation client-side instead and manages the
	// primitive resources directly.
	if existingTopology != nil {
		return r.reconcileTopology(
			ctx,
			req,
			namespace,
			prepared,
		)
	}
	if !req.NoTopologyCR {
		return r.deployTopology(ctx, req, namespace, prepared, primitiveExists)
	}

	managedNamespace := req.Namespace == "" && r.labNamespaceOverride == ""
	namespaceCreated, err := r.ensureLabNamespace(
		ctx,
		req.Name,
		namespace,
		managedNamespace,
	)
	if err != nil {
		return nil, err
	}
	cleanupNamespace := func() {
		if namespaceCreated {
			_, _ = r.deleteManagedLabNamespace(ctx, req.Name, namespace)
		}
	}
	if err = r.applyStagedConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
		cleanupNamespace()

		return nil, err
	}

	operation := "Creating"
	if primitiveExists {
		operation = "Reconciling"
	}
	log.Info(
		operation+" clabernetes lab resources",
		"name", req.Name,
		"namespace", namespace,
		"nodes", len(primitives.nodes),
		"links", len(primitives.links),
	)
	appliedNodes, createdResources, err := r.reconcilePrimitiveResources(
		ctx,
		namespace,
		req.Name,
		primitives,
	)
	if err != nil {
		if !primitiveExists {
			r.deleteCreatedPrimitiveResources(ctx, namespace, createdResources)
			r.deleteStagedConfigMaps(ctx, namespace, stagedConfigMaps)
			cleanupNamespace()
		}

		return nil, err
	}

	if err = r.setStagedConfigMapNodeOwnerReferences(
		ctx,
		namespace,
		stagedConfigMaps,
		appliedNodes,
	); err != nil {
		if !primitiveExists {
			r.deleteCreatedPrimitiveResources(ctx, namespace, createdResources)
			r.deleteStagedConfigMaps(ctx, namespace, stagedConfigMaps)
			cleanupNamespace()
		}

		return nil, err
	}
	if err = r.deleteStaleConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
		return nil, err
	}
	if !req.Wait {
		return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}

	if err = r.waitPrimitiveLinksResolved(
		ctx,
		namespace,
		primitives.links,
		req.Timeout,
	); err != nil {
		if !primitiveExists {
			r.deleteCreatedPrimitiveResources(ctx, namespace, createdResources)
			r.deleteStagedConfigMaps(ctx, namespace, stagedConfigMaps)
			cleanupNamespace()
		}

		return nil, err
	}

	if err := r.waitReady(ctx, req.Name, namespace, req.Timeout); err != nil {
		// A fresh deployment is transactional: timeout/failure removes only resources this
		// operation created. A failed reconciliation retains the existing lab for diagnosis and
		// recovery because rolling it back would destroy prior working state.
		if !primitiveExists {
			r.deleteCreatedPrimitiveResources(ctx, namespace, createdResources)
			r.deleteStagedConfigMaps(ctx, namespace, stagedConfigMaps)
			cleanupNamespace()
		}

		return nil, err
	}

	return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
}

func (r *Runtime) Destroy(ctx context.Context, req clablabruntime.DestroyRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topology name is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return err
	}
	selector := labels.Set{labelTopologyOwner: req.Name}.String()

	log.Info("Deleting clabernetes lab resources", "name", req.Name, "namespace", namespace)

	var deleteErrors []error
	// Delete the owning Topology first when present so the controller cannot recreate compiler
	// output while the primitive resources are being removed.
	err = r.client.Resource(topologyGVR).Namespace(namespace).
		Delete(ctx, req.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		deleteErrors = append(deleteErrors, fmt.Errorf(
			"failed to delete clabernetes topology %s/%s: %w",
			namespace, req.Name, err))
	}

	for _, gvr := range []struct {
		name string
		gvr  schema.GroupVersionResource
	}{
		{name: "nodes", gvr: nodeGVR},
		{name: "links", gvr: linkGVR},
		{name: "node profiles", gvr: nodeProfileGVR},
	} {
		list, listErr := r.primitiveResourcesForTopology(ctx, gvr.gvr, req.Name, namespace)
		if listErr != nil {
			deleteErrors = append(deleteErrors, listErr)
			continue
		}
		for idx := range list.Items {
			err = r.client.Resource(gvr.gvr).Namespace(namespace).
				Delete(ctx, list.Items[idx].GetName(), metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				deleteErrors = append(deleteErrors, fmt.Errorf(
					"failed to delete c9s %s %s/%s: %w",
					gvr.name, namespace, list.Items[idx].GetName(), err))
			}
		}
	}

	configMaps, err := r.kubeClient.CoreV1().ConfigMaps(namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: selector},
	)
	if err != nil {
		deleteErrors = append(deleteErrors, fmt.Errorf(
			"failed to list staged ConfigMaps for c9s lab %s/%s: %w",
			namespace, req.Name, err))
	} else {
		for idx := range configMaps.Items {
			err = r.kubeClient.CoreV1().ConfigMaps(namespace).
				Delete(ctx, configMaps.Items[idx].Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				deleteErrors = append(deleteErrors, fmt.Errorf(
					"failed to delete staged ConfigMap %s/%s: %w",
					namespace, configMaps.Items[idx].Name, err))
			}
		}
	}

	if len(deleteErrors) != 0 {
		return errors.Join(deleteErrors...)
	}

	if req.Wait {
		if err := r.waitDeleted(ctx, req.Name, namespace, req.Timeout); err != nil {
			return err
		}
	}

	namespaceDeleted, err := r.deleteManagedLabNamespace(ctx, req.Name, namespace)
	if err != nil {
		return err
	}
	if !req.Wait || !namespaceDeleted {
		return nil
	}

	return r.waitNamespaceDeleted(ctx, namespace, req.Timeout)
}

func (r *Runtime) Inspect(
	ctx context.Context,
	req clablabruntime.InspectRequest,
) (*clablabruntime.LabState, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return nil, err
	}
	obj, err := r.client.Resource(topologyGVR).Namespace(namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to inspect clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	var state *clablabruntime.LabState
	if err == nil {
		state = stateFromTopology(obj, namespace)
	} else {
		nodes, listErr := r.nodesForTopology(ctx, req.Name, namespace)
		if listErr != nil {
			return nil, listErr
		}
		if len(nodes.Items) == 0 {
			return nil, fmt.Errorf("c9s lab %s/%s was not found", namespace, req.Name)
		}
		state = stateFromNodeResources(req.Name, namespace, nodes.Items)
	}

	if err := r.enrichState(ctx, state); err != nil {
		log.Debug("failed to enrich clabernetes lab state", "error", err)
	}

	return state, nil
}

func (r *Runtime) LabExists(
	ctx context.Context,
	req clablabruntime.InspectRequest,
) (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("topology name is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return false, err
	}
	_, err = r.client.Resource(topologyGVR).Namespace(namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		return true, nil
	case !apierrors.IsNotFound(err):
		return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	return r.primitiveLabExists(ctx, req.Name, namespace)
}

func (r *Runtime) List(
	ctx context.Context,
	req clablabruntime.ListRequest,
) ([]*clablabruntime.LabState, error) {
	namespace := r.namespaceFor(req.Namespace)
	if req.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	topologyList, err := r.client.Resource(topologyGVR).Namespace(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes topologies: %w", err)
	}

	statesByLab := make(map[string]*clablabruntime.LabState, len(topologyList.Items))
	for idx := range topologyList.Items {
		state := stateFromTopology(&topologyList.Items[idx], namespace)
		statesByLab[state.Namespace+"/"+state.Name] = state
	}

	nodeList, err := r.client.Resource(nodeGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelTopologyOwner,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list c9s nodes: %w", err)
	}

	nodesByLab := map[string][]unstructured.Unstructured{}
	for idx := range nodeList.Items {
		node := nodeList.Items[idx]
		labName := node.GetLabels()[labelTopologyOwner]
		if labName == "" {
			continue
		}
		key := node.GetNamespace() + "/" + labName
		nodesByLab[key] = append(nodesByLab[key], node)
	}
	for key, nodes := range nodesByLab {
		if _, exists := statesByLab[key]; exists {
			continue
		}
		statesByLab[key] = stateFromNodeResources(
			nodes[0].GetLabels()[labelTopologyOwner],
			nodes[0].GetNamespace(),
			nodes,
		)
	}

	states := make([]*clablabruntime.LabState, 0, len(statesByLab))
	for _, state := range statesByLab {
		if err := r.enrichState(ctx, state); err != nil {
			log.Debug("failed to enrich clabernetes lab state",
				"name", state.Name,
				"namespace", state.Namespace,
				"error", err,
			)
		}
		states = append(states, state)
	}

	sort.Slice(states, func(i, j int) bool {
		if states[i].Namespace == states[j].Namespace {
			return states[i].Name < states[j].Name
		}
		return states[i].Namespace < states[j].Namespace
	})

	return states, nil
}

func (r *Runtime) waitReady(
	ctx context.Context,
	name, namespace string,
	timeout time.Duration,
) error {
	effectiveTimeout := r.timeoutFor(timeout)
	waitCtx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	var lastState *clablabruntime.LabState
	readinessProgress := nodeReadinessProgress{}
	imageProgress := imagePullProgress{}
	phaseProgress := nodePhaseProgress{}
	log.Info("Waiting for clabernetes lab to become ready", "name", name, "namespace", namespace)

	err := wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			state, err := r.Inspect(ctx, clablabruntime.InspectRequest{
				Name:      name,
				Namespace: namespace,
			})
			if err != nil {
				// client-go's rate limiter can refuse a request shortly before the context
				// expires with "would exceed context deadline". Let the poller return the
				// authoritative timeout instead of masking it with that implementation detail.
				if ctx.Err() != nil || contextDeadlineIsImminent(ctx) {
					return false, nil
				}

				return false, fmt.Errorf("failed to inspect clabernetes lab %s/%s: %w",
					namespace, name, err)
			}
			lastState = state
			imageProgress.observe(ctx, r, namespace, state)
			phaseProgress.observe(ctx, r, name, namespace)
			readinessProgress.report(state)

			if state.Ready {
				return true, nil
			}
			if failure := phaseProgress.terminalFailure(); failure != "" {
				return false, fmt.Errorf(
					"clabernetes lab %s/%s cannot be deployed: %s",
					namespace, name, failure)
			}
			if state.State == "deployfailed" {
				return false, fmt.Errorf("clabernetes topology %s/%s reported deployfailed",
					namespace, name)
			}

			log.Debug("Waiting for clabernetes lab",
				"name", name,
				"namespace", namespace,
				"state", state.State,
			)

			return false, nil
		})
	if err == nil {
		log.Info("Clabernetes lab is ready", "name", name, "namespace", namespace)

		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	pendingNodes := make([]string, 0)
	if lastState != nil {
		for _, node := range lastState.Nodes {
			if node.Ready {
				continue
			}
			state := node.State
			if state == "" {
				state = "unknown"
			}
			pendingNodes = append(pendingNodes, fmt.Sprintf("%s (%s)", node.Name, state))
		}
	}

	if len(pendingNodes) == 0 {
		return fmt.Errorf(
			"timed out after %s waiting for clabernetes lab %s/%s to become ready",
			effectiveTimeout,
			namespace,
			name,
		)
	}

	return fmt.Errorf(
		"timed out after %s waiting for clabernetes lab %s/%s to become ready; "+
			"pending nodes: %s",
		effectiveTimeout,
		namespace,
		name,
		strings.Join(pendingNodes, ", "),
	)
}

type trackedNodeReadiness struct {
	state string
	ready bool
}

type nodeReadinessProgress struct {
	nodes map[string]trackedNodeReadiness
}

func (p *nodeReadinessProgress) report(state *clablabruntime.LabState) {
	if state == nil {
		return
	}

	readyCount := 0
	for _, node := range state.Nodes {
		if node.Ready {
			readyCount++
		}
	}

	next := make(map[string]trackedNodeReadiness, len(state.Nodes))
	for _, node := range state.Nodes {
		current := trackedNodeReadiness{state: node.State, ready: node.Ready}
		previous, seen := p.nodes[node.Name]
		next[node.Name] = current

		// The initial non-ready snapshot is summarized by the wait message. Report nodes that
		// are already ready on the first poll, and every meaningful transition after that.
		if !node.Ready && (!seen || previous == current) {
			continue
		}
		if seen && previous == current {
			continue
		}

		if node.Ready {
			log.Info(
				"Clabernetes node is ready",
				"node", node.Name,
				"ready", readyCount,
				"total", len(state.Nodes),
			)
			continue
		}

		nodeState := node.State
		if nodeState == "" {
			nodeState = "unknown"
		}
		log.Info(
			"Clabernetes node is not ready",
			"node", node.Name,
			"state", nodeState,
			"ready", readyCount,
			"total", len(state.Nodes),
		)
	}

	p.nodes = next
}

func contextDeadlineIsImminent(ctx context.Context) bool {
	deadline, ok := ctx.Deadline()

	return ok && time.Until(deadline) <= pollInterval
}

func (r *Runtime) waitDeleted(
	ctx context.Context,
	name, namespace string,
	timeout time.Duration,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			_, err := r.client.Resource(topologyGVR).Namespace(namespace).
				Get(ctx, name, metav1.GetOptions{})
			switch {
			case err != nil:
				if apierrors.IsNotFound(err) {
					break
				}
				return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
					namespace, name, err)
			default:
				return false, nil
			}

			for _, gvr := range []schema.GroupVersionResource{
				nodeGVR,
				linkGVR,
				nodeProfileGVR,
			} {
				list, err := r.primitiveResourcesForTopology(ctx, gvr, name, namespace)
				if err != nil {
					return false, err
				}
				if len(list.Items) != 0 {
					return false, nil
				}
			}

			return true, nil
		})
}
