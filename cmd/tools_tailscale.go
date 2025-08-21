package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabels "github.com/srl-labs/containerlab/labels"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const tailscaleConst string = "tailscale"

// TailscaleListItem defines the structure for Tailscale container info in JSON output.
type TailscaleListItem struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	State       string `json:"state"`
	IPv4Address string `json:"ipv4_address"`
	TailscaleIP string `json:"tailscale_ip"`
	Owner       string `json:"owner"`
}

// TailscaleNode implements runtime.Node interface for Tailscale containers.
type TailscaleNode struct {
	config *clabtypes.NodeConfig
}

// ToolsTailscaleOptions holds options for tailscale commands.
type ToolsTailscaleOptions struct {
	ContainerName string
	AuthKey       string
	Image         string
	Owner         string
	AcceptRoutes  bool
	Ephemeral     bool
	Format        string
}

func tailscaleCmd(o *Options) (*cobra.Command, error) {
	// Initialize tailscale options if not present
	if o.ToolsTailscale == nil {
		o.ToolsTailscale = &ToolsTailscaleOptions{
			Image:     "tailscale/tailscale:latest",
			Ephemeral: true,
			Format:    "table",
		}
	}

	c := &cobra.Command{
		Use:   tailscaleConst,
		Short: "Tailscale VPN operations",
		Long:  "Attach or detach lab mgmt subnet to a Tailscale tailnet",
	}

	c.PersistentFlags().StringVarP(&o.ToolsTailscale.Format, "format", "f", o.ToolsTailscale.Format,
		"output format for 'list' command (table, json)")

	tailscaleListCmd := &cobra.Command{
		Use:   "list",
		Short: "list active Tailscale containers",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return tailscaleList(cobraCmd, o)
		},
	}

	c.AddCommand(tailscaleListCmd)

	tailscaleAttachCmd := &cobra.Command{
		Use:   "attach",
		Short: "attach a lab to a Tailscale tailnet",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return tailscaleAttach(cobraCmd, o)
		},
	}

	c.AddCommand(tailscaleAttachCmd)

	tailscaleAttachCmd.Flags().StringVarP(&o.Global.TopologyName, "lab", "l", o.Global.TopologyName,
		"name of the lab to attach Tailscale container to")
	tailscaleAttachCmd.Flags().StringVarP(&o.ToolsTailscale.ContainerName, "name", "", o.ToolsTailscale.ContainerName,
		"name of the Tailscale container (defaults to tailscale-<labname>)")
	tailscaleAttachCmd.Flags().StringVarP(&o.ToolsTailscale.AuthKey, "auth-key", "k", o.ToolsTailscale.AuthKey,
		"Tailscale auth key for authentication")
	tailscaleAttachCmd.Flags().StringVarP(&o.ToolsTailscale.Image, "image", "i", o.ToolsTailscale.Image,
		"container image to use for Tailscale")
	tailscaleAttachCmd.Flags().StringVarP(&o.ToolsTailscale.Owner, "owner", "o", o.ToolsTailscale.Owner,
		"lab owner name for the Tailscale container")
	tailscaleAttachCmd.Flags().BoolVarP(&o.ToolsTailscale.AcceptRoutes, "accept-routes", "", o.ToolsTailscale.AcceptRoutes,
		"accept subnet routes advertised by other nodes")
	tailscaleAttachCmd.Flags().BoolVarP(&o.ToolsTailscale.Ephemeral, "ephemeral", "", o.ToolsTailscale.Ephemeral,
		"make this node ephemeral")

	tailscaleDetachCmd := &cobra.Command{
		Use:   "detach",
		Short: "detach a lab management subnet from a tailscale tailnet",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return tailscaleDetach(cobraCmd, o)
		},
	}

	c.AddCommand(tailscaleDetachCmd)

	tailscaleDetachCmd.Flags().StringVarP(&o.Global.TopologyName, "lab", "l", o.Global.TopologyName,
		"name of the lab where Tailscale container is attached")

	tailscaleReattachCmd := &cobra.Command{
		Use:   "reattach",
		Short: "detach and reattach a Tailscale container to a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return tailscaleReattach(cobraCmd, o)
		},
	}

	c.AddCommand(tailscaleReattachCmd)

	tailscaleReattachCmd.Flags().StringVarP(&o.Global.TopologyName, "lab", "l", o.Global.TopologyName,
		"name of the lab to reattach Tailscale container to")
	tailscaleReattachCmd.Flags().StringVarP(&o.ToolsTailscale.ContainerName, "name", "", o.ToolsTailscale.ContainerName,
		"name of the Tailscale container (defaults to tailscale-<labname>)")
	tailscaleReattachCmd.Flags().StringVarP(&o.ToolsTailscale.AuthKey, "auth-key", "k", o.ToolsTailscale.AuthKey,
		"Tailscale auth key for authentication")
	tailscaleReattachCmd.Flags().StringVarP(&o.ToolsTailscale.Image, "image", "i", o.ToolsTailscale.Image,
		"container image to use for Tailscale")
	tailscaleReattachCmd.Flags().StringVarP(&o.ToolsTailscale.Owner, "owner", "o", o.ToolsTailscale.Owner,
		"lab owner name for the Tailscale container")
	tailscaleReattachCmd.Flags().BoolVarP(&o.ToolsTailscale.AcceptRoutes, "accept-routes", "", o.ToolsTailscale.AcceptRoutes,
		"accept subnet routes advertised by other nodes")
	tailscaleReattachCmd.Flags().BoolVarP(&o.ToolsTailscale.Ephemeral, "ephemeral", "", o.ToolsTailscale.Ephemeral,
		"make this node ephemeral")

	return c, nil
}

