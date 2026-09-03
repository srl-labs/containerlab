package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
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
		err := c.LabRuntime.Destroy(ctx, clablabruntime.DestroyRequest{
			Name:    c.Config.Name,
			Wait:    true,
			Timeout: c.timeout,
		})
		if err != nil {
			return nil, err
		}
	}

	req, err := c.labRuntimeDeployRequest()
	if err != nil {
		return nil, err
	}
	req.Wait = true
	req.NoTopologyCR = options != nil && options.noTopologyCR
	if options != nil {
		req.ImagePullSecret = options.imagePullSecret
		req.ExposeType = options.exposeType
		req.NoPersistence = options.noPersistence
	}

	state, err := c.LabRuntime.Deploy(ctx, req)
	if err != nil {
		return nil, err
	}

	return c.containersFromLabState(state), nil
}

func (c *CLab) labRuntimeDeployRequest() (clablabruntime.DeployRequest, error) {
	if c.Config.Name == "" {
		return clablabruntime.DeployRequest{}, fmt.Errorf("topology name is required")
	}
	if len(c.renderedTopology) == 0 {
		return clablabruntime.DeployRequest{}, fmt.Errorf("rendered topology is empty")
	}

	topologyFile := ""
	topologyLabDir := ""
	if c.TopoPaths != nil {
		topologyFile = c.TopoPaths.TopologyFilenameAbsPath()
		topologyLabDir = c.TopoPaths.TopologyLabDir()
	}

	return clablabruntime.DeployRequest{
		Name:               c.Config.Name,
		Owner:              c.labOwner(),
		TopologyFile:       topologyFile,
		TopologyLabDir:     topologyLabDir,
		TopologyDefinition: c.renderedTopology,
		Timeout:            c.timeout,
	}, nil
}

// ValidateLabRuntimeTopology executes the selected runtime's compiler without mutation.
func (c *CLab) ValidateLabRuntimeTopology(ctx context.Context) error {
	if c.LabRuntime == nil {
		return fmt.Errorf("no lab runtime selected")
	}

	validator, ok := c.LabRuntime.(clablabruntime.TopologyValidator)
	if !ok {
		return fmt.Errorf(
			"lab runtime %q does not support topology validation",
			c.globalRuntimeName,
		)
	}

	req, err := c.labRuntimeDeployRequest()
	if err != nil {
		return err
	}

	return validator.Validate(ctx, req)
}

func (c *CLab) destroyWithLabRuntime(ctx context.Context, opts *DestroyOptions) error {
	if len(opts.nodeFilter) != 0 {
		return fmt.Errorf(
			"node-filter is not supported for lab runtime %q; no resources were deleted",
			c.globalRuntimeName,
		)
	}

	if opts.all {
		return c.destroyAllWithLabRuntime(ctx, opts)
	}

	if c.Config.Name == "" {
		return fmt.Errorf("topology name is required")
	}

	err := c.LabRuntime.Destroy(ctx, clablabruntime.DestroyRequest{
		Name:    c.Config.Name,
		Wait:    true,
		Timeout: c.timeout,
	})
	if err != nil {
		return err
	}

	if opts.cleanup {
		return c.cleanupLabRuntimeLabDir()
	}

	return nil
}

// cleanupLabRuntimeLabDir removes the local lab directory for --cleanup parity with local
// runtimes. Controller-driven runtimes keep node state remotely, so the directory usually only
// exists when a lab was previously deployed locally or configs were copied out.
func (c *CLab) cleanupLabRuntimeLabDir() error {
	if c.TopoPaths == nil || !c.TopoPaths.TopologyFileIsSet() {
		return nil
	}

	labDir := c.TopoPaths.TopologyLabDir()
	if labDir == "" {
		return nil
	}
	if _, err := os.Stat(labDir); os.IsNotExist(err) {
		return nil
	}

	log.Info("Removing lab directory", "path", labDir)

	if err := os.RemoveAll(labDir); err != nil {
		return fmt.Errorf("failed to remove lab directory %q: %w", labDir, err)
	}

	return nil
}

