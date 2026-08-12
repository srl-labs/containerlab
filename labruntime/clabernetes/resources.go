package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	clabernetesapisv1alpha1 "github.com/clabernetes/clabernetes/apis/v1alpha1"
	clabernetesconstants "github.com/clabernetes/clabernetes/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (r *Runtime) nodesForTopology(
	ctx context.Context,
	name,
	namespace string,
) (*unstructured.UnstructuredList, error) {
	namespace = r.namespaceFor(namespace)

	list, err := r.client.Resource(nodeGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{labelTopologyOwner: name}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list c9s nodes for topology %s/%s: %w",
			namespace, name, err)
	}

	return list, nil
}

func (r *Runtime) primitiveResourcesForTopology(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	name,
	namespace string,
) (*unstructured.UnstructuredList, error) {
	namespace = r.namespaceFor(namespace)

	list, err := r.client.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{labelTopologyOwner: name}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list c9s %s for lab %s/%s: %w",
			gvr.Resource, namespace, name, err)
	}

	return list, nil
}

func (r *Runtime) primitiveLabExists(
	ctx context.Context,
	name,
	namespace string,
) (bool, error) {
	for _, gvr := range []schema.GroupVersionResource{nodeGVR, linkGVR, launcherProfileGVR} {
		list, err := r.primitiveResourcesForTopology(ctx, gvr, name, namespace)
		if err != nil {
			return false, err
		}
		if len(list.Items) != 0 {
			return true, nil
		}
	}

	return false, nil
}

type createdPrimitiveResource struct {
	gvr  schema.GroupVersionResource
	name string
}

func (r *Runtime) waitPrimitiveLinksResolved(
	ctx context.Context,
	namespace string,
	desiredLinks []*unstructured.Unstructured,
	timeout time.Duration,
) error {
	if len(desiredLinks) == 0 {
		return nil
	}

	desiredNames := make(map[string]struct{}, len(desiredLinks))
	for _, link := range desiredLinks {
		desiredNames[link.GetName()] = struct{}{}
	}

	effectiveTimeout := r.timeoutFor(timeout)
	waitCtx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	var pending []string
	err := wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			links, err := r.client.Resource(linkGVR).Namespace(namespace).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				if ctx.Err() != nil || contextDeadlineIsImminent(ctx) {
					return false, nil
				}

				return false, fmt.Errorf("failed to inspect c9s links in namespace %s: %w",
					namespace, err)
			}

			linksByName := make(map[string]*unstructured.Unstructured, len(links.Items))
			for idx := range links.Items {
				linksByName[links.Items[idx].GetName()] = &links.Items[idx]
			}

			pending = pending[:0]
			for name := range desiredNames {
				link, ok := linksByName[name]
				if !ok {
					pending = append(pending, name+" (not found)")
					continue
				}

				if reason := primitiveLinkPendingReason(link); reason != "" {
					pending = append(pending, fmt.Sprintf("%s (%s)", name, reason))
				}
			}
			sort.Strings(pending)

			return len(pending) == 0, nil
		})
	if err == nil {
		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	return fmt.Errorf(
		"timed out after %s waiting for c9s links in namespace %s to resolve; pending links: %s",
		effectiveTimeout,
		namespace,
		strings.Join(pending, ", "),
	)
}

func primitiveLinkPendingReason(link *unstructured.Unstructured) string {
	if link == nil {
		return "not found"
	}

	if statusError, _, _ := unstructured.NestedString(
		link.Object,
		"status",
		"error",
	); statusError != "" {
		return statusError
	}

	for _, endpointName := range []string{"endpointA", "endpointB"} {
		nodeName, _, _ := unstructured.NestedString(
			link.Object,
			"status",
			"resolvedEndpoints",
			endpointName,
			"nodeName",
		)
		if nodeName == "" {
			return "waiting for endpoint binding"
		}
		if nodeName == clabernetesapisv1alpha1.LinkHostNodeName {
			continue
		}

		uid, _, _ := unstructured.NestedString(
			link.Object,
			"status",
			"resolvedEndpoints",
			endpointName,
			"uid",
		)
		if uid == "" {
			return "waiting for endpoint identity"
		}
	}

	return ""
}

func (r *Runtime) enablePrimitiveNodeDeployments(
	ctx context.Context,
	namespace string,
	nodes map[string]*unstructured.Unstructured,
) error {
	patch := []byte(fmt.Sprintf(
		`{"metadata":{"labels":{"%s":null}}}`,
		clabernetesconstants.LabelDisableDeployments,
	))

	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		_, err := r.client.Resource(nodeGVR).Namespace(namespace).Patch(
			ctx,
			name,
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to enable c9s Node deployment %s/%s: %w",
				namespace, name, err)
		}
	}

	return nil
}

func (r *Runtime) deleteCreatedPrimitiveResources(
	ctx context.Context,
	namespace string,
	created []createdPrimitiveResource,
) {
	for idx := len(created) - 1; idx >= 0; idx-- {
		resource := created[idx]
		err := r.client.Resource(resource.gvr).Namespace(namespace).
			Delete(ctx, resource.name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// This is rollback after a more useful create error; keep the original error as the one
			// returned to the caller.
			continue
		}
	}
}

func nodeNetworkMode(node *unstructured.Unstructured) string {
	if node == nil {
		return ""
	}

	networkMode, _, _ := unstructured.NestedString(node.Object, "spec", "network-mode")

	return networkMode
}

func resolveLauncherNode(nodeName string, networkModes map[string]string) string {
	current := nodeName
	seen := map[string]struct{}{}

	for {
		if _, ok := seen[current]; ok {
			return current
		}
		seen[current] = struct{}{}

		primary, ok := strings.CutPrefix(networkModes[current], "container:")
		if !ok || primary == "" {
			return current
		}

		current = primary
	}
}

func (r *Runtime) launcherNodeNames(
	ctx context.Context,
	topologyName,
	namespace string,
	nodeNames []string,
) (map[string]string, error) {
	nodes, err := r.nodesForTopology(ctx, topologyName, namespace)
	if err != nil {
		return nil, err
	}

	networkModes := make(map[string]string, len(nodes.Items))
	for idx := range nodes.Items {
		node := &nodes.Items[idx]
		networkModes[node.GetName()] = nodeNetworkMode(node)
	}

	resolved := make(map[string]string, len(nodeNames))
	for _, nodeName := range nodeNames {
		if _, ok := networkModes[nodeName]; !ok {
			return nil, fmt.Errorf("node %q was not found in topology %s/%s",
				nodeName, r.namespaceFor(namespace), topologyName)
		}
		resolved[nodeName] = resolveLauncherNode(nodeName, networkModes)
	}

	return resolved, nil
}

func uniqueLauncherNodes(nodeNames []string, launchers map[string]string) []string {
	unique := make([]string, 0, len(nodeNames))
	seen := map[string]struct{}{}

	for _, nodeName := range nodeNames {
		launcher := launchers[nodeName]
		if launcher == "" {
			launcher = nodeName
		}
		if _, ok := seen[launcher]; ok {
			continue
		}

		seen[launcher] = struct{}{}
		unique = append(unique, launcher)
	}

	return unique
}
