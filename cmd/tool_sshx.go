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
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	sshxNetworkName   string
	sshxContainerName string
	sshxEnableReaders bool
	sshxImage         string
	outputFormat      string
	sshxOwner         string
)

// Struct ONLY for list JSON output
type SSHXListItem struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	State       string `json:"state"`
	IPv4Address string `json:"ipv4_address"`
	Link        string `json:"link"`
	Owner       string `json:"owner"`
}

func init() {
	toolsCmd.AddCommand(sshxCmd)
	sshxCmd.AddCommand(sshxAttachCmd)
	sshxCmd.AddCommand(sshxDetachCmd)
	sshxCmd.AddCommand(sshxListCmd)

	sshxCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "table", "output format for 'list' command (table, json)")

	// Attach command flags
	sshxAttachCmd.Flags().StringVarP(&sshxNetworkName, "network", "n", "clab", "name of the network to attach SSHX container to")
	sshxAttachCmd.Flags().StringVarP(&sshxContainerName, "name", "", "", "name of the SSHX container (defaults to sshx-<network>)")
	sshxAttachCmd.Flags().BoolVarP(&sshxEnableReaders, "enable-readers", "w", false, "enable read-only access links")
	sshxAttachCmd.Flags().StringVarP(&sshxImage, "image", "i", "ghcr.io/srl-labs/network-multitool", "container image to use for SSHX")
	sshxAttachCmd.Flags().StringVarP(&sshxOwner, "owner", "o", "", "lab owner name for the SSHX container")

	// Detach command flags
	sshxDetachCmd.Flags().StringVarP(&sshxNetworkName, "network", "n", "clab", "name of the network where SSHX container is attached")
	sshxDetachCmd.Flags().StringVarP(&sshxContainerName, "name", "", "", "name of the SSHX container to detach")
}

// sshxCmd represents the sshx command container.
var sshxCmd = &cobra.Command{
	Use:   "sshx",
	Short: "SSHX terminal sharing operations",
	Long:  "Attach or detach SSHX terminal sharing containers to lab networks",
}

// SSHXNode implements runtime.Node interface for SSHX containers
type SSHXNode struct {
	config *types.NodeConfig
}