func (c *CLab) destroyAllWithLabRuntime(ctx context.Context, opts *DestroyOptions) error {
	states, err := c.LabRuntime.List(ctx, clablabruntime.ListRequest{AllNamespaces: true})
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
		if err := c.LabRuntime.Destroy(ctx, clablabruntime.DestroyRequest{
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

func (c *CLab) ListLabRuntimeContainers(
	ctx context.Context,
	all bool,
	options ...ListOption,
) ([]clabruntime.GenericContainer, error) {
	opts := NewListOptions()
	for _, opt := range options {
		opt(opts)
	}

	if all || c.Config.Name == "" {
		states, err := c.LabRuntime.List(ctx, clablabruntime.ListRequest{AllNamespaces: true})
		if err != nil {
			return nil, err
		}

		var containers []clabruntime.GenericContainer
		for _, state := range states {
			containers = append(containers, c.containersFromLabState(state)...)
		}

		return filterLabRuntimeContainers(containers, opts.ToFilters()), nil
	}

	state, err := c.LabRuntime.Inspect(ctx, clablabruntime.InspectRequest{Name: c.Config.Name})
	if err != nil {
		return nil, err
	}

	return filterLabRuntimeContainers(c.containersFromLabState(state), opts.ToFilters()), nil
}

func (c *CLab) execWithLabRuntime(
	ctx context.Context,
	cmds []string,
	listOptions ...ListOption,
) (*clabexec.ExecCollection, error) {
	containers, err := c.ListLabRuntimeContainers(ctx, false, listOptions...)
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, fmt.Errorf("filter did not match any containers")
	}

	var execCmds []*clabexec.ExecCmd
	for _, execCmdStr := range cmds {
		execCmd, err := clabexec.NewExecCmdFromString(execCmdStr)
		if err != nil {
			return nil, err
		}
		execCmds = append(execCmds, execCmd)
	}

	resultCollection := clabexec.NewExecCollection()
	var execErrors []error
	for idx := range containers {
		namespace, labName, nodeName, err := labRuntimeContainerParts(containers[idx])
		if err != nil {
			log.Warnf("exec target %s is invalid: %v", containers[idx].Names[0], err)
			execErrors = append(execErrors, fmt.Errorf(
				"exec target %s is invalid: %w", containers[idx].Names[0], err))
			continue
		}

		for _, execCmd := range execCmds {
			result, err := c.LabRuntime.Exec(ctx, clablabruntime.ExecRequest{
				Name:      labName,
				Namespace: namespace,
				NodeName:  nodeName,
				Command:   execCmd.GetCmd(),
			})
			if err != nil {
				log.Warnf("exec on %s failed: %v", containers[idx].Names[0], err)
				execErrors = append(execErrors, fmt.Errorf(
					"exec on %s failed: %w", containers[idx].Names[0], err))
				continue
			}

			resultCollection.Add(containers[idx].Names[0], result)
			if result.GetReturnCode() != 0 {
				execErrors = append(execErrors, fmt.Errorf(
					"exec on %s returned exit code %d",
					containers[idx].Names[0],
					result.GetReturnCode(),
				))
			}
		}
	}

	return resultCollection, errors.Join(execErrors...)
}

func (c *CLab) startNodesWithLabRuntime(ctx context.Context, nodeNames []string) error {
	return c.LabRuntime.Start(ctx, clablabruntime.NodeRequest{
		Name:    c.Config.Name,
		Nodes:   nodeNames,
		Timeout: c.timeout,
	})
}

func (c *CLab) stopNodesWithLabRuntime(ctx context.Context, nodeNames []string) error {
	return c.LabRuntime.Stop(ctx, clablabruntime.NodeRequest{
		Name:    c.Config.Name,
		Nodes:   nodeNames,
		Timeout: c.timeout,
	})
}

func (c *CLab) restartNodesWithLabRuntime(ctx context.Context, nodeNames []string) error {
	return c.LabRuntime.Restart(ctx, clablabruntime.NodeRequest{
		Name:    c.Config.Name,
		Nodes:   nodeNames,
		Timeout: c.timeout,
	})
}

func (c *CLab) saveWithLabRuntime(ctx context.Context, opts *SaveOptions) error {
	if opts.copyDst != "" {
		resolvedDst, err := c.resolveCopyOutDst(opts.copyDst)
		if err != nil {
			return err
		}
		opts.copyDst = resolvedDst
	}

	result, err := c.LabRuntime.Save(ctx, clablabruntime.SaveRequest{
		Name:  c.Config.Name,
		Nodes: c.nodeFilter,
		Copy:  opts.copyDst != "",
	})
	if err != nil {
		return err
	}

	if opts.copyDst == "" || result == nil {
		return nil
	}

	return c.copyLabRuntimeSavedFiles(result, opts.copyDst)
}

func (c *CLab) containersFromLabState(
	state *clablabruntime.LabState,
) []clabruntime.GenericContainer {
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

		nodes = make([]clablabruntime.NodeState, 0, len(nodeNames))
		for _, nodeName := range nodeNames {
			nodes = append(nodes, clablabruntime.NodeState{
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
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
) clabruntime.GenericContainer {
	nodeName := fmt.Sprintf("%s-%s", state.Name, node.Name)
	containerState := node.State
	containerStatus := node.State
	if containerState == "" {
		containerState = state.State
	}
	if node.Ready {
		containerState = "running"
		containerStatus = "healthy"
	}

	labels := map[string]string{
		clabconstants.Containerlab: state.Name,
		clabconstants.NodeName:     node.Name,
		clabconstants.LongName:     nodeName,
		clabconstants.TopoFile:     labRuntimeTopologyPath(c, state),
	}

	var image string
	if c.Config.Topology != nil {
		topologyNode := c.topologyNodeName(node.Name)
		labels[clabconstants.NodeKind] = c.Config.Topology.GetNodeKind(topologyNode)
		image = c.Config.Topology.GetNodeImage(topologyNode)

		if group := c.Config.Topology.GetNodeGroup(topologyNode); group != "" {
			labels[clabconstants.NodeGroup] = group
		}
	}
	if node.Kind != "" {
		labels[clabconstants.NodeKind] = node.Kind
	}
	if node.Image != "" {
		image = node.Image
	}

	if state.Owner != "" {
		labels[clabconstants.Owner] = state.Owner
	} else if c.customOwner != "" {
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
		NetworkSettings: managementAddress(node),
	}
}

// topologyNodeName maps a node name a lab runtime reports back onto the topology node it belongs
// to. The runtime renames the nodes Kubernetes cannot carry, so the state it reports does not
// always use the name the topology file wrote.
func (c *CLab) topologyNodeName(name string) string {
	if c.Config.Topology == nil {
		return name
	}
	if _, ok := c.Config.Topology.Nodes[name]; ok {
		return name
	}

	for nodeName := range c.Config.Topology.Nodes {
		if clablabruntime.SanitizeName(nodeName) == name {
			return nodeName
		}
	}

	return name
}

// managementAddress reports the address a user reaches the node at: the LoadBalancer address
// when the node is exposed, otherwise the allocated management address.
func managementAddress(node clablabruntime.NodeState) clabruntime.GenericMgmtIPs {
	settings := clabruntime.GenericMgmtIPs{}

	if ip := net.ParseIP(node.LoadBalancerAddress); ip != nil {
		if ip.To4() != nil {
			settings.IPv4addr = node.LoadBalancerAddress
			settings.IPv4pLen = 32
		} else {
			settings.IPv6addr = node.LoadBalancerAddress
			settings.IPv6pLen = 128
		}
	}

	if settings.IPv4addr == "" {
		settings.IPv4addr, settings.IPv4pLen = splitManagementCIDR(node.MgmtIPv4Address)
	}
	if settings.IPv6addr == "" {
		settings.IPv6addr, settings.IPv6pLen = splitManagementCIDR(node.MgmtIPv6Address)
	}

	return settings
}

func splitManagementCIDR(cidr string) (string, int) {
	if cidr == "" {
		return "", 0
	}

	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		if net.ParseIP(cidr) != nil {
			return cidr, 0
		}

		return "", 0
	}

	prefixLen, _ := network.Mask.Size()

	return ip.String(), prefixLen
}

func labRuntimeTopologyPath(c *CLab, state *clablabruntime.LabState) string {
	if c.TopoPaths.TopologyFileIsSet() {
		return c.TopoPaths.TopologyFilenameAbsPath()
	}

	if state.TopologyPath != "" {
		return state.TopologyPath
	}

	return fmt.Sprintf("k8s://%s/topologies/%s", state.Namespace, state.Name)
}

func labRuntimeContainerParts(c clabruntime.GenericContainer) (string, string, string, error) {
	parts := strings.Split(c.ID, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("expected namespace/lab/node id, got %q", c.ID)
	}

	return parts[0], parts[1], parts[2], nil
}

func filterLabRuntimeContainers(
	containers []clabruntime.GenericContainer,
	filters []*clabtypes.GenericFilter,
) []clabruntime.GenericContainer {
	if len(filters) == 0 {
		return containers
	}

	filtered := make([]clabruntime.GenericContainer, 0, len(containers))
	for _, container := range containers {
		if labRuntimeContainerMatches(container, filters) {
			filtered = append(filtered, container)
		}
	}

	return filtered
}

func labRuntimeContainerMatches(
	container clabruntime.GenericContainer,
	filters []*clabtypes.GenericFilter,
) bool {
	for _, filter := range filters {
		switch filter.FilterType {
		case "name":
			if !labRuntimeNameMatches(container.Names, filter.Match) {
				return false
			}
		case "id":
			if container.ID != filter.Match && !strings.HasPrefix(container.ID, filter.Match) {
				return false
			}
		case "label":
			if !labRuntimeLabelMatches(container.Labels, filter) {
				return false
			}
		}
	}

	return true
}

func labRuntimeNameMatches(names []string, match string) bool {
	for _, name := range names {
		if name == match || strings.Contains(name, match) {
			return true
		}
	}

	return false
}

func labRuntimeLabelMatches(labels map[string]string, filter *clabtypes.GenericFilter) bool {
	value, ok := labels[filter.Field]

	switch filter.Operator {
	case "exists":
		return ok
	case "!=":
		return !ok || value != filter.Match
	default:
		return ok && value == filter.Match
	}
}

func (c *CLab) copyLabRuntimeSavedFiles(
	result *clablabruntime.SaveResult,
	dstRoot string,
) error {
	for _, file := range result.Files {
		if file.NodeName == "" || file.Name == "" {
			continue
		}

		relPath, ok := cleanLabRuntimeSavedPath(file.Name)
		if !ok {
			return fmt.Errorf("refusing to copy unsafe saved config path %q", file.Name)
		}

		nodeDstDir := filepath.Join(dstRoot, file.NodeName)
		dstPath := filepath.Join(nodeDstDir, relPath)
		if err := os.MkdirAll(
			filepath.Dir(dstPath),
			clabconstants.PermissionsDirDefault,
		); err != nil {
			return fmt.Errorf("failed to create save dst directory %q: %w",
				filepath.Dir(dstPath), err)
		}

		if file.LinkTarget != "" {
			linkTarget, ok := cleanLabRuntimeSavedLinkTarget(file.LinkTarget)
			if !ok {
				return fmt.Errorf("refusing to create unsafe saved config symlink %q -> %q",
					file.Name, file.LinkTarget)
			}

			_ = os.Remove(dstPath)
			if err := os.Symlink(linkTarget, dstPath); err != nil {
				return fmt.Errorf("failed to create symlink %q -> %q: %w",
					dstPath, linkTarget, err)
			}

			continue
		}

		mode := os.FileMode(file.Mode)
		if mode == 0 {
			mode = clabconstants.PermissionsFileDefault
		}

		if err := os.WriteFile(dstPath, file.Data, mode); err != nil {
			return fmt.Errorf("failed to write saved config %q: %w", dstPath, err)
		}

		log.Info(
			"copied saved config",
			"node", file.NodeName,
			"dst", dstPath,
		)
	}

	return nil
}

func cleanLabRuntimeSavedPath(name string) (string, bool) {
	cleaned := filepath.Clean(name)
	if cleaned == "." || filepath.IsAbs(cleaned) ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) ||
		cleaned == ".." {
		return "", false
	}

	return cleaned, true
}

func cleanLabRuntimeSavedLinkTarget(target string) (string, bool) {
	cleaned := filepath.Clean(target)
	if filepath.IsAbs(cleaned) ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) ||
		cleaned == ".." {
		return "", false
	}

	return cleaned, true
}
