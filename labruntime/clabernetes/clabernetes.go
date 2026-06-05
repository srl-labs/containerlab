package clabernetes

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	kubeexec "k8s.io/client-go/util/exec"
)

const (
	defaultNamespace = "default"
	pollInterval     = 2 * time.Second

	envKubeconfig = "CLAB_KUBECONFIG"
	envContext    = "CLAB_KUBE_CONTEXT"
	envNamespace  = "CLAB_KUBE_NAMESPACE"

	labelApp              = "clabernetes/app"
	labelTopologyOwner    = "clabernetes/topologyOwner"
	labelTopologyNode     = "clabernetes/topologyNode"
	labelIgnoreReconcile  = "clabernetes/ignoreReconcile"
	clabernetesAppValue   = "clabernetes"
	restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"
)

var topologyGVR = schema.GroupVersionResource{
	Group:    "clabernetes.containerlab.dev",
	Version:  "v1alpha1",
	Resource: "topologies",
}

type Runtime struct {
	client     dynamic.Interface
	kubeClient kubernetes.Interface
	restConfig *rest.Config
	namespace  string
	timeout    time.Duration
}

func init() {
	clablabruntime.Register(clablabruntime.ClabernetesRuntimeName, New)
}

func New(cfg clablabruntime.Config) (clablabruntime.LabRuntime, error) {
	kubeConfig, namespace, err := kubeClientConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	return &Runtime{
		client:     client,
		kubeClient: kubeClient,
		restConfig: kubeConfig,
		namespace:  namespace,
		timeout:    cfg.Timeout,
	}, nil
}

func (r *Runtime) Capabilities() clablabruntime.RuntimeCapabilities {
	return clablabruntime.RuntimeCapabilities{
		Deploy:  true,
		Destroy: true,
		Inspect: true,
		List:    true,
		Exec:    true,
		Start:   true,
		Stop:    true,
		Restart: true,
		Save:    true,
		Events:  true,
	}
}

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
	desired := topologyObject(req.Name, namespace, req.Owner, string(req.TopologyDefinition))

	existing, err := resource.Get(ctx, req.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		log.Info("Creating clabernetes topology", "name", req.Name, "namespace", namespace)
		_, err = resource.Create(ctx, desired, metav1.CreateOptions{})
	case err != nil:
		return nil, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	default:
		log.Info("Updating clabernetes topology", "name", req.Name, "namespace", namespace)
		desired.SetResourceVersion(existing.GetResourceVersion())
		_, err = resource.Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to apply clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}

	if err := r.waitReady(ctx, req.Name, namespace, req.Timeout); err != nil {
		return nil, err
	}

	return r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
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

func (r *Runtime) Start(ctx context.Context, req clablabruntime.NodeRequest) error {
	return r.setNodesReplicas(ctx, req, 1)
}

func (r *Runtime) Stop(ctx context.Context, req clablabruntime.NodeRequest) error {
	if err := r.setTopologyIgnoreReconcile(ctx, req.Name, req.Namespace, true); err != nil {
		return err
	}

	return r.setNodesReplicas(ctx, req, 0)
}

func (r *Runtime) Restart(ctx context.Context, req clablabruntime.NodeRequest) error {
	targets, namespace, err := r.targetNodes(ctx, req)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, nodeName := range targets {
		deployment, err := r.deploymentForNode(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return err
		}

		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
		}
		deployment.Spec.Template.ObjectMeta.Annotations[restartedAtAnnotation] = now

		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
			replicas := int32(1)
			deployment.Spec.Replicas = &replicas
		}

		_, err = r.kubeClient.AppsV1().Deployments(namespace).
			Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to restart clabernetes node %s/%s/%s: %w",
				namespace, req.Name, nodeName, err)
		}

		if err := r.waitDeploymentReplicas(ctx, namespace, deployment.Name, 1, req.Timeout); err != nil {
			return err
		}
	}

	return r.clearIgnoreWhenAllStarted(ctx, req.Name, namespace)
}

