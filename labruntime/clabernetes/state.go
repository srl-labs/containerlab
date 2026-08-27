package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func stateFromNodeResources(
	name,
	namespace string,
	nodeResources []unstructured.Unstructured,
) *clablabruntime.LabState {
	state := &clablabruntime.LabState{
		Name:         name,
		Namespace:    namespace,
		TopologyPath: fmt.Sprintf("k8s://%s/labs/%s", namespace, name),
	}
	if len(nodeResources) != 0 {
		state.Owner = nodeResources[0].GetLabels()[clabconstants.Owner]
		if state.Owner == "" {
			state.Owner = nodeResources[0].GetAnnotations()[clabconstants.Owner]
		}
	}

	state.Nodes = make([]clablabruntime.NodeState, 0, len(nodeResources))
	for idx := range nodeResources {
		state.Nodes = append(state.Nodes, nodeStateFromResource(&nodeResources[idx]))
	}

	return state
}

func nodeStateFromResource(nodeResource *unstructured.Unstructured) clablabruntime.NodeState {
	node := clablabruntime.NodeState{Name: nodeResource.GetName()}
	node.Kind, _, _ = unstructured.NestedString(nodeResource.Object, "spec", "kind")
	node.Image, _, _ = unstructured.NestedString(nodeResource.Object, "spec", "image")
	node.State, _, _ = unstructured.NestedString(nodeResource.Object, "status", "readiness")
	node.Ready = node.State == "ready"
	node.LoadBalancerAddress, _, _ = unstructured.NestedString(
		nodeResource.Object,
		"status",
		"exposedPorts",
		"loadBalancerAddress",
	)
	node.MgmtIPv4Address, _, _ = unstructured.NestedString(
		nodeResource.Object,
		"status",
		"directManagement",
		"ipv4",
	)
	node.MgmtIPv6Address, _, _ = unstructured.NestedString(
		nodeResource.Object,
		"status",
		"directManagement",
		"ipv6",
	)

	return node
}

func (r *Runtime) enrichState(ctx context.Context, state *clablabruntime.LabState) error {
	if state == nil || state.Name == "" {
		return nil
	}

	nodeResources, err := r.nodesForTopology(ctx, state.Name, state.Namespace)
	if err != nil {
		return err
	}

	nodesByName := map[string]clablabruntime.NodeState{}
	for _, node := range state.Nodes {
		nodesByName[node.Name] = node
	}

	networkModes := make(map[string]string, len(nodeResources.Items))
	for idx := range nodeResources.Items {
		nodeResource := &nodeResources.Items[idx]
		nodesByName[nodeResource.GetName()] = nodeStateFromResource(nodeResource)
		networkModes[nodeResource.GetName()] = nodeNetworkMode(nodeResource)
	}

	deployments, err := r.deploymentsForTopology(ctx, state.Name, state.Namespace)
	if err != nil {
		return err
	}

	pods, err := r.kubeClient.CoreV1().Pods(state.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: state.Name,
		}.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list clabernetes pods for topology %s/%s: %w",
			state.Namespace, state.Name, err)
	}

	podsByNode := map[string]*corev1.Pod{}
	for idx := range pods.Items {
		nodeName := pods.Items[idx].Labels[labelTopologyNode]
		if nodeName == "" {
			continue
		}
		if pods.Items[idx].Status.Phase == corev1.PodRunning {
			podsByNode[nodeName] = &pods.Items[idx]
			continue
		}
		if _, ok := podsByNode[nodeName]; !ok {
			podsByNode[nodeName] = &pods.Items[idx]
		}
	}

	for idx := range deployments.Items {
		deployment := &deployments.Items[idx]
		nodeName := deployment.Labels[labelTopologyNode]
		if nodeName == "" {
			continue
		}

		replicas := int32(1)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}

		deploymentState := "notready"
		deploymentReady := false
		switch {
		case replicas == 0:
			deploymentState = "stopped"
		case deployment.Status.ReadyReplicas > 0:
			deploymentState = "ready"
			deploymentReady = true
		case podsByNode[nodeName] != nil && podsByNode[nodeName].Status.Phase != "":
			deploymentState = strings.ToLower(string(podsByNode[nodeName].Status.Phase))
		}

		matched := false
		for logicalNodeName, node := range nodesByName {
			if resolvePrimaryNode(logicalNodeName, networkModes) != nodeName {
				continue
			}

			// Node status is the authoritative signal that the nested containerlab node is ready.
			// A launcher Deployment can be ready before containerlab has created the inner
			// container. Deployment state remains the fallback for older resources without Node
			// readiness, and replicas=0 is always an explicit stopped state.
			if replicas == 0 || node.State == "" {
				node.State = deploymentState
				node.Ready = deploymentReady
			}
			nodesByName[logicalNodeName] = node
			matched = true
		}

		if !matched {
			node := nodesByName[nodeName]
			node.Name = nodeName
			node.State = deploymentState
			node.Ready = deploymentReady
			nodesByName[nodeName] = node
		}
	}

	nodeNames := make([]string, 0, len(nodesByName))
	for nodeName := range nodesByName {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	state.Nodes = make([]clablabruntime.NodeState, 0, len(nodeNames))
	allReady := len(nodeNames) > 0
	allStopped := len(nodeNames) > 0
	for _, nodeName := range nodeNames {
		node := nodesByName[nodeName]
		state.Nodes = append(state.Nodes, node)
		allReady = allReady && node.Ready
		allStopped = allStopped && node.State == "stopped"
	}

	switch {
	case allReady:
		state.State = "running"
		state.Ready = true
	case allStopped:
		state.State = "stopped"
		state.Ready = false
	case len(nodeNames) != 0:
		state.State = "partial"
		state.Ready = false
	}

	return nil
}
