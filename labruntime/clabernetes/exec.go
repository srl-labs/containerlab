package clabernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	kubeexec "k8s.io/client-go/util/exec"
)

func (r *Runtime) Exec(
	ctx context.Context,
	req clablabruntime.ExecRequest,
) (*clabexec.ExecResult, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}
	if req.NodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}
	if len(req.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return nil, err
	}

	nodeName, err := r.resolveNodeName(ctx, req.Name, namespace, req.NodeName)
	if err != nil {
		return nil, err
	}

	pod, err := r.devicePod(ctx, req.Name, namespace, nodeName)
	if err != nil {
		return nil, err
	}

	containerName, err := r.deviceContainerName(ctx, pod, req.Name, namespace, nodeName)
	if err != nil {
		return nil, err
	}

	execCmd := clabexec.NewExecCmdFromSlice(req.Command)
	result := clabexec.NewExecResult(execCmd)

	stdout, stderr, rc, err := r.execInPod(ctx, pod, containerName, req.Command)
	if err != nil {
		return nil, err
	}

	result.SetReturnCode(rc)
	result.SetStdOut(stdout)
	result.SetStdErr(stderr)

	return result, nil
}

// deviceContainerName selects the Kubernetes container representing the logical node inside its
// device pod. The Node controller publishes the deterministic container names in
// status.directContainers; a single-container node uses it directly, and a multi-container
// (chassis) node prefers the primary application container, then the active-preferred CPM
// component ("a" before "b").
func (r *Runtime) deviceContainerName(
	ctx context.Context,
	pod *corev1.Pod,
	topologyName,
	namespace,
	nodeName string,
) (string, error) {
	namespace, err := r.namespaceForLab(topologyName, namespace)
	if err != nil {
		return "", err
	}

	node, err := r.client.Resource(nodeGVR).Namespace(namespace).
		Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get c9s node %s/%s/%s: %w",
			namespace, topologyName, nodeName, err)
	}

	name, err := preferredDeviceContainerName(node, nodeName)
	if err == nil {
		return name, nil
	}

	// Before the plan is applied the Node has no container observations yet; the pod's
	// default-container annotation still identifies the primary application container.
	if pod != nil && pod.Labels[labelTopologyNode] == nodeName {
		if fallback := pod.Annotations["kubectl.kubernetes.io/default-container"]; fallback != "" {
			return fallback, nil
		}
	}

	return "", err
}

func preferredDeviceContainerName(node *unstructured.Unstructured, nodeName string) (string, error) {
	containers, _, _ := unstructured.NestedSlice(node.Object, "status", "directContainers")

	type deviceContainer struct {
		name        string
		componentID string
	}

	observed := make([]deviceContainer, 0, len(containers))
	for _, raw := range containers {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		componentID, _ := entry["componentID"].(string)
		observed = append(observed, deviceContainer{name: name, componentID: componentID})
	}

	if len(observed) == 0 {
		return "", fmt.Errorf("device container for node %q was not observed yet", nodeName)
	}
	if len(observed) == 1 {
		return observed[0].name, nil
	}

	for _, container := range observed {
		if container.componentID == "" {
			return container.name, nil
		}
	}

	sort.Slice(observed, func(i, j int) bool {
		return observed[i].componentID < observed[j].componentID
	})
	// Chassis component IDs carry the imported per-component runtime identity, e.g. "sros-a"
	// for node "sros" slot A. Prefer the active-preferred CPM slots.
	for _, cpmSuffix := range []string{"a", "b"} {
		for _, container := range observed {
			if strings.EqualFold(container.componentID, cpmSuffix) ||
				strings.EqualFold(container.componentID, "cpm-"+cpmSuffix) ||
				strings.EqualFold(container.componentID, nodeName+"-"+cpmSuffix) {
				return container.name, nil
			}
		}
	}

	componentIDs := make([]string, 0, len(observed))
	for _, container := range observed {
		componentIDs = append(componentIDs, container.componentID)
	}

	return "", fmt.Errorf(
		"CPM component container for node %q was not found among %v",
		nodeName,
		componentIDs,
	)
}

// resolveNodeName maps a node name a caller supplied onto the node the lab carries in Kubernetes.
// A name the runtime would not have renamed is used as it is, so the common case costs nothing.
func (r *Runtime) resolveNodeName(
	ctx context.Context,
	topologyName,
	namespace,
	nodeName string,
) (string, error) {
	if clablabruntime.SanitizeName(nodeName) == nodeName {
		return nodeName, nil
	}

	nodes, err := r.nodesForTopology(ctx, topologyName, namespace)
	if err != nil {
		return "", err
	}

	known := make(map[string]struct{}, len(nodes.Items))
	for idx := range nodes.Items {
		known[nodes.Items[idx].GetName()] = struct{}{}
	}

	resolved, ok := resolveKnownNodeName(known, nodeName)
	if !ok {
		return "", fmt.Errorf("node %q was not found in topology %s/%s",
			nodeName, r.namespaceFor(namespace), topologyName)
	}

	return resolved, nil
}

// devicePod resolves the pod hosting the given logical node. Nodes grouped through
// network-mode container: chains share the pod of the group's primary node.
func (r *Runtime) devicePod(
	ctx context.Context,
	name,
	namespace,
	nodeName string,
) (*corev1.Pod, error) {
	var err error
	namespace, err = r.namespaceForLab(name, namespace)
	if err != nil {
		return nil, err
	}
	primaries, err := r.primaryNodeNames(ctx, name, namespace, []string{nodeName})
	if err != nil {
		return nil, err
	}
	primaryNode := primaries[nodeName]

	list, err := r.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: name,
			labelTopologyNode:  primaryNode,
		}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes device pods for node %s/%s/%s: %w",
			namespace, name, nodeName, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("clabernetes device pod for node %s/%s/%s was not found",
			namespace, name, nodeName)
	}

	candidates := make([]*corev1.Pod, 0, len(list.Items))
	for idx := range list.Items {
		if list.Items[idx].Status.Phase == corev1.PodRunning {
			candidates = append(candidates, &list.Items[idx])
		}
	}
	if len(candidates) == 0 {
		for idx := range list.Items {
			candidates = append(candidates, &list.Items[idx])
		}
	}

	// more than one pod can match during a rolling update; use the newest one
	pod := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.CreationTimestamp.After(pod.CreationTimestamp.Time) {
			pod = candidate
		}
	}

	if len(list.Items) > 1 {
		log.Warn("multiple clabernetes device pods matched node, using newest",
			"namespace", namespace,
			"lab", name,
			"node", nodeName,
			"pod", pod.Name,
		)
	}

	return pod, nil
}

func (r *Runtime) execInPod(
	ctx context.Context,
	pod *corev1.Pod,
	containerName string,
	command []string,
) ([]byte, []byte, int, error) {
	if pod == nil {
		return nil, nil, 0, fmt.Errorf("device pod is nil")
	}
	if len(command) == 0 {
		return nil, nil, 0, fmt.Errorf("command is required")
	}

	if containerName == "" && len(pod.Spec.Containers) != 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	req := r.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(r.restConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to create Kubernetes exec executor: %w", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	rc := 0
	if err != nil {
		var exitErr kubeexec.ExitError
		if errors.As(err, &exitErr) {
			rc = exitErr.ExitStatus()
			err = nil
		}
	}
	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), rc, fmt.Errorf(
			"failed to execute command in pod %s/%s: %w",
			pod.Namespace,
			pod.Name,
			err,
		)
	}

	return stdout.Bytes(), stderr.Bytes(), rc, nil
}
