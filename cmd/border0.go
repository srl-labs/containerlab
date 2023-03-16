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
)

func init() {
	toolsCmd.AddCommand(border0Cmd)

	border0Cmd.AddCommand(border0LoginCmd)
	border0LoginCmd.Flags().StringVarP(&border0Email, "email", "e", "", "Email address")
	border0LoginCmd.Flags().StringVarP(&border0Password, "password", "p", "", "Password")
	_ = border0LoginCmd.MarkFlagRequired("email")
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

		return border0_api.Login(ctx, border0Email, border0Password)
	},
}