func (r *Runtime) Save(
	ctx context.Context,
	req clablabruntime.SaveRequest,
) (*clablabruntime.SaveResult, error) {
	targets, namespace, err := r.targetNodes(ctx, clablabruntime.NodeRequest{
		Name:      req.Name,
		Namespace: req.Namespace,
		Nodes:     req.Nodes,
	})
	if err != nil {
		return nil, err
	}

	result := &clablabruntime.SaveResult{}
	for _, nodeName := range targets {
		pod, err := r.launcherPod(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return nil, err
		}

		copyDir := ""
		command := []string{"containerlab", "save", "-t", "/clabernetes/topo.clab.yaml"}
		if req.Copy {
			copyDir = fmt.Sprintf("/tmp/clab-save-copy-%s-%s-%d",
				req.Name, nodeName, time.Now().UnixNano())
			_, _, _, _ = r.execInPod(ctx, pod, []string{"rm", "-rf", copyDir})
			command = append(command, "--copy", copyDir)
		}

		stdout, stderr, rc, err := r.execInPod(ctx, pod, command)
		if err != nil {
			return nil, err
		}

		if len(stdout) != 0 {
			log.Info("clabernetes save output", "node", nodeName, "stdout", strings.TrimSpace(string(stdout)))
		}
		if len(stderr) != 0 {
			log.Info("clabernetes save output", "node", nodeName, "stderr", strings.TrimSpace(string(stderr)))
		}
		if rc != 0 {
			return nil, fmt.Errorf("save failed for clabernetes node %s/%s/%s: rc=%d",
				namespace, req.Name, nodeName, rc)
		}

		if req.Copy {
			files, err := r.collectSavedFiles(ctx, pod, nodeName, copyDir)
			if cleanupDir := copyDir; cleanupDir != "" {
				_, _, _, _ = r.execInPod(ctx, pod, []string{"rm", "-rf", cleanupDir})
			}
			if err != nil {
				return nil, err
			}
			result.Files = append(result.Files, files...)
		}
	}

	return result, nil
}

func (r *Runtime) StreamEvents(
	ctx context.Context,
	req clablabruntime.EventStreamRequest,
) (<-chan clablabruntime.Event, <-chan error, error) {
	events := make(chan clablabruntime.Event, 128)
	errs := make(chan error, 2)

	namespace := r.namespaceFor(req.Namespace)
	if req.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	if req.IncludeInitialState {
		go r.emitInitialEvents(ctx, namespace, events, errs)
	}

	if req.IncludeInterfaceStats {
		go r.pollInterfaceStats(ctx, namespace, req.StatsInterval, events)
	}

	go r.watchTopologies(ctx, namespace, events, errs)
	go r.watchPods(ctx, namespace, events, errs)

	return events, errs, nil
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

func (r *Runtime) targetNodes(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]string, string, error) {
	if req.Name == "" {
		return nil, "", fmt.Errorf("topology name is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	deployments, err := r.deploymentsForTopology(ctx, req.Name, namespace)
	if err != nil {
		return nil, "", err
	}

	known := map[string]struct{}{}
	for idx := range deployments.Items {
		nodeName := deployments.Items[idx].Labels[labelTopologyNode]
		if nodeName != "" {
			known[nodeName] = struct{}{}
		}
	}

	if len(known) == 0 {
		state, err := r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
		if err != nil {
			return nil, "", err
		}
		for _, node := range state.Nodes {
			known[node.Name] = struct{}{}
		}
	}

	if len(known) == 0 {
		return nil, "", fmt.Errorf("topology %s/%s has no nodes", namespace, req.Name)
	}

	var targets []string
	if len(req.Nodes) == 0 {
		targets = make([]string, 0, len(known))
		for nodeName := range known {
			targets = append(targets, nodeName)
		}
		sort.Strings(targets)

		return targets, namespace, nil
	}

	for _, nodeName := range req.Nodes {
		if _, ok := known[nodeName]; !ok {
			return nil, "", fmt.Errorf("node %q was not found in topology %s/%s",
				nodeName, namespace, req.Name)
		}
		targets = append(targets, nodeName)
	}

	return targets, namespace, nil
}

func (r *Runtime) setNodesReplicas(
	ctx context.Context,
	req clablabruntime.NodeRequest,
	replicas int32,
) error {
	targets, namespace, err := r.targetNodes(ctx, req)
	if err != nil {
		return err
	}

	for _, nodeName := range targets {
		deployment, err := r.deploymentForNode(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return err
		}

		deployment.Spec.Replicas = &replicas
		_, err = r.kubeClient.AppsV1().Deployments(namespace).
			Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to set clabernetes node %s/%s/%s replicas to %d: %w",
				namespace, req.Name, nodeName, replicas, err)
		}

		if err := r.waitDeploymentReplicas(ctx, namespace, deployment.Name, replicas, req.Timeout); err != nil {
			return err
		}
	}

	if replicas > 0 {
		return r.clearIgnoreWhenAllStarted(ctx, req.Name, namespace)
	}

	return nil
}

