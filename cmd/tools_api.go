// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"github.com/spf13/cobra"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func apiServerCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   "api-server",
		Short: "Containerlab API server operations",
		Long:  "Start, stop, and manage Containerlab API server containers",
	}

	apiServerStartCmd := &cobra.Command{
		Use:   "start",
		Short: "start Containerlab API server container",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return apiServerStart(cobraCmd, o)
		},
	}

	c.AddCommand(apiServerStartCmd)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.Image,
		"image",
		"i",
		o.ToolsAPI.Image,
		"container image to use for API server",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.Name,
		"name",
		"n",
		o.ToolsAPI.Name,
		"name of the API server container",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.LabsDirectory,
		"labs-dir",
		"l",
		o.ToolsAPI.LabsDirectory,
		"directory to mount as shared labs directory",
	)
	apiServerStartCmd.Flags().UintVarP(
		&o.ToolsAPI.Port,
		"port",
		"p",
		o.ToolsAPI.Port,
		"port to expose the API server on",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.Host,
		"host",
		"",
		o.ToolsAPI.Host,
		"host address for the API server",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.JWTSecret,
		"jwt-secret",
		"", o.ToolsAPI.JWTSecret,
		"JWT secret key for authentication (generated randomly if not provided)",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.JWTExpiration,
		"jwt-expiration",
		"",
		o.ToolsAPI.JWTExpiration,
		"JWT token expiration time",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.UserGroup,
		"user-group",
		"",
		o.ToolsAPI.UserGroup,
		"user group for API access",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.SuperUserGroup,
		"superuser-group",
		"",
		o.ToolsAPI.SuperUserGroup,
		"superuser group name",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.LogLevel,
		"log-level",
		"",
		o.ToolsAPI.LogLevel,
		"log level (debug/info/warn/error)",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.GinMode,
		"gin-mode",
		"",
		o.ToolsAPI.GinMode,
		"Gin framework mode (debug/release/test)",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.TrustedProxies,
		"trusted-proxies",
		"",
		o.ToolsAPI.TrustedProxies,
		"comma-separated list of trusted proxies",
	)
	apiServerStartCmd.Flags().BoolVarP(
		&o.ToolsAPI.TLSEnable,
		"tls-enable",
		"",
		o.ToolsAPI.TLSEnable,
		"enable TLS for the API server",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.TLSCertFile,
		"tls-cert",
		"",
		o.ToolsAPI.TLSCertFile,
		"path to TLS certificate file",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.TLSKeyFile,
		"tls-key",
		"",
		o.ToolsAPI.TLSKeyFile,
		"path to TLS key file",
	)
	apiServerStartCmd.Flags().UintVarP(
		&o.ToolsAPI.SSHBasePort,
		"ssh-base-port",
		"",
		o.ToolsAPI.SSHBasePort,
		"SSH proxy base port",
	)
	apiServerStartCmd.Flags().UintVarP(
		&o.ToolsAPI.SSHMaxPort,
		"ssh-max-port",
		"",
		o.ToolsAPI.SSHMaxPort,
		"SSH proxy maximum port",
	)
	apiServerStartCmd.Flags().StringVarP(
		&o.ToolsAPI.Owner,
		"owner",
		"o",
		o.ToolsAPI.Owner,
		"owner name for the API server container",
	)

	apiServerStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "show status of active Containerlab API server containers",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return apiServerStatus(cobraCmd, o)
		},
	}
	c.AddCommand(apiServerStatusCmd)
	apiServerStatusCmd.Flags().StringVarP(
		&o.ToolsAPI.OutputFormat,
		"format",
		"f",
		o.ToolsAPI.OutputFormat,
		"output format for 'status' command (table, json)",
	)

	apiServerStopCmd := &cobra.Command{
		Use:   "stop",
		Short: "stop Containerlab API server container",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return apiServerStop(cobraCmd, o)
		},
	}
	c.AddCommand(apiServerStopCmd)
	apiServerStopCmd.Flags().StringVarP(
		&o.ToolsAPI.Name,
		"name",
		"n",
		o.ToolsAPI.Name,
		"name of the API server container to stop",
	)

	return c, nil
}
