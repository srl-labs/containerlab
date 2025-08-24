// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func NewAPIServerNode(name, image, labsDir string, runtime clabruntime.ContainerRuntime,
	env map[string]string, labels map[string]string,
) (*APIServerNode, error) {
	log.Debugf(
		"Creating APIServerNode: name=%s, image=%s, labsDir=%s, runtime=%s",
		name,
		image,
		labsDir,
		runtime,
	)

	// Set up binds based on the runtime
	binds := clabtypes.Binds{
		//	types.NewBind(netnsPath, netnsPath, ""),
		clabtypes.NewBind("/etc/passwd", "/etc/passwd", "ro"),
		clabtypes.NewBind("/etc/shadow", "/etc/shadow", "ro"),
		clabtypes.NewBind("/etc/group", "/etc/group", "ro"),
		clabtypes.NewBind("/home", "/home", ""),
	}
	// if /etc/gshadow exists, add it to the binds
	if clabutils.FileExists("/etc/gshadow") {
		binds = append(binds, clabtypes.NewBind("/etc/gshadow", "/etc/gshadow", "ro"))
	}

	// get the runtime socket path
	rtSocket, err := runtime.GetRuntimeSocket()
	if err != nil {
		return nil, err
	}

	// build the bindmount for the socket, path sound be the same in the container as is on the host
	// append the socket to the binds
	binds = append(binds, clabtypes.NewBind(rtSocket, rtSocket, ""))

	// append the mounts required for container out of container operation
	binds = append(binds, runtime.GetCooCBindMounts()...)

	// Find containerlab binary and add bind mount if found
	clabPath, err := getclabBinaryPath()
	if err != nil {
		return nil, fmt.Errorf(
			"could not find containerlab binary: %v. API server might not function correctly "+
				"if containerlab is not in its PATH",
			err,
		)
	}

	binds = append(binds, clabtypes.NewBind(clabPath, "/usr/bin/containerlab", "ro"))

	nodeConfig := &clabtypes.NodeConfig{
		LongName:    name,
		ShortName:   name,
		Image:       image,
		Env:         env,
		Binds:       binds.ToStringSlice(),
		Labels:      labels,
		NetworkMode: "host", // Use host network namespace
		PidMode:     "host",
	}

	return &APIServerNode{
		config: nodeConfig,
	}, nil
}

func (n *APIServerNode) Config() *clabtypes.NodeConfig {
	return n.config
}

// GetEndpoints implementation for the Node interface.
func (*APIServerNode) GetEndpoints() []clablinks.Endpoint {
	return nil
}

// createLabels creates container labels.
func createAPIServerLabels(
	containerName,
	owner string,
	port uint,
	labsDir,
	host,
	runtimeType string,
) map[string]string {
	labels := map[string]string{
		clabconstants.NodeName: containerName,
		clabconstants.NodeKind: "linux",
		clabconstants.NodeType: "tool",
		clabconstants.ToolType: "api-server",
		"clab-api-port":        fmt.Sprintf("%d", port),
		"clab-api-host":        host,
		"clab-labs-dir":        labsDir,
		"clab-runtime":         runtimeType,
	}

	// Add owner label if available
	if owner != "" {
		labels[clabconstants.Owner] = owner
	}

	return labels
}

// getOwnerName gets owner name from flag or environment variables.
func getOwnerName(o *Options) string {
	if o.ToolsAPI.Owner != "" {
		return o.ToolsAPI.Owner
	}

	if owner := os.Getenv("SUDO_USER"); owner != "" {
		return owner
	}

	return os.Getenv("USER")
}

