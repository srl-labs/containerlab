package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/go-connections/nat"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	gotty         = "gotty"
	gottyWaitTime = 5 * time.Second
)

// GoTTYListItem defines the structure for GoTTY container info in JSON output.
type GoTTYListItem struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	State       string `json:"state"`
	IPv4Address string `json:"ipv4_address"`
	Port        uint   `json:"port"`
	WebURL      string `json:"web_url"`
	Owner       string `json:"owner"`
}

// GoTTYNode implements runtime.Node interface for GoTTY containers.
type GoTTYNode struct {
	config *clabtypes.NodeConfig
}

func gottyCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   gotty,
		Short: "GoTTY web terminal operations",
		Long:  "Attach or detach GoTTY web terminal containers to labs",
	}

	gottyListCmd := &cobra.Command{
		Use:   "list",
		Short: "list active GoTTY containers",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return gottyList(cobraCmd, o)
		},
	}

	c.AddCommand(gottyListCmd)

	c.PersistentFlags().StringVarP(
		&o.ToolsGoTTY.Format,
		"format",
		"f",
		o.ToolsGoTTY.Format,
		"output format for 'list' command (table, json)",
	)

	gottyAttachCmd := &cobra.Command{
		Use:   "attach",
		Short: "attach GoTTY web terminal to a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return gottyAttach(cobraCmd, o)
		},
	}

	c.AddCommand(gottyAttachCmd)

	gottyAttachCmd.Flags().StringVarP(
		&o.Global.TopologyName,
		"lab",
		"l",
		o.Global.TopologyName,
		"name of the lab to attach GoTTY container to",
	)
	gottyAttachCmd.Flags().StringVarP(&o.ToolsGoTTY.ContainerName,
		"name",
		"",
		o.ToolsGoTTY.ContainerName,
		"name of the GoTTY container (defaults to gotty-<labname>)",
	)
	gottyAttachCmd.Flags().UintVarP(&o.ToolsGoTTY.Port,
		"port",
		"p",
		o.ToolsGoTTY.Port,
		"port for GoTTY web terminal",
	)
	gottyAttachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Username,
		"username",
		"u",
		o.ToolsGoTTY.Username,
		"username for GoTTY web terminal authentication",
	)
	gottyAttachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Password,
		"password",
		"P",
		o.ToolsGoTTY.Password,
		"password for GoTTY web terminal authentication",
	)
	gottyAttachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Shell,
		"shell",
		"s",
		o.ToolsGoTTY.Shell,
		"shell to use for GoTTY web terminal",
	)
	gottyAttachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Image,
		"image",
		"i",
		o.ToolsGoTTY.Image,
		"container image to use for GoTTY",
	)
	gottyAttachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Owner,
		"owner",
		"o",
		o.ToolsGoTTY.Owner,
		"lab owner name for the GoTTY container",
	)

	gottyDetachCmd := &cobra.Command{
		Use:   "detach",
		Short: "detach GoTTY web terminal from a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return gottyDetach(cobraCmd, o)
		},
	}

	c.AddCommand(gottyDetachCmd)

	gottyDetachCmd.Flags().StringVarP(
		&o.Global.TopologyName,
		"lab",
		"l",
		o.Global.TopologyName,
		"name of the lab where GoTTY container is attached",
	)

	gottyReattachCmd := &cobra.Command{
		Use:   "reattach",
		Short: "detach and reattach GoTTY web terminal to a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return gottyReattach(cobraCmd, o)
		},
	}

	c.AddCommand(gottyReattachCmd)

	gottyReattachCmd.Flags().StringVarP(
		&o.Global.TopologyName,
		"lab",
		"l",
		o.Global.TopologyName,
		"name of the lab to reattach GoTTY container to",
	)
	gottyReattachCmd.Flags().StringVarP(&o.ToolsGoTTY.ContainerName,
		"name",
		"",
		o.ToolsGoTTY.ContainerName,
		"name of the GoTTY container (defaults to gotty-<labname>)",
	)
	gottyReattachCmd.Flags().UintVarP(&o.ToolsGoTTY.Port,
		"port",
		"p",
		o.ToolsGoTTY.Port,
		"port for GoTTY web terminal",
	)
	gottyReattachCmd.Flags().StringVarP(&o.ToolsGoTTY.Username,
		"username",
		"u",
		o.ToolsGoTTY.Username,
		"username for GoTTY web terminal authentication",
	)
	gottyReattachCmd.Flags().StringVarP(&o.ToolsGoTTY.Password,
		"password",
		"P",
		o.ToolsGoTTY.Password,
		"password for GoTTY web terminal authentication",
	)
	gottyReattachCmd.Flags().StringVarP(&o.ToolsGoTTY.Shell,
		"shell",
		"s",
		o.ToolsGoTTY.Shell,
		"shell to use for GoTTY web terminal",
	)
	gottyReattachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Image,
		"image",
		"i",
		o.ToolsGoTTY.Image,
		"container image to use for GoTTY",
	)
	gottyReattachCmd.Flags().StringVarP(
		&o.ToolsGoTTY.Owner,
		"owner",
		"o",
		o.ToolsGoTTY.Owner,
		"lab owner name for the GoTTY container",
	)

	return c, nil
}

