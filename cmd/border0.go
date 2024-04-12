// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/borderzero/border0-go"
	"github.com/borderzero/border0-go/client"
	"github.com/borderzero/border0-go/types/service"
	"github.com/gosimple/slug"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/border0_api"
)

const (
	border0NodeClabConfigWithTokenVolumeFmt = `
    border0:
      kind: linux
      image: ghcr.io/borderzero/border0
      cmd: connector start --config /etc/border0/border0.yaml
      binds:
        - /var/run/docker.sock:/var/run/docker.sock
        - %s:/etc/border0/border0.yaml
`

	// used when writing connector token file fails
	border0NodeClabConfigWithTokenEnvFmt = `
    border0:
      kind: linux
      image: ghcr.io/borderzero/border0
      cmd: connector start
      binds:
        - /var/run/docker.sock:/var/run/docker.sock
      env:
        BORDER0_TOKEN: %s
`
)

var (
	border0Email    string
	border0Password string

	border0DisableBrowser bool

	border0LabName string
)

func init() {
	toolsCmd.AddCommand(border0Cmd)

	border0Cmd.AddCommand(border0InitCmd)
	border0InitCmd.Flags().StringVarP(&border0LabName, "lab-name", "l", "", "Lab name")

	border0Cmd.AddCommand(border0LoginCmd)
	border0LoginCmd.Flags().BoolVarP(&border0DisableBrowser, "disable-browser", "b", false, "Disable opening the browser")

	// Programmatic user authentication for the Border0 service was deprecated on 11/2023,
	// so we hide the email and password flags though we keep them around for backwards
	// compatibility because some containerlabs users have been allowlisted for programmatic
	// authentication. Note that as of 12/2023 no new allowlist requests are considered by
	// Border0. Instead Border0 users who wish to integrate with containerlabs will need to
	// use Border0 "admin tokens" i.e. service identities. For info on how to create tokens,
	// see https://docs.border0.com/docs/creating-access-token
	border0LoginCmd.Flags().StringVarP(&border0Email, "email", "e", "", "Email address")
	border0LoginCmd.Flags().StringVarP(&border0Password, "password", "p", "", "Password")
	border0LoginCmd.Flags().MarkHidden("email")
	border0LoginCmd.Flags().MarkHidden("password")
}

// border0Cmd represents the border0 command container.
var border0Cmd = &cobra.Command{
	Use:   "border0",
	Short: "border0.com commands",
}

// border0LoginCmd represents the border0-login command.
var border0LoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Logs in to the border0.com service and saves a token to current working directory",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		_, err := border0_api.Login(ctx, border0Email, border0Password, border0DisableBrowser, true)
		return err
	},
}

var border0InitCmd = &cobra.Command{
	Use:   "setup",
	Short: "Provisions a border0.com organization with resources for a new ContainerLab environment",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		if border0LabName == "" {
			border0LabName = fmt.Sprintf("border0-clab-%d", time.Now().Unix()%10000)
		}
		if !slug.IsSlug(border0LabName) {
			return fmt.Errorf("lab-name must be in slug format e.g. my-border0-clab-123")
		}

		// always force a fresh login for now...
		token, err := border0_api.Login(ctx, border0Email, border0Password, border0DisableBrowser, false)
		if err != nil {
			return fmt.Errorf("failed to authenticate with Border0: %v", err)
		}

		// initialize border0 sdk
		api := border0.NewAPIClient(
			client.WithAuthToken(token),
			client.WithRetryMax(2),
		)

		// create new connector
		connector, err := api.CreateConnector(ctx, &client.Connector{
			Name:                     border0LabName,
			Description:              "ContainerLab Connector",
			BuiltInSshServiceEnabled: false,
		})
		if err != nil {
			return fmt.Errorf("failed to create new Border0 connector: %v", err)
		}

		// create new docker_exec socket
		socket, err := api.CreateSocket(ctx, &client.Socket{
			Name:             fmt.Sprintf("%s-containers", border0LabName),
			Description:      "Docker Exec socket for ContainerLab environment",
			RecordingEnabled: true,
			ConnectorID:      connector.ConnectorID,
			SocketType:       service.ServiceTypeSsh,
			UpstreamConfig: &service.Configuration{
				ServiceType: service.ServiceTypeSsh,
				SshServiceConfiguration: &service.SshServiceConfiguration{
					SshServiceType:                    service.SshServiceTypeDockerExec,
					DockerExecSshServiceConfiguration: &service.DockerExecSshServiceConfiguration{
						// no filters (expose all containers)
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create new Border0 socket: %v", err)
		}

		// create token for new connector
		connectorToken, err := api.CreateConnectorToken(ctx, &client.ConnectorToken{
			Name:        fmt.Sprintf("%s-token-%d", border0LabName, time.Now().Unix()%10000),
			ExpiresAt:   client.FlexibleTime{Time: time.Time{}},
			ConnectorID: connector.ConnectorID,
		})
		if err != nil {
			return fmt.Errorf("failed to create new Border0 socket: %v", err)
		}

		fmt.Printf("\nNew lab initialized with the Border0 service 🚀\n")

		localConnectorTokenFilePath := fmt.Sprintf("/etc/border0/%s-config.yaml", connector.Name)

		config := fmt.Sprintf(border0NodeClabConfigWithTokenVolumeFmt, localConnectorTokenFilePath)
		if err = writeConnectorConfig(localConnectorTokenFilePath, connectorToken.Token); err != nil {
			fmt.Printf("Warning: failed to write Border0 connector configuration file (will use BORDER0_TOKEN env instead of mount): %v\n", err)
			config = fmt.Sprintf(border0NodeClabConfigWithTokenEnvFmt, connectorToken.Token)
		}

		fmt.Println("Add the following configuration to your *.clab.yaml file:")
		fmt.Println(config)

		fmt.Printf("\nOnce you deploy your ContainerLab environment, your containers will be available at:\nhttps://client.border0.com/#/ssh/%s\n\n\n", socket.DNS)

		return nil
	},
}

func writeConnectorConfig(filePath, token string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(fmt.Sprintf("token: %s", token)), 0644)
}
