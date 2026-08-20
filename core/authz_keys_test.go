// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestIsSSHAgentUnavailableErr(t *testing.T) {
	tests := map[string]struct {
		err  error
		want bool
	}{
		"permission denied": {
			err:  fmt.Errorf("dial failed: %w", syscall.EACCES),
			want: true,
		},
		"socket not found": {
			err:  fmt.Errorf("dial failed: %w", syscall.ENOENT),
			want: true,
		},
		"connection refused": {
			err:  fmt.Errorf("dial failed: %w", syscall.ECONNREFUSED),
			want: true,
		},
		"different error": {
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := isSSHAgentUnavailableErr(tc.err)
			if got != tc.want {
				t.Fatalf("want %v got %v for error %v", tc.want, got, tc.err)
			}
		})
	}
}