// NewGoTTYNode creates a new GoTTY node configuration.
func NewGoTTYNode(
	name,
	image,
	network string,
	port uint,
	username,
	password,
	shell string,
	labels map[string]string,
) *GoTTYNode {
	log.Debugf(
		"Creating GoTTYNode: name=%s, image=%s, network=%s, port=%d, username=%s, shell=%s",
		name, image, network, port, username, shell,
	)

	// Create gotty startup command exactly matching the working manual example
	gottyCmd := fmt.Sprintf(
		`gotty-service start %d %s %s %s && tail -f /var/log/gotty/gotty-%d.log`,
		port, username, password, shell, port,
	)

	_, gid, _ := clabutils.GetRealUserIDs()

	// user `user` is a sudo user in srl-labs/network-multitool
	userName := "root"

	// Create port bindings for the GoTTY web interface
	portStr := fmt.Sprintf("%d/tcp", port)
	portBindings := nat.PortMap{
		nat.Port(portStr): []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: fmt.Sprintf("%d", port),
			},
		},
	}

	// Create port set
	portSet := nat.PortSet{
		nat.Port(portStr): struct{}{},
	}

	nodeConfig := &clabtypes.NodeConfig{
		LongName:     name,
		ShortName:    name,
		Image:        image,
		Entrypoint:   "",
		Cmd:          "sh -c '" + gottyCmd + "'",
		MgmtNet:      network,
		Labels:       labels,
		User:         userName,
		Group:        strconv.Itoa(gid), // gid is set to current user's gid to ensure
		PortBindings: portBindings,
		PortSet:      portSet,
	}

	return &GoTTYNode{
		config: nodeConfig,
	}
}

func (n *GoTTYNode) Config() *clabtypes.NodeConfig {
	return n.config
}

func (n *GoTTYNode) GetEndpoints() []clablinks.Endpoint {
	return nil
}

// Simplified version of getGoTTYStatus.
func getGoTTYStatus(
	ctx context.Context,
	rt clabruntime.ContainerRuntime,
	containerName string,
	port uint,
) (running bool, webURL string) {
	// Pass the port parameter to the status command
	statusCmd := fmt.Sprintf("gotty-service status %d", port)

	execCmd, err := clabexec.NewExecCmdFromString(statusCmd)
	if err != nil {
		log.Debugf("Failed to create exec cmd: %v", err)
		return false, ""
	}

	execResult, err := rt.Exec(ctx, containerName, execCmd)
	if err != nil {
		log.Debugf("Failed to execute command: %v", err)
		return false, ""
	}

	output := execResult.GetStdOutString()

	log.Debugf("GoTTY status output for port %d: %s", port, output)

	return strings.Contains(output, "GoTTY service is running"), fmt.Sprintf("http://HOST_IP:%d", port)
}

