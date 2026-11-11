// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// stubBackend answers the calls Run makes around the UI.
type stubBackend struct {
	clabinternaltuibackend.Backend

	jobs   []clabinternaltuibackend.JobInfo
	closed bool
}

func (s *stubBackend) Start(ctx context.Context) error { return ctx.Err() }

func (s *stubBackend) Jobs() []clabinternaltuibackend.JobInfo { return s.jobs }

func (s *stubBackend) Close() { s.closed = true }

// TestRunStoppedWhileStarting ends the UI while the labs are listed for the first time,
// which is no failure.
func TestRunStoppedWhileStarting(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := Run(ctx, &stubBackend{}, Options{}); err != nil {
		t.Errorf("Run() = %v, want nil", err)
	}
}

// TestCloseBackendTellsWhatIsWaitedFor checks that quitting with operations that still run
// says what it waits for.
func TestCloseBackendTellsWhatIsWaitedFor(t *testing.T) {
	be := &stubBackend{jobs: []clabinternaltuibackend.JobInfo{
		{Title: "deploy lab1", State: clabinternaltuibackend.JobCancelling},
		{Title: "destroy lab2", State: clabinternaltuibackend.JobSucceeded},
	}}

	var out bytes.Buffer

	closeBackend(be, &out)

	if !be.closed {
		t.Error("the backend was not closed")
	}

	if got := out.String(); !strings.Contains(got, "1 operation(s)") ||
		!strings.Contains(got, "deploy lab1") || strings.Contains(got, "lab2") {
		t.Errorf("unexpected notice: %q", got)
	}

	out.Reset()

	be.jobs = be.jobs[1:]

	closeBackend(be, &out)

	if out.Len() != 0 {
		t.Errorf("a notice without running operations: %q", out.String())
	}
}
