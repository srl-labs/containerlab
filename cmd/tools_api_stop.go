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
	apiServerCmd.AddCommand(apiServerStopCmd)

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