func gottyAttach(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
	ctx := cobraCmd.Context()

	log.Debugf(
		"gotty attach called with flags: labName='%s', containerName='%s', port=%d, "+
			"username='%s', password='%s', shell='%s', image='%s', topo='%s'",
		o.Global.TopologyName,
		o.ToolsGoTTY.ContainerName,
		o.ToolsGoTTY.Port,
		o.ToolsGoTTY.Username,
		o.ToolsGoTTY.Password,
		o.ToolsGoTTY.Shell,
		o.ToolsGoTTY.Image,
		o.Global.TopologyFile,
	)

	// Get lab topology information
	clabInstance, err := clabcore.NewClabFromTopologyFileOrLabName(
		o.Global.TopologyFile,
		o.Global.TopologyName,
		o.Global.VarsFile,
		o.Global.Runtime,
		o.Global.DebugCount > 0,
		o.Global.Timeout,
		o.Global.GracefulShutdown,
	)
	if err != nil {
		return err
	}

	labName := clabInstance.Config.Name
	networkName := clabInstance.Config.Mgmt.Network

	if networkName == "" {
		networkName = "clab-" + labName
	}

	// Set container name if not provided
	if o.ToolsGoTTY.ContainerName == "" {
		o.ToolsGoTTY.ContainerName = fmt.Sprintf("clab-%s-gotty", labName)
		log.Debugf("Container name not provided, generated name: %s", o.ToolsGoTTY.ContainerName)
	}

	// Initialize runtime
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer for '%s': %w", o.Global.Runtime, err)
	}

	rt := rinit()

	err = rt.Init(
		clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}),
		clabruntime.WithMgmtNet(&clabtypes.MgmtNet{Network: networkName}),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Check if container already exists
	filter := []*clabtypes.GenericFilter{{FilterType: "name", Match: o.ToolsGoTTY.ContainerName}}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) > 0 {
		return fmt.Errorf("container %s already exists", o.ToolsGoTTY.ContainerName)
	}

	// Pull the container image
	log.Infof("Pulling image %s...", o.ToolsGoTTY.Image)

	if err := rt.PullImage(ctx, o.ToolsGoTTY.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsGoTTY.Image, err)
	}

	// Create container labels
	owner := o.ToolsGoTTY.Owner
	if owner == "" {
		owner = clabutils.GetOwner()
	}

	labelsMap := createLabelsMap(
		clabInstance.TopoPaths.TopologyFilenameAbsPath(),
		labName,
		o.ToolsGoTTY.ContainerName,
		owner,
		gotty,
	)

	// Create and start GoTTY container
	log.Infof("Creating GoTTY container %s on network '%s'", o.ToolsGoTTY.ContainerName, networkName)
	gottyNode := NewGoTTYNode(
		o.ToolsGoTTY.ContainerName,
		o.ToolsGoTTY.Image,
		networkName,
		o.ToolsGoTTY.Port,
		o.ToolsGoTTY.Username,
		o.ToolsGoTTY.Password,
		o.ToolsGoTTY.Shell,
		labelsMap,
	)

	id, err := rt.CreateContainer(ctx, gottyNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create GoTTY container: %w", err)
	}

	if _, err := rt.StartContainer(ctx, id, gottyNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsGoTTY.ContainerName)
		return fmt.Errorf("failed to start GoTTY container: %w", err)
	}

	log.Infof(
		"GoTTY container %s started. Waiting for GoTTY service to initialize...",
		o.ToolsGoTTY.ContainerName,
	)

	// Wait for GoTTY service with retries
	var running bool

	var webURL string

	maxRetries := 3

	for i := range maxRetries {
		time.Sleep(gottyWaitTime)

		running, webURL = getGoTTYStatus(ctx, rt, o.ToolsGoTTY.ContainerName, o.ToolsGoTTY.Port)
		if running {
			break
		}

		log.Debugf("Waiting for GoTTY service (attempt %d/%d)...", i+1, maxRetries)
	}

	if !running {
		log.Warnf("GoTTY container started but service may not be running.")
		log.Warnf("Check the container logs: docker logs %s", o.ToolsGoTTY.ContainerName)

		return nil
	}

	log.Info("GoTTY web terminal successfully started",
		"url", webURL,
		"username", o.ToolsGoTTY.Username,
		"password", o.ToolsGoTTY.Password,
		"note", fmt.Sprintf(
			"From the web terminal, you can connect to lab nodes using SSH: ssh admin@clab-%s-<node-name>",
			labName,
		))

	return nil
}

