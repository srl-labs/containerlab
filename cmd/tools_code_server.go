// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/docker/go-connections/nat"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	codeServerPort       = 8080
	codeServerDirPerm    = 0o755
	codeServerConfigPerm = 0o644
	codeServerMarkerName = ".initialized"
	defaultLabsDir       = "~/.clab"
)

type codeServerPaths struct {
	dataDir       string
	configDir     string
	extensionsDir string
	userDataDir   string
	markerFile    string
	configFile    string
}

func newCodeServerPaths(homeDir, name string) codeServerPaths {
	basePath := fmt.Sprintf("%s/.clab/code-server/%s", homeDir, name)

	return codeServerPaths{
		dataDir:       fmt.Sprintf("%s/data", basePath),
		configDir:     fmt.Sprintf("%s/config", basePath),
		extensionsDir: fmt.Sprintf("%s/extensions", basePath),
		userDataDir:   fmt.Sprintf("%s/user-data", basePath),
		markerFile:    fmt.Sprintf("%s/extensions/%s", basePath, codeServerMarkerName),
		configFile:    fmt.Sprintf("%s/config/config.yaml", basePath),
	}
}

func prepareCodeServerPersistence(paths *codeServerPaths) (bool, error) {
	directories := []string{
		paths.dataDir,
		paths.configDir,
		paths.extensionsDir,
		paths.userDataDir,
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, codeServerDirPerm); err != nil {
			return false, fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}

	isFirstRun, err := ensureExtensionsInitialized(paths.markerFile)
	if err != nil {
		return false, err
	}

	if err := writeCodeServerConfig(paths.configFile); err != nil {
		return false, err
	}

	return isFirstRun, nil
}

func ensureExtensionsInitialized(markerFile string) (bool, error) {
	if _, err := os.Stat(markerFile); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check code-server marker file: %w", err)
	}

	if err := os.WriteFile(markerFile, []byte("initialized"), codeServerConfigPerm); err != nil {
		return false, fmt.Errorf("failed to create code-server marker file: %w", err)
	}

	return true, nil
}

func writeCodeServerConfig(configFile string) error {
	const configContent = `bind-addr: 0.0.0.0:8080
auth: password
password: clab
cert: false
`

	if err := os.WriteFile(configFile, []byte(configContent), codeServerConfigPerm); err != nil {
		return fmt.Errorf("failed to create code-server config file: %w", err)
	}

	return nil
}

func buildCodeServerBinds(
	homeDir string,
	runtime clabruntime.ContainerRuntime,
	paths *codeServerPaths,
) (clabtypes.Binds, error) {
	binds := clabtypes.Binds{
		clabtypes.NewBind(homeDir, "/labs", ""),
		clabtypes.NewBind("/home", "/home", ""),
		clabtypes.NewBind(paths.dataDir, "/root/.local/share/code-server", ""),
		clabtypes.NewBind(paths.configDir, "/root/.config/code-server", ""),
		clabtypes.NewBind(paths.extensionsDir, "/persistent-extensions", ""),
		clabtypes.NewBind(paths.userDataDir, "/persistent-user-data", ""),
	}

	rtSocket, err := runtime.GetRuntimeSocket()
	if err != nil {
		return nil, err
	}

	binds = append(binds, clabtypes.NewBind(rtSocket, rtSocket, ""))
	binds = append(binds, runtime.GetCooCBindMounts()...)

	rtBinPath, err := runtime.GetRuntimeBinary()
	if err != nil {
		return nil, fmt.Errorf("could not find docker binary: %v. "+
			"code-server might not function correctly if docker is not available", err)
	}

	binds = append(binds, clabtypes.NewBind(rtBinPath, "/usr/bin/docker", "ro"))

	clabPath, err := getclabBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("could not find containerlab binary: %v. "+
			"code-server might not function correctly if containerlab is not in its PATH", err)
	}

	binds = append(binds, clabtypes.NewBind(clabPath, "/usr/bin/containerlab", "ro"))

	return binds, nil
}

