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
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	clabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func init() {

	apiServerCmd.AddCommand(apiServerStartCmd)

	// Start command flags
	apiServerStartCmd.Flags().StringVarP(&apiServerImage, "image", "i", "ghcr.io/srl-labs/clab-api-server/clab-api-server:latest",
		"container image to use for API server")
	apiServerStartCmd.Flags().StringVarP(&apiServerName, "name", "n", "clab-api-server",
		"name of the API server container")
	apiServerStartCmd.Flags().StringVarP(&apiServerLabsDir, "labs-dir", "l", "",
		"directory to mount as shared labs directory")
	apiServerStartCmd.Flags().IntVarP(&apiServerPort, "port", "p", 8080,
		"port to expose the API server on")
	apiServerStartCmd.Flags().StringVarP(&apiServerHost, "host", "", "localhost",
		"host address for the API server")
	apiServerStartCmd.Flags().StringVarP(&apiServerJWTSecret, "jwt-secret", "", "",
		"JWT secret key for authentication (generated randomly if not provided)")
	apiServerStartCmd.Flags().StringVarP(&apiServerJWTExpiration, "jwt-expiration", "", "60m",
		"JWT token expiration time")
	apiServerStartCmd.Flags().StringVarP(&apiServerUserGroup, "user-group", "", "clab_api",
		"user group for API access")
	apiServerStartCmd.Flags().StringVarP(&apiServerSuperUserGroup, "superuser-group", "", "clab_admins",
		"superuser group name")
	apiServerStartCmd.Flags().StringVarP(&apiServerLogLevel, "log-level", "", "debug",
		"log level (debug/info/warn/error)")
	apiServerStartCmd.Flags().StringVarP(&apiServerGinMode, "gin-mode", "", "release",
		"Gin framework mode (debug/release/test)")
	apiServerStartCmd.Flags().StringVarP(&apiServerTrustedProxies, "trusted-proxies", "", "",
		"comma-separated list of trusted proxies")
	apiServerStartCmd.Flags().BoolVarP(&apiServerTLSEnable, "tls-enable", "", false,
		"enable TLS for the API server")
	apiServerStartCmd.Flags().StringVarP(&apiServerTLSCertFile, "tls-cert", "", "",
		"path to TLS certificate file")
	apiServerStartCmd.Flags().StringVarP(&apiServerTLSKeyFile, "tls-key", "", "",
		"path to TLS key file")
	apiServerStartCmd.Flags().IntVarP(&apiServerSSHBasePort, "ssh-base-port", "", 2223,
		"SSH proxy base port")
	apiServerStartCmd.Flags().IntVarP(&apiServerSSHMaxPort, "ssh-max-port", "", 2322,
		"SSH proxy maximum port")
	apiServerStartCmd.Flags().StringVarP(&apiServerOwner, "owner", "o", "",
		"owner name for the API server container")
	apiServerStartCmd.Flags().StringVarP(&apiServerRuntime, "runtime", "r", "docker",
		"container runtime to use for API server")
}

func NewAPIServerNode(name, image, labsDir string, runtime runtime.ContainerRuntime, env map[string]string, labels map[string]string) (*APIServerNode, error) {
	log.Debugf("Creating APIServerNode: name=%s, image=%s, labsDir=%s, runtime=%s", name, image, labsDir, runtime)

	// Set up binds based on the runtime
	binds := types.Binds{
		//	types.NewBind(netnsPath, netnsPath, ""),
		types.NewBind("/etc/passwd", "/etc/passwd", "ro"),
		types.NewBind("/etc/shadow", "/etc/shadow", "ro"),
		types.NewBind("/etc/group", "/etc/group", "ro"),
		types.NewBind("/home", "/home", ""),
	}
	// if /etc/gshadow exists, add it to the binds
	if utils.FileExists("/etc/gshadow") {
		binds = append(binds, types.NewBind("/etc/gshadow", "/etc/gshadow", "ro"))
	}

	// get the runtime socket path
	rtSocket, err := runtime.GetRuntimeSocket()
	if err != nil {
		return nil, err
	}

	// build the bindmount for the socket, path sound be the same in the container as is on the host
	// append the socket to the binds
	binds = append(binds, types.NewBind(rtSocket, rtSocket, ""))

	// append the mounts required for container out of container operation
	binds = append(binds, runtime.GetCooCBindMounts()...)

	// Find containerlab binary and add bind mount if found
	clabPath, err := getContainerlabBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("could not find containerlab binary: %v. API server might not function correctly if containerlab is not in its PATH", err)
	}
	binds = append(binds, types.NewBind(clabPath, "/usr/bin/containerlab", "ro"))

	nodeConfig := &types.NodeConfig{
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

func (n *APIServerNode) Config() *types.NodeConfig {
	return n.config
}

// GetEndpoints implementation for the Node interface
func (*APIServerNode) GetEndpoints() []links.Endpoint {
	return nil
}

// getContainerlabBinaryPath determine the binary path of the running executable
func getContainerlabBinaryPath() (string, error) {
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

// createLabels creates container labels
func createAPIServerLabels(containerName, owner string, port int, labsDir, host, runtimeType string) map[string]string {
	labels := map[string]string{
		"clab-node-name": containerName,
		"clab-node-kind": "linux",
		"clab-node-type": "tool",
		"tool-type":      "api-server",
		"clab-api-port":  fmt.Sprintf("%d", port),
		"clab-api-host":  host,
		"clab-labs-dir":  labsDir,
		"clab-runtime":   runtimeType,
	}

	// Add owner label if available
	if owner != "" {
		labels[clabels.Owner] = owner
	}

	return labels
}

// getOwnerName gets owner name from flag or environment variables
func getOwnerName() string {
	if apiServerOwner != "" {
		return apiServerOwner
	}

	if owner := os.Getenv("SUDO_USER"); owner != "" {
		return owner
	}

	return os.Getenv("USER")
}

// apiServerStartCmd starts API server container
var apiServerStartCmd = &cobra.Command{
	Use:     "start",
	Short:   "start Containerlab API server container",
	PreRunE: common.CheckAndGetRootPrivs,
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

		runtimeName := common.Runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(runtimeName)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer for '%s': %w", runtimeName, err)
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Check if container already exists
		filter := []*types.GenericFilter{{FilterType: "name", Match: apiServerName}}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) > 0 {
			return fmt.Errorf("container %s already exists", apiServerName)
		}

		// Pull the container image
		log.Infof("Pulling image %s...", apiServerImage)
		if err := rt.PullImage(ctx, apiServerImage, types.PullPolicyAlways); err != nil {
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
