// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
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

// Configuration variables for the SSHX commands
var (
	sshxLabName       string
	sshxContainerName string
	sshxEnableReaders bool
	sshxImage         string
	outputFormat      string
	sshxOwner         string
	sshxMountSSHDir   bool // New flag to control SSH directory mounting
)

// SSHXListItem defines the structure for SSHX container info in JSON output
type SSHXListItem struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	State       string `json:"state"`
	IPv4Address string `json:"ipv4_address"`
	Link        string `json:"link"`
	Owner       string `json:"owner"`
}

// SSHXNode implements runtime.Node interface for SSHX containers
type SSHXNode struct {
	config *types.NodeConfig
}

func init() {
	toolsCmd.AddCommand(sshxCmd)
	sshxCmd.AddCommand(sshxAttachCmd)
	sshxCmd.AddCommand(sshxDetachCmd)
	sshxCmd.AddCommand(sshxListCmd)

	sshxCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "table",
		"output format for 'list' command (table, json)")

	// Attach command flags
	sshxAttachCmd.Flags().StringVarP(&sshxLabName, "lab", "l", "",
		"name of the lab to attach SSHX container to")
	sshxAttachCmd.Flags().StringVarP(&sshxContainerName, "name", "", "",
		"name of the SSHX container (defaults to sshx-<labname>)")
	sshxAttachCmd.Flags().BoolVarP(&sshxEnableReaders, "enable-readers", "w", false,
		"enable read-only access links")
	sshxAttachCmd.Flags().StringVarP(&sshxImage, "image", "i", "ghcr.io/srl-labs/network-multitool",
		"container image to use for SSHX")
	sshxAttachCmd.Flags().StringVarP(&sshxOwner, "owner", "o", "",
		"lab owner name for the SSHX container")
	sshxAttachCmd.Flags().BoolVarP(&sshxMountSSHDir, "expose-ssh", "s", false,
		"mount host user's SSH directory (~/.ssh) to the sshx container")

	// Detach command flags
	sshxDetachCmd.Flags().StringVarP(&sshxLabName, "lab", "l", "",
		"name of the lab where SSHX container is attached")
}

// sshxCmd represents the sshx command container
var sshxCmd = &cobra.Command{
	Use:   "sshx",
	Short: "SSHX terminal sharing operations",
	Long:  "Attach or detach SSHX terminal sharing containers to labs",
}

// NewSSHXNode creates a new SSHX node configuration
func NewSSHXNode(name, image, network string, enableReaders bool, labels map[string]string, mountSSH bool) *SSHXNode {
	log.Debugf("Creating SSHXNode: name=%s, image=%s, network=%s, enableReaders=%t, exposeSSH=%t",
		name, image, network, enableReaders, mountSSH)

	enableReadersFlag := ""
	if enableReaders {
		enableReadersFlag = "--enable-readers"
	}

	sshxCmd := fmt.Sprintf(
		`curl -sSf https://sshx.io/get | sh > /dev/null ; sshx -q %s > /tmp/sshx & while [ ! -s /tmp/sshx ]; do sleep 1; done && cat /tmp/sshx ; sleep infinity`,
		enableReadersFlag,
	)

	_, gid, _ := utils.GetRealUserIDs()

	// user `user` is a sudo user in srl-labs/network-multitool
	userName := "user"

	nodeConfig := &types.NodeConfig{
		LongName:   name,
		ShortName:  name,
		Image:      image,
		Entrypoint: "",
		Cmd:        "ash -c '" + sshxCmd + "'",
		MgmtNet:    network,
		Labels:     labels,
		User:       userName,
		Group:      strconv.Itoa(gid), // gid is set to current user's gid to ensure
	}

	// Add SSH directory mount if enabled
	if mountSSH {
		// Get user's home directory
		sshDir := utils.ExpandHome("~/.ssh")
		// Check if the directory exists
		if _, err := os.Stat(sshDir); err == nil {
			nodeConfig.Binds = append(nodeConfig.Binds, fmt.Sprintf("%s:/home/%s/.ssh:ro", sshDir, userName))
			log.Debugf("Mounting SSH directory: %s -> /home/%s/.ssh", sshDir, userName)
		} else {
			log.Warnf("User's SSH directory not found at %s, skipping mount", sshDir)
		}

	}

	return &SSHXNode{
		config: nodeConfig,
	}
}

func (n *SSHXNode) Config() *types.NodeConfig {
	return n.config
}

func (n *SSHXNode) GetEndpoints() []links.Endpoint {
	return nil
}

