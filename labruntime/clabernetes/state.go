package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

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
		nodeName := nodeResource.GetName()
		node := nodesByName[nodeName]
		node.Name = nodeName
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
		nodesByName[nodeName] = node
		networkModes[nodeName] = nodeNetworkMode(nodeResource)
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
			if resolveLauncherNode(logicalNodeName, networkModes) != nodeName {
				continue
			}

			node.State = deploymentState
			node.Ready = deploymentReady
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