// NewTailscaleNode creates a new Tailscale node configuration.
func NewTailscaleNode(name, image, network, authKey string, acceptRoutes bool, isEphemeral bool, rt clabruntime.ContainerRuntime, labels map[string]string) *TailscaleNode {
	log.Debugf("Creating TailscaleNode: name=%s, image=%s, network=%s, acceptRoutes=%t, ephemeral=%t",
		name, image, network, acceptRoutes, isEphemeral)

	// Build tailscale up command with options
	var tsExtraArgs []string
	// extra args for tailscaled
	var tsdExtraArgs []string
	if isEphemeral {
		tsdExtraArgs = append(tsdExtraArgs, "--state=mem:")
	}
	if acceptRoutes {
		tsExtraArgs = append(tsExtraArgs, "--accept-routes")
	}

	subnets := getMgmtNetworkSubnets(rt)
	if len(subnets) > 0 {
		routesArg := "--advertise-routes=" + strings.Join(subnets, ",")
		tsExtraArgs = append(tsExtraArgs, routesArg)
		log.Debugf("Adding advertise routes argument: %s", routesArg)
	} else {
		log.Warn("No management network subnets found to advertise")
	}

	tsExtraArgs = append(tsExtraArgs, "--reset")

	nodeConfig := &clabtypes.NodeConfig{
		LongName:   name,
		ShortName:  name,
		Image:      image,
		Entrypoint: "",
		Cmd:        "",
		MgmtNet:    network,
		Labels:     labels,
		Env: map[string]string{
			"TS_AUTHKEY":   authKey,
			"TS_STATE_DIR": "/var/lib/tailscale",
			"TS_SOCKET":    "/var/run/tailscale/tailscaled.sock",
			"TS_USERSPACE": "false",
		},
		Sysctls: map[string]string{
			"net.ipv4.ip_forward":          "1",
			"net.ipv6.conf.all.forwarding": "1",
		},
		CapAdd: []string{"NET_ADMIN", "NET_RAW"},
		Binds: []string{
			"/dev/net/tun:/dev/net/tun",
		},
		Healthcheck: &clabtypes.HealthcheckConfig{ //  healthcheck to check if ts is up & connected
			Test:        []string{"CMD", "tailscale", "status", "--self"},
			Interval:    5,
			Timeout:     3,
			StartPeriod: 10,
			Retries:     3,
		},
	}

	// Add up args as environment variable if any are set
	if len(tsExtraArgs) > 0 {
		nodeConfig.Env["TS_EXTRA_ARGS"] = strings.Join(tsExtraArgs, " ")
	}
	if len(tsdExtraArgs) > 0 {
		nodeConfig.Env["TS_TAILSCALED_EXTRA_ARGS"] = strings.Join(tsdExtraArgs, " ")
	}

	return &TailscaleNode{
		config: nodeConfig,
	}
}

