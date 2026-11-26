// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package schemas holds the JSON schema of the containerlab topology file.
package schemas

import (
	_ "embed"
	"slices"
)

//go:embed clab.schema.json
var topology []byte

// Topology returns the JSON schema of the topology file of this containerlab version.
func Topology() []byte {
	return slices.Clone(topology)
}
