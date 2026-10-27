package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	kubeexec "k8s.io/client-go/util/exec"
	streamhttp "k8s.io/streaming/pkg/httpstream"
)

// podMetricsGVR is the resource metrics API metrics-server serves.
var podMetricsGVR = schema.GroupVersionResource{
	Group:    "metrics.k8s.io",
	Version:  "v1beta1",
	Resource: "pods",
}

// defaultAttachCommand runs when an attach request names no command.
var defaultAttachCommand = []string{"sh"} //nolint:gochecknoglobals

// portForwardRequestID numbers the port-forward streams of this process.
var portForwardRequestID atomic.Int64 //nolint:gochecknoglobals

// LocateNode reports the pod, primary device container, Node UID and exposed ports of the
// requested nodes, or of every node of the lab when the request names none. Node names are
// accepted as the topology or as the cluster spells them.
func (r *Runtime) LocateNode(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]clablabruntime.NodeLocation, error) {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return nil, err
	}

	state, err := r.Inspect(ctx, clablabruntime.InspectRequest{
		Name:      req.Name,
		Namespace: req.Namespace,
	})
	if err != nil {
		return nil, err
	}

	nodes := make(map[string]clablabruntime.NodeState, len(state.Nodes))
	for _, node := range state.Nodes {
		nodes[node.Name] = node
	}

	if len(req.Nodes) == 0 {
		locations := make([]clablabruntime.NodeLocation, 0, len(state.Nodes))
		for _, node := range state.Nodes {
			locations = append(locations, nodeLocation(state, node))
		}

		return locations, nil
	}

	locations := make([]clablabruntime.NodeLocation, 0, len(req.Nodes))
	for _, nodeName := range req.Nodes {
		resolved, ok := resolveKnownNodeName(nodes, nodeName)
		if !ok {
			return nil, fmt.Errorf("node %q was not found in c9s lab %s/%s",
				nodeName, state.Namespace, state.Name)
		}
		locations = append(locations, nodeLocation(state, nodes[resolved]))
	}

	return locations, nil
}

func nodeLocation(
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
) clablabruntime.NodeLocation {
	return clablabruntime.NodeLocation{
		NodeName:            node.Name,
		Namespace:           state.Namespace,
		Pod:                 node.Pod,
		Container:           node.Container,
		NodeUID:             node.NodeUID,
		ExposedPorts:        node.ExposedPorts,
		LoadBalancerAddress: node.LoadBalancerAddress,
	}
}

// runningNode locates one node and requires it to have a pod and a device container.
func (r *Runtime) runningNode(
	ctx context.Context,
	lab,
	namespace,
	nodeName string,
) (clablabruntime.NodeLocation, error) {
	if lab == "" {
		return clablabruntime.NodeLocation{}, fmt.Errorf("topology name is required")
	}
	if nodeName == "" {
		return clablabruntime.NodeLocation{}, fmt.Errorf("node name is required")
	}

	locations, err := r.LocateNode(ctx, clablabruntime.NodeRequest{
		Name:      lab,
		Namespace: namespace,
		Nodes:     []string{nodeName},
	})
	if err != nil {
		return clablabruntime.NodeLocation{}, err
	}

	location := locations[0]
	if location.Pod == "" {
		return clablabruntime.NodeLocation{}, fmt.Errorf(
			"c9s node %s/%s/%s has no pod; is it running?",
			location.Namespace, lab, location.NodeName)
	}

	return location, nil
}

// Attach runs an interactive command in a node's device container, or in the container the
// request names, with stdin, an optional TTY and terminal resizing.
func (r *Runtime) Attach(
	ctx context.Context,
	req clablabruntime.AttachRequest,
	streams clablabruntime.AttachStreams,
) error {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return err
	}

	location, err := r.runningNode(ctx, req.Name, req.Namespace, req.NodeName)
	if err != nil {
		return err
	}

	container := req.Container
	if container == "" {
		container = location.Container
	}
	command := req.Command
	if len(command) == 0 {
		command = defaultAttachCommand
	}

	stderr := streams.Stderr
	if req.TTY {
		// a terminal merges stderr into stdout
		stderr = nil
	}

	execURL := r.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(location.Pod).
		Namespace(location.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     streams.Stdin != nil,
			Stdout:    streams.Stdout != nil,
			Stderr:    stderr != nil,
			TTY:       req.TTY,
		}, scheme.ParameterCodec).URL()

	executor, err := r.newExecutor(execURL)
	if err != nil {
		return err
	}

	options := remotecommand.StreamOptions{
		Stdin:  streams.Stdin,
		Stdout: streams.Stdout,
		Stderr: stderr,
		Tty:    req.TTY,
	}
	if req.TTY && streams.Resize != nil {
		options.TerminalSizeQueue = newTerminalSizeQueue(ctx, streams.Resize)
	}

	err = executor.StreamWithContext(ctx, options)

	var exitErr kubeexec.ExitError
	if errors.As(err, &exitErr) {
		return &clablabruntime.CommandExitError{Code: exitErr.ExitStatus()}
	}
	if err != nil {
		return fmt.Errorf("failed to attach to c9s node %s/%s/%s: %w",
			location.Namespace, req.Name, location.NodeName, err)
	}

	return nil
}