func (r *Runtime) clearIgnoreWhenAllStarted(ctx context.Context, name, namespace string) error {
	deployments, err := r.deploymentsForTopology(ctx, name, namespace)
	if err != nil {
		return err
	}

	for idx := range deployments.Items {
		replicas := int32(1)
		if deployments.Items[idx].Spec.Replicas != nil {
			replicas = *deployments.Items[idx].Spec.Replicas
		}
		if replicas == 0 {
			return nil
		}
	}

	return r.setTopologyIgnoreReconcile(ctx, name, namespace, false)
}

func (r *Runtime) setTopologyIgnoreReconcile(
	ctx context.Context,
	name,
	namespace string,
	enabled bool,
) error {
	if name == "" {
		return fmt.Errorf("topology name is required")
	}

	namespace = r.namespaceFor(namespace)
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	obj, err := resource.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, name, err)
	}

	labelsMap := obj.GetLabels()
	if labelsMap == nil {
		labelsMap = map[string]string{}
	}

	if enabled {
		labelsMap[labelIgnoreReconcile] = "true"
	} else {
		delete(labelsMap, labelIgnoreReconcile)
	}

	obj.SetLabels(labelsMap)

	_, err = resource.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update clabernetes topology %s/%s labels: %w",
			namespace, name, err)
	}

	return nil
}

func (r *Runtime) waitDeploymentReplicas(
	ctx context.Context,
	namespace,
	name string,
	replicas int32,
	timeout time.Duration,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			deployment, err := r.kubeClient.AppsV1().Deployments(namespace).
				Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to get clabernetes deployment %s/%s: %w",
					namespace, name, err)
			}

			if replicas == 0 {
				return deployment.Status.Replicas == 0 &&
					deployment.Status.AvailableReplicas == 0, nil
			}

			return deployment.Status.ReadyReplicas >= replicas &&
				deployment.Status.AvailableReplicas >= replicas, nil
		})
}

func (r *Runtime) deploymentsForTopology(
	ctx context.Context,
	name,
	namespace string,
) (*appsv1.DeploymentList, error) {
	namespace = r.namespaceFor(namespace)
	list, err := r.kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: name,
		}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes deployments for topology %s/%s: %w",
			namespace, name, err)
	}

	return list, nil
}

func (r *Runtime) deploymentForNode(
	ctx context.Context,
	name,
	namespace,
	nodeName string,
) (*appsv1.Deployment, error) {
	namespace = r.namespaceFor(namespace)
	list, err := r.kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: name,
			labelTopologyNode:  nodeName,
		}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes deployment for node %s/%s/%s: %w",
			namespace, name, nodeName, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("clabernetes deployment for node %s/%s/%s was not found",
			namespace, name, nodeName)
	}

	return &list.Items[0], nil
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

	for idx := range list.Items {
		if list.Items[idx].Status.Phase == corev1.PodRunning {
			return &list.Items[idx], nil
		}
	}

	return &list.Items[0], nil
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

func (r *Runtime) collectSavedFiles(
	ctx context.Context,
	pod *corev1.Pod,
	nodeName,
	copyDir string,
) ([]clablabruntime.SavedFile, error) {
	if copyDir == "" {
		return nil, nil
	}

	nodeCopyDir := path.Join(copyDir, "clab-clabernetes-"+nodeName, nodeName)
	_, _, rc, err := r.execInPod(ctx, pod, []string{"test", "-d", nodeCopyDir})
	if err != nil {
		return nil, err
	}
	if rc != 0 {
		log.Debug("no clabernetes saved config copy directory found",
			"node", nodeName,
			"path", nodeCopyDir,
		)

		return nil, nil
	}

	stdout, stderr, rc, err := r.execInPod(ctx, pod,
		[]string{"tar", "cf", "-", "-C", nodeCopyDir, "."})
	if err != nil {
		return nil, err
	}
	if rc != 0 {
		return nil, fmt.Errorf("failed to archive saved config copy for node %s: rc=%d stderr=%s",
			nodeName, rc, strings.TrimSpace(string(stderr)))
	}

	files, err := savedFilesFromTar(nodeName, stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved config archive for node %s: %w",
			nodeName, err)
	}

	return files, nil
}

