// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// APIServerNode implements runtime.Node interface for API server containers.
type APIServerNode struct {
	config *clabtypes.NodeConfig
}

// generateRandomJWTSecret creates a random string for use as JWT secret.
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

func apiServerStop(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	log.Debugf("Container name for deletion: %s", o.ToolsAPI.Name)

	// Use common.Runtime if available, otherwise use the api-server flag
	runtimeName := o.Global.Runtime
	if runtimeName == "" {
		runtimeName = o.ToolsAPI.Runtime
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

	log.Infof("Removing API server container %s", o.ToolsAPI.Name)
	if err := rt.DeleteContainer(ctx, o.ToolsAPI.Name); err != nil {
		return fmt.Errorf("failed to remove API server container: %w", err)
	}

	log.Infof("API server container %s removed successfully", o.ToolsAPI.Name)
	return nil
}