// newExecutor streams a pod exec over websockets, falling back to SPDY when the API server
// does not upgrade the websocket, like kubectl. The websocket executor reports upgrade
// failures with the k8s.io/streaming error types. Both transports are built from the runtime's
// rest config, so they carry its impersonation headers.
func (r *Runtime) newExecutor(execURL *url.URL) (remotecommand.Executor, error) {
	websocketExecutor, err := remotecommand.NewWebSocketExecutor(
		r.restConfig, http.MethodGet, execURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes websocket exec executor: %w", err)
	}

	spdyExecutor, err := remotecommand.NewSPDYExecutor(r.restConfig, http.MethodPost, execURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes exec executor: %w", err)
	}

	return remotecommand.NewFallbackExecutor(websocketExecutor, spdyExecutor,
		func(err error) bool {
			return streamhttp.IsUpgradeFailure(err) || streamhttp.IsHTTPSProxyError(err)
		})
}

// terminalSizeQueue adapts a resize channel to the remotecommand size queue.
type terminalSizeQueue struct {
	ctx    context.Context
	resize <-chan clablabruntime.TerminalSize
}

func newTerminalSizeQueue(
	ctx context.Context,
	resize <-chan clablabruntime.TerminalSize,
) *terminalSizeQueue {
	return &terminalSizeQueue{ctx: ctx, resize: resize}
}

// Next blocks until the next size change; nil ends resizing.
func (q *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case <-q.ctx.Done():
		return nil
	case size, ok := <-q.resize:
		if !ok {
			return nil
		}

		return &remotecommand.TerminalSize{Width: size.Width, Height: size.Height}
	}
}

// StreamLogs writes the log of a node's device container, or of the container the request
// names, to w.
func (r *Runtime) StreamLogs(
	ctx context.Context,
	req clablabruntime.LogRequest,
	w io.Writer,
) error {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return err
	}

	location, err := r.runningNode(ctx, req.Name, req.Namespace, req.NodeName)
	if err != nil {
		return err
	}

	container := req.Container
	if container == "" {
		container = location.Container
	}

	stream, err := r.kubeClient.CoreV1().Pods(location.Namespace).
		GetLogs(location.Pod, &corev1.PodLogOptions{
			Container:  container,
			Follow:     req.Follow,
			Timestamps: req.Timestamps,
			TailLines:  req.Tail,
		}).Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream the log of c9s node %s/%s/%s: %w",
			location.Namespace, req.Name, location.NodeName, err)
	}
	defer stream.Close()

	if _, err := io.Copy(w, stream); err != nil && ctx.Err() == nil {
		return fmt.Errorf("failed to stream the log of c9s node %s/%s/%s: %w",
			location.Namespace, req.Name, location.NodeName, err)
	}

	return nil
}

// DialPort opens a connection to a TCP port of a node's pod through the Kubernetes port-forward
// subresource.
func (r *Runtime) DialPort(
	ctx context.Context,
	req clablabruntime.NodeRequest,
	port int,
) (net.Conn, error) {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return nil, err
	}

	if len(req.Nodes) != 1 {
		return nil, fmt.Errorf("port-forward needs exactly one node, got %d", len(req.Nodes))
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port %d", port)
	}

	location, err := r.runningNode(ctx, req.Name, req.Namespace, req.Nodes[0])
	if err != nil {
		return nil, err
	}

	conn, err := r.dialPodPort(location.Namespace, location.Pod, port)
	if err != nil {
		return nil, fmt.Errorf("failed to port-forward to port %d of c9s node %s/%s/%s: %w",
			port, location.Namespace, req.Name, location.NodeName, err)
	}

	return conn, nil
}

// dialPodPort opens a forwarded connection to a pod port over SPDY. The transport is built
// from the runtime's rest config, so it carries its impersonation headers.
func (r *Runtime) dialPodPort(namespace, pod string, port int) (net.Conn, error) {
	forwardURL := r.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(r.restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes port-forward transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost,
		forwardURL)

	streamConn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return nil, err
	}

	conn, err := newPortForwardConn(streamConn, port)
	if err != nil {
		_ = streamConn.Close()

		return nil, err
	}

	return conn, nil
}

