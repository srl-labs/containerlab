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
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/cmd/common"
	clabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

// Configuration variables for the GoTTY commands
var (
	gottyLabName       string
	gottyContainerName string
	gottyPort          int
	gottyUsername      string
	gottyPassword      string
	gottyShell         string
	gottyImage         string
	gottyOutputFormat  string
	gottyOwner         string
)

// GoTTYListItem defines the structure for GoTTY container info in JSON output
type GoTTYListItem struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	State       string `json:"state"`
	IPv4Address string `json:"ipv4_address"`
	Port        int    `json:"port"`
	WebURL      string `json:"web_url"`
	Owner       string `json:"owner"`
}

// GoTTYNode implements runtime.Node interface for GoTTY containers
type GoTTYNode struct {
	config *types.NodeConfig
}

func init() {
	toolsCmd.AddCommand(gottyCmd)
	gottyCmd.AddCommand(gottyAttachCmd)
	gottyCmd.AddCommand(gottyDetachCmd)
	gottyCmd.AddCommand(gottyListCmd)
	gottyCmd.AddCommand(gottyReattachCmd)

	gottyCmd.PersistentFlags().StringVarP(&gottyOutputFormat, "format", "f", "table",
		"output format for 'list' command (table, json)")

	// Attach command flags
	gottyAttachCmd.Flags().StringVarP(&gottyLabName, "lab", "l", "",
		"name of the lab to attach GoTTY container to")
	gottyAttachCmd.Flags().StringVarP(&gottyContainerName, "name", "", "",
		"name of the GoTTY container (defaults to gotty-<labname>)")
	gottyAttachCmd.Flags().IntVarP(&gottyPort, "port", "p", 8080,
		"port for GoTTY web terminal")
	gottyAttachCmd.Flags().StringVarP(&gottyUsername, "username", "u", "admin",
		"username for GoTTY web terminal authentication")
	gottyAttachCmd.Flags().StringVarP(&gottyPassword, "password", "P", "admin",
		"password for GoTTY web terminal authentication")
	gottyAttachCmd.Flags().StringVarP(&gottyShell, "shell", "s", "bash",
		"shell to use for GoTTY web terminal")
	gottyAttachCmd.Flags().StringVarP(&gottyImage, "image", "i", "ghcr.io/srl-labs/network-multitool",
		"container image to use for GoTTY")
	gottyAttachCmd.Flags().StringVarP(&gottyOwner, "owner", "o", "",
		"lab owner name for the GoTTY container")

	// Detach command flags
	gottyDetachCmd.Flags().StringVarP(&gottyLabName, "lab", "l", "",
		"name of the lab where GoTTY container is attached")

	// Reattach command flags
	gottyReattachCmd.Flags().StringVarP(&gottyLabName, "lab", "l", "",
		"name of the lab to reattach GoTTY container to")
	gottyReattachCmd.Flags().StringVarP(&gottyContainerName, "name", "", "",
		"name of the GoTTY container (defaults to gotty-<labname>)")
	gottyReattachCmd.Flags().IntVarP(&gottyPort, "port", "p", 8080,
		"port for GoTTY web terminal")
	gottyReattachCmd.Flags().StringVarP(&gottyUsername, "username", "u", "admin",
		"username for GoTTY web terminal authentication")
	gottyReattachCmd.Flags().StringVarP(&gottyPassword, "password", "P", "admin",
		"password for GoTTY web terminal authentication")
	gottyReattachCmd.Flags().StringVarP(&gottyShell, "shell", "s", "bash",
		"shell to use for GoTTY web terminal")
	gottyReattachCmd.Flags().StringVarP(&gottyImage, "image", "i", "ghcr.io/srl-labs/network-multitool",
		"container image to use for GoTTY")
	gottyReattachCmd.Flags().StringVarP(&gottyOwner, "owner", "o", "",
		"lab owner name for the GoTTY container")
}

