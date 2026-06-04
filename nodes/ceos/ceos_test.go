// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import (
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
)

func TestCeosLinkApplyMode(t *testing.T) {
	if got := (&ceos{}).LinkApplyMode(); got != clabnodes.LinkApplyModeRestart {
		t.Fatalf("LinkApplyMode() = %q, want %q", got, clabnodes.LinkApplyModeRestart)
	}
}