func savedFilesFromTar(nodeName string, data []byte) ([]clablabruntime.SavedFile, error) {
	reader := tar.NewReader(bytes.NewReader(data))
	var files []clablabruntime.SavedFile

	for {
		header, err := reader.Next()
		switch {
		case errors.Is(err, io.EOF):
			return files, nil
		case err != nil:
			return nil, err
		}

		name, ok := cleanTarPath(header.Name)
		if !ok || name == "." {
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			content, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}

			files = append(files, clablabruntime.SavedFile{
				NodeName: nodeName,
				Name:     name,
				Data:     content,
				Mode:     header.Mode,
			})
		case tar.TypeSymlink:
			files = append(files, clablabruntime.SavedFile{
				NodeName:   nodeName,
				Name:       name,
				Mode:       header.Mode,
				LinkTarget: header.Linkname,
			})
		}
	}
}

func cleanTarPath(name string) (string, bool) {
	name = strings.TrimPrefix(name, "./")
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == "" {
		return cleaned, true
	}
	if strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || cleaned == ".." {
		return "", false
	}

	return cleaned, true
}

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

func (r *Runtime) emitInitialEvents(
	ctx context.Context,
	namespace string,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	states, err := r.List(ctx, clablabruntime.ListRequest{
		Namespace:     namespace,
		AllNamespaces: namespace == metav1.NamespaceAll,
	})
	if err != nil {
		sendEventError(ctx, errSink, err)
		return
	}

	for _, state := range states {
		for _, node := range state.Nodes {
			action := node.State
			if node.Ready {
				action = "running"
			}
			if action == "" {
				action = state.State
			}
			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp: time.Now(),
				Type:      "container",
				Action:    action,
				ActorID:   fmt.Sprintf("%s/%s/%s", state.Namespace, state.Name, node.Name),
				ActorName: fmt.Sprintf("%s-%s", state.Name, node.Name),
				Attributes: map[string]string{
					"namespace": state.Namespace,
					"lab":       state.Name,
					"node":      node.Name,
					"state":     node.State,
				},
			})
		}
	}
}

func (r *Runtime) watchTopologies(
	ctx context.Context,
	namespace string,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	for {
		watcher, err := resource.Watch(ctx, metav1.ListOptions{})
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(ctx, errSink, fmt.Errorf("failed to watch clabernetes topologies: %w", err))
			return
		}

		if !r.forwardTopologyWatch(ctx, namespace, watcher, eventSink, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

func (r *Runtime) forwardTopologyWatch(
	ctx context.Context,
	namespace string,
	watcher watch.Interface,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) bool {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				log.Debug("clabernetes topology watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(ctx, errSink, fmt.Errorf("clabernetes topology watch returned an error"))
				return false
			}

			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			state := stateFromTopology(obj, namespace)
			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp: time.Now(),
				Type:      "topology",
				Action:    strings.ToLower(string(ev.Type)),
				ActorID:   fmt.Sprintf("%s/%s", state.Namespace, state.Name),
				ActorName: state.Name,
				Attributes: map[string]string{
					"namespace": state.Namespace,
					"lab":       state.Name,
					"state":     state.State,
					"ready":     fmt.Sprintf("%t", state.Ready),
				},
			})
		}
	}
}

