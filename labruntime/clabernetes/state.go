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
	// Without a Topology resource the Nodes record the file the lab was deployed from.
	for idx := range nodeResources {
		if path := nodeResources[idx].GetAnnotations()[annotationTopologyFile]; path != "" {
			state.TopologyPath = path
			break
		}
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
	node := clablabruntime.NodeState{
		Name:    nodeResource.GetName(),
		NodeUID: string(nodeResource.GetUID()),
	}
	if original := nodeResource.GetLabels()[labelOriginalNodeName]; original != node.Name {
		node.TopologyName = original
	}
	node.ExposedPorts = nodeExposedPorts(nodeResource)
	// The same container exec targets: the observed primary device container.
	node.Container, _ = preferredDeviceContainerName(nodeResource, node.Name)
	node.Kind, _, _ = unstructured.NestedString(nodeResource.Object, "spec", "kind")
	node.Image, _, _ = unstructured.NestedString(nodeResource.Object, "spec", "image")
	node.Type, _, _ = unstructured.NestedString(nodeResource.Object, "spec", "type")
	node.Group, _, _ = unstructured.NestedString(nodeResource.Object, "spec", "group")
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

// nodeExposedPorts reads the expose-port allocations the controller published on the Node.
func nodeExposedPorts(nodeResource *unstructured.Unstructured) []clablabruntime.ExposedPort {
	rawPorts, _, _ := unstructured.NestedSlice(
		nodeResource.Object,
		"status",
		"exposedPorts",
		"ports",
	)

	ports := make([]clablabruntime.ExposedPort, 0, len(rawPorts))
	for _, raw := range rawPorts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		port := clablabruntime.ExposedPort{
			Port:       intField(entry["destinationPort"]),
			ExposePort: intField(entry["exposePort"]),
			Protocol:   strings.ToLower(fmt.Sprint(entry["protocol"])),
		}
		if port.Port == 0 {
			continue
		}
		if port.Protocol == "" || port.Protocol == "<nil>" {
			port.Protocol = "tcp"
		}

		ports = append(ports, port)
	}

	if len(ports) == 0 {
		return nil
	}

	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Port == ports[j].Port {
			return ports[i].Protocol < ports[j].Protocol
		}

		return ports[i].Port < ports[j].Port
	})

	return ports
}

func intField(value any) int {
	switch v := value.(type) {
	case int64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// newestPod prefers a pod that is not being deleted, then a running pod and, among equals, the
// newest one: several pods can match a node during a rolling update or a restart, when the old
// pod terminates while its replacement starts.
func newestPod(current, candidate *corev1.Pod) *corev1.Pod {
	if current == nil {
		return candidate
	}

	currentDeleting := current.DeletionTimestamp != nil
	candidateDeleting := candidate.DeletionTimestamp != nil
	if currentDeleting != candidateDeleting {
		if candidateDeleting {
			return current
		}

		return candidate
	}

	currentRunning := current.Status.Phase == corev1.PodRunning
	candidateRunning := candidate.Status.Phase == corev1.PodRunning
	if currentRunning != candidateRunning {
		if candidateRunning {
			return candidate
		}

		return current
	}

	if candidate.CreationTimestamp.After(current.CreationTimestamp.Time) {
		return candidate
	}

	return current
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
	resourcesByName := make(map[string]*unstructured.Unstructured, len(nodeResources.Items))
	for idx := range nodeResources.Items {
		nodeResource := &nodeResources.Items[idx]
		nodesByName[nodeResource.GetName()] = nodeStateFromResource(nodeResource)
		networkModes[nodeResource.GetName()] = nodeNetworkMode(nodeResource)
		resourcesByName[nodeResource.GetName()] = nodeResource
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
		podsByNode[nodeName] = newestPod(podsByNode[nodeName], &pods.Items[idx])
	}

	// The Node's status.readiness is the authoritative device signal. The primary Node's run
	// state governs the pod a group of nodes shares: a stopped primary stops the whole group.
	// Nodes without readiness yet report their pod phase.
	for nodeName, node := range nodesByName {
		primary := resolvePrimaryNode(nodeName, networkModes)
		primaryResource := resourcesByName[primary]
		stopRequested := primaryResource != nil &&
			nodeRunState(primaryResource) == nodeRunStateStopped
		if stopRequested {
			node.State = "stopping"
			if condition := nodeCondition(primaryResource, nodeConditionStopped); condition != nil &&
				condition.status == string(metav1.ConditionTrue) {
				node.State = "stopped"
			}
			node.Ready = false
		} else if pod := podsByNode[primary]; node.State == "" && pod != nil &&
			pod.Status.Phase != "" {
			node.State = strings.ToLower(string(pod.Status.Phase))
		}
		node.ContainerState = nodeContainerState(stopRequested, podsByNode[primary])
		if node.ContainerState == clablabruntime.ContainerStateRunning {
			node.Health = nodeHealth(node, resourcesByName[nodeName])
		}
		nodesByName[nodeName] = node
	}

	// Nodes grouped through network-mode container: share the pod of the group's primary
	// node. Before the controller observed the device containers, the pod's default-container
	// annotation names the primary node's container.
	for nodeName, node := range nodesByName {
		primary := resolvePrimaryNode(nodeName, networkModes)
		pod := podsByNode[primary]
		if pod == nil {
			continue
		}

		node.Pod = pod.Name
		if node.Container == "" && primary == nodeName {
			node.Container = pod.Annotations[defaultContainerAnnotation]
		}
		nodesByName[nodeName] = node
	}

	nodeNames := make([]string, 0, len(nodesByName))
	for nodeName := range nodesByName {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	state.Nodes = make([]clablabruntime.NodeState, 0, len(nodeNames))
	allReady := len(nodeNames) > 0 // codespell:ignore allready
	allStopped := len(nodeNames) > 0
	for _, nodeName := range nodeNames {
		node := nodesByName[nodeName]
		state.Nodes = append(state.Nodes, node)
		allReady = allReady && node.Ready // codespell:ignore allready
		allStopped = allStopped && node.State == "stopped"
	}

	switch {
	case allReady: // codespell:ignore allready
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

// nodeContainerState is the state of a node in container runtime terms, from its pod: created
// while the pod is scheduled and prepared (init containers), running once the device containers
// run, and exited once the node was stopped, its pod is being deleted, or the pod ended. Between
// the pods of a restart the node is exited, then created.
func nodeContainerState(stopRequested bool, pod *corev1.Pod) string {
	switch {
	case pod == nil && stopRequested:
		return clablabruntime.ContainerStateExited
	case pod == nil:
		return clablabruntime.ContainerStateCreated
	case stopRequested || pod.DeletionTimestamp != nil:
		return clablabruntime.ContainerStateExited
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		return clablabruntime.ContainerStateRunning
	case corev1.PodSucceeded, corev1.PodFailed:
		return clablabruntime.ContainerStateExited
	default:
		return clablabruntime.ContainerStateCreated
	}
}

// nodeHealth is the health of a running node: healthy once the Node is ready or its device
// containers are, starting before. Device containers are the analogue of a container
// healthcheck: a node whose links wait for a stopped peer is not ready, but its device is.
func nodeHealth(node clablabruntime.NodeState, resource *unstructured.Unstructured) string {
	if node.Ready {
		return clablabruntime.ContainerHealthHealthy
	}
	if resource != nil {
		if condition := nodeCondition(resource, nodeConditionContainersReady); condition != nil &&
			condition.status == string(metav1.ConditionTrue) {
			return clablabruntime.ContainerHealthHealthy
		}
	}

	return clablabruntime.ContainerHealthStarting
}
