// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"os"

	"github.com/charmbracelet/fang"
	clabcmd "github.com/srl-labs/containerlab/cmd"
)

func main() {
	ctx, cancel := clabcmd.SignalHandledContext()

	clabcmd.RootCmd.SetContext(ctx)

	err := fang.Execute(ctx, clabcmd.RootCmd, fang.WithoutVersion())

	// ensure cancel is *always* called (os.Exit bypasses)
	cancel()

	if err != nil {
		os.Exit(1)
	}
}
