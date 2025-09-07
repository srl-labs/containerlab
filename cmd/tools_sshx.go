// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

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
	sshx         = "sshx"
	sshxWaitTime = 5 * time.Second
)

// SSHXListItem defines the structure for SSHX container info in JSON output.
type SSHXListItem struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	State       string `json:"state"`
	IPv4Address string `json:"ipv4_address"`
	Link        string `json:"link"`
	Owner       string `json:"owner"`
}

// SSHXNode implements runtime.Node interface for SSHX containers.
type SSHXNode struct {
	config *clabtypes.NodeConfig
}

func sshxCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   sshx,
		Short: "SSHX terminal sharing operations",
		Long:  "Attach or detach SSHX terminal sharing containers to labs",
	}

	sshxListCmd := &cobra.Command{
		Use:   "list",
		Short: "list active SSHX containers",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return sshxList(cobraCmd, o)
		},
	}

	c.AddCommand(sshxListCmd)

	c.PersistentFlags().StringVarP(
		&o.ToolsSSHX.Format,
		"format",
		"f",
		o.ToolsSSHX.Format,
		"output format for 'list' command (table, json)",
	)

	sshxAttachCmd := &cobra.Command{
		Use:   "attach",
		Short: "attach SSHX terminal sharing to a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return sshxAttach(cobraCmd, o)
		},
	}

	c.AddCommand(sshxAttachCmd)

	sshxAttachCmd.Flags().StringVarP(
		&o.Global.TopologyName,
		"lab",
		"l",
		o.Global.TopologyName,
		"name of the lab to attach SSHX container to",
	)
	sshxAttachCmd.Flags().StringVarP(
		&o.ToolsSSHX.ContainerName,
		"name",
		"",
		o.ToolsSSHX.ContainerName,
		"name of the SSHX container (defaults to sshx-<labname>)",
	)
	sshxAttachCmd.Flags().BoolVarP(
		&o.ToolsSSHX.EnableReaders,
		"enable-readers",
		"w",
		o.ToolsSSHX.EnableReaders,
		"enable read-only access links",
	)
	sshxAttachCmd.Flags().StringVarP(
		&o.ToolsSSHX.Image,
		"image",
		"i",
		o.ToolsSSHX.Image,
		"container image to use for SSHX",
	)
	sshxAttachCmd.Flags().StringVarP(
		&o.ToolsSSHX.Owner,
		"owner",
		"o",
		o.ToolsSSHX.Owner,
		"lab owner name for the SSHX container",
	)
	sshxAttachCmd.Flags().BoolVarP(
		&o.ToolsSSHX.MountSSHDir,
		"expose-ssh",
		"s",
		o.ToolsSSHX.MountSSHDir,
		"mount host user's SSH directory (~/.ssh) to the sshx container",
	)

	sshxDetachCmd := &cobra.Command{
		Use:   "detach",
		Short: "detach SSHX terminal sharing from a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return sshxDetach(cobraCmd, o)
		},
	}

	c.AddCommand(sshxDetachCmd)

	sshxDetachCmd.Flags().StringVarP(&o.Global.TopologyName, "lab", "l", o.Global.TopologyName,
		"name of the lab where SSHX container is attached")

	sshxReattachCmd := &cobra.Command{
		Use:   "reattach",
		Short: "detach and reattach SSHX terminal sharing to a lab",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return sshxReattach(cobraCmd, o)
		},
	}

	c.AddCommand(sshxReattachCmd)

	sshxReattachCmd.Flags().StringVarP(
		&o.Global.TopologyName,
		"lab",
		"l", o.Global.TopologyName,
		"name of the lab to reattach SSHX container to",
	)
	sshxReattachCmd.Flags().StringVarP(
		&o.ToolsSSHX.ContainerName,
		"name",
		"",
		o.ToolsSSHX.ContainerName,
		"name of the SSHX container (defaults to sshx-<labname>)",
	)
	sshxReattachCmd.Flags().BoolVarP(
		&o.ToolsSSHX.EnableReaders,
		"enable-readers",
		"w",
		o.ToolsSSHX.EnableReaders,
		"enable read-only access links",
	)
	sshxReattachCmd.Flags().StringVarP(
		&o.ToolsSSHX.Image,
		"image",
		"i",
		o.ToolsSSHX.Image,
		"container image to use for SSHX",
	)
	sshxReattachCmd.Flags().StringVarP(
		&o.ToolsSSHX.Owner,
		"owner",
		"o",
		o.ToolsSSHX.Owner,
		"lab owner name for the SSHX container",
	)
	sshxReattachCmd.Flags().BoolVarP(
		&o.ToolsSSHX.MountSSHDir,
		"expose-ssh",
		"s",
		o.ToolsSSHX.MountSSHDir,
		"mount host user's SSH directory (~/.ssh) to the sshx container",
	)

	return c, nil
}