func buildCodeServerCommand(isFirstRun bool, defaultDir string) string {
	baseCommand := strings.Join([]string{
		"code-server --config /root/.config/code-server/config.yaml",
		"--extensions-dir /persistent-extensions",
		"--user-data-dir /persistent-user-data ", defaultDir,
	}, " ")

	if !isFirstRun {
		return fmt.Sprintf("-c %q", baseCommand)
	}

	copyExtensionsCommand := "cp -r /extensions/* /persistent-extensions/" +
		" 2>/dev/null || true"

	firstRunCommand := copyExtensionsCommand + "; " + baseCommand

	return fmt.Sprintf("-c %q", firstRunCommand)
}

// codeServerNode implements runtime.Node interface for code-server containers.
type codeServerNode struct {
	config *clabtypes.NodeConfig
}

func codeServerCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "code-server",
		Short: "Containerlab code-server server operations",
		Long:  "Start, stop, and manage Containerlab code-server containers",
	}

	codeServerStartCmd := &cobra.Command{
		Use:   "start",
		Short: "start Containerlab code-server container",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return codeServerStart(cobraCmd, o)
		},
	}

	c.AddCommand(codeServerStartCmd)
	codeServerStartCmd.Flags().StringVarP(&o.ToolsCodeServer.Image, "image", "i",
		o.ToolsCodeServer.Image,
		"container image to use for code-server")
	codeServerStartCmd.Flags().StringVarP(&o.ToolsCodeServer.Name, "name", "n",
		o.ToolsCodeServer.Name,
		"name of the code-server container")
	codeServerStartCmd.Flags().StringVarP(&o.ToolsCodeServer.LabsDirectory, "labs-dir", "l",
		o.ToolsCodeServer.LabsDirectory,
		"directory to mount as shared labs directory")
	codeServerStartCmd.Flags().UintVarP(&o.ToolsCodeServer.Port, "port", "p",
		o.ToolsCodeServer.Port,
		"port to expose the code-server on")
	codeServerStartCmd.Flags().StringVarP(&o.ToolsCodeServer.Owner, "owner", "o",
		o.ToolsCodeServer.Owner,
		"owner name for the code-server container")

	codeServerStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "show status of active Containerlab code-server containers",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return codeServerStatus(cobraCmd, o)
		},
	}
	c.AddCommand(codeServerStatusCmd)
	codeServerStatusCmd.Flags().StringVarP(&o.ToolsCodeServer.OutputFormat, "format", "f",
		o.ToolsCodeServer.OutputFormat,
		"output format for 'status' command (table, json)")

	codeServerStopCmd := &cobra.Command{
		Use:   "stop",
		Short: "stop Containerlab code-server container",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return codeServerStop(cobraCmd, o)
		},
	}
	c.AddCommand(codeServerStopCmd)
	codeServerStopCmd.Flags().StringVarP(&o.ToolsCodeServer.Name,
		"name", "n", o.ToolsCodeServer.Name,
		"name of the code-server container to stop")

	return c, nil
}

func NewCodeServerNode(name, image, labsDir string,
	port uint,
	runtime clabruntime.ContainerRuntime,
	labels map[string]string,
) (*codeServerNode, error) {
	log.With("name", name,
		"image", image,
		"labsDir", labsDir,
		"runtime", runtime).Debug("Creating new code-server node.")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	paths := newCodeServerPaths(homeDir, name)

	isFirstRun, err := prepareCodeServerPersistence(&paths)
	if err != nil {
		return nil, err
	}

	binds, err := buildCodeServerBinds(homeDir, runtime, &paths)
	if err != nil {
		return nil, err
	}

	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)

	containerPort, err := nat.NewPort("tcp", fmt.Sprintf("%d", codeServerPort))
	if err != nil {
		return nil, fmt.Errorf("failed to create container port: %w", err)
	}

	exposedPorts[containerPort] = struct{}{}

	var hostPort uint = 0
	if port != 0 {
		hostPort = port
	}

	portBindings[containerPort] = []nat.PortBinding{
		{
			HostIP:   "0.0.0.0",
			HostPort: fmt.Sprintf("%d", hostPort),
		},
	}

	cmd := buildCodeServerCommand(isFirstRun, homeDir)

	nodeConfig := &clabtypes.NodeConfig{
		LongName:     name,
		ShortName:    name,
		Image:        image,
		Binds:        binds.ToStringSlice(),
		Labels:       labels,
		PortSet:      exposedPorts,
		PortBindings: portBindings,
		NetworkMode:  "bridge",
		User:         "0",
		Entrypoint:   "/bin/sh",
		Cmd:          cmd,
	}

	return &codeServerNode{
		config: nodeConfig,
	}, nil
}