// newPortForwardConn opens the error and data streams of one forwarded connection and returns
// one end of an in-memory pipe whose other end is copied to and from the data stream. The pipe
// gives the caller a complete net.Conn, deadlines included.
func newPortForwardConn(streamConn httpstream.Connection, port int) (net.Conn, error) {
	requestID := strconv.FormatInt(portForwardRequestID.Add(1), 10)

	headers := http.Header{}
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	headers.Set(corev1.PortHeader, strconv.Itoa(port))
	headers.Set(corev1.PortForwardRequestIDHeader, requestID)

	errorStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to create the error stream: %w", err)
	}
	// the client sends nothing on the error stream
	_ = errorStream.Close()

	headers.Set(corev1.StreamType, corev1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to create the data stream: %w", err)
	}

	local, remote := net.Pipe()

	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			_ = remote.Close()
			_ = dataStream.Reset()
			_ = streamConn.Close()
		})
	}

	go func() {
		// the API server reports a failure to reach the port on the error stream
		message, _ := io.ReadAll(errorStream)
		if len(message) != 0 {
			_ = remote.Close()
		}
	}()

	go func() {
		defer closeAll()
		_, _ = io.Copy(remote, dataStream)
	}()

	go func() {
		defer closeAll()
		_, _ = io.Copy(dataStream, remote)
	}()

	go func() {
		<-streamConn.CloseChan()
		closeAll()
	}()

	return local, nil
}

// SampleStats reports the CPU and memory usage of every running node of a lab from the
// resource metrics API, summing the node's device containers.
func (r *Runtime) SampleStats(
	ctx context.Context,
	req clablabruntime.InspectRequest,
) ([]clablabruntime.NodeStats, error) {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return nil, err
	}

	state, err := r.Inspect(ctx, req)
	if err != nil {
		return nil, err
	}

	metricsList, err := r.client.Resource(podMetricsGVR).Namespace(state.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err) ||
			apierrors.IsMethodNotSupported(err) {
			return nil, fmt.Errorf("%w: %v", clablabruntime.ErrMetricsUnavailable, err)
		}

		return nil, fmt.Errorf("failed to read pod metrics in namespace %s: %w",
			state.Namespace, err)
	}

	metricsByPod := make(map[string]*unstructured.Unstructured, len(metricsList.Items))
	for idx := range metricsList.Items {
		metricsByPod[metricsList.Items[idx].GetName()] = &metricsList.Items[idx]
	}

	deviceContainers, err := r.nodeDeviceContainers(ctx, state)
	if err != nil {
		return nil, err
	}

	stats := make([]clablabruntime.NodeStats, 0, len(state.Nodes))
	for _, node := range state.Nodes {
		podMetrics := metricsByPod[node.Pod]
		if node.Pod == "" || podMetrics == nil {
			continue
		}

		containers := deviceContainers[node.Name]
		if len(containers) == 0 && node.Container != "" {
			containers = map[string]bool{node.Container: true}
		}

		sample, ok := podMetricsSample(podMetrics, containers)
		if !ok {
			continue
		}

		sample.NodeName = node.Name
		sample.Namespace = state.Namespace
		sample.Pod = node.Pod
		stats = append(stats, sample)
	}

	return stats, nil
}

// nodeDeviceContainers reads the device containers of every node from its Node status.
func (r *Runtime) nodeDeviceContainers(
	ctx context.Context,
	state *clablabruntime.LabState,
) (map[string]map[string]bool, error) {
	nodes, err := r.nodesForTopology(ctx, state.Name, state.Namespace)
	if err != nil {
		return nil, err
	}

	result := make(map[string]map[string]bool, len(nodes.Items))
	for idx := range nodes.Items {
		directContainers, _, _ := unstructured.NestedSlice(
			nodes.Items[idx].Object, "status", "directContainers")

		names := map[string]bool{}
		for _, raw := range directContainers {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if name, _ := entry["name"].(string); name != "" {
				names[name] = true
			}
		}
		result[nodes.Items[idx].GetName()] = names
	}

	return result, nil
}

// podMetricsSample sums the usage of the given containers of a PodMetrics object.
func podMetricsSample(
	podMetrics *unstructured.Unstructured,
	containers map[string]bool,
) (clablabruntime.NodeStats, bool) {
	rawContainers, _, _ := unstructured.NestedSlice(podMetrics.Object, "containers")

	var (
		cpu    resource.Quantity
		memory resource.Quantity
		found  bool
	)
	for _, raw := range rawContainers {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if !containers[name] {
			continue
		}

		usage, _ := entry["usage"].(map[string]any)
		if value, ok := usage["cpu"].(string); ok {
			if quantity, err := resource.ParseQuantity(value); err == nil {
				cpu.Add(quantity)
			}
		}
		if value, ok := usage["memory"].(string); ok {
			if quantity, err := resource.ParseQuantity(value); err == nil {
				memory.Add(quantity)
			}
		}
		found = true
	}
	if !found {
		return clablabruntime.NodeStats{}, false
	}

	sample := clablabruntime.NodeStats{
		CPUMilliCores: cpu.MilliValue(),
		MemoryBytes:   memory.Value(),
	}
	if value, _, _ := unstructured.NestedString(podMetrics.Object, "timestamp"); value != "" {
		sample.Timestamp, _ = time.Parse(time.RFC3339, value)
	}
	if value, _, _ := unstructured.NestedString(podMetrics.Object, "window"); value != "" {
		sample.Window, _ = time.ParseDuration(value)
	}

	return sample, true
}