// gottyCmd represents the gotty command container
var gottyCmd = &cobra.Command{
	Use:   "gotty",
	Short: "GoTTY web terminal operations",
	Long:  "Attach or detach GoTTY web terminal containers to labs",
}

// NewGoTTYNode creates a new GoTTY node configuration
func NewGoTTYNode(name, image, network string, port int, username, password, shell string, labels map[string]string) *GoTTYNode {
	log.Debugf("Creating GoTTYNode: name=%s, image=%s, network=%s, port=%d, username=%s, shell=%s",
		name, image, network, port, username, shell)

	// Create gotty startup command exactly matching the working manual example
	gottyCmd := fmt.Sprintf(
		`gotty-service start %d %s %s %s && tail -f /var/log/gotty/gotty-%d.log`,
		port, username, password, shell, port,
	)

	_, gid, _ := utils.GetRealUserIDs()

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

	nodeConfig := &types.NodeConfig{
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

func (n *GoTTYNode) Config() *types.NodeConfig {
	return n.config
}

func (n *GoTTYNode) GetEndpoints() []links.Endpoint {
	return nil
}

// Simplified version of getGoTTYStatus
func getGoTTYStatus(ctx context.Context, rt runtime.ContainerRuntime, containerName string, port int) (bool, string) {
	// Pass the port parameter to the status command
	statusCmd := fmt.Sprintf("gotty-service status %d", port)
	execCmd, err := exec.NewExecCmdFromString(statusCmd)
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

	// Check if service is running based on output
	running := strings.Contains(output, "GoTTY service is running")

	// Simply return a fixed format URL with HOST_IP placeholder
	webURL := fmt.Sprintf("http://HOST_IP:%d", port)

	return running, webURL
}

// gottyAttachCmd attaches GoTTY web terminal to a lab
var gottyAttachCmd = &cobra.Command{
	Use:     "attach",
	Short:   "attach GoTTY web terminal to a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("gotty attach called with flags: labName='%s', containerName='%s', port=%d, username='%s', password='%s', shell='%s', image='%s', topo='%s'",
			gottyLabName, gottyContainerName, gottyPort, gottyUsername, gottyPassword, gottyShell, gottyImage, common.Topo)

		// Get lab name and network
		labName, networkName, _, err := common.GetLabConfig(ctx, gottyLabName)
		if err != nil {
			return err
		}

		// Set container name if not provided
		if gottyContainerName == "" {
			gottyContainerName = fmt.Sprintf("clab-%s-gotty", labName)
			log.Debugf("Container name not provided, generated name: %s", gottyContainerName)
		}

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(common.Runtime)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
		}

		rt := rinit()
		err = rt.Init(
			runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}),
			runtime.WithMgmtNet(&types.MgmtNet{Network: networkName}),
		)
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Check if container already exists
		filter := []*types.GenericFilter{{FilterType: "name", Match: gottyContainerName}}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) > 0 {
			return fmt.Errorf("container %s already exists", gottyContainerName)
		}

		// Pull the container image
		log.Infof("Pulling image %s...", gottyImage)
		if err := rt.PullImage(ctx, gottyImage, types.PullPolicyAlways); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", gottyImage, err)
		}

		// Create container labels
		owner := utils.GetOwner(gottyOwner)
		labelsMap := common.CreateLabels(labName, gottyContainerName, owner, "gotty")

		// Create and start GoTTY container
		log.Infof("Creating GoTTY container %s on network '%s'", gottyContainerName, networkName)
		gottyNode := NewGoTTYNode(gottyContainerName, gottyImage, networkName, gottyPort, gottyUsername, gottyPassword, gottyShell, labelsMap)

		id, err := rt.CreateContainer(ctx, gottyNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create GoTTY container: %w", err)
		}

		if _, err := rt.StartContainer(ctx, id, gottyNode); err != nil {
			// Clean up on failure
			rt.DeleteContainer(ctx, gottyContainerName)
			return fmt.Errorf("failed to start GoTTY container: %w", err)
		}

		log.Infof("GoTTY container %s started. Waiting for GoTTY service to initialize...", gottyContainerName)

		// Wait for GoTTY service with retries
		var running bool
		var webURL string
		maxRetries := 3

		for i := 0; i < maxRetries; i++ {
			time.Sleep(3 * time.Second)
			running, webURL = getGoTTYStatus(ctx, rt, gottyContainerName, gottyPort)
			if running {
				break
			}
			log.Debugf("Waiting for GoTTY service (attempt %d/%d)...", i+1, maxRetries)
		}

		if !running {
			log.Warnf("GoTTY container started but service may not be running.")
			log.Warnf("Check the container logs: docker logs %s", gottyContainerName)
			return nil
		}

		log.Info("GoTTY web terminal successfully started",
			"url", webURL,
			"username", gottyUsername,
			"password", gottyPassword,
			"note", fmt.Sprintf("From the web terminal, you can connect to lab nodes using SSH: ssh admin@clab-%s-<node-name>", labName))

		return nil
	},
}

