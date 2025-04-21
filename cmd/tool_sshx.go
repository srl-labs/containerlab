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
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	sshxLabName       string
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
	sshxAttachCmd.Flags().StringVarP(&sshxLabName, "lab", "l", "", "name of the lab to attach SSHX container to")
	sshxAttachCmd.Flags().StringVarP(&sshxContainerName, "name", "", "", "name of the SSHX container (defaults to sshx-<labname>)")
	sshxAttachCmd.Flags().BoolVarP(&sshxEnableReaders, "enable-readers", "w", false, "enable read-only access links")
	sshxAttachCmd.Flags().StringVarP(&sshxImage, "image", "i", "ghcr.io/srl-labs/network-multitool", "container image to use for SSHX")
	sshxAttachCmd.Flags().StringVarP(&sshxOwner, "owner", "o", "", "lab owner name for the SSHX container")

	// Detach command flags
	sshxDetachCmd.Flags().StringVarP(&sshxLabName, "lab", "l", "", "name of the lab where SSHX container is attached")
}

// sshxCmd represents the sshx command container.
var sshxCmd = &cobra.Command{
	Use:   "sshx",
	Short: "SSHX terminal sharing operations",
	Long:  "Attach or detach SSHX terminal sharing containers to labs",
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
	Short:   "attach SSHX terminal sharing to a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("sshx attach called with flags: labName='%s', containerName='%s', enableReaders=%t, image='%s', topo='%s', owner='%s'",
			sshxLabName, sshxContainerName, sshxEnableReaders, sshxImage, common.Topo, sshxOwner)

		// Determine owner
		ownerToSet := sshxOwner
		if ownerToSet == "" {
			ownerToSet = os.Getenv("SUDO_USER")
			if ownerToSet == "" {
				ownerToSet = os.Getenv("USER")
			}
			if ownerToSet == "" {
				log.Warnf("Could not determine owner from flags or environment (SUDO_USER/USER). Owner label will not be set.")
			} else {
				log.Debugf("Determined owner from environment: %s", ownerToSet)
			}
		} else {
			log.Debugf("Using owner from --owner flag: %s", ownerToSet)
		}

		// Must have either a lab name or a topo file
		if sshxLabName == "" && common.Topo == "" {
			return fmt.Errorf("no lab name (-l) or topology file (-t) provided. Please specify one of these options")
		}

		var labName string
		var networkName string

		// Create containerlab instance with the topology
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

		// If lab name is provided but topo file is not, try to find running containers and get the topo file
		if sshxLabName != "" && common.Topo == "" {
			log.Debugf("Lab name provided, finding containers for lab: %s", sshxLabName)
			containers, err := listContainers(ctx, "")
			if err != nil {
				return fmt.Errorf("failed to list containers: %w", err)
			}

			// Filter containers by lab name
			var labContainers []runtime.GenericContainer
			for _, c := range containers {
				if c.Labels["containerlab"] == sshxLabName {
					labContainers = append(labContainers, c)
				}
			}

			if len(labContainers) == 0 {
				return fmt.Errorf("lab '%s' not found - no running containers for this lab", sshxLabName)
			}

			// Get topology file path from container labels
			topoFile := labContainers[0].Labels["clab-topo-file"]
			if topoFile != "" {
				log.Debugf("Found topology file for lab %s: %s", sshxLabName, topoFile)
				common.Topo = topoFile
			} else {
				return fmt.Errorf("could not determine topology file from lab containers")
			}
		}

		if common.Topo != "" {
			opts = append(opts, clab.WithTopoPath(common.Topo, common.VarsFile))
		} else {
			return fmt.Errorf("no topology file found or provided")
		}

		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return fmt.Errorf("failed to create containerlab instance: %w", err)
		}

		// Get lab and network name from topology
		if c.Config != nil {
			labName = c.Config.Name
			if c.Config.Mgmt.Network != "" {
				networkName = c.Config.Mgmt.Network
			} else {
				networkName = "clab-" + labName
			}
			log.Debugf("Using network name: %s, lab name: %s", networkName, labName)
		} else {
			return fmt.Errorf("failed to load lab configuration")
		}

		if sshxContainerName == "" {
			sshxContainerName = fmt.Sprintf("clab-%s-sshx", labName)
			log.Debugf("Container name not provided, generated name: %s", sshxContainerName)
		}

		// Initialize runtime
		rt, err := initRuntime(networkName)
		if err != nil {
			return err
		}

		// Check if container already exists
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

		// Pull the container image
		log.Infof("Pulling image %s...", sshxImage)
		err = rt.PullImage(ctx, sshxImage, types.PullPolicyIfNotPresent)
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", sshxImage, err)
		}

		// Create containerlab labels
		labelsMap := map[string]string{
			// Core containerlab labels
			"containerlab":       labName,
			"clab-node-name":     strings.Replace(sshxContainerName, "clab-"+labName+"-", "", 1),
			"clab-node-longname": sshxContainerName,
			"clab-node-kind":     "linux",
			"clab-node-group":    "",
			"clab-node-type":     "tool",
			"tool-type":          "sshx",
		}

		// Add topology file path
		if common.Topo != "" {
			absPath, err := filepath.Abs(common.Topo)
			if err == nil {
				labelsMap["clab-topo-file"] = absPath
			} else {
				labelsMap["clab-topo-file"] = common.Topo
			}
		}

		// Set node lab directory
		if common.Topo != "" {
			baseDir := filepath.Dir(common.Topo)
			labDir := filepath.Join(baseDir, "clab-"+labName,
				strings.Replace(sshxContainerName, "clab-"+labName+"-", "", 1))
			labelsMap["clab-node-lab-dir"] = labDir
		}

		// Add owner label if available
		if ownerToSet != "" {
			labelsMap["clab-owner"] = ownerToSet
		}

		// Create and start SSHX container
		log.Infof("Creating SSHX container %s on network '%s'", sshxContainerName, networkName)
		sshxNode := NewSSHXNode(sshxContainerName, sshxImage, networkName, sshxEnableReaders, labelsMap)

		id, err := rt.CreateContainer(ctx, sshxNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create SSHX container: %w", err)
		}

		_, err = rt.StartContainer(ctx, id, sshxNode)
		if err != nil {
			log.Debugf("Removing container due to start error: %s", sshxContainerName)
			delErr := rt.DeleteContainer(ctx, sshxContainerName)
			if delErr != nil {
				log.Warnf("Failed to clean up container %s after start error: %v", sshxContainerName, delErr)
			}
			return fmt.Errorf("failed to start SSHX container: %w", err)
		}

		log.Infof("SSHX container %s started. Waiting for SSHX link...", sshxContainerName)
		time.Sleep(5 * time.Second)

		// Get SSHX link
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
	Short:   "detach SSHX terminal sharing from a lab",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var labName string

		// Create containerlab instance with the topology, if needed
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

		// First try to get the lab name
		if sshxLabName != "" {
			// If lab name is provided directly, use it
			labName = sshxLabName
		} else if common.Topo != "" {
			// If topology file is provided, load it to get lab name
			opts = append(opts, clab.WithTopoPath(common.Topo, common.VarsFile))
			c, err := clab.NewContainerLab(opts...)
			if err != nil {
				return fmt.Errorf("failed to create containerlab instance: %w", err)
			}

			if c.Config != nil {
				labName = c.Config.Name
				log.Debugf("Extracted lab name from topology: %s", labName)
			} else {
				return fmt.Errorf("failed to load lab configuration from topology file")
			}
		} else {
			return fmt.Errorf("no lab name (-l) or topology file (-t) provided. Please specify one of these options")
		}

		if labName == "" {
			return fmt.Errorf("could not determine lab name")
		}

		// Form the container name
		containerName := fmt.Sprintf("clab-%s-sshx", labName)
		log.Debugf("Container name for deletion: %s", containerName)

		// Initialize runtime
		rt, err := initSimpleRuntime()
		if err != nil {
			return err
		}

		log.Infof("Removing SSHX container %s", containerName)
		err = rt.DeleteContainer(ctx, containerName)
		if err != nil {
			return fmt.Errorf("failed to remove SSHX container: %w", err)
		}

		log.Infof("SSHX container %s removed successfully", containerName)
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

		// Initialize runtime
		rt, err := initSimpleRuntime()
		if err != nil {
			return err
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
					if execErr == nil && execResult != nil && execResult.GetReturnCode() == 0 {
						linkContent := strings.TrimSpace(execResult.GetStdOutString())
						if strings.Contains(linkContent, "https://sshx.io/") {
							link = linkContent
						} else {
							link = "Error: Invalid link content"
						}
					} else if execErr != nil {
						link = "Error: Failed to exec"
					} else if execResult != nil {
						link = "Error: Link file not found/ready"
					}
				} else {
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
			t.SetStyle(table.StyleRounded)
			t.Style().Format.Header = text.FormatTitle
			t.Style().Options.SeparateRows = true

			t.AppendHeader(table.Row{"NAME", "NETWORK", "STATUS", "IPv4 ADDRESS", "LINK", "OWNER"})

			rows := []table.Row{}
			for _, item := range listItems {
				rows = append(rows, table.Row{
					item.Name,
					item.Network,
					item.State,
					item.IPv4Address,
					item.Link,
					item.Owner,
				})
			}
			t.AppendRows(rows)
			t.Render()
		}

		return nil
	},
}

// initRuntime initializes a runtime with a specific network name
func initRuntime(networkName string) (runtime.ContainerRuntime, error) {
	_, rinit, err := clab.RuntimeInitializer(common.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
	}

	rt := rinit()

	mgmtNet := &types.MgmtNet{
		Network: networkName,
	}

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

// initSimpleRuntime initializes a runtime without a specific network
func initSimpleRuntime() (runtime.ContainerRuntime, error) {
	_, rinit, err := clab.RuntimeInitializer(common.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
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
		return nil, fmt.Errorf("failed to initialize runtime: %w", err)
	}

	return rt, nil
}