// NewSSHXNode creates a new SSHX node configuration.
func NewSSHXNode(
	name,
	image,
	network,
	labName string,
	enableReaders bool,
	labels map[string]string, mountSSH bool,
) *SSHXNode {
	log.Debugf("Creating SSHXNode: name=%s, image=%s, network=%s, enableReaders=%t, exposeSSH=%t",
		name, image, network, enableReaders, mountSSH)

	enableReadersFlag := ""
	if enableReaders {
		enableReadersFlag = "--enable-readers"
	}

	sshxCmd := fmt.Sprintf(
		"curl -sSf https://sshx.io/get | sh > /dev/null "+
			"; sshx -q %s > /tmp/sshx & while [ ! -s /tmp/sshx ]; do sleep 1; done "+
			"&& cat /tmp/sshx ; sleep infinity",
		enableReadersFlag,
	)

	_, gid, _ := clabutils.GetRealUserIDs()

	// user `admin` is a sudo user in srl-labs/network-multitool
	userName := "admin"

	nodeConfig := &clabtypes.NodeConfig{
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
		sshDir := clabutils.ExpandHome("~/.ssh")
		// Check if the directory exists
		if _, err := os.Stat(sshDir); err == nil {
			nodeConfig.Binds = append(nodeConfig.Binds,
				fmt.Sprintf("%s:/home/%s/.ssh:ro", sshDir, userName))
			log.Debugf("Mounting SSH directory: %s -> /home/%s/.ssh", sshDir, userName)
		} else {
			log.Warnf("User's SSH directory not found at %s, skipping mount", sshDir)
		}
	}

	// mount lab ssh config
	labSSHConfFile := fmt.Sprintf("/etc/ssh/ssh_config.d/clab-%s.conf", labName)
	if _, err := os.Stat(labSSHConfFile); err == nil {
		nodeConfig.Binds = append(nodeConfig.Binds,
			fmt.Sprintf("%s:/%s:ro", labSSHConfFile, labSSHConfFile))
		log.Debugf("Mounting SSH directory: %s -> %s", labSSHConfFile, labSSHConfFile)
	} else {
		log.Warnf("Lab's SSH config file not found at %s, skipping the mount", labSSHConfFile)
	}

	return &SSHXNode{
		config: nodeConfig,
	}
}

func (n *SSHXNode) Config() *clabtypes.NodeConfig {
	return n.config
}

func (*SSHXNode) GetEndpoints() []clablinks.Endpoint {
	return nil
}

