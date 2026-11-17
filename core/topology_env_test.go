// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

const envTopology = `name: env-test
topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
      env:
        __IMPORT_ENVS: "true"
        EXPANDED: ${GIVEN}-${CLAB_TEST_SECRET:=unset}
`

func TestWithTopologyEnv(t *testing.T) {
	tests := map[string]struct {
		opts []ClabOption
		// want holds the env vars of node1 the test checks, an empty value means unset
		want map[string]string
		// wantTopologyEnv is the env node1 expands its startup config templates with
		wantTopologyEnv []string
	}{
		"given_env": {
			opts: []ClabOption{WithTopologyEnv([]string{"GIVEN=given"})},
			want: map[string]string{
				"EXPANDED":         "given-unset",
				"GIVEN":            "given",
				"CLAB_TEST_SECRET": "",
			},
			wantTopologyEnv: []string{"GIVEN=given"},
		},
		"empty_env": {
			opts: []ClabOption{WithTopologyEnv([]string{})},
			want: map[string]string{
				"EXPANDED":         "$GIVEN-unset",
				"GIVEN":            "",
				"CLAB_TEST_SECRET": "",
			},
			wantTopologyEnv: []string{},
		},
		"process_env": {
			opts: nil,
			want: map[string]string{
				"EXPANDED":         "process-secret",
				"GIVEN":            "process",
				"CLAB_TEST_SECRET": "secret",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("GIVEN", "process")
			t.Setenv("CLAB_TEST_SECRET", "secret")

			opts := append(tc.opts, WithTopoPath(writeTopo(t, envTopology), nil))

			c, err := NewContainerLab(opts...)
			if err != nil {
				t.Fatal(err)
			}

			cfg := c.Nodes["node1"].Config()
			got := map[string]string{}
			for k := range tc.want {
				got[k] = cfg.Env[k]
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Fatalf("env vars mismatch (-want +got):\n%s", d)
			}
			if d := cmp.Diff(tc.wantTopologyEnv, cfg.TopologyEnv); d != "" {
				t.Fatalf("topology env mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestWithTopologyEnvAfterTopology(t *testing.T) {
	_, err := NewContainerLab(
		WithTopoPath(writeTopo(t, envTopology), nil),
		WithTopologyEnv([]string{"GIVEN=given"}),
	)
	if err == nil {
		t.Fatal("expected an error when the topology env is set after the topology")
	}
}

func TestWithTopologyEnvRejectsLabRuntimes(t *testing.T) {
	tests := map[string][]string{
		"given_env": {"GIVEN=given"},
		"empty_env": {},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewContainerLab(
				WithTopologyEnv(env),
				WithRuntime(clablabruntime.ClabernetesRuntimeName, nil),
				WithTopoPath(writeTopo(t, envTopology), nil),
			)
			if err == nil || !strings.Contains(err.Error(), "not supported with lab runtimes") {
				t.Fatalf("NewContainerLab() error = %v, want unsupported lab runtime error", err)
			}
		})
	}
}
