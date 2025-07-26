// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"os"

	"github.com/charmbracelet/fang"
	"github.com/srl-labs/containerlab/cmd"
)

func main() {
	ctx, cancel := cmd.SignalHandledContext()
	defer cancel()

	cmd.RootCmd.SetContext(ctx)

	err := fang.Execute(ctx, cmd.RootCmd, fang.WithoutVersion())
	if err != nil {
		os.Exit(1)
	}
}
