package clabernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	kubeexec "k8s.io/client-go/util/exec"
)

// streamPodExec runs a command in a pod container without a terminal and streams its stdout
// and stderr. A non-nil stdin is attached: when it ends, the process's stdin is closed (the
// websocket v5 close signal, or a half-closed SPDY stream). A nonzero exit returns a
// *labruntime.CommandExitError.
func (r *Runtime) streamPodExec(
	ctx context.Context,
	namespace, pod, container string,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	execURL := r.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    stderr != nil,
		}, scheme.ParameterCodec).URL()

	executor, err := r.newExecutor(execURL)
	if err != nil {
		return err
	}

	// client-go reports streams cut by a cancelled context through the context's logger; a
	// capture ends that way on purpose, so it goes to the debug log
	err = executor.StreamWithContext(withKubernetesClientDebugLogs(ctx),
		remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
		})

	var exitErr kubeexec.ExitError
	if errors.As(err, &exitErr) {
		return &clablabruntime.CommandExitError{Code: exitErr.ExitStatus()}
	}
	if err != nil {
		return fmt.Errorf("failed to execute command in pod %s/%s container %s: %w",
			namespace, pod, container, err)
	}

	return nil
}

// InspectInterfaces lists the capturable link interfaces of the requested nodes, or of every
// running node, in the shape of `inspect interfaces`: per node (named <lab>-<node> like in
// inspect), the interface and its alias as clabwire reports them, and the MAC address, index,
// MTU and operational state read from sysfs in the device container. A node whose device
// container has no shell reports its interfaces without these attributes.
func (r *Runtime) InspectInterfaces(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]*clabtypes.ContainerInterfaces, error) {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return nil, err
	}

	locations, err := r.LocateNode(ctx, clablabruntime.NodeRequest{
		Name: req.Name, Namespace: req.Namespace, Nodes: req.Nodes, Identity: req.Identity,
	})
	if err != nil {
		return nil, err
	}

	running := make([]clablabruntime.NodeLocation, 0, len(locations))
	for _, location := range locations {
		if location.Pod == "" {
			log.Warn("Skipping interface detection of a node that does not run",
				"node", location.NodeName)
			continue
		}
		running = append(running, location)
	}

	listed, err := r.listCaptureInterfaces(ctx, req.Name, running)
	if err != nil {
		return nil, err
	}
	states := r.sampleInterfaceStates(ctx, running)

	result := make([]*clabtypes.ContainerInterfaces, 0, len(running))
	for _, location := range running {
		container := &clabtypes.ContainerInterfaces{
			ContainerName: req.Name + "-" + location.NodeName,
			Interfaces:    []*clabtypes.ContainerInterfaceDetails{},
		}
		for _, entry := range listed[location.NodeName] {
			details := &clabtypes.ContainerInterfaceDetails{
				InterfaceName:  entry.Interface,
				InterfaceAlias: entry.Alias,
				// clabwire realizes every lab link interface as a veth pair
				InterfaceType: "veth",
			}
			if state, ok := states[location.NodeName][entry.Interface]; ok {
				details.InterfaceMAC = state.MAC
				details.InterfaceIndex = state.Index
				details.InterfaceMTU = state.MTU
				details.InterfaceState = state.OperState
			}
			container.Interfaces = append(container.Interfaces, details)
		}
		sort.Slice(container.Interfaces, func(i, j int) bool {
			return container.Interfaces[i].InterfaceName < container.Interfaces[j].InterfaceName
		})
		result = append(result, container)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ContainerName < result[j].ContainerName
	})

	return result, nil
}

// sampleInterfaceStates reads the sysfs interface attributes of every node's device container,
// one exec per node, with the script the interface events use.
func (r *Runtime) sampleInterfaceStates(
	ctx context.Context,
	locations []clablabruntime.NodeLocation,
) map[string]map[string]c9sIfaceState {
	var mu sync.Mutex
	result := map[string]map[string]c9sIfaceState{}

	var wg sync.WaitGroup
	workers := make(chan struct{}, interfaceSampleWorkers)
	exec := r.execInPodContainer()
	for _, location := range locations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workers <- struct{}{}
			defer func() { <-workers }()

			var stdout, stderr bytes.Buffer
			err := exec(ctx, location.Namespace, location.Pod, location.Container,
				[]string{"sh", "-c", interfaceSampleScript}, nil, &stdout, &stderr)
			if err != nil {
				log.Debug("Cannot read the interface state of a c9s node",
					"node", location.NodeName, "error", err)
				return
			}
			_, ifaces, _, err := parseInterfaceSample(stdout.Bytes())
			if err != nil {
				log.Debug("Cannot parse the interface state of a c9s node",
					"node", location.NodeName, "error", err)
				return
			}

			byName := make(map[string]c9sIfaceState, len(ifaces))
			for _, iface := range ifaces {
				byName[iface.Name] = iface
			}
			mu.Lock()
			result[location.NodeName] = byName
			mu.Unlock()
		}()
	}
	wg.Wait()

	return result
}