func gottyDetach(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	// Get lab topology information
	clabInstance, err := clabcore.NewClabFromTopologyFileOrLabName(
		o.Global.TopologyFile,
		o.Global.TopologyName,
		o.Global.VarsFile,
		o.Global.Runtime,
		o.Global.DebugCount > 0,
		o.Global.Timeout,
		o.Global.GracefulShutdown,
	)
	if err != nil {
		return err
	}

	labName := clabInstance.Config.Name
	if clabInstance.TopoPaths != nil && clabInstance.TopoPaths.TopologyFileIsSet() {
		o.Global.TopologyFile = clabInstance.TopoPaths.TopologyFilenameAbsPath()
	}

	// Form the container name
	containerName := fmt.Sprintf("clab-%s-gotty", labName)
	log.Debugf("Container name for deletion: %s", containerName)

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

	log.Infof("Removing GoTTY container %s", containerName)

	if err := rt.DeleteContainer(ctx, containerName); err != nil {
		return fmt.Errorf("failed to remove GoTTY container: %w", err)
	}

	log.Infof("GoTTY container %s removed successfully", containerName)

	return nil
}

func gottyList(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
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

	// Filter only by GoTTY label
	filter := []*clabtypes.GenericFilter{
		{
			FilterType: "label",
			Field:      clabconstants.ToolType,
			Operator:   "=",
			Match:      gotty,
		},
	}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		if o.ToolsGoTTY.Format == clabconstants.FormatJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No active GoTTY containers found")
		}

		return nil
	}

	// Process containers and format output
	listItems := make([]GoTTYListItem, 0, len(containers))
	for idx := range containers {
		name := strings.TrimPrefix(containers[idx].Names[0], "/")

		network := containers[idx].NetworkName
		if network == "" {
			network = "unknown"
		}

		// Get owner from container labels
		owner := clabconstants.NotApplicable

		ownerVal, exists := containers[idx].Labels[clabconstants.Owner]
		if exists && ownerVal != "" {
			owner = ownerVal
		}

		// Get port from container
		port := o.ToolsGoTTY.Port // Default port

		if len(containers[idx].Ports) > 0 {
			for _, p := range containers[idx].Ports {
				if p.HostPort != 0 {
					port = uint(p.HostPort)
					break
				}
			}
		}

		// Try to get the GoTTY status if container is running
		webURL := clabconstants.NotApplicable

		if containers[idx].State == "running" {
			running, url := getGoTTYStatus(ctx, rt, name, port)
			if running && url != "" {
				webURL = url
			} else if containers[idx].NetworkSettings.IPv4addr != "" {
				webURL = fmt.Sprintf("http://%s:%d", containers[idx].NetworkSettings.IPv4addr, port)
			}
		}

		listItems = append(listItems, GoTTYListItem{
			Name:        name,
			Network:     network,
			State:       containers[idx].State,
			IPv4Address: containers[idx].NetworkSettings.IPv4addr,
			Port:        port,
			WebURL:      webURL,
			Owner:       owner,
		})
	}

	// Output based on format
	if o.ToolsGoTTY.Format == clabconstants.FormatJSON {
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

		t.AppendHeader(
			table.Row{"NAME", "NETWORK", "STATUS", "IPv4 ADDRESS", "PORT", "WEB URL", "OWNER"},
		)

		for _, item := range listItems {
			t.AppendRow(table.Row{
				item.Name,
				item.Network,
				item.State,
				item.IPv4Address,
				item.Port,
				item.WebURL,
				item.Owner,
			})
		}

		t.Render()
	}

	return nil
}

