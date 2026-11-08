// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package progress

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/google/go-cmp/cmp"
)

func TestNewLogsToContextLogger(t *testing.T) {
	var output bytes.Buffer
	ctx := log.WithContext(context.Background(), log.New(&output))

	r, stop := New(ctx, &bytes.Buffer{})
	defer stop()

	r.Start("alpine:3")
	r.Done("alpine:3", nil)

	got := output.String()
	for _, want := range []string{"Pulling image", "Done pulling image", "image=alpine:3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in the context logger output, got:\n%s", want, got)
		}
	}
}

// fdWriter is a writer with a file descriptor, like os.Stderr.
type fdWriter struct{ bytes.Buffer }

func (*fdWriter) Fd() uintptr { return 2 }

func TestUseTUI(t *testing.T) {
	logCtx := log.WithContext(context.Background(), log.New(&bytes.Buffer{}))

	tests := map[string]struct {
		ctx      context.Context
		w        io.Writer
		terminal bool
		want     bool
	}{
		"terminal": {
			ctx:      context.Background(),
			w:        &fdWriter{},
			terminal: true,
			want:     true,
		},
		"not_a_terminal": {
			ctx: context.Background(),
			w:   &fdWriter{},
		},
		"writer_without_fd": {
			ctx:      context.Background(),
			w:        &bytes.Buffer{},
			terminal: true,
		},
		"logger_in_ctx_on_terminal": {
			ctx:      logCtx,
			w:        &fdWriter{},
			terminal: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isTerminal = func(uintptr) bool { return tc.terminal }
			defer func() { isTerminal = term.IsTerminal }()

			if d := cmp.Diff(tc.want, useTUI(tc.ctx, tc.w)); d != "" {
				t.Fatalf("useTUI mismatch (-want +got):\n%s", d)
			}
		})
	}
}
