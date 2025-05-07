// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

// Configuration variables for the API Server commands
var (
	apiServerImage          string
	apiServerName           string
	apiServerLabsDir        string
	apiServerPort           int
	apiServerHost           string
	apiServerJWTSecret      string
	apiServerJWTExpiration  string
	apiServerUserGroup      string
	apiServerSuperUserGroup string
	apiServerRuntime        string
	apiServerLogLevel       string
	apiServerGinMode        string
	apiServerTrustedProxies string
	apiServerTLSEnable      bool
	apiServerTLSCertFile    string
	apiServerTLSKeyFile     string
	apiServerSSHBasePort    int
	apiServerSSHMaxPort     int
	apiServerOwner          string
	outputFormatAPI         string
)

// APIServerNode implements runtime.Node interface for API server containers
type APIServerNode struct {
	config *types.NodeConfig
}

// generateRandomJWTSecret creates a random string for use as JWT secret
func generateRandomJWTSecret() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Encode as base64 string
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func init() {
	toolsCmd.AddCommand(apiServerCmd)
	apiServerCmd.AddCommand(apiServerStartCmd)
	apiServerCmd.AddCommand(apiServerStopCmd)
	apiServerCmd.AddCommand(apiServerStatusCmd)

	apiServerCmd.PersistentFlags().StringVarP(&outputFormatAPI, "format", "f", "table",
		"output format for 'status' command (table, json)")

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

	// Removed: apiServerStartCmd.MarkFlagRequired("jwt-secret")

	// Stop command flags
	apiServerStopCmd.Flags().StringVarP(&apiServerName, "name", "n", "clab-api-server",
		"name of the API server container to stop")
}

var apiServerStopCmd = &cobra.Command{
	Use:     "stop",
	Short:   "stop Containerlab API server container",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("Container name for deletion: %s", apiServerName)

		// Use common.Runtime if available, otherwise use the api-server flag
		runtimeName := common.Runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(runtimeName)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer: %w", err)
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		log.Infof("Removing API server container %s", apiServerName)
		if err := rt.DeleteContainer(ctx, apiServerName); err != nil {
			return fmt.Errorf("failed to remove API server container: %w", err)
		}

		log.Infof("API server container %s removed successfully", apiServerName)
		return nil
	},
}