func (r *Runtime) watchPods(
	ctx context.Context,
	namespace string,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	for {
		watcher, err := r.kubeClient.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: labelTopologyOwner,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(ctx, errSink, fmt.Errorf("failed to watch clabernetes pods: %w", err))
			return
		}

		if !r.forwardPodWatch(ctx, watcher, eventSink, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

func (r *Runtime) forwardPodWatch(
	ctx context.Context,
	watcher watch.Interface,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) bool {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				log.Debug("clabernetes pod watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(ctx, errSink, fmt.Errorf("clabernetes pod watch returned an error"))
				return false
			}

			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			labName := pod.Labels[labelTopologyOwner]
			nodeName := pod.Labels[labelTopologyNode]
			if labName == "" || nodeName == "" {
				continue
			}

			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp:   time.Now(),
				Type:        "container",
				Action:      strings.ToLower(string(ev.Type)),
				ActorID:     fmt.Sprintf("%s/%s/%s", pod.Namespace, labName, nodeName),
				ActorName:   fmt.Sprintf("%s-%s", labName, nodeName),
				ActorFullID: pod.Name,
				Attributes: map[string]string{
					"namespace": pod.Namespace,
					"lab":       labName,
					"node":      nodeName,
					"pod":       pod.Name,
					"phase":     string(pod.Status.Phase),
					"pod_ip":    pod.Status.PodIP,
				},
			})
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (r *Runtime) pollInterfaceStats(
	ctx context.Context,
	namespace string,
	interval time.Duration,
	eventSink chan<- clablabruntime.Event,
) {
	if interval <= 0 {
		interval = time.Second
	}

	samples := map[string]c9sIfaceStatsSample{}

	sample := func() {
		states, err := r.List(ctx, clablabruntime.ListRequest{
			Namespace:     namespace,
			AllNamespaces: namespace == metav1.NamespaceAll,
		})
		if err != nil {
			log.Debug("failed to list clabernetes topologies for interface stats", "error", err)
			return
		}

		now := time.Now()
		for _, state := range states {
			for _, node := range state.Nodes {
				if !node.Ready {
					continue
				}

				pod, err := r.launcherPod(ctx, state.Name, state.Namespace, node.Name)
				if err != nil {
					log.Debug("failed to resolve clabernetes launcher pod for interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"error", err,
					)
					continue
				}

				stdout, stderr, rc, err := r.execInPod(ctx, pod,
					[]string{"docker", "exec", node.Name, "cat", "/proc/net/dev"})
				if err != nil {
					log.Debug("failed to collect clabernetes interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"error", err,
					)
					continue
				}
				if rc != 0 {
					log.Debug("failed to collect clabernetes interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"rc", rc,
						"stderr", strings.TrimSpace(string(stderr)),
					)
					continue
				}

				stats, err := parseProcNetDev(stdout)
				if err != nil {
					log.Debug("failed to parse clabernetes interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"error", err,
					)
					continue
				}

				for _, stat := range stats {
					key := c9sIfaceStatsKey(state.Namespace, state.Name, node.Name, stat.Name)
					current := c9sIfaceStatsSample{
						Stats:     stat,
						Timestamp: now,
					}

					if previous, ok := samples[key]; ok {
						event := c9sIfaceStatsEvent(state, node, pod, stat, previous, current)
						r.sendEvent(ctx, eventSink, event)
					}

					samples[key] = current
				}
			}
		}
	}

	sample()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sample()
		}
	}
}

type c9sIfaceStats struct {
	Name      string
	RxBytes   uint64
	RxPackets uint64
	TxBytes   uint64
	TxPackets uint64
}

type c9sIfaceStatsSample struct {
	Stats     c9sIfaceStats
	Timestamp time.Time
}

func parseProcNetDev(data []byte) ([]c9sIfaceStats, error) {
	lines := strings.Split(string(data), "\n")
	stats := make([]c9sIfaceStats, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		ifName := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			return nil, fmt.Errorf("unexpected /proc/net/dev line for %q: %q", ifName, line)
		}

		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rx bytes for %q: %w", ifName, err)
		}
		rxPackets, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rx packets for %q: %w", ifName, err)
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx bytes for %q: %w", ifName, err)
		}
		txPackets, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx packets for %q: %w", ifName, err)
		}

		stats = append(stats, c9sIfaceStats{
			Name:      ifName,
			RxBytes:   rxBytes,
			RxPackets: rxPackets,
			TxBytes:   txBytes,
			TxPackets: txPackets,
		})
	}

	return stats, nil
}