// gottyDetachCmd detaches GoTTY web terminal from a lab
var gottyDetachCmd = &cobra.Command{
	Use:     "detach",
	Short:   "detach GoTTY web terminal from a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Get lab name
		labName, _, _, err := common.GetLabConfig(ctx, gottyLabName)
		if err != nil {
			return err
		}

		// Form the container name
		containerName := fmt.Sprintf("clab-%s-gotty", labName)
		log.Debugf("Container name for deletion: %s", containerName)

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(common.Runtime)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer: %w", err)
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		log.Infof("Removing GoTTY container %s", containerName)
		if err := rt.DeleteContainer(ctx, containerName); err != nil {
			return fmt.Errorf("failed to remove GoTTY container: %w", err)
		}

		log.Infof("GoTTY container %s removed successfully", containerName)
		return nil
	},
}

// gottyListCmd lists active GoTTY containers
var gottyListCmd = &cobra.Command{
	Use:   "list",
	Short: "list active GoTTY containers",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(common.Runtime)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer: %w", err)
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Filter only by GoTTY label
		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      "tool-type",
				Operator:   "=",
				Match:      "gotty",
			},
		}

		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) == 0 {
			if gottyOutputFormat == "json" {
				fmt.Println("[]")
			} else {
				fmt.Println("No active GoTTY containers found")
			}
			return nil
		}

		// Process containers and format output
		listItems := make([]GoTTYListItem, 0, len(containers))
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")
			network := c.NetworkName
			if network == "" {
				network = "unknown"
			}

			// Get owner from container labels
			owner := "N/A"
			if ownerVal, exists := c.Labels[clabels.Owner]; exists && ownerVal != "" {
				owner = ownerVal
			}

			// Get port from container
			port := gottyPort // Default port
			if len(c.Ports) > 0 {
				for _, p := range c.Ports {
					if p.HostPort != 0 {
						port = p.HostPort
						break
					}
				}
			}

			// Try to get the GoTTY status if container is running
			webURL := "N/A"
			if c.State == "running" {
				running, url := getGoTTYStatus(ctx, rt, name, port)
				if running && url != "" {
					webURL = url
				} else if c.NetworkSettings.IPv4addr != "" {
					webURL = fmt.Sprintf("http://%s:%d", c.NetworkSettings.IPv4addr, port)
				}
			}

			listItems = append(listItems, GoTTYListItem{
				Name:        name,
				Network:     network,
				State:       c.State,
				IPv4Address: c.NetworkSettings.IPv4addr,
				Port:        port,
				WebURL:      webURL,
				Owner:       owner,
			})
		}

		// Output based on format
		if gottyOutputFormat == "json" {
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

			t.AppendHeader(table.Row{"NAME", "NETWORK", "STATUS", "IPv4 ADDRESS", "PORT", "WEB URL", "OWNER"})

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
	},
}

