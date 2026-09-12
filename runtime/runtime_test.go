// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package runtime

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForContainerRunningHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)

	go func() {
		done <- WaitForContainerRunning(ctx, nil, "external", "node")
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WaitForContainerRunning() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitForContainerRunning did not return after context cancellation")
	}
}

type statusTestRuntime struct {
	ContainerRuntime
	status ContainerStatus
}

func (r *statusTestRuntime) GetContainerStatus(context.Context, string) ContainerStatus {
	return r.status
}

func TestWaitForContainerRunningChecksImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		if err := WaitForContainerRunning(context.Background(), &statusTestRuntime{
			status: Running,
		}, "target", "sidecar"); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Fatalf("running target incurred a polling delay of %s", elapsed)
		}
	})
}

func TestWaitForContainerRunningCancelsDuringWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- WaitForContainerRunning(ctx, &statusTestRuntime{status: Stopped}, "target", "sidecar")
		}()
		synctest.Wait()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
	})
}

func TestContainerHasJoinableNetns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status ContainerStatus
		want   bool
	}{
		{Running, true},
		{Paused, true},
		{Stopped, false},
		{Created, false},
		{Restarting, false},
		{Removing, false},
		{NotFound, false},
		{"unknown", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			if got := ContainerHasJoinableNetns(tt.status); got != tt.want {
				t.Fatalf("ContainerHasJoinableNetns(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