func (n *codeServerNode) Config() *clabtypes.NodeConfig {
	return n.config
}

// GetEndpoints implementation for the Node interface.
func (*codeServerNode) GetEndpoints() []clablinks.Endpoint {
	return nil
}

// createLabels creates container labels.
func createCodeServerLabels(containerName, owner, labsDir string) map[string]string {
	labels := map[string]string{
		clabconstants.NodeName: containerName,
		clabconstants.NodeKind: "linux",
		clabconstants.NodeType: "tool",
		clabconstants.ToolType: "code-server",
		"clab-labs-dir":        labsDir,
	}

	// Add owner label if available
	if owner != "" {
		labels[clabconstants.Owner] = owner
	}

	return labels
}

func codeServerStart(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	log.With(
		"name", o.ToolsCodeServer.Name,
		"image", o.ToolsCodeServer.Image,
		"labsDir", o.ToolsCodeServer.LabsDirectory,
		"port", o.ToolsCodeServer.Port).Debug("code-server start called.")

	runtimeName := o.Global.Runtime
	if runtimeName == "" {
		runtimeName = clabruntimedocker.RuntimeName
	}

	// Initialize runtime
	_, rinit, err := clabcore.RuntimeInitializer(runtimeName)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer for '%s': %w", runtimeName, err)
	}

	rt := rinit()

	err = rt.Init(clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Set management network to bridge for default Docker networking
	rt.WithMgmtNet(&clabtypes.MgmtNet{Network: "bridge"})

	// Check if container already exists
	filter := []*clabtypes.GenericFilter{{FilterType: "name", Match: o.ToolsCodeServer.Name}}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) > 0 {
		return fmt.Errorf("container %s already exists", o.ToolsCodeServer.Name)
	}

	// Pull the container image
	log.Infof("Pulling image %s...", o.ToolsCodeServer.Image)

	if err := rt.PullImage(ctx, o.ToolsCodeServer.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsCodeServer.Image, err)
	}

	// Create container labels
	if o.ToolsCodeServer.LabsDirectory == "" {
		o.ToolsCodeServer.LabsDirectory = defaultLabsDir
	}

	owner := getOwnerName(o)
	labels := createCodeServerLabels(o.ToolsCodeServer.Name, owner,
		o.ToolsCodeServer.LabsDirectory)

	// Create and start code server container
	log.Info("Creating code server container", "name", o.ToolsCodeServer.Name)

	codeServerNode, err := NewCodeServerNode(o.ToolsCodeServer.Name, o.ToolsCodeServer.Image,
		o.ToolsCodeServer.LabsDirectory, o.ToolsCodeServer.Port, rt, labels)
	if err != nil {
		return err
	}

	id, err := rt.CreateContainer(ctx, codeServerNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create code-server container: %w", err)
	}

	if _, err := rt.StartContainer(ctx, id, codeServerNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsCodeServer.Name)
		return fmt.Errorf("failed to start code-server container: %w", err)
	}

	log.Infof("code-server container %s started successfully.", o.ToolsCodeServer.Name)

	// Get the actual assigned port from the container if using random port
	if o.ToolsCodeServer.Port == 0 {
		// Get container info to find the assigned port
		containers, err := rt.ListContainers(ctx, []*clabtypes.GenericFilter{{
			FilterType: "name", Match: o.ToolsCodeServer.Name,
		}})
		if err == nil && len(containers) > 0 && len(containers[0].Ports) > 0 {
			for _, portMapping := range containers[0].Ports {
				if portMapping.ContainerPort == codeServerPort {
					// log the HOST PORT
					log.Infof("code-server available at: http://0.0.0.0:%d", portMapping.HostPort)
					break
				}
			}
		} else {
			log.Infof("code-server container started. Check 'docker ps' for the assigned port.")
		}
	} else {
		log.Infof("code-server available at: http://0.0.0.0:%d", o.ToolsCodeServer.Port)
	}

	return nil
}