var gottyReattachCmd = &cobra.Command{
	Use:     "reattach",
	Short:   "detach and reattach GoTTY web terminal to a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("gotty reattach called with flags: labName='%s', containerName='%s', port=%d, username='%s', password='%s', shell='%s', image='%s', topo='%s'",
			gottyLabName, gottyContainerName, gottyPort, gottyUsername, gottyPassword, gottyShell, gottyImage, common.Topo)

		// Get lab name and network
		labName, networkName, _, err := common.GetLabConfig(ctx, gottyLabName)
		if err != nil {
			return err
		}

		// Set container name if not provided
		if gottyContainerName == "" {
			gottyContainerName = fmt.Sprintf("clab-%s-gotty", labName)
			log.Debugf("Container name not provided, generated name: %s", gottyContainerName)
		}

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(common.Runtime)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
		}

		rt := rinit()
		err = rt.Init(
			runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}),
			runtime.WithMgmtNet(&types.MgmtNet{Network: networkName}),
		)
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Step 1: Detach (remove) existing GoTTY container if it exists
		log.Infof("Removing existing GoTTY container %s if present...", gottyContainerName)
		err = rt.DeleteContainer(ctx, gottyContainerName)
		if err != nil {
			// Just log the error but continue - the container might not exist
			log.Debugf("Could not remove container %s: %v. This is normal if it doesn't exist.", gottyContainerName, err)
		} else {
			log.Infof("Successfully removed existing GoTTY container")
		}

		// Step 2: Create and attach new GoTTY container
		// Pull the container image
		log.Infof("Pulling image %s...", gottyImage)
		if err := rt.PullImage(ctx, gottyImage, types.PullPolicyAlways); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", gottyImage, err)
		}

		// Create container labels
		owner := utils.GetOwner(gottyOwner)
		labelsMap := common.CreateLabels(labName, gottyContainerName, owner, "gotty")

		// Create and start GoTTY container
		log.Infof("Creating new GoTTY container %s on network '%s'", gottyContainerName, networkName)
		gottyNode := NewGoTTYNode(gottyContainerName, gottyImage, networkName, gottyPort, gottyUsername, gottyPassword, gottyShell, labelsMap)

		id, err := rt.CreateContainer(ctx, gottyNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create GoTTY container: %w", err)
		}

		if _, err := rt.StartContainer(ctx, id, gottyNode); err != nil {
			// Clean up on failure
			rt.DeleteContainer(ctx, gottyContainerName)
			return fmt.Errorf("failed to start GoTTY container: %w", err)
		}

		log.Infof("GoTTY container %s started. Waiting for GoTTY service to initialize...", gottyContainerName)
		time.Sleep(5 * time.Second)

		// Get GoTTY status
		running, webURL := getGoTTYStatus(ctx, rt, gottyContainerName, gottyPort)
		if !running {
			// Use direct formatting to avoid the %s issue
			log.Warnf("GoTTY container started but service may not be running.")
			log.Warnf("Check the container logs: docker logs %s", gottyContainerName)
			return nil
		}

		// Get container IP if webURL is empty
		if webURL == "" {
			filter := []*types.GenericFilter{{FilterType: "name", Match: gottyContainerName}}
			containers, err := rt.ListContainers(ctx, filter)
			if err == nil && len(containers) > 0 {
				webURL = fmt.Sprintf("http://%s:%d", containers[0].NetworkSettings.IPv4addr, gottyPort)
			}
		}

		log.Info("GoTTY web terminal successfully reattached",
			"url", webURL,
			"username", gottyUsername,
			"password", gottyPassword,
			"note", fmt.Sprintf("From the web terminal, you can connect to lab nodes using SSH: ssh admin@clab-%s-<node-name>", labName))

		return nil
	},
}