func (n *TailscaleNode) Config() *clabtypes.NodeConfig {
	return n.config
}

func (*TailscaleNode) GetEndpoints() []clablinks.Endpoint {
	return nil
}

// getTailscaleStatus retrieves the Tailscale status from the container.
func getTailscaleStatus(ctx context.Context, rt clabruntime.ContainerRuntime, containerName string) string {
	execCmd, err := clabexec.NewExecCmdFromString("tailscale ip")
	if err != nil {
		return ""
	}

	execResult, err := rt.Exec(ctx, containerName, execCmd)
	if err != nil || execResult.GetReturnCode() != 0 {
		return ""
	}

	ip := strings.TrimSpace(execResult.GetStdOutString())
	return ip
}

// get the actual node name in the tailnet (in case of duplicate names tailscale appends a hyphen + number)
func getTailscaleNodeName(ctx context.Context, rt clabruntime.ContainerRuntime, containerName string) string {
	execCmd, err := clabexec.NewExecCmdFromString("tailscale status --self --json")
	if err != nil {
		return ""
	}

	execResult, err := rt.Exec(ctx, containerName, execCmd)
	if err != nil || execResult.GetReturnCode() != 0 {
		return ""
	}

	var statusData map[string]any
	if err := json.Unmarshal([]byte(execResult.GetStdOutString()), &statusData); err != nil {
		log.Debugf("Failed to parse Tailscale status JSON: %v", err)
		return ""
	}

	if self, ok := statusData["Self"].(map[string]any); ok {
		if name, ok := self["HostName"].(string); ok {
			return name
		}
	}

	return ""
}

func waitForTailscaleReady(ctx context.Context, rt clabruntime.ContainerRuntime, containerName string, timeout time.Duration) error {
	log.Debug("Waiting for tailscale to be ready", "container", containerName, "timeout", timeout)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	isHealthy, err := rt.IsHealthy(timeoutCtx, containerName)
	if err != nil {
		log.Debug("Initial tailscale health check failed", "container", containerName, "error", err)
	} else if isHealthy {
		log.Debug("Tailscale container is healthy", "container", containerName)
		return nil
	}

	for {
		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("tailscale container %s did not become healthy within %v", containerName, timeout)
			}
			return fmt.Errorf("context cancelled while waiting for tailscale: %v", timeoutCtx.Err())

		case <-ticker.C:
			isHealthy, err := rt.IsHealthy(timeoutCtx, containerName)
			if err != nil {
				log.Debug("tailscale health check failed", "container", containerName, "error", err)
			} else if isHealthy {
				log.Debug("tailscale container healthy", "container", containerName)
				return nil
			}
		}
	}
}

func getMgmtNetworkSubnets(rt clabruntime.ContainerRuntime) []string {
	mgmtNet := rt.Mgmt()
	var subnets []string

	log.Debug("Tailscale mgmt net info", "network", mgmtNet.Network, "ipv4", mgmtNet.IPv4Subnet, "ipv6", mgmtNet.IPv6Subnet)

	if mgmtNet.IPv4Subnet != "" {
		subnets = append(subnets, mgmtNet.IPv4Subnet)
	}
	if mgmtNet.IPv6Subnet != "" {
		subnets = append(subnets, mgmtNet.IPv6Subnet)
	}

	if len(subnets) == 0 && mgmtNet.Network != "" {
		log.Debug("Runtime has no subnet info", "network", mgmtNet.Network)
	}

	log.Debug("Got management network for tailscale", "subnets", subnets)
	return subnets
}

