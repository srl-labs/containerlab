package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	namespace := r.namespaceFor(req.Namespace)
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	_, err := resource.Get(ctx, req.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		topologyDefinition, stagedConfigMaps, naming, err := stageTopologyLocalFiles(req)
		if err != nil {
			return nil, err
		}

		desired := topologyObject(
			req.Name,
			namespace,
			req.Owner,
			string(topologyDefinition),
			topologyWithNaming(naming),
		)
		if err := setTopologyFilesFromConfigMaps(desired, stagedConfigMaps); err != nil {
			return nil, err
		}

		if err = r.applyStagedConfigMaps(ctx, namespace, req.Name, stagedConfigMaps); err != nil {
			return nil, err
		}

		log.Info("Creating clabernetes topology", "name", req.Name, "namespace", namespace)
		created, createErr := resource.Create(ctx, desired, metav1.CreateOptions{})
		if createErr != nil {
			r.deleteStagedConfigMaps(ctx, namespace, stagedConfigMaps)

			err = createErr
			if apierrors.IsAlreadyExists(err) {
				return nil, duplicateTopologyError(req.Name, namespace)
			}

			return nil, fmt.Errorf("failed to create clabernetes topology %s/%s: %w",
				namespace, req.Name, err)
		}

		if err = r.setStagedConfigMapOwnerReferences(ctx, namespace, stagedConfigMaps, created); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	default:
		return nil, duplicateTopologyError(req.Name, namespace)
	}

	if !req.Wait {
		return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}

	if err := r.waitReady(ctx, req.Name, namespace, req.Timeout); err != nil {
		return nil, err
	}

	return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
}

func duplicateTopologyError(name, namespace string) error {
	return fmt.Errorf(
		"the '%s' lab has already been deployed in namespace '%s'. "+
			"Destroy the lab before deploying a lab with the same name, "+
			"or use '--reconfigure' to redeploy it",
		name,
		namespace,
	)
}

func (r *Runtime) Destroy(ctx context.Context, req clablabruntime.DestroyRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topology name is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	log.Info("Deleting clabernetes topology", "name", req.Name, "namespace", namespace)

	err := resource.Delete(ctx, req.Name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		log.Info("clabernetes topology not found", "name", req.Name, "namespace", namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return nil
	}

	return r.waitDeleted(ctx, req.Name, namespace, req.Timeout)
}

func (r *Runtime) Inspect(
	ctx context.Context,
	req clablabruntime.InspectRequest,
) (*clablabruntime.LabState, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	obj, err := r.client.Resource(topologyGVR).Namespace(namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	state := stateFromTopology(obj, namespace)
	if err := r.enrichState(ctx, state); err != nil {
		log.Debug("failed to enrich clabernetes topology state", "error", err)
	}

	return state, nil
}

func (r *Runtime) List(
	ctx context.Context,
	req clablabruntime.ListRequest,
) ([]*clablabruntime.LabState, error) {
	namespace := r.namespaceFor(req.Namespace)
	if req.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	list, err := r.client.Resource(topologyGVR).Namespace(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes topologies: %w", err)
	}

	states := make([]*clablabruntime.LabState, 0, len(list.Items))
	for idx := range list.Items {
		state := stateFromTopology(&list.Items[idx], namespace)
		if err := r.enrichState(ctx, state); err != nil {
			log.Debug("failed to enrich clabernetes topology state",
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

func (r *Runtime) waitReady(ctx context.Context, name, namespace string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			obj, err := resource.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
					namespace, name, err)
			}

			state := stateFromTopology(obj, namespace)
			if state.Ready {
				return true, nil
			}
			if state.State == "deployfailed" {
				return false, fmt.Errorf("clabernetes topology %s/%s reported deployfailed",
					namespace, name)
			}

			log.Debug("Waiting for clabernetes topology",
				"name", name,
				"namespace", namespace,
				"state", state.State,
			)

			return false, nil
		})
}

func (r *Runtime) waitDeleted(ctx context.Context, name, namespace string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			_, err := resource.Get(ctx, name, metav1.GetOptions{})
			switch {
			case apierrors.IsNotFound(err):
				return true, nil
			case err != nil:
				return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
					namespace, name, err)
			default:
				return false, nil
			}
		})
}