func c9sIfaceStatsKey(namespace, lab, node, ifName string) string {
	return namespace + "/" + lab + "/" + node + "/" + ifName
}

func c9sIfaceStatsEvent(
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	pod *corev1.Pod,
	stat c9sIfaceStats,
	previous,
	current c9sIfaceStatsSample,
) clablabruntime.Event {
	interval := current.Timestamp.Sub(previous.Timestamp)
	if interval <= 0 {
		interval = time.Second
	}

	seconds := interval.Seconds()
	rxBytesDelta := counterDelta(stat.RxBytes, previous.Stats.RxBytes)
	txBytesDelta := counterDelta(stat.TxBytes, previous.Stats.TxBytes)
	rxPacketsDelta := counterDelta(stat.RxPackets, previous.Stats.RxPackets)
	txPacketsDelta := counterDelta(stat.TxPackets, previous.Stats.TxPackets)

	actorName := fmt.Sprintf("%s-%s", state.Name, node.Name)
	podName := ""
	if pod != nil {
		podName = pod.Name
	}

	return clablabruntime.Event{
		Timestamp:   current.Timestamp,
		Type:        "interface",
		Action:      "stats",
		ActorID:     c9sIfaceStatsKey(state.Namespace, state.Name, node.Name, stat.Name),
		ActorName:   actorName,
		ActorFullID: podName,
		Attributes: map[string]string{
			"namespace":        state.Namespace,
			"lab":              state.Name,
			"node":             node.Name,
			"name":             actorName,
			"pod":              podName,
			"ifname":           stat.Name,
			"origin":           "clabernetes",
			"rx_bytes":         strconv.FormatUint(stat.RxBytes, 10),
			"tx_bytes":         strconv.FormatUint(stat.TxBytes, 10),
			"rx_packets":       strconv.FormatUint(stat.RxPackets, 10),
			"tx_packets":       strconv.FormatUint(stat.TxPackets, 10),
			"rx_bps":           strconv.FormatFloat(float64(rxBytesDelta*8)/seconds, 'f', -1, 64),
			"tx_bps":           strconv.FormatFloat(float64(txBytesDelta*8)/seconds, 'f', -1, 64),
			"rx_pps":           strconv.FormatFloat(float64(rxPacketsDelta)/seconds, 'f', -1, 64),
			"tx_pps":           strconv.FormatFloat(float64(txPacketsDelta)/seconds, 'f', -1, 64),
			"interval_seconds": strconv.FormatFloat(seconds, 'f', -1, 64),
		},
	}
}

func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}

	return current - previous
}

func (r *Runtime) sendEvent(
	ctx context.Context,
	eventSink chan<- clablabruntime.Event,
	event clablabruntime.Event,
) {
	select {
	case eventSink <- event:
	case <-ctx.Done():
	}
}

func sendEventError(ctx context.Context, errSink chan<- error, err error) {
	select {
	case errSink <- err:
	case <-ctx.Done():
	}
}

func (r *Runtime) namespaceFor(namespace string) string {
	if namespace != "" {
		return namespace
	}
	if r.namespace != "" {
		return r.namespace
	}
	return defaultNamespace
}

func (r *Runtime) timeoutFor(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if r.timeout > 0 {
		return r.timeout
	}
	return 10 * time.Minute
}

func topologyObject(name, namespace, owner, definition string) *unstructured.Unstructured {
	topologyLabels := map[string]any{
		"containerlab.dev/runtime": clablabruntime.ClabernetesRuntimeName,
	}
	topologyAnnotations := map[string]any{}
	if owner != "" {
		topologyAnnotations[clabconstants.Owner] = owner
		if len(validation.IsValidLabelValue(owner)) == 0 {
			topologyLabels[clabconstants.Owner] = owner
		}
	}

	metadata := map[string]any{
		"name":      name,
		"namespace": namespace,
		"labels":    topologyLabels,
	}
	if len(topologyAnnotations) != 0 {
		metadata["annotations"] = topologyAnnotations
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "clabernetes.containerlab.dev/v1alpha1",
			"kind":       "Topology",
			"metadata":   metadata,
			"spec": map[string]any{
				"definition": map[string]any{
					"containerlab": definition,
				},
			},
		},
	}
}

