// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	"regexp"
	"strings"
	"testing"
)

// enabledDaemons extracts the set of daemons switched on in a rendered
// daemons file.
func enabledDaemons(t *testing.T, rendered string) map[string]bool {
	t.Helper()

	re := regexp.MustCompile(`(?m)^([a-z0-9]+)=(yes|no)$`)

	got := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(rendered, -1) {
		got[m[1]] = m[2] == "yes"
	}

	if len(got) != len(configurableDaemons) {
		t.Fatalf("rendered %d toggles, want %d", len(got), len(configurableDaemons))
	}

	return got
}

func TestRenderDaemons(t *testing.T) {
	tests := map[string]struct {
		daemons     []string
		wantEnabled []string // nil means "all of them"
		wantErr     string
	}{
		"nil list enables every daemon": {
			daemons:     nil,
			wantEnabled: nil,
		},
		"empty list enables every daemon": {
			daemons:     []string{},
			wantEnabled: nil,
		},
		"explicit list enables exactly those": {
			daemons:     []string{"ospfd"},
			wantEnabled: []string{"ospfd"},
		},
		"several daemons": {
			daemons:     []string{"bgpd", "bfdd", "ldpd"},
			wantEnabled: []string{"bgpd", "bfdd", "ldpd"},
		},
		"always-on daemons are accepted and do not appear as toggles": {
			daemons:     []string{"zebra", "staticd", "mgmtd", "watchfrr"},
			wantEnabled: []string{},
		},
		"always-on daemons mix with configurable ones": {
			daemons:     []string{"zebra", "isisd"},
			wantEnabled: []string{"isisd"},
		},
		"unknown daemon is rejected": {
			daemons: []string{"bgpd", "bogusd"},
			wantErr: "bogusd",
		},
		"a daemon name is not guessed from a prefix": {
			daemons: []string{"ospf"},
			wantErr: "ospf",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rendered, err := renderDaemons(tc.daemons)

			switch {
			case tc.wantErr != "":
				if err == nil {
					t.Fatalf("expected an error mentioning %q, got none", tc.wantErr)
				}

				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not mention %q", err, tc.wantErr)
				}

				return
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			}

			got := enabledDaemons(t, rendered)

			want := map[string]bool{}

			if tc.wantEnabled == nil {
				for _, d := range configurableDaemons {
					want[d] = true
				}
			} else {
				for _, d := range configurableDaemons {
					want[d] = false
				}

				for _, d := range tc.wantEnabled {
					want[d] = true
				}
			}

			for d, w := range want {
				if got[d] != w {
					t.Errorf("daemon %s: got enabled=%v, want %v", d, got[d], w)
				}
			}
		})
	}
}

// The rendered file must keep the upstream per-daemon option lines, since they
// carry the VTY bind addresses the daemons are started with.
func TestRenderDaemonsPreservesUpstreamOptions(t *testing.T) {
	rendered, err := renderDaemons([]string{"bgpd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		`zebra_options="  -A 127.0.0.1 -s 90000000"`,
		`bgpd_options="   -A 127.0.0.1"`,
		`ospf6d_options=" -A ::1"`,
		"vtysh_enable=yes",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered daemons file is missing %q", want)
		}
	}
}

// No template action may survive into the rendered file.
func TestRenderDaemonsLeavesNoTemplateActions(t *testing.T) {
	rendered, err := renderDaemons(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(rendered, "{{") || strings.Contains(rendered, "}}") {
		t.Errorf("rendered daemons file still contains template actions")
	}
}
