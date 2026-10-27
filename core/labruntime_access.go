package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// The helpers in this file give library users per-node access to the lab selected by the CLab
// (its topology file or WithTopologyName). Node names are accepted as the topology file spells
// them or as a lab runtime sanitized them. Lab runtime calls act as the identity set with
// WithLabRuntimeIdentity.

// LocateLabRuntimeNodes reports the workload (namespace, pod, primary device container, Node
// UID, exposed ports) of the given nodes, or of every node of the lab when none are given.
func (c *CLab) LocateLabRuntimeNodes(
	ctx context.Context,
	nodeNames ...string,
) ([]clablabruntime.NodeLocation, error) {
	locator, err := labRuntimeCapability[clablabruntime.NodeLocator](c, "locating nodes")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	return locator.LocateNode(ctx, clablabruntime.NodeRequest{
		Name:     c.Config.Name,
		Nodes:    nodeNames,
		Identity: c.labRuntimeIdentity,
	})
}

// AttachLabRuntimeNode runs an interactive command in a node's device container (or in
// req.Container) with stdin, an optional TTY and terminal resizing. The lab name and identity
// default to the CLab's. A command exiting nonzero returns a *labruntime.CommandExitError.
func (c *CLab) AttachLabRuntimeNode(
	ctx context.Context,
	req clablabruntime.AttachRequest,
	streams clablabruntime.AttachStreams,
) error {
	attacher, err := labRuntimeCapability[clablabruntime.NodeAttacher](c, "attaching to nodes")
	if err != nil {
		return err
	}

	if req.Name == "" {
		if err := c.requireLabName(); err != nil {
			return err
		}
		req.Name = c.Config.Name
	}
	if req.Identity.IsZero() {
		req.Identity = c.labRuntimeIdentity
	}

	return attacher.Attach(ctx, req, streams)
}

// DialLabRuntimeNodePort opens a connection to a TCP port of a node (SSH, web UIs) through the
// lab runtime, such as a Kubernetes port-forward.
func (c *CLab) DialLabRuntimeNodePort(
	ctx context.Context,
	nodeName string,
	port int,
) (net.Conn, error) {
	forwarder, err := labRuntimeCapability[clablabruntime.NodePortForwarder](
		c, "port-forwarding to nodes")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	return forwarder.DialPort(ctx, clablabruntime.NodeRequest{
		Name:     c.Config.Name,
		Nodes:    []string{nodeName},
		Identity: c.labRuntimeIdentity,
	}, port)
}

// SampleLabRuntimeNodeStats reports the CPU and memory usage of the lab's running nodes. It
// returns an error wrapping labruntime.ErrMetricsUnavailable when the cluster serves no resource
// metrics.
func (c *CLab) SampleLabRuntimeNodeStats(ctx context.Context) ([]clablabruntime.NodeStats, error) {
	sampler, err := labRuntimeCapability[clablabruntime.NodeStatsSampler](
		c, "sampling node stats")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	return sampler.SampleStats(ctx, clablabruntime.InspectRequest{
		Name:     c.Config.Name,
		Identity: c.labRuntimeIdentity,
	})
}

// NodeLogOptions selects the node log NodeLogs writes.
type NodeLogOptions struct {
	// NodeName is the node whose log is written.
	NodeName string
	// Container overrides a lab runtime node's primary device container. Local runtimes do
	// not support it.
	Container string
	// Tail limits the output to the last lines of the log; nil writes the whole log.
	Tail *int64
	// Follow keeps writing new log lines until the context ends.
	Follow bool
	// Timestamps prefixes every line with its timestamp.
	Timestamps bool
}

// NodeLogs writes the log of a lab node with every runtime: the container log with Docker and
// Podman, the device container log with a lab runtime. Lab runtimes write one combined stream to
// stdout.
func (c *CLab) NodeLogs(
	ctx context.Context,
	opts NodeLogOptions,
	stdout, stderr io.Writer,
) error {
	if opts.NodeName == "" {
		return fmt.Errorf("node name is required")
	}
	if err := c.requireLabName(); err != nil {
		return err
	}

	if c.LabRuntime != nil {
		streamer, err := labRuntimeCapability[clablabruntime.NodeLogStreamer](
			c, "streaming node logs")
		if err != nil {
			return err
		}

		return streamer.StreamLogs(ctx, clablabruntime.LogRequest{
			Name:       c.Config.Name,
			NodeName:   opts.NodeName,
			Container:  opts.Container,
			Tail:       opts.Tail,
			Follow:     opts.Follow,
			Timestamps: opts.Timestamps,
			Identity:   c.labRuntimeIdentity,
		}, stdout)
	}

	if opts.Container != "" {
		return fmt.Errorf("selecting a container is only supported with lab runtimes")
	}

	runtime, containerName, err := c.nodeContainer(ctx, opts.NodeName)
	if err != nil {
		return err
	}

	writer, ok := runtime.(clabruntime.ContainerLogWriter)
	if !ok {
		return fmt.Errorf("container runtime %q does not support node logs", runtime.GetName())
	}

	return writer.WriteContainerLogs(ctx, containerName, clabruntime.ContainerLogOptions{
		Tail:       opts.Tail,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}, stdout, stderr)
}

// nodeContainer finds the container of a lab node in the local container runtimes by the lab
// and node name labels.
func (c *CLab) nodeContainer(
	ctx context.Context,
	nodeName string,
) (clabruntime.ContainerRuntime, string, error) {
	filters := []*clabtypes.GenericFilter{
		{
			FilterType: "label",
			Field:      clabconstants.Containerlab,
			Operator:   "=",
			Match:      c.Config.Name,
		},
		{
			FilterType: "label",
			Field:      clabconstants.NodeName,
			Operator:   "=",
			Match:      nodeName,
		},
	}

	runtimeNames := make([]string, 0, len(c.Runtimes))
	for name := range c.Runtimes {
		runtimeNames = append(runtimeNames, name)
	}
	sort.Strings(runtimeNames)

	for _, name := range runtimeNames {
		runtime := c.Runtimes[name]

		containers, err := runtime.ListContainers(ctx, filters)
		if err != nil {
			return nil, "", fmt.Errorf("could not list containers: %w", err)
		}
		if len(containers) != 0 && len(containers[0].Names) != 0 {
			return runtime, containers[0].Names[0], nil
		}
	}

	return nil, "", fmt.Errorf("%w: node %q of lab %q has no container",
		ErrNodeNotFound, nodeName, c.Config.Name)
}

func (c *CLab) requireLabName() error {
	if c.Config == nil || c.Config.Name == "" {
		return fmt.Errorf("provide a lab name (--name) or a topology file (--topo)")
	}

	return nil
}

// labRuntimeCapability returns the CLab's lab runtime as an optional lab runtime interface.
func labRuntimeCapability[T any](c *CLab, operation string) (T, error) {
	var zero T

	if c.LabRuntime == nil {
		return zero, fmt.Errorf("%s needs a lab runtime such as %q",
			operation, clablabruntime.ClabernetesRuntimeName)
	}

	capability, ok := c.LabRuntime.(T)
	if !ok {
		return zero, fmt.Errorf("lab runtime %q does not support %s",
			c.globalRuntimeName, operation)
	}

	return capability, nil
}