// getLabConfig gets lab configuration and returns lab name, network name and containerlab instance
func getLabConfig(ctx context.Context) (string, string, *clab.CLab, error) {
	var labName string
	var c *clab.CLab
	var err error

	// If lab name is provided directly, use it
	if sshxLabName != "" {
		labName = sshxLabName
	}

	// If topo file is provided or discovered
	if common.Topo == "" && labName == "" {
		// Auto-discover topology files in current directory
		cwd, err := os.Getwd()
		if err == nil {
			entries, err := os.ReadDir(cwd)
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".clab.yml") {
						// Found a topology file
						common.Topo = filepath.Join(cwd, entry.Name())
						log.Debugf("Found topology file: %s", common.Topo)
						break
					}
				}
			}
		}
	}

	// If we have lab name but no topo file, try to find it from containers
	if labName != "" && common.Topo == "" {
		_, rinit, err := clab.RuntimeInitializer(common.Runtime)
		if err != nil {
			return "", "", nil, err
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return "", "", nil, err
		}

		// Find containers for this lab
		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      "containerlab",
				Operator:   "=",
				Match:      labName,
			},
		}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return "", "", nil, err
		}

		if len(containers) == 0 {
			return "", "", nil, fmt.Errorf("lab '%s' not found - no running containers", labName)
		}

		// Get topo file from container labels
		topoFile := containers[0].Labels["clab-topo-file"]
		if topoFile == "" {
			return "", "", nil, fmt.Errorf("could not determine topology file from container labels")
		}

		log.Debugf("Found topology file for lab %s: %s", labName, topoFile)
		common.Topo = topoFile
	}

	// Create a single containerlab instance
	opts := []clab.ClabOption{
		clab.WithTimeout(common.Timeout),
		clab.WithRuntime(common.Runtime, &runtime.RuntimeConfig{
			Debug:            common.Debug,
			Timeout:          common.Timeout,
			GracefulShutdown: common.Graceful,
		}),
		clab.WithDebug(common.Debug),
	}

	if common.Topo != "" {
		opts = append(opts, clab.WithTopoPath(common.Topo, common.VarsFile))
	} else {
		return "", "", nil, fmt.Errorf("no topology file found or provided")
	}

	c, err = clab.NewContainerLab(opts...)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create containerlab instance: %w", err)
	}

	if c.Config == nil {
		return "", "", nil, fmt.Errorf("failed to load lab configuration")
	}

	// Get lab name if not provided
	if labName == "" {
		labName = c.Config.Name
	}

	// Get network name
	networkName := c.Config.Mgmt.Network
	if networkName == "" {
		networkName = "clab-" + c.Config.Name
	}

	return labName, networkName, c, nil
}

// getOwner gets owner name from flag or environment variables
func getOwner() string {
	if sshxOwner != "" {
		return sshxOwner
	}

	if owner := os.Getenv("SUDO_USER"); owner != "" {
		return owner
	}

	return os.Getenv("USER")
}

