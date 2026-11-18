package cmd

import (
	"context"
	"strings"
	"testing"
)

func TestExecRejectsFlagsAnAttachedExecDoesNotSupport(t *testing.T) {
	tests := map[string]struct {
		args    []string
		wantErr string
	}{
		"stdin with two commands": {
			args:    []string{"--stdin", "--cmd", "true", "--cmd", "false"},
			wantErr: "exactly one command",
		},
		"tty with two commands": {
			args:    []string{"--tty", "--cmd", "true", "--cmd", "false"},
			wantErr: "exactly one command",
		},
		"stdin with a format": {
			args:    []string{"-i", "--format", "plain", "--cmd", "sh"},
			wantErr: "--format",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			o := &Options{Exec: &ExecOptions{Format: "plain"}, Filter: &FilterOptions{}}

			c, err := execCmd(o)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.ParseFlags(tc.args); err != nil {
				t.Fatal(err)
			}
			c.SetContext(context.Background())

			err = execFn(c, o)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("execFn() error = %v, want an error containing %q", err, tc.wantErr)
			}
		})
	}
}