// getSSHXLink retrieves the SSHX link from the container.
func getSSHXLink(
	ctx context.Context,
	rt clabruntime.ContainerRuntime,
	containerName string,
) string {
	execCmd, err := clabexec.NewExecCmdFromString("cat /tmp/sshx")
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

func sshxAttach(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
	ctx := cobraCmd.Context()

	log.Debugf(
		"sshx attach called with flags: labName='%s', containerName='%s', "+
			"enableReaders=%t, image='%s', topo='%s', exposeSSH=%t",
		o.Global.TopologyName,
		o.ToolsSSHX.ContainerName,
		o.ToolsSSHX.EnableReaders,
		o.ToolsSSHX.Image,
		o.Global.TopologyFile,
		o.ToolsSSHX.MountSSHDir,
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
	if o.ToolsSSHX.ContainerName == "" {
		o.ToolsSSHX.ContainerName = fmt.Sprintf("clab-%s-sshx", labName)
		log.Debugf("Container name not provided, generated name: %s", o.ToolsSSHX.ContainerName)
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
	filter := []*clabtypes.GenericFilter{{FilterType: "name", Match: o.ToolsSSHX.ContainerName}}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) > 0 {
		return fmt.Errorf("container %s already exists", o.ToolsSSHX.ContainerName)
	}

	// Pull the container image
	log.Infof("Pulling image %s...", o.ToolsSSHX.Image)

	if err := rt.PullImage(ctx, o.ToolsSSHX.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsSSHX.Image, err)
	}

	// Create container labels
	owner := o.ToolsSSHX.Owner
	if owner == "" {
		owner = clabutils.GetOwner()
	}

	labelsMap := createLabelsMap(
		clabInstance.TopoPaths.TopologyFilenameAbsPath(),
		labName,
		o.ToolsSSHX.ContainerName,
		owner,
		sshx,
	)

	log.Infof("Creating SSHX container %s on network '%s'", o.ToolsSSHX.ContainerName, networkName)

	sshxNode := NewSSHXNode(
		o.ToolsSSHX.ContainerName,
		o.ToolsSSHX.Image,
		networkName,
		labName,
		o.ToolsSSHX.EnableReaders,
		labelsMap,
		o.ToolsSSHX.MountSSHDir,
	)

	id, err := rt.CreateContainer(ctx, sshxNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create SSHX container: %w", err)
	}

	if _, err := rt.StartContainer(ctx, id, sshxNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsSSHX.ContainerName)

		return fmt.Errorf("failed to start SSHX container: %w", err)
	}

	log.Infof("SSHX container %s started. Waiting for SSHX link...", o.ToolsSSHX.ContainerName)
	time.Sleep(sshxWaitTime)

	// Get SSHX link
	link := getSSHXLink(ctx, rt, o.ToolsSSHX.ContainerName)
	if link == "" {
		log.Warn(
			"SSHX container started but failed to retrieve the link.\n"+
				"Check the container logs: docker logs %s",
			o.ToolsSSHX.ContainerName,
		)

		return nil
	}

	log.Info("SSHX successfully started", "link", link, "note",
		fmt.Sprintf(
			"Inside the shared terminal, you can connect to lab nodes using SSH:\n"+
				"ssh admin@clab-%s-<node-name>",
			labName,
		),
	)

	// Display read-only link if enabled
	if o.ToolsSSHX.EnableReaders {
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

	if o.ToolsSSHX.MountSSHDir {
		fmt.Println(
			"\nYour SSH keys and configuration have been mounted to allow direct authentication.",
		)
	}

	return nil
}

func sshxDetach(cobraCmd *cobra.Command, o *Options) error {
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
	containerName := fmt.Sprintf("clab-%s-sshx", labName)
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

	log.Infof("Removing SSHX container %s", containerName)

	if err := rt.DeleteContainer(ctx, containerName); err != nil {
		return fmt.Errorf("failed to remove SSHX container: %w", err)
	}

	log.Infof("SSHX container %s removed successfully", containerName)

	return nil
}

func sshxList(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
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

	// Filter only by SSHX label
	filter := []*clabtypes.GenericFilter{
		{
			FilterType: "label",
			Field:      clabconstants.ToolType,
			Operator:   "=",
			Match:      sshx,
		},
	}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		if o.ToolsSSHX.Format == clabconstants.FormatJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No active SSHX containers found")
		}

		return nil
	}

	// Process containers and format output
	listItems := make([]SSHXListItem, 0, len(containers))
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

		// Try to get the SSHX link if container is running
		link := clabconstants.NotApplicable

		if containers[idx].State == "running" {
			if linkContent := getSSHXLink(ctx, rt, name); linkContent != "" {
				link = linkContent
			}
		}

		listItems = append(listItems, SSHXListItem{
			Name:        name,
			Network:     network,
			State:       containers[idx].State,
			IPv4Address: containers[idx].NetworkSettings.IPv4addr,
			Link:        link,
			Owner:       owner,
		})
	}

	// Output based on format
	if o.ToolsSSHX.Format == clabconstants.FormatJSON {
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
}

