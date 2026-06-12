package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (r *Runtime) enrichState(ctx context.Context, state *clablabruntime.LabState) error {
	if state == nil || state.Name == "" {
		return nil
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

	nodesByName := map[string]clablabruntime.NodeState{}
	for _, node := range state.Nodes {
		nodesByName[node.Name] = node
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

		node := nodesByName[nodeName]
		node.Name = nodeName
		replicas := int32(1)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}

		switch {
		case replicas == 0:
			node.State = "stopped"
			node.Ready = false
		case deployment.Status.ReadyReplicas > 0:
			node.State = "ready"
			node.Ready = true
		case podsByNode[nodeName] != nil && podsByNode[nodeName].Status.Phase != "":
			node.State = strings.ToLower(string(podsByNode[nodeName].Status.Phase))
			node.Ready = false
		default:
			node.State = "notready"
			node.Ready = false
		}

		nodesByName[nodeName] = node
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
