// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"github.com/spf13/cobra"
)

func apiServerCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "api-server",
		Short: "Containerlab API server operations",
		Long:  "Start, stop, and manage Containerlab API server containers",
	}

	c.AddCommand(apiServerStartCmd)
	apiServerStartCmd.Flags().StringVarP(&apiServerImage, "image", "i",
		"ghcr.io/srl-labs/clab-api-server/clab-api-server:latest",
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

	c.AddCommand(apiServerStatusCmd)
	apiServerStatusCmd.Flags().StringVarP(&outputFormatAPI, "format", "f", "table",
		"output format for 'status' command (table, json)")

	c.AddCommand(apiServerStopCmd)
	apiServerStopCmd.Flags().StringVarP(&apiServerName, "name", "n", "clab-api-server",
		"name of the API server container to stop")

	return c
}
