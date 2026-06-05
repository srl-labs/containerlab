package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"golang.org/x/term"
)

func (c *CLab) deployWithLabRuntime(
	ctx context.Context,
	options *DeployOptions,
) ([]clabruntime.GenericContainer, error) {
	if len(c.nodeFilter) != 0 {
		return nil, fmt.Errorf("node-filter is not supported for lab runtime %q",
			c.globalRuntimeName)
	}

	if len(c.renderedTopology) == 0 {
		return nil, fmt.Errorf("rendered topology is empty")
	}

	if options != nil && options.reconfigure {
		err := c.LabRuntime.Destroy(ctx, labruntime.DestroyRequest{
			Name:    c.Config.Name,
			Wait:    true,
			Timeout: c.timeout,
		})
		if err != nil {
			return nil, err
		}
	}

	state, err := c.LabRuntime.Deploy(ctx, labruntime.DeployRequest{
		Name:               c.Config.Name,
		TopologyDefinition: c.renderedTopology,
		Wait:               true,
		Timeout:            c.timeout,
	})
	if err != nil {
		return nil, err
	}

	return c.containersFromLabState(state), nil
}

func (c *CLab) destroyWithLabRuntime(ctx context.Context, opts *DestroyOptions) error {
	if opts.all {
		return c.destroyAllWithLabRuntime(ctx, opts)
	}

	if c.Config.Name == "" {
		return fmt.Errorf("topology name is required")
	}

	return c.LabRuntime.Destroy(ctx, labruntime.DestroyRequest{
		Name:    c.Config.Name,
		Wait:    true,
		Timeout: c.timeout,
	})
}

func (c *CLab) destroyAllWithLabRuntime(ctx context.Context, opts *DestroyOptions) error {
	states, err := c.LabRuntime.List(ctx, labruntime.ListRequest{AllNamespaces: true})
	if err != nil {
		return err
	}

	if len(states) == 0 {
		log.Info("no clabernetes topologies found")
		return nil
	}

	topos := make(map[string]string, len(states))
	for _, state := range states {
		topos[state.TopologyPath] = ""
	}

	if opts.terminalPrompt && term.IsTerminal(int(os.Stdin.Fd())) {
		if err := cliPromptToDestroyAll(topos); err != nil {
			return err
		}
	}

	var errs []error
	for _, state := range states {
		if err := c.LabRuntime.Destroy(ctx, labruntime.DestroyRequest{
			Name:      state.Name,
			Namespace: state.Namespace,
			Wait:      true,
			Timeout:   c.timeout,
		}); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("error(s) occurred during clabernetes topology deletion: %w",
			errors.Join(errs...))
	}

	return nil
}

func (c *CLab) unsupportedLabRuntimeOperation(operation string) error {
	return fmt.Errorf("%s is not supported for lab runtime %q yet",
		operation, c.globalRuntimeName)
}

func (c *CLab) ListLabRuntimeContainers(
	ctx context.Context,
	all bool,
) ([]clabruntime.GenericContainer, error) {
	if all {
		states, err := c.LabRuntime.List(ctx, labruntime.ListRequest{AllNamespaces: true})
		if err != nil {
			return nil, err
		}

		var containers []clabruntime.GenericContainer
		for _, state := range states {
			containers = append(containers, c.containersFromLabState(state)...)
		}

		return containers, nil
	}

	if c.Config.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}

	state, err := c.LabRuntime.Inspect(ctx, labruntime.InspectRequest{Name: c.Config.Name})
	if err != nil {
		return nil, err
	}

	return c.containersFromLabState(state), nil
}

func (c *CLab) containersFromLabState(state *labruntime.LabState) []clabruntime.GenericContainer {
	if state == nil {
		return nil
	}

	nodes := state.Nodes
	if len(nodes) == 0 && c.Config.Topology != nil {
		nodeNames := make([]string, 0, len(c.Config.Topology.Nodes))
		for nodeName := range c.Config.Topology.Nodes {
			nodeNames = append(nodeNames, nodeName)
		}
		sort.Strings(nodeNames)

		nodes = make([]labruntime.NodeState, 0, len(nodeNames))
		for _, nodeName := range nodeNames {
			nodes = append(nodes, labruntime.NodeState{
				Name:  nodeName,
				Kind:  c.Config.Topology.GetNodeKind(nodeName),
				Image: c.Config.Topology.GetNodeImage(nodeName),
				State: state.State,
				Ready: state.Ready,
			})
		}
	}

	containers := make([]clabruntime.GenericContainer, 0, len(nodes))
	for _, node := range nodes {
		containers = append(containers, c.containerFromLabNode(state, node))
	}

	return containers
}

func (c *CLab) containerFromLabNode(
	state *labruntime.LabState,
	node labruntime.NodeState,
) clabruntime.GenericContainer {
	nodeName := fmt.Sprintf("%s-%s", state.Name, node.Name)
	containerState := state.State
	containerStatus := node.State
	if node.Ready {
		containerState = "running"
		containerStatus = "healthy"
	}
	if containerState == "" {
		containerState = node.State
	}

	labels := map[string]string{
		clabconstants.Containerlab: state.Name,
		clabconstants.NodeName:     node.Name,
		clabconstants.TopoFile:     labRuntimeTopologyPath(c, state),
	}

	var image string
	if c.Config.Topology != nil {
		labels[clabconstants.NodeKind] = c.Config.Topology.GetNodeKind(node.Name)
		image = c.Config.Topology.GetNodeImage(node.Name)

		if group := c.Config.Topology.GetNodeGroup(node.Name); group != "" {
			labels[clabconstants.NodeGroup] = group
		}
	}
	if node.Kind != "" {
		labels[clabconstants.NodeKind] = node.Kind
	}
	if node.Image != "" {
		image = node.Image
	}

	if c.customOwner != "" {
		labels[clabconstants.Owner] = c.customOwner
	}

	return clabruntime.GenericContainer{
		Names:           []string{nodeName},
		ID:              fmt.Sprintf("%s/%s/%s", state.Namespace, state.Name, node.Name),
		ShortID:         "c9s",
		Image:           image,
		State:           containerState,
		Status:          containerStatus,
		Labels:          labels,
		NetworkName:     state.Namespace,
		NetworkSettings: managementAddress(node.LoadBalancerAddress),
	}
}

func managementAddress(addr string) clabruntime.GenericMgmtIPs {
	ip := net.ParseIP(addr)
	if ip == nil {
		return clabruntime.GenericMgmtIPs{}
	}

	if ip.To4() != nil {
		return clabruntime.GenericMgmtIPs{
			IPv4addr: addr,
			IPv4pLen: 32,
		}
	}

	return clabruntime.GenericMgmtIPs{
		IPv6addr: addr,
		IPv6pLen: 128,
	}
}

func labRuntimeTopologyPath(c *CLab, state *labruntime.LabState) string {
	if c.TopoPaths.TopologyFileIsSet() {
		return c.TopoPaths.TopologyFilenameAbsPath()
	}

	if state.TopologyPath != "" {
		return state.TopologyPath
	}

	return fmt.Sprintf("k8s://%s/topologies/%s", state.Namespace, state.Name)
}
