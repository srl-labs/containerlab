package clabernetes

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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