// NewSSHXNode
func NewSSHXNode(name, image, network string, enableReaders bool, labels map[string]string) *SSHXNode {
	log.Debugf("Creating SSHXNode config: name=%s, image=%s, network=%s, enableReaders=%t",
		name, image, network, enableReaders)

	enableReadersFlag := ""
	if enableReaders {
		enableReadersFlag = "--enable-readers"
	}

	sshxScript := fmt.Sprintf(
		`curl -sSf https://sshx.io/get | sh > /dev/null ; sshx -q %s > /tmp/sshx & while [ ! -s /tmp/sshx ]; do sleep 1; done && cat /tmp/sshx ; sleep infinity`,
		enableReadersFlag,
	)

	nodeConfig := &types.NodeConfig{
		LongName:   name,
		ShortName:  name,
		Image:      image,
		Entrypoint: "",
		Cmd:        "ash -c '" + sshxScript + "'",
		MgmtNet:    network,
		Labels:     labels,
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

// sshxAttachCmd
var sshxAttachCmd = &cobra.Command{
	Use:     "attach",
	Short:   "attach SSHX terminal sharing to a lab network",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("sshx attach called with flags: networkName='%s', containerName='%s', enableReaders=%t, image='%s', topo='%s', owner='%s'",
			sshxNetworkName, sshxContainerName, sshxEnableReaders, sshxImage, common.Topo, sshxOwner)

		ownerToSet := sshxOwner

		if ownerToSet == "" { // If the --owner flag was NOT provided
			ownerToSet = os.Getenv("SUDO_USER")
			if ownerToSet == "" {
				ownerToSet = os.Getenv("USER")
			}

			// Handle case where neither is set
			if ownerToSet == "" {
				log.Warnf("Could not determine owner from flags or environment (SUDO_USER/USER). Owner label will not be set.")
			} else {
				log.Debugf("Determined owner from environment: %s", ownerToSet)
			}
		} else {
			log.Debugf("Using owner from --owner flag: %s", ownerToSet)
		}

		// Get the network from topo file if provided
		networkName, err := getNetworkName(ctx)
		if err != nil {
			return err
		}
		log.Debugf("Using network name: %s", networkName)

		if sshxContainerName == "" {
			netName := networkName
			netName = strings.Replace(netName, "clab-", "", 1)
			sshxContainerName = fmt.Sprintf("sshx-%s", netName)
			log.Debugf("Container name not provided, generated name: %s", sshxContainerName)
		}

		rt, err := initRuntime(networkName)
		if err != nil {
			return err
		}
		log.Debugf("Runtime initialized: %s", rt.GetName())

		filter := []*types.GenericFilter{
			{
				FilterType: "name",
				Match:      sshxContainerName,
			},
		}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) > 0 {
			return fmt.Errorf("container %s already exists", sshxContainerName)
		}

		log.Infof("Using network name '%s'", networkName)
		log.Infof("Creating SSHX container %s on network '%s'", sshxContainerName, networkName)

		// Create labels map
		labelsMap := map[string]string{
			// Use the constant from the labels package for consistency
			labels.NodeName: sshxContainerName,
			"tool-type":     "sshx",
		}

		// Add owner label if it was determined (either from flag or env)
		if ownerToSet != "" {
			// Use the constant from the labels package
			labelsMap[labels.Owner] = ownerToSet
			log.Debugf("Setting owner label to: %s", ownerToSet)
		} else {
			log.Debugf("Owner label is empty, not setting.")
		}

		log.Debugf("Creating SSHXNode with labels: %v", labelsMap)
		// Pass the final labelsMap to the NewSSHXNode function
		sshxNode := NewSSHXNode(sshxContainerName, sshxImage, networkName, sshxEnableReaders, labelsMap)

		id, err := rt.CreateContainer(ctx, sshxNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create SSHX container: %w", err)
		}
		log.Debugf("Container %s created with ID: %s", sshxContainerName, id)

		_, err = rt.StartContainer(ctx, id, sshxNode)
		if err != nil {
			log.Debugf("Removing container due to start error: %s", sshxContainerName)
			delErr := rt.DeleteContainer(ctx, sshxContainerName)
			if delErr != nil {
				log.Warnf("Failed to clean up container %s after start error: %v", sshxContainerName, delErr)
			}
			return fmt.Errorf("failed to start SSHX container: %w", err)
		}
		log.Debugf("Container %s started successfully", sshxContainerName)

		log.Infof("SSHX container %s started. Waiting for SSHX link...", sshxContainerName)
		time.Sleep(5 * time.Second)

		execCmd, err := exec.NewExecCmdFromString("cat /tmp/sshx")
		if err != nil {
			return fmt.Errorf("failed to create exec command: %w", err)
		}

		execResult, err := rt.Exec(ctx, sshxContainerName, execCmd)
		if err != nil {
			fmt.Println("SSHX container started but failed to retrieve the link.")
			fmt.Printf("Check the container logs: docker logs %s\n", sshxContainerName)
			return nil
		}

		if execResult.GetReturnCode() == 0 {
			sshxLink := strings.TrimSpace(execResult.GetStdOutString())
			if strings.Contains(sshxLink, "https://sshx.io/") {
				fmt.Println("SSHX link for collaborative terminal access:")
				fmt.Println(sshxLink)

				if sshxEnableReaders {
					parts := strings.Split(sshxLink, "#")
					if len(parts) > 1 {
						accessParts := strings.Split(parts[1], ",")
						if len(accessParts) > 1 {
							readerLink := fmt.Sprintf("%s#%s", parts[0], accessParts[0])
							fmt.Println("\nRead-only access link:")
							fmt.Println(readerLink)
						}
					}
				}
				return nil
			}
		}

		fmt.Println("SSHX container started, but link not found or invalid.")
		fmt.Printf("Check logs: docker logs %s\n", sshxContainerName)
		return nil
	},
}

// sshxDetachCmd
var sshxDetachCmd = &cobra.Command{
	Use:     "detach",
	Short:   "detach SSHX terminal sharing from a lab network",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Get the network from topo file if provided
		networkName, err := getNetworkName(ctx)
		if err != nil {
			return err
		}

		if sshxContainerName == "" {
			netName := networkName
			netName = strings.Replace(netName, "clab-", "", 1)
			sshxContainerName = fmt.Sprintf("sshx-%s", netName)
		}

		rt, err := initRuntime(networkName)
		if err != nil {
			return err
		}

		filter := []*types.GenericFilter{
			{
				FilterType: "name",
				Match:      sshxContainerName,
			},
		}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) == 0 {
			log.Infof("SSHX container %s does not exist, nothing to detach", sshxContainerName)
			return nil
		}

		log.Infof("Removing SSHX container %s", sshxContainerName)

		err = rt.DeleteContainer(ctx, sshxContainerName)
		if err != nil {
			return fmt.Errorf("failed to remove SSHX container: %w", err)
		}

		log.Infof("SSHX container %s removed successfully", sshxContainerName)
		return nil
	},
}