func sshxReattach(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
	ctx := cobraCmd.Context()

	log.Debugf(
		"sshx reattach called with flags: labName='%s', containerName='%s', "+
			"enableReaders=%t, image='%s', topo='%s', exposeSSH=%t",
		o.Global.TopologyName,
		o.ToolsSSHX.ContainerName,
		o.ToolsSSHX.EnableReaders,
		o.ToolsSSHX.Image,
		o.Global.TopologyFile,
		o.ToolsSSHX.MountSSHDir,
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
	if o.ToolsSSHX.ContainerName == "" {
		o.ToolsSSHX.ContainerName = fmt.Sprintf("clab-%s-sshx", labName)
		log.Debugf("Container name not provided, generated name: %s", o.ToolsSSHX.ContainerName)
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

	// Step 1: Detach (remove) existing SSHX container if it exists
	log.Infof("Removing existing SSHX container %s if present...", o.ToolsSSHX.ContainerName)

	err = rt.DeleteContainer(ctx, o.ToolsSSHX.ContainerName)
	if err != nil {
		// Just log the error but continue - the container might not exist
		log.Debugf(
			"Could not remove container %s: %v. This is normal if it doesn't exist.",
			o.ToolsSSHX.ContainerName,
			err,
		)
	} else {
		log.Infof("Successfully removed existing SSHX container")
	}

	// Step 2: Create and attach new SSHX container
	// Pull the container image
	log.Infof("Pulling image %s...", o.ToolsSSHX.Image)

	if err := rt.PullImage(ctx, o.ToolsSSHX.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsSSHX.Image, err)
	}

	// Create container labels
	owner := o.ToolsSSHX.Owner
	if owner == "" {
		owner = clabutils.GetOwner()
	}

	labelsMap := createLabelsMap(
		clabInstance.TopoPaths.TopologyFilenameAbsPath(),
		labName,
		o.ToolsSSHX.ContainerName,
		owner,
		sshx,
	)

	// Create and start SSHX container
	log.Infof(
		"Creating new SSHX container %s on network '%s'",
		o.ToolsSSHX.ContainerName,
		networkName,
	)
	sshxNode := NewSSHXNode(
		o.ToolsSSHX.ContainerName,
		o.ToolsSSHX.Image,
		networkName,
		labName,
		o.ToolsSSHX.EnableReaders,
		labelsMap,
		o.ToolsSSHX.MountSSHDir,
	)

	id, err := rt.CreateContainer(ctx, sshxNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create SSHX container: %w", err)
	}

	if _, err := rt.StartContainer(ctx, id, sshxNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsSSHX.ContainerName)

		return fmt.Errorf("failed to start SSHX container: %w", err)
	}

	log.Infof("SSHX container %s started. Waiting for SSHX link...", o.ToolsSSHX.ContainerName)
	time.Sleep(sshxWaitTime)

	// Get SSHX link
	link := getSSHXLink(ctx, rt, o.ToolsSSHX.ContainerName)
	if link == "" {
		log.Warn(
			"SSHX container started but failed to retrieve the link.\n"+
				"Check the container logs: docker logs %s",
			o.ToolsSSHX.ContainerName,
		)

		return nil
	}

	log.Info(
		"SSHX successfully reattached",
		"link",
		link,
		"note",
		fmt.Sprintf(
			"Inside the shared terminal, you can connect to lab nodes using SSH:"+
				"\nssh admin@clab-%s-<node-name>", labName,
		),
	)

	// Display read-only link if enabled
	if o.ToolsSSHX.EnableReaders {
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

	if o.ToolsSSHX.MountSSHDir {
		fmt.Println(
			"\nYour SSH keys and configuration have been mounted to allow direct authentication.",
		)
	}

	return nil
}
