package clabernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	pod, err := r.launcherPod(ctx, req.Name, req.Namespace, req.NodeName)
	if err != nil {
		return nil, err
	}

	execCmd := clabexec.NewExecCmdFromSlice(req.Command)
	result := clabexec.NewExecResult(execCmd)
	cmd := append([]string{"docker", "exec", req.NodeName}, req.Command...)

	stdout, stderr, rc, err := r.execInPod(ctx, pod, cmd)
	if err != nil {
		return nil, err
	}

	result.SetReturnCode(rc)
	result.SetStdOut(stdout)
	result.SetStdErr(stderr)

	return result, nil
}

func (r *Runtime) launcherPod(
	ctx context.Context,
	name,
	namespace,
	nodeName string,
) (*corev1.Pod, error) {
	namespace = r.namespaceFor(namespace)
	list, err := r.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: name,
			labelTopologyNode:  nodeName,
		}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes launcher pods for node %s/%s/%s: %w",
			namespace, name, nodeName, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("clabernetes launcher pod for node %s/%s/%s was not found",
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
		log.Warn("multiple clabernetes launcher pods matched node, using newest",
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
	command []string,
) ([]byte, []byte, int, error) {
	if pod == nil {
		return nil, nil, 0, fmt.Errorf("launcher pod is nil")
	}
	if len(command) == 0 {
		return nil, nil, 0, fmt.Errorf("command is required")
	}

	containerName := ""
	if len(pod.Spec.Containers) != 0 {
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
		return stdout.Bytes(), stderr.Bytes(), rc, fmt.Errorf("failed to execute command in pod %s/%s: %w",
			pod.Namespace, pod.Name, err)
	}

	return stdout.Bytes(), stderr.Bytes(), rc, nil
}