// codeServerListItem defines the structure for API server container info in JSON output.
type codeServerListItem struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	LabsDir string `json:"labs_dir"`
	Owner   string `json:"owner"`
}

func codeServerStatus(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	// Use common.Runtime for consistency with other commands
	runtimeName := o.Global.Runtime
	if runtimeName == "" {
		runtimeName = clabruntimedocker.RuntimeName
	}

	// Initialize containerlab with runtime using the same approach as inspect command
	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(o.Global.Timeout),
		clabcore.WithRuntime(runtimeName,
			&clabruntime.RuntimeConfig{
				Debug:            o.Global.DebugCount > 0,
				Timeout:          o.Global.Timeout,
				GracefulShutdown: o.Global.GracefulShutdown,
			},
		),
		clabcore.WithDebug(o.Global.DebugCount > 0),
	}

	c, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	// Check connectivity like inspect does
	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	containers, err := c.ListContainers(ctx, clabcore.WithListToolType("code-server"))
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		if o.ToolsCodeServer.OutputFormat == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No active code-server containers found")
		}

		return nil
	}

	// Process containers and format output
	listItems := make([]codeServerListItem, 0, len(containers))
	for idx := range containers {
		name := strings.TrimPrefix(containers[idx].Names[0], "/")

		// Get port from labels or use default
		port := containers[idx].Ports[0].HostPort

		// Get labs dir from labels or use default
		labsDir := defaultLabsDir // default
		if dirsVal, ok := containers[idx].Labels["clab-labs-dir"]; ok {
			labsDir = dirsVal
		}

		// Get owner from container labels
		owner := "N/A"
		if ownerVal, exists := containers[idx].Labels[clabconstants.Owner]; exists &&
			ownerVal != "" {
			owner = ownerVal
		}

		listItems = append(listItems, codeServerListItem{
			Name:    name,
			State:   containers[idx].State,
			Port:    port,
			LabsDir: labsDir,
			Owner:   owner,
		})
	}

	if o.ToolsCodeServer.OutputFormat == "json" {
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

		t.AppendHeader(table.Row{"NAME", "STATUS", "PORT", "LABS DIR", "OWNER"})

		for _, item := range listItems {
			t.AppendRow(table.Row{
				item.Name,
				item.State,
				item.Port,
				item.LabsDir,
				item.Owner,
			})
		}

		t.Render()
	}

	return nil
}

func codeServerStop(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	log.Debugf("Container name for deletion: %s", o.ToolsCodeServer.Name)

	// Use common.Runtime if available, otherwise use the api-server flag
	runtimeName := o.Global.Runtime

	if runtimeName == "" {
		runtimeName = clabruntimedocker.RuntimeName
	}

	// Initialize runtime
	_, rinit, err := clabcore.RuntimeInitializer(runtimeName)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer: %w", err)
	}

	rt := rinit()

	err = rt.Init(clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	log.Info("Removing code-server container", "name", o.ToolsCodeServer.Name)

	if err := rt.DeleteContainer(ctx, o.ToolsCodeServer.Name); err != nil {
		return fmt.Errorf("failed to remove code-server container: %w", err)
	}

	log.Info("code server container removed", "name", o.ToolsCodeServer.Name)

	return nil
}