func kubeClientConfig() (*rest.Config, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig := os.Getenv(envKubeconfig); kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextName := os.Getenv(envContext); contextName != "" {
		overrides.CurrentContext = contextName
	}
	if namespace := os.Getenv(envNamespace); namespace != "" {
		overrides.Context.Namespace = namespace
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		overrides,
	)

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		namespace = defaultNamespace
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load Kubernetes client config: %w", err)
	}

	return restConfig, namespace, nil
}

func stateFromTopology(obj *unstructured.Unstructured, namespace string) *clablabruntime.LabState {
	if obj.GetNamespace() != "" {
		namespace = obj.GetNamespace()
	}

	ready, _, _ := unstructured.NestedBool(obj.Object, "status", "topologyReady")
	state, _, _ := unstructured.NestedString(obj.Object, "status", "topologyState")
	owner := obj.GetLabels()[clabconstants.Owner]
	if owner == "" {
		owner = obj.GetAnnotations()[clabconstants.Owner]
	}
	nodeReadiness, _, _ := unstructured.NestedStringMap(
		obj.Object,
		"status",
		"nodeReadiness",
	)
	exposedPorts, _, _ := unstructured.NestedMap(obj.Object, "status", "exposedPorts")
	nodeSpecs := nodeSpecsFromTopology(obj)

	nodeNames := make([]string, 0, len(nodeSpecs)+len(nodeReadiness))
	seenNodes := map[string]struct{}{}
	for nodeName := range nodeSpecs {
		nodeNames = append(nodeNames, nodeName)
		seenNodes[nodeName] = struct{}{}
	}
	for nodeName := range nodeReadiness {
		if _, ok := seenNodes[nodeName]; ok {
			continue
		}
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	nodes := make([]clablabruntime.NodeState, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		nodeState := nodeReadiness[nodeName]
		spec := nodeSpecs[nodeName]
		nodes = append(nodes, clablabruntime.NodeState{
			Name:                nodeName,
			Kind:                spec.Kind,
			Image:               spec.Image,
			State:               nodeState,
			Ready:               nodeState == "ready",
			LoadBalancerAddress: loadBalancerAddress(exposedPorts, nodeName),
		})
	}

	return &clablabruntime.LabState{
		Name:         obj.GetName(),
		Namespace:    namespace,
		Owner:        owner,
		TopologyPath: fmt.Sprintf("k8s://%s/topologies/%s", namespace, obj.GetName()),
		State:        state,
		Ready:        ready,
		Nodes:        nodes,
	}
}

type nodeSpec struct {
	Kind  string `yaml:"kind"`
	Image string `yaml:"image"`
}

type containerlabDefinition struct {
	Topology struct {
		Nodes map[string]nodeSpec `yaml:"nodes"`
	} `yaml:"topology"`
}

func nodeSpecsFromTopology(obj *unstructured.Unstructured) map[string]nodeSpec {
	specs := map[string]nodeSpec{}

	statusConfigs, _, _ := unstructured.NestedStringMap(obj.Object, "status", "configs")
	for _, config := range statusConfigs {
		mergeNodeSpecs(specs, config)
	}

	if len(specs) != 0 {
		return specs
	}

	definition, _, _ := unstructured.NestedString(
		obj.Object,
		"spec",
		"definition",
		"containerlab",
	)
	mergeNodeSpecs(specs, definition)

	return specs
}

func mergeNodeSpecs(specs map[string]nodeSpec, definition string) {
	if definition == "" {
		return
	}

	var parsed containerlabDefinition
	if err := yaml.Unmarshal([]byte(definition), &parsed); err != nil {
		log.Debug("failed to parse clabernetes topology definition", "error", err)
		return
	}

	for nodeName, spec := range parsed.Topology.Nodes {
		specs[nodeName] = spec
	}
}

func loadBalancerAddress(exposedPorts map[string]any, nodeName string) string {
	raw, ok := exposedPorts[nodeName]
	if !ok {
		return ""
	}

	nodeExpose, ok := raw.(map[string]any)
	if !ok {
		return ""
	}

	addr, ok := nodeExpose["loadBalancerAddress"].(string)
	if !ok {
		return ""
	}

	if net.ParseIP(addr) == nil {
		return ""
	}

	return addr
}
