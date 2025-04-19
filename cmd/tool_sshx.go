// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	sshxNetworkName   string
	sshxContainerName string
	sshxEnableReaders bool
	sshxImage         string
	sshxNetworkID     string // Added to store the full network ID
)

func init() {
	toolsCmd.AddCommand(sshxCmd)
	sshxCmd.AddCommand(sshxAttachCmd)
	sshxCmd.AddCommand(sshxDetachCmd)
	sshxCmd.AddCommand(sshxListCmd)

	// Attach command flags
	sshxAttachCmd.Flags().StringVarP(&sshxNetworkName, "network", "n", "clab", "name of the network to attach SSHX container to")
	sshxAttachCmd.Flags().StringVarP(&sshxNetworkID, "network-id", "", "", "ID of the network to attach SSHX container to (takes precedence over network name)")
	sshxAttachCmd.Flags().StringVarP(&sshxContainerName, "name", "", "", "name of the SSHX container (defaults to sshx-<network>)")
	sshxAttachCmd.Flags().BoolVarP(&sshxEnableReaders, "enable-readers", "o", true, "enable read-only access links")
	sshxAttachCmd.Flags().StringVarP(&sshxImage, "image", "i", "ghcr.io/srl-labs/network-multitool", "container image to use for SSHX")

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

func NewSSHXNode(name, image, network string, enableReaders bool) *SSHXNode {
	log.Debugf("Creating SSHXNode config: name=%s, image=%s, network=%s, enableReaders=%t", name, image, network, enableReaders)

	enableReadersFlag := ""
	if enableReaders {
		enableReadersFlag = "--enable-readers"
	}

	sshxCmdStr := fmt.Sprintf(
		`ash -c "curl -sSf https://sshx.io/get | sh > /dev/null ; sshx -q %s > /tmp/sshx & while [ ! -s /tmp/sshx ]; do sleep 1; done && cat /tmp/sshx"`,
		enableReadersFlag,
	)

	// Create labels that match containerlab's format
	labels := map[string]string{
		"containerlab":   "sshx-tool",
		"clab-node-name": name,
		"tool-type":      "sshx",
	}

	// Create node config
	nodeConfig := &types.NodeConfig{
		LongName:   name,
		ShortName:  name,
		Image:      image,
		Entrypoint: "ash",
		Cmd:        "-c " + sshxCmdStr,
		MgmtNet:    network, // The network reference
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

var sshxAttachCmd = &cobra.Command{
	Use:     "attach",
	Short:   "attach SSHX terminal sharing to a lab network",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Add some logging for flags
		log.Debugf("sshx attach called with flags: networkName='%s', containerName='%s', enableReaders=%t, image='%s'",
			sshxNetworkName, sshxContainerName, sshxEnableReaders, sshxImage)

		// Generate the container name if not specified
		if sshxContainerName == "" {
			netName := sshxNetworkName
			// Strip "clab-" prefix if present for cleaner container naming
			netName = strings.Replace(netName, "clab-", "", 1)
			sshxContainerName = fmt.Sprintf("sshx-%s", netName)
			log.Debugf("Container name not provided, generated name: %s", sshxContainerName)
		}

		// Initialize the runtime, **passing the network name**
		// rt, err := initRuntime() // Old call
		rt, err := initRuntime(sshxNetworkName) // ** New call **
		if err != nil {
			return err
		}
		log.Debugf("Runtime initialized: %s", rt.GetName())

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

		log.Infof("Using network name '%s'", sshxNetworkName)                                       // Added log
		log.Infof("Creating SSHX container %s on network '%s'", sshxContainerName, sshxNetworkName) // Added quotes

		// Create a proper Node implementation for SSHX
		log.Debugf("Creating SSHXNode config: name=%s, image=%s, network=%s, enableReaders=%t",
			sshxContainerName, sshxImage, sshxNetworkName, sshxEnableReaders) // Added log
		sshxNode := NewSSHXNode(sshxContainerName, sshxImage, sshxNetworkName, sshxEnableReaders)

		// Create the container
		id, err := rt.CreateContainer(ctx, sshxNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create SSHX container: %w", err)
		}
		log.Debugf("Container %s created with ID: %s", sshxContainerName, id)

		// Start the container
		_, err = rt.StartContainer(ctx, id, sshxNode)
		if err != nil {
			// Attempt to clean up the created container if start fails
			log.Debugf("Removing container due to start error: %s", sshxContainerName) // Added log
			delErr := rt.DeleteContainer(ctx, sshxContainerName)
			if delErr != nil {
				log.Warnf("Failed to clean up container %s after start error: %v", sshxContainerName, delErr)
			}
			return fmt.Errorf("failed to start SSHX container: %w", err)
		}
		log.Debugf("Container %s started successfully", sshxContainerName)

		log.Infof("SSHX container %s started. Waiting for SSHX link...", sshxContainerName)
		time.Sleep(5 * time.Second)

		// Try to retrieve the link
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

				// Parse and display reader link if enabled
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

		fmt.Println("SSHX container started, but link not found.")
		fmt.Printf("Check logs: docker logs %s\n", sshxContainerName)
		return nil
	},
}

var sshxDetachCmd = &cobra.Command{
	Use:     "detach",
	Short:   "detach SSHX terminal sharing from a lab network",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Generate the container name if not specified
		if sshxContainerName == "" {
			netName := sshxNetworkName
			netName = strings.Replace(netName, "clab-", "", 1)
			sshxContainerName = fmt.Sprintf("sshx-%s", netName)
		}

		// Initialize the runtime, **passing the network name**
		// rt, err := initRuntime() // Old call
		rt, err := initRuntime(sshxNetworkName) // ** Updated call **
		if err != nil {
			return err
		}

		// Check if container exists
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

		// Delete the container
		err = rt.DeleteContainer(ctx, sshxContainerName)
		if err != nil {
			return fmt.Errorf("failed to remove SSHX container: %w", err)
		}

		log.Infof("SSHX container %s removed successfully", sshxContainerName)
		return nil
	},
}

var sshxListCmd = &cobra.Command{
	Use:   "list",
	Short: "list active SSHX containers",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Initialize the runtime, **passing the network name**
		// The network name isn't strictly required for listing by label,
		// but we pass it for consistency with the updated function signature.
		// rt, err := initRuntime() // Old call
		rt, err := initRuntime(sshxNetworkName) // ** Updated call **
		if err != nil {
			return err
		}

		// Filter for SSHX containers using the label
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
			fmt.Println("No active SSHX containers found")
			return nil
		}

		// Create a table writer
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "NAME\tNETWORK\tSTATUS\tIPv4 ADDRESS\tLINK")

		for _, c := range containers {
			name := c.Names[0]
			if strings.HasPrefix(name, "/") {
				name = name[1:] // Remove leading slash if present
			}

			// Get network name - this would be in a label or we can infer from container name
			network := "unknown"
			// If container name follows sshx-<network> pattern, extract network
			if strings.HasPrefix(name, "sshx-") {
				network = strings.TrimPrefix(name, "sshx-")
			}

			// Try to get the SSHX link if container is running
			link := "N/A"
			if c.State == "running" {
				execCmd, err := exec.NewExecCmdFromString("cat /tmp/sshx")
				if err == nil {
					execResult, err := rt.Exec(ctx, name, execCmd)
					// Check both errors and return code
					if err == nil && execResult != nil && execResult.GetReturnCode() == 0 {
						link = strings.TrimSpace(execResult.GetStdOutString())
					} else if err != nil {
						log.Debugf("Error executing 'cat /tmp/sshx' in %s: %v", name, err)
					} else if execResult != nil {
						log.Debugf("'cat /tmp/sshx' in %s exited with code %d", name, execResult.GetReturnCode())
					}
				} else {
					log.Debugf("Error creating exec command for %s: %v", name, err)
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				name,
				network,
				c.State,
				c.NetworkSettings.IPv4addr, // Assuming this gets populated correctly by ListContainers now
				link,
			)
		}
		w.Flush()
		return nil
	},
}

// initRuntime initializes the container runtime specified by the user
func initRuntime(networkName string) (runtime.ContainerRuntime, error) {
	_, rinit, err := clab.RuntimeInitializer(common.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime initializer for '%s': %w", common.Runtime, err)
	}

	rt := rinit()

	// Create a minimal MgmtNet struct for the tool context
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
		// Pass the management network config using WithMgmtNet
		runtime.WithMgmtNet(mgmtNet),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize runtime: %w", err)
	}

	return rt, nil
}