// createLabels creates container labels
func createLabels(labName, containerName, owner string) map[string]string {
	shortName := strings.Replace(containerName, "clab-"+labName+"-", "", 1)

	labels := map[string]string{
		"containerlab":       labName,
		"clab-node-name":     shortName,
		"clab-node-longname": containerName,
		"clab-node-kind":     "linux",
		"clab-node-group":    "",
		"clab-node-type":     "tool",
		"tool-type":          "sshx",
	}

	// Add topology file path
	if common.Topo != "" {
		absPath, err := filepath.Abs(common.Topo)
		if err == nil {
			labels["clab-topo-file"] = absPath
		} else {
			labels["clab-topo-file"] = common.Topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(common.Topo)
		labels["clab-node-lab-dir"] = filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[clabels.Owner] = owner
	}

	return labels
}

// getSSHXLink retrieves the SSHX link from the container
func getSSHXLink(ctx context.Context, rt runtime.ContainerRuntime, containerName string) string {
	execCmd, err := exec.NewExecCmdFromString("cat /tmp/sshx")
	if err != nil {
		return ""
	}

	execResult, err := rt.Exec(ctx, containerName, execCmd)
	if err != nil || execResult.GetReturnCode() != 0 {
		return ""
	}

	link := strings.TrimSpace(execResult.GetStdOutString())
	if !strings.Contains(link, "https://sshx.io/") {
		return ""
	}

	return link
}

// sshxAttachCmd attaches SSHX terminal sharing to a lab
var sshxAttachCmd = &cobra.Command{
	Use:     "attach",
	Short:   "attach SSHX terminal sharing to a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("sshx attach called with flags: labName='%s', containerName='%s', enableReaders=%t, image='%s', topo='%s', exposeSSH=%t",
			sshxLabName, sshxContainerName, sshxEnableReaders, sshxImage, common.Topo, sshxMountSSHDir)

		// Get lab name and network
		labName, networkName, _, err := getLabConfig(ctx)
		if err != nil {
			return err
		}

		// Set container name if not provided
		if sshxContainerName == "" {
			sshxContainerName = fmt.Sprintf("clab-%s-sshx", labName)
			log.Debugf("Container name not provided, generated name: %s", sshxContainerName)
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
		filter := []*types.GenericFilter{{FilterType: "name", Match: sshxContainerName}}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) > 0 {
			return fmt.Errorf("container %s already exists", sshxContainerName)
		}

		// Pull the container image
		log.Infof("Pulling image %s...", sshxImage)
		if err := rt.PullImage(ctx, sshxImage, types.PullPolicyIfNotPresent); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", sshxImage, err)
		}

		// Create container labels
		owner := getOwner()
		labelsMap := createLabels(labName, sshxContainerName, owner)

		// Create and start SSHX container
		log.Infof("Creating SSHX container %s on network '%s'", sshxContainerName, networkName)
		sshxNode := NewSSHXNode(sshxContainerName, sshxImage, networkName, sshxEnableReaders, labelsMap, sshxMountSSHDir)

		id, err := rt.CreateContainer(ctx, sshxNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create SSHX container: %w", err)
		}

		if _, err := rt.StartContainer(ctx, id, sshxNode); err != nil {
			// Clean up on failure
			rt.DeleteContainer(ctx, sshxContainerName)
			return fmt.Errorf("failed to start SSHX container: %w", err)
		}

		log.Infof("SSHX container %s started. Waiting for SSHX link...", sshxContainerName)
		time.Sleep(5 * time.Second)

		// Get SSHX link
		link := getSSHXLink(ctx, rt, sshxContainerName)
		if link == "" {
			log.Warn("SSHX container started but failed to retrieve the link.\nCheck the container logs: docker logs %s", sshxContainerName)
			return nil
		}

		log.Info("SSHX successfully started", "link", link, "note", fmt.Sprintf("Inside the shared terminal, you can connect to lab nodes using SSH:\nssh admin@clab-%s-<node-name>", labName))

		// Display read-only link if enabled
		if sshxEnableReaders {
			parts := strings.Split(link, "#")
			if len(parts) > 1 {
				accessParts := strings.Split(parts[1], ",")
				if len(accessParts) > 1 {
					readerLink := fmt.Sprintf("%s#%s", parts[0], accessParts[0])
					fmt.Println("\nRead-only access link:")
					fmt.Println(readerLink)
				}
			}
		}

		if sshxMountSSHDir {
			fmt.Println("\nYour SSH keys and configuration have been mounted to allow direct authentication.")
		}

		return nil
	},
}

// sshxDetachCmd detaches SSHX terminal sharing from a lab
var sshxDetachCmd = &cobra.Command{
	Use:     "detach",
	Short:   "detach SSHX terminal sharing from a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Get lab name
		labName, _, _, err := getLabConfig(ctx)
		if err != nil {
			return err
		}

		// Form the container name
		containerName := fmt.Sprintf("clab-%s-sshx", labName)
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

		log.Infof("Removing SSHX container %s", containerName)
		if err := rt.DeleteContainer(ctx, containerName); err != nil {
			return fmt.Errorf("failed to remove SSHX container: %w", err)
		}

		log.Infof("SSHX container %s removed successfully", containerName)
		return nil
	},
}

// sshxListCmd lists active SSHX containers
var sshxListCmd = &cobra.Command{
	Use:   "list",
	Short: "list active SSHX containers",
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

		// Filter only by SSHX label
		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      "tool-type",
				Operator:   "=",
				Match:      "sshx",
			},
		}

		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) == 0 {
			if outputFormat == "json" {
				fmt.Println("[]")
			} else {
				fmt.Println("No active SSHX containers found")
			}
			return nil
		}

		// Process containers and format output
		listItems := make([]SSHXListItem, 0, len(containers))
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

			// Try to get the SSHX link if container is running
			link := "N/A"
			if c.State == "running" {
				if linkContent := getSSHXLink(ctx, rt, name); linkContent != "" {
					link = linkContent
				}
			}

			listItems = append(listItems, SSHXListItem{
				Name:        name,
				Network:     network,
				State:       c.State,
				IPv4Address: c.NetworkSettings.IPv4addr,
				Link:        link,
				Owner:       owner,
			})
		}

		// Output based on format
		if outputFormat == "json" {
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

			t.AppendHeader(table.Row{"NAME", "NETWORK", "STATUS", "IPv4 ADDRESS", "LINK", "OWNER"})

			for _, item := range listItems {
				t.AppendRow(table.Row{
					item.Name,
					item.Network,
					item.State,
					item.IPv4Address,
					item.Link,
					item.Owner,
				})
			}
			t.Render()
		}

		return nil
	},
}
