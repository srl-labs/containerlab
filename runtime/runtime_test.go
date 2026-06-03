// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package runtime

import "testing"

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
