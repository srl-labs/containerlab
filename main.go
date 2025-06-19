// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/srl-labs/containerlab/cmd"
)

func main() {
	if err := fang.Execute(context.TODO(), cmd.RootCmd); err != nil {
		os.Exit(1)
	}
}