func apiServerStart(cobraCmd *cobra.Command, o *Options) error { //nolint: funlen
	ctx := cobraCmd.Context()

	log.Debugf(
		"api-server start called with flags: name='%s', image='%s', labsDir='%s', port=%d, host='%s'",
		o.ToolsAPI.Name,
		o.ToolsAPI.Image,
		o.ToolsAPI.LabsDirectory,
		o.ToolsAPI.Port,
		o.ToolsAPI.Host,
	)

	// Generate random JWT secret if not provided
	if o.ToolsAPI.JWTSecret == "" {
		var err error

		o.ToolsAPI.JWTSecret, err = generateRandomJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to generate random JWT secret: %w", err)
		}

		log.Infof("Generated random JWT secret for API server")
	}

	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime initializer for '%s': %w", o.Global.Runtime, err)
	}

	rt := rinit()

	err = rt.Init(clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: o.Global.Timeout}))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Check if container already exists
	filter := []*clabtypes.GenericFilter{{FilterType: "name", Match: o.ToolsAPI.Name}}

	containers, err := rt.ListContainers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) > 0 {
		return fmt.Errorf("container %s already exists", o.ToolsAPI.Name)
	}

	// Pull the container image
	log.Infof("Pulling image %s...", o.ToolsAPI.Image)

	if err := rt.PullImage(ctx, o.ToolsAPI.Image, clabtypes.PullPolicyAlways); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", o.ToolsAPI.Image, err)
	}

	// Create environment variables map
	env := map[string]string{
		"CLAB_SHARED_LABS_DIR":   o.ToolsAPI.LabsDirectory,
		"API_PORT":               fmt.Sprintf("%d", o.ToolsAPI.Port),
		"API_SERVER_HOST":        o.ToolsAPI.Host,
		"JWT_SECRET":             o.ToolsAPI.JWTSecret,
		"JWT_EXPIRATION_MINUTES": o.ToolsAPI.JWTExpiration,
		"API_USER_GROUP":         o.ToolsAPI.UserGroup,
		"SUPERUSER_GROUP":        o.ToolsAPI.SuperUserGroup,
		"CLAB_RUNTIME":           o.Global.Runtime,
		"LOG_LEVEL":              o.ToolsAPI.LogLevel,
		"GIN_MODE":               o.ToolsAPI.GinMode,
	}

	// Add optional environment variables
	if o.ToolsAPI.TrustedProxies != "" {
		env["TRUSTED_PROXIES"] = o.ToolsAPI.TrustedProxies
	}

	if o.ToolsAPI.TLSEnable {
		env["TLS_ENABLE"] = "true"
		if o.ToolsAPI.TLSCertFile != "" {
			env["TLS_CERT_FILE"] = o.ToolsAPI.TLSCertFile
		}

		if o.ToolsAPI.TLSKeyFile != "" {
			env["TLS_KEY_FILE"] = o.ToolsAPI.TLSKeyFile
		}
	}

	if o.ToolsAPI.SSHBasePort > 0 {
		env["SSH_BASE_PORT"] = fmt.Sprintf("%d", o.ToolsAPI.SSHBasePort)
	}

	if o.ToolsAPI.SSHMaxPort > 0 {
		env["SSH_MAX_PORT"] = fmt.Sprintf("%d", o.ToolsAPI.SSHMaxPort)
	}

	// Create container labels
	if o.ToolsAPI.LabsDirectory == "" {
		o.ToolsAPI.LabsDirectory = "~/.clab"
	}

	owner := getOwnerName(o)

	labels := createAPIServerLabels(
		o.ToolsAPI.Name,
		owner,
		o.ToolsAPI.Port,
		o.ToolsAPI.LabsDirectory,
		o.ToolsAPI.Host,
		o.Global.Runtime,
	)

	// Create and start API server container
	log.Infof("Creating API server container %s", o.ToolsAPI.Name)

	apiServerNode, err := NewAPIServerNode(
		o.ToolsAPI.Name,
		o.ToolsAPI.Image,
		o.ToolsAPI.LabsDirectory,
		rt,
		env,
		labels,
	)
	if err != nil {
		return err
	}

	id, err := rt.CreateContainer(ctx, apiServerNode.Config())
	if err != nil {
		return fmt.Errorf("failed to create API server container: %w", err)
	}

	if _, err := rt.StartContainer(ctx, id, apiServerNode); err != nil {
		// Clean up on failure
		rt.DeleteContainer(ctx, o.ToolsAPI.Name)

		return fmt.Errorf("failed to start API server container: %w", err)
	}

	log.Infof("API server container %s started successfully.", o.ToolsAPI.Name)
	log.Infof("API Server available at: http://%s:%d", o.ToolsAPI.Host, o.ToolsAPI.Port)

	if o.ToolsAPI.TLSEnable {
		log.Infof("API Server TLS enabled at: https://%s:%d", o.ToolsAPI.Host, o.ToolsAPI.Port)
	}

	return nil
}
