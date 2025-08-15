// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clablabels "github.com/srl-labs/containerlab/labels"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func NewAPIServerNode(name, image, labsDir string, runtime clabruntime.ContainerRuntime,
	env map[string]string, labels map[string]string,
) (*APIServerNode, error) {
	log.Debugf("Creating APIServerNode: name=%s, image=%s, labsDir=%s, runtime=%s", name, image, labsDir, runtime)

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
		return nil, fmt.Errorf("could not find containerlab binary: %v. API server might not function correctly if containerlab is not in its PATH", err)
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

// getclabBinaryPath determine the binary path of the running executable.
func getclabBinaryPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	absPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

// createLabels creates container labels.
func createAPIServerLabels(containerName, owner string, port int, labsDir, host, runtimeType string) map[string]string {
	labels := map[string]string{
		clablabels.NodeName: containerName,
		clablabels.NodeKind: "linux",
		clablabels.NodeType: "tool",
		clablabels.ToolType: "api-server",
		"clab-api-port":     fmt.Sprintf("%d", port),
		"clab-api-host":     host,
		"clab-labs-dir":     labsDir,
		"clab-runtime":      runtimeType,
	}

	// Add owner label if available
	if owner != "" {
		labels[clablabels.Owner] = owner
	}

	return labels
}

// getOwnerName gets owner name from flag or environment variables.
func getOwnerName() string {
	if apiServerOwner != "" {
		return apiServerOwner
	}

	if owner := os.Getenv("SUDO_USER"); owner != "" {
		return owner
	}

	return os.Getenv("USER")
}

// apiServerStartCmd starts API server container.
var apiServerStartCmd = &cobra.Command{
	Use:     "start",
	Short:   "start Containerlab API server container",
	PreRunE: clabutils.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("api-server start called with flags: name='%s', image='%s', labsDir='%s', port=%d, host='%s'",
			apiServerName, apiServerImage, apiServerLabsDir, apiServerPort, apiServerHost)

		// Generate random JWT secret if not provided
		if apiServerJWTSecret == "" {
			var err error
			apiServerJWTSecret, err = generateRandomJWTSecret()
			if err != nil {
				return fmt.Errorf("failed to generate random JWT secret: %w", err)
			}
			log.Infof("Generated random JWT secret for API server")
		}

		runtimeName := runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize runtime
		_, rinit, err := clabcore.RuntimeInitializer(runtimeName)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer for '%s': %w", runtimeName, err)
		}

		rt := rinit()
		err = rt.Init(clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Check if container already exists
		filter := []*clabtypes.GenericFilter{{FilterType: "name", Match: apiServerName}}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) > 0 {
			return fmt.Errorf("container %s already exists", apiServerName)
		}

		// Pull the container image
		log.Infof("Pulling image %s...", apiServerImage)
		if err := rt.PullImage(ctx, apiServerImage, clabtypes.PullPolicyAlways); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", apiServerImage, err)
		}

		// Create environment variables map
		env := map[string]string{
			"CLAB_SHARED_LABS_DIR":   apiServerLabsDir,
			"API_PORT":               fmt.Sprintf("%d", apiServerPort),
			"API_SERVER_HOST":        apiServerHost,
			"JWT_SECRET":             apiServerJWTSecret,
			"JWT_EXPIRATION_MINUTES": apiServerJWTExpiration,
			"API_USER_GROUP":         apiServerUserGroup,
			"SUPERUSER_GROUP":        apiServerSuperUserGroup,
			"CLAB_RUNTIME":           apiServerRuntime,
			"LOG_LEVEL":              apiServerLogLevel,
			"GIN_MODE":               apiServerGinMode,
		}

		// Add optional environment variables
		if apiServerTrustedProxies != "" {
			env["TRUSTED_PROXIES"] = apiServerTrustedProxies
		}
		if apiServerTLSEnable {
			env["TLS_ENABLE"] = "true"
			if apiServerTLSCertFile != "" {
				env["TLS_CERT_FILE"] = apiServerTLSCertFile
			}
			if apiServerTLSKeyFile != "" {
				env["TLS_KEY_FILE"] = apiServerTLSKeyFile
			}
		}
		if apiServerSSHBasePort > 0 {
			env["SSH_BASE_PORT"] = fmt.Sprintf("%d", apiServerSSHBasePort)
		}
		if apiServerSSHMaxPort > 0 {
			env["SSH_MAX_PORT"] = fmt.Sprintf("%d", apiServerSSHMaxPort)
		}

		// Create container labels
		if apiServerLabsDir == "" {
			apiServerLabsDir = "~/.clab"
		}
		owner := getOwnerName()
		labels := createAPIServerLabels(apiServerName, owner, apiServerPort, apiServerLabsDir, apiServerHost, runtimeName)

		// Create and start API server container
		log.Infof("Creating API server container %s", apiServerName)
		apiServerNode, err := NewAPIServerNode(apiServerName, apiServerImage, apiServerLabsDir, rt, env, labels)
		if err != nil {
			return err
		}

		id, err := rt.CreateContainer(ctx, apiServerNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create API server container: %w", err)
		}

		if _, err := rt.StartContainer(ctx, id, apiServerNode); err != nil {
			// Clean up on failure
			rt.DeleteContainer(ctx, apiServerName)
			return fmt.Errorf("failed to start API server container: %w", err)
		}

		log.Infof("API server container %s started successfully.", apiServerName)
		log.Infof("API Server available at: http://%s:%d", apiServerHost, apiServerPort)
		if apiServerTLSEnable {
			log.Infof("API Server TLS enabled at: https://%s:%d", apiServerHost, apiServerPort)
		}

		return nil
	},
}