// sshxListCmd
var sshxListCmd = &cobra.Command{
	Use:   "list",
	Short: "list active SSHX containers",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Initialize runtime without specific network
		_, rinit, err := clab.RuntimeInitializer(common.Runtime)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
		}

		rt := rinit()
		err = rt.Init(
			runtime.WithConfig(
				&runtime.RuntimeConfig{
					Timeout: common.Timeout,
				},
			),
		)
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Filter only by SSHX label, not by network
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

		// Prepare data structure for both outputs
		var listItems []SSHXListItem

		if len(containers) == 0 {
			if outputFormat == "json" {
				// Output empty JSON array
				fmt.Println("[]")
			} else {
				fmt.Println("No active SSHX containers found")
			}
			return nil
		}

		// Populate listItems
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")

			network := c.NetworkName
			if network == "" {
				network = "unknown"
			}

			// Get owner from container labels
			owner := "N/A"
			if ownerVal, exists := c.Labels["clab-owner"]; exists && ownerVal != "" {
				owner = ownerVal
			}

			// Try to get the SSHX link if container is running
			link := "N/A"
			if c.State == "running" {
				execCmd, cmdErr := exec.NewExecCmdFromString("cat /tmp/sshx")
				if cmdErr == nil {
					execResult, execErr := rt.Exec(ctx, name, execCmd)
					// Check both errors and return code
					if execErr == nil && execResult != nil && execResult.GetReturnCode() == 0 {
						linkContent := strings.TrimSpace(execResult.GetStdOutString())
						if strings.Contains(linkContent, "https://sshx.io/") {
							link = linkContent
						} else {
							// File exists but content is invalid or empty
							link = "Error: Invalid link content"
							log.Debugf("Invalid content in /tmp/sshx for %s: %s", name, linkContent)
						}
					} else if execErr != nil {
						log.Debugf("Error executing 'cat /tmp/sshx' in %s: %v", name, execErr)
						link = "Error: Failed to exec"
					} else if execResult != nil { // execErr is nil, but return code != 0
						log.Debugf("'cat /tmp/sshx' in %s exited with code %d", name, execResult.GetReturnCode())
						link = "Error: Link file not found/ready"
					}
				} else {
					log.Debugf("Error creating exec command for %s: %v", name, cmdErr)
					link = "Error: Failed to create exec cmd"
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
				return fmt.Errorf("failed to marshal list output to JSON: %w", err)
			}
			fmt.Println(string(b))
		} else {
			// Use go-pretty table
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleRounded) // Or StyleDefault, StyleLight, etc.
			t.Style().Format.Header = text.FormatTitle
			t.Style().Options.SeparateRows = true // Add lines between rows

			// Add OWNER to the header
			t.AppendHeader(table.Row{"NAME", "NETWORK", "STATUS", "IPv4 ADDRESS", "LINK", "OWNER"})

			rows := []table.Row{}
			for _, item := range listItems {
				rows = append(rows, table.Row{
					item.Name,
					item.Network,
					item.State,
					item.IPv4Address,
					item.Link,
					item.Owner, // Add owner to the table row
				})
			}
			t.AppendRows(rows)
			t.Render()
		}

		return nil
	},
}

// getNetworkName determines which network name to use based on the presence of a topology file
// or explicit network name provided
func getNetworkName(ctx context.Context) (string, error) {
	if sshxNetworkName != "clab" {
		log.Infof("Using explicitly provided network name: %s", sshxNetworkName)
		return sshxNetworkName, nil
	}
	// Explicitly check for topology files in current directory if none provided
	if common.Topo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Debugf("Failed to get current working directory: %v", err)
		} else {
			entries, err := os.ReadDir(cwd)
			if err != nil {
				log.Debugf("Failed to read directory contents: %v", err)
			} else {
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".clab.yml") {
						// Found a topology file, explicitly set it
						common.Topo = filepath.Join(cwd, entry.Name())
						log.Debugf("Found topology file: %s", common.Topo)
						break
					}
				}
			}
		}
	}

	// Prepare options for CLab instance
	opts := []clab.ClabOption{
		clab.WithTimeout(common.Timeout),
		clab.WithRuntime(common.Runtime,
			&runtime.RuntimeConfig{
				Debug:            common.Debug,
				Timeout:          common.Timeout,
				GracefulShutdown: common.Graceful,
			},
		),
		clab.WithDebug(common.Debug),
	}

	if common.Topo != "" {
		log.Debugf("Using topology file: %s", common.Topo)
		opts = append(opts, clab.WithTopoPath(common.Topo, common.VarsFile))
	} else {
		log.Debugf("No topology file provided or found")
	}

	// Create CLab instance with options
	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		log.Debugf("Error creating containerlab instance: %v", err)
		log.Debugf("Using default network name: %s", sshxNetworkName)
		return sshxNetworkName, nil
	}

	// Get network name from topology if available
	if c.Config != nil {
		// First try to get network from mgmt configuration
		if c.Config.Mgmt.Network != "" {
			networkName := c.Config.Mgmt.Network
			log.Debugf("Using network name from topology mgmt config: %s", networkName)
			return networkName, nil
		}

		// Otherwise use the lab name
		if c.Config.Name != "" {
			networkName := "clab-" + c.Config.Name
			log.Debugf("Using lab name for network: %s", networkName)
			return networkName, nil
		}
	}

	// Fall back to default if all else fails
	log.Debugf("No topology network found, using default network: %s", sshxNetworkName)
	return sshxNetworkName, nil
}

// initRuntime
func initRuntime(networkName string) (runtime.ContainerRuntime, error) {
	_, rinit, err := clab.RuntimeInitializer(common.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
	}

	rt := rinit()

	mgmtNet := &types.MgmtNet{
		Network: networkName,
	}
	log.Debugf("Initializing runtime with MgmtNet: %+v", mgmtNet)

	err = rt.Init(
		runtime.WithConfig(
			&runtime.RuntimeConfig{
				Timeout: common.Timeout,
			},
		),
		runtime.WithMgmtNet(mgmtNet),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize runtime: %w", err)
	}

	return rt, nil
}
