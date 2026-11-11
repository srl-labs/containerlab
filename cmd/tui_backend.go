// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
	clabinternaltuibackendlocal "github.com/srl-labs/containerlab/internal/tui/backend/local"
)

// newTUIBackend creates the data source of the TUI: the simulated labs of the demo mode,
// the container runtime of this host, or the lab runtime that was asked for.
func newTUIBackend(o *Options) (clabinternaltuibackend.Backend, error) {
	if o.TUI.Demo {
		return clabinternaltuibackendfake.New(clabinternaltuibackendfake.Options{
			Live: true,
			Seed: uint64(time.Now().UnixNano()), //nolint:gosec // any seed will do
		}), nil
	}

	return clabinternaltuibackendlocal.New(clabinternaltuibackendlocal.Options{
		Runtime:   o.Global.Runtime,
		Namespace: o.Global.Namespace,
		Timeout:   o.Global.Timeout,
		LabsDirs:  o.TUI.LabsDirs,
		ReadOnly:  o.TUI.ReadOnly,
		Debug:     o.Global.DebugCount > 0,
	})
}