func tailscaleAttach(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	log.Debug("tailscale attach called",
		"labName", o.Global.TopologyName,
		"containerName", o.ToolsTailscale.ContainerName,
		"image", o.ToolsTailscale.Image,
		"topoFile", o.Global.TopologyFile,
		"acceptRoutes", o.ToolsTailscale.AcceptRoutes,
		"ephemeral", o.ToolsTailscale.Ephemeral)

	// Get lab topology information
	clabInstance, err := clabcore.NewclabFromTopologyFileOrLabName(ctx, o.Global.TopologyFile,
		o.Global.TopologyName, o.Global.VarsFile, o.Global.Runtime, o.Global.DebugCount > 0, o.Global.Timeout, o.Destroy.GracefulShutdown)
	if err != nil {
		return err
	}

	labName := clabInstance.Config.Name

	networkName := clabInstance.Config.Mgmt.Network
	if networkName == "" {
		networkName = "clab-" + labName
	}

	// Set container name if not provided
	if o.ToolsTailscale.ContainerName == "" {
		o.ToolsTailscale.ContainerName = fmt.Sprintf("clab-%s-tailscale", labName)
		log.Debugf("Container name not provided, generated name: %s", o.ToolsTailscale.ContainerName)
	}

	if o.ToolsTailscale.AuthKey == "" {
		// grab from system env
		if envKey := os.Getenv("TS_AUTHKEY"); envKey != "" {
			o.ToolsTailscale.AuthKey = envKey
		} else {
			return fmt.Errorf("auth key is required for tailscale. Use --auth-key flag or set the TS_AUTHKEY env var")
		}
	}

	// Initialize runtime with management network info from the deployed lab
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer for '%s': %w", o.Global.Runtime, err)
	}

	rt := rinit()

	mgmtNet := clabInstance.Config.Mgmt
	log.Debugf("Using mgmt network from deployed lab: %+v", mgmtNet)

	err = rt.Init(
		clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}),
		clabruntime.WithMgmtNet(mgmtNet),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Check if container already exists
	filter := []*clabtypes.GenericFilter{{FilterType: "name", Match: o.ToolsTailscale.ContainerName}}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) > 0 {
		return fmt.Errorf("container %s already exists", o.ToolsTailscale.ContainerName)
	}

	log.Infof("Pulling image %s...", o.ToolsTailscale.Image)
	if err := rt.PullImage(ctx, o.ToolsTailscale.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsTailscale.Image, err)
	}

	owner := o.ToolsTailscale.Owner
	if owner == "" {
		owner = clabutils.GetOwner()
	}

	labelsMap := createLabelsMap(
		clabInstance.TopoPaths.TopologyFilenameAbsPath(),
		labName,
		o.ToolsTailscale.ContainerName,
		owner,
		tailscaleConst,
	)

	log.Infof("Creating tailscale container %s on network '%s'", o.ToolsTailscale.ContainerName, networkName)

	tailscaleNode := NewTailscaleNode(o.ToolsTailscale.ContainerName, o.ToolsTailscale.Image, networkName, o.ToolsTailscale.AuthKey,
		o.ToolsTailscale.AcceptRoutes, o.ToolsTailscale.Ephemeral, rt, labelsMap)

	id, err := rt.CreateContainer(ctx, tailscaleNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create tailscale container: %v", err)
	}

	if _, err := rt.StartContainer(ctx, id, tailscaleNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsTailscale.ContainerName)
		return fmt.Errorf("failed to start tailscale container: %v", err)
	}

	log.Info("Tailscale container started. Waiting for tailnet connection", "container", o.ToolsTailscale.ContainerName)

	if err := waitForTailscaleReady(ctx, rt, o.ToolsTailscale.ContainerName, 60*time.Second); err != nil {
		log.With("error", err).Warnf("Tailscale healthcheck failed, check the logs for more details: docker logs %s", o.ToolsTailscale.ContainerName)
		return nil
	}

	tsIPAddrs := getTailscaleStatus(ctx, rt, o.ToolsTailscale.ContainerName)
	if tsIPAddrs == "" {
		log.Warn("Tailscale container is healthy but failed to retrieve IP address. Check the logs for more details: docker logs %s", o.ToolsTailscale.ContainerName)
		return nil
	}

	tsNodeName := getTailscaleNodeName(ctx, rt, o.ToolsTailscale.ContainerName)

	subnets := getMgmtNetworkSubnets(rt)

	log.Info("Tailscale attached",
		"tailscale ip", tsIPAddrs,
		"lab subnet", strings.Join(subnets, "\n"),
		"tailscale node", tsNodeName)

	return nil
}