func gottyReattach(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
	ctx := cobraCmd.Context()

	log.Debugf(
		"gotty reattach called with flags: labName='%s', containerName='%s', "+
			"port=%d, username='%s', password='%s', shell='%s', image='%s', topo='%s'",
		o.Global.TopologyName,
		o.ToolsGoTTY.ContainerName,
		o.ToolsGoTTY.Port,
		o.ToolsGoTTY.Username,
		o.ToolsGoTTY.Password,
		o.ToolsGoTTY.Shell,
		o.ToolsGoTTY.Image,
		o.Global.TopologyFile,
	)

	// Get lab topology information
	clabInstance, err := clabcore.NewClabFromTopologyFileOrLabName(
		o.Global.TopologyFile,
		o.Global.TopologyName,
		o.Global.VarsFile,
		o.Global.Runtime,
		o.Global.DebugCount > 0,
		o.Global.Timeout,
		o.Global.GracefulShutdown,
	)
	if err != nil {
		return err
	}

	labName := clabInstance.Config.Name

	networkName := clabInstance.Config.Mgmt.Network
	if networkName == "" {
		networkName = "clab-" + labName
	}

	if clabInstance.TopoPaths != nil && clabInstance.TopoPaths.TopologyFileIsSet() {
		o.Global.TopologyFile = clabInstance.TopoPaths.TopologyFilenameAbsPath()
	}

	// Set container name if not provided
	if o.ToolsGoTTY.ContainerName == "" {
		o.ToolsGoTTY.ContainerName = fmt.Sprintf("clab-%s-gotty", labName)
		log.Debugf("Container name not provided, generated name: %s", o.ToolsGoTTY.ContainerName)
	}

	// Initialize runtime
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer for '%s': %w", o.Global.Runtime, err)
	}

	rt := rinit()

	err = rt.Init(
		clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}),
		clabruntime.WithMgmtNet(&clabtypes.MgmtNet{Network: networkName}),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Step 1: Detach (remove) existing GoTTY container if it exists
	log.Infof("Removing existing GoTTY container %s if present...", o.ToolsGoTTY.ContainerName)

	err = rt.DeleteContainer(ctx, o.ToolsGoTTY.ContainerName)
	if err != nil {
		// Just log the error but continue - the container might not exist
		log.Debugf(
			"Could not remove container %s: %v. This is normal if it doesn't exist.",
			o.ToolsGoTTY.ContainerName,
			err,
		)
	} else {
		log.Infof("Successfully removed existing GoTTY container")
	}

	// Step 2: Create and attach new GoTTY container
	// Pull the container image
	log.Infof("Pulling image %s...", o.ToolsGoTTY.Image)

	if err := rt.PullImage(ctx, o.ToolsGoTTY.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsGoTTY.Image, err)
	}

	// Create container labels
	owner := o.ToolsGoTTY.Owner
	if owner == "" {
		owner = clabutils.GetOwner()
	}

	labelsMap := createLabelsMap(
		clabInstance.TopoPaths.TopologyFilenameAbsPath(),
		labName,
		o.ToolsGoTTY.ContainerName,
		owner,
		gotty,
	)

	// Create and start GoTTY container
	log.Infof(
		"Creating new GoTTY container %s on network '%s'",
		o.ToolsGoTTY.ContainerName,
		networkName,
	)

	gottyNode := NewGoTTYNode(
		o.ToolsGoTTY.ContainerName,
		o.ToolsGoTTY.Image,
		networkName,
		o.ToolsGoTTY.Port,
		o.ToolsGoTTY.Username,
		o.ToolsGoTTY.Password,
		o.ToolsGoTTY.Shell,
		labelsMap,
	)

	id, err := rt.CreateContainer(ctx, gottyNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create GoTTY container: %w", err)
	}

	if _, err := rt.StartContainer(ctx, id, gottyNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsGoTTY.ContainerName)
		return fmt.Errorf("failed to start GoTTY container: %w", err)
	}

	log.Infof(
		"GoTTY container %s started. Waiting for GoTTY service to initialize...",
		o.ToolsGoTTY.ContainerName,
	)

	time.Sleep(gottyWaitTime)

	// Get GoTTY status
	running, webURL := getGoTTYStatus(ctx, rt, o.ToolsGoTTY.ContainerName, o.ToolsGoTTY.Port)
	if !running {
		// Use direct formatting to avoid the %s issue
		log.Warnf("GoTTY container started but service may not be running.")
		log.Warnf("Check the container logs: docker logs %s", o.ToolsGoTTY.ContainerName)

		return nil
	}

	// Get container IP if webURL is empty
	if webURL == "" {
		filter := []*clabtypes.GenericFilter{{
			FilterType: "name", Match: o.ToolsGoTTY.ContainerName,
		}}

		containers, err := rt.ListContainers(ctx, filter)
		if err == nil && len(containers) > 0 {
			webURL = fmt.Sprintf(
				"http://%s:%d",
				containers[0].NetworkSettings.IPv4addr,
				o.ToolsGoTTY.Port,
			)
		}
	}

	log.Info("GoTTY web terminal successfully reattached",
		"url", webURL,
		"username", o.ToolsGoTTY.Username,
		"password", o.ToolsGoTTY.Password,
		"note", fmt.Sprintf(
			"From the web terminal, you can connect to lab nodes using SSH: ssh admin@clab-%s-<node-name>",
			labName,
		))

	return nil
}
