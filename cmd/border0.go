// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/border0_api"
)

var (
	border0Email    string
	border0Password string

	border0DisableBrowser bool
)

func init() {
	toolsCmd.AddCommand(border0Cmd)

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
	Short: "Logs in to border0.com service and saves a token to current working directory",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		return border0_api.Login(ctx, border0Email, border0Password, border0DisableBrowser)
	},
}