func tailscaleDetach(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	// Get lab topology information
	clabInstance, err := clabcore.NewclabFromTopologyFileOrLabName(ctx, o.Global.TopologyFile,
		o.Global.TopologyName, o.Global.VarsFile, o.Global.Runtime, o.Global.DebugCount > 0, o.Global.Timeout, o.Destroy.GracefulShutdown)
	if err != nil {
		return err
	}
	labName := clabInstance.Config.Name

	log.Debug("tailscale detach called", "labName", labName)

	// Initialize runtime
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer: %w", err)
	}

	rt := rinit()
	err = rt.Init(clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	filter := []*clabtypes.GenericFilter{
		{
			FilterType: "label",
			Field:      clablabels.ToolType,
			Operator:   "=",
			Match:      tailscaleConst,
		},
		{
			FilterType: "label",
			Field:      clablabels.Containerlab,
			Operator:   "=",
			Match:      labName,
		},
	}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list tailscale containers: %w", err)
	}

	if len(containers) == 0 {
		log.Info("No tailscale containers found for lab", "lab", labName)
		return nil
	}

	log.Info("Found tailscale containers for lab", "lab", labName, "count", len(containers))

	for _, container := range containers {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		log.Info("Removing tailscale container", "container", containerName)

		if err := rt.DeleteContainer(ctx, containerName); err != nil {
			log.Error("Failed to remove tailscale container", "container", containerName, "error", err)
			return fmt.Errorf("failed to remove tailscale container %s: %w", containerName, err)
		}

		log.Info("Tailscale container removed", "container", containerName)
	}

	return nil
}

func tailscaleList(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	// Initialize runtime
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer: %w", err)
	}

	rt := rinit()
	err = rt.Init(clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Filter only by Tailscale label
	filter := []*clabtypes.GenericFilter{
		{
			FilterType: "label",
			Field:      clablabels.ToolType,
			Operator:   "=",
			Match:      tailscaleConst,
		},
	}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		if o.ToolsTailscale.Format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No active Tailscale containers found")
		}
		return nil
	}

	// Process containers and format output
	listItems := make([]TailscaleListItem, 0, len(containers))
	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		network := c.NetworkName
		if network == "" {
			network = "unknown"
		}

		// Get owner from container labels
		owner := "N/A"
		if ownerVal, exists := c.Labels[clablabels.Owner]; exists && ownerVal != "" {
			owner = ownerVal
		}

		// Try to get the Tailscale IP if container is running
		tailscaleIP := "N/A"
		if c.State == "running" {
			if ip := getTailscaleStatus(ctx, rt, name); ip != "" {
				tailscaleIP = ip
			}
		}

		listItems = append(listItems, TailscaleListItem{
			Name:        name,
			Network:     network,
			State:       c.State,
			IPv4Address: c.NetworkSettings.IPv4addr,
			TailscaleIP: tailscaleIP,
			Owner:       owner,
		})
	}

	// Output based on format
	if o.ToolsTailscale.Format == "json" {
		b, err := json.MarshalIndent(listItems, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		fmt.Println(string(b))
	} else {
		// Use go-pretty table
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleRounded)
		t.Style().Format.Header = text.FormatTitle
		t.Style().Options.SeparateRows = true

		t.AppendHeader(table.Row{"NAME", "NETWORK", "STATUS", "IPv4 ADDRESS", "TAILSCALE IP", "OWNER"})

		for _, item := range listItems {
			t.AppendRow(table.Row{
				item.Name,
				item.Network,
				item.State,
				item.IPv4Address,
				item.TailscaleIP,
				item.Owner,
			})
		}
		t.Render()
	}

	return nil
}

func tailscaleReattach(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	log.Debug("tailscale reattach called",
		"labName", o.Global.TopologyName,
		"containerName", o.ToolsTailscale.ContainerName,
		"image", o.ToolsTailscale.Image,
		"topoFile", o.Global.TopologyFile,
		"acceptRoutes", o.ToolsTailscale.AcceptRoutes,
		"ephemeral", o.ToolsTailscale.Ephemeral)

	// Get lab topology information
	clabInstance, err := clabcore.NewclabFromTopologyFileOrLabName(ctx, o.Global.TopologyFile,
		o.Global.TopologyName, o.Global.VarsFile, o.Global.Runtime, o.Global.DebugCount > 0, o.Global.Timeout, o.Destroy.GracefulShutdown)
	if err != nil {
		return err
	}

	labName := clabInstance.Config.Name
	networkName := clabInstance.Config.Mgmt.Network
	if networkName == "" {
		networkName = "clab-" + labName
	}

	// Set container name if not provided
	if o.ToolsTailscale.ContainerName == "" {
		o.ToolsTailscale.ContainerName = fmt.Sprintf("clab-%s-tailscale", labName)
		log.Debugf("Container name not provided, generated name: %s", o.ToolsTailscale.ContainerName)
	}

	if o.ToolsTailscale.AuthKey == "" {
		// grab from system env
		if envKey := os.Getenv("TS_AUTHKEY"); envKey != "" {
			o.ToolsTailscale.AuthKey = envKey
		} else {
			return fmt.Errorf("auth key is required for tailscale. Use --auth-key flag or set the TS_AUTHKEY env var")
		}
	}

	// Initialize runtime with management network info from the deployed lab
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer for '%s': %w", o.Global.Runtime, err)
	}

	rt := rinit()

	mgmtNet := clabInstance.Config.Mgmt
	log.Debugf("Using mgmt network from deployed lab: %+v", mgmtNet)

	err = rt.Init(
		clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}),
		clabruntime.WithMgmtNet(mgmtNet),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Step 1: Remove existing Tailscale containers using labels
	log.Info("Removing existing tailscale containers for lab", "lab", labName)

	// Use labels to find Tailscale containers for this lab
	filter := []*clabtypes.GenericFilter{
		{
			FilterType: "label",
			Field:      clablabels.ToolType,
			Operator:   "=",
			Match:      tailscaleConst,
		},
		{
			FilterType: "label",
			Field:      clablabels.Containerlab,
			Operator:   "=",
			Match:      labName,
		},
	}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		log.Debug("Removing existing tailscale container", "container", containerName)

		if err := rt.DeleteContainer(ctx, containerName); err != nil {
			log.Debug("Could not remove container", "container", containerName, "error", err)
		} else {
			log.Debug("Removed existing tailscale container", "container", containerName)
		}
	}

	// Step 2: Create and attach new Tailscale container
	log.Info("Pulling tailscale image", "image", o.ToolsTailscale.Image)
	if err := rt.PullImage(ctx, o.ToolsTailscale.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsTailscale.Image, err)
	}

	// Create container labels
	owner := o.ToolsTailscale.Owner
	if owner == "" {
		owner = clabutils.GetOwner()
	}

	labelsMap := createLabelsMap(
		clabInstance.TopoPaths.TopologyFilenameAbsPath(),
		labName,
		o.ToolsTailscale.ContainerName,
		owner,
		tailscaleConst,
	)

	log.Infof("Creating tailscale container %s on network '%s'", o.ToolsTailscale.ContainerName, networkName)

	tailscaleNode := NewTailscaleNode(o.ToolsTailscale.ContainerName, o.ToolsTailscale.Image, networkName, o.ToolsTailscale.AuthKey,
		o.ToolsTailscale.AcceptRoutes, o.ToolsTailscale.Ephemeral, rt, labelsMap)

	id, err := rt.CreateContainer(ctx, tailscaleNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create tailscale container: %v", err)
	}

	if _, err := rt.StartContainer(ctx, id, tailscaleNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsTailscale.ContainerName)
		return fmt.Errorf("failed to start tailscale container: %v", err)
	}

	log.Info("Tailscale container started. Waiting for tailnet connection", "container", o.ToolsTailscale.ContainerName)

	// Wait for Tailscale to be ready using healthcheck
	if err := waitForTailscaleReady(ctx, rt, o.ToolsTailscale.ContainerName, 60*time.Second); err != nil {
		log.Warn("Tailscale container failed to become ready", "error", err, "container", o.ToolsTailscale.ContainerName)
		log.Info("Check the logs for more details", "command", fmt.Sprintf("docker logs %s", o.ToolsTailscale.ContainerName))
		return nil
	}

	tsIPAddrs := getTailscaleStatus(ctx, rt, o.ToolsTailscale.ContainerName)
	if tsIPAddrs == "" {
		log.Warn("Tailscale container is healthy but failed to retrieve IP address.\nCheck the logs: docker logs %s", o.ToolsTailscale.ContainerName)
		return nil
	}

	tsNodeName := getTailscaleNodeName(ctx, rt, o.ToolsTailscale.ContainerName)

	subnets := getMgmtNetworkSubnets(rt)

	log.Info("Tailscale reattached",
		"tailscale ip", tsIPAddrs,
		"lab subnet", strings.Join(subnets, "\n"),
		"tailscale node", tsNodeName)

	return nil
}
