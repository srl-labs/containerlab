// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

func TestDetectPrivilege(t *testing.T) {
	noGroup := func() (bool, bool, string) { return false, false, "" }
	member := func() (bool, bool, string) { return true, true, "alice" }
	outsider := func() (bool, bool, string) { return true, false, "bob" }

	tests := map[string]struct {
		euid, suid   int
		setuidBinary bool
		group        adminGroupCheck
		wantOK       bool
		wantReason   string
	}{
		"root": {euid: 0, suid: 0, group: outsider, wantOK: true},
		"root through sudo with a regular binary": {
			euid: 0, suid: 0, group: noGroup, wantOK: true,
		},
		"a regular user with a regular binary": {
			euid: 1000, suid: 1000, group: noGroup, wantReason: reasonNeedsRoot,
		},
		// Being in the admin group does not help without a way to become root.
		"a member of the admin group with a regular binary": {
			euid: 1000, suid: 1000, group: member, wantReason: reasonNeedsRoot,
		},
		"the running binary is setuid": {
			euid: 1000, suid: 0, group: noGroup, wantOK: true,
		},
		"the binary run for commands is setuid": {
			euid: 1000, suid: 1000, setuidBinary: true, group: noGroup, wantOK: true,
		},
		"setuid and a member of the admin group": {
			euid: 1000, suid: 0, setuidBinary: true, group: member, wantOK: true,
		},
		"setuid but not in the admin group": {
			euid: 1000, suid: 0, setuidBinary: true, group: outsider,
			wantReason: `user "bob" is not a member of the clab_admins group`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := detectPrivilege(tt.euid, tt.suid, tt.setuidBinary, tt.group)
			if got.ok != tt.wantOK || !strings.Contains(got.reason, tt.wantReason) ||
				(tt.wantOK && got.reason != "") {
				t.Errorf(
					"got %+v, want ok=%v with a reason about %q",
					got,
					tt.wantOK,
					tt.wantReason,
				)
			}
		})
	}
}

func TestIsSetuidRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "binary")
	writeFile(t, file, "#!/bin/sh\n")

	if isSetuidRoot(file) {
		t.Error("a regular file counts as setuid root")
	}

	if isSetuidRoot(filepath.Join(t.TempDir(), "missing")) {
		t.Error("a missing file counts as setuid root")
	}

	// The bit alone does not do it, the file has to belong to root.
	if err := os.Chmod(file, 0o755|os.ModeSetuid); err != nil {
		t.Skipf("cannot set the setuid bit: %v", err)
	}

	if got, want := isSetuidRoot(file), os.Getuid() == 0; got != want {
		t.Errorf("isSetuidRoot() = %v for a setuid file of uid %d", got, os.Getuid())
	}
}

func TestCapabilities(t *testing.T) {
	notRunning := "the interface state stream is not running yet"

	tests := map[string]struct {
		adjust func(o *Options, d *deps)
		want   clabinternaltuibackend.Capabilities
	}{
		"privileged on a desktop": {
			want: clabinternaltuibackend.Capabilities{
				Lifecycle: true, Netem: true, Capture: true, Share: true, Wireshark: true,
				// The state of the links is known once its stream runs.
				Reasons: map[string]string{capLinkState: notRunning},
			},
		},
		"without a display": {
			adjust: func(_ *Options, d *deps) { d.getenv = func(string) string { return "" } },
			want: clabinternaltuibackend.Capabilities{
				Lifecycle: true, Netem: true, Capture: true, Share: true,
				Reasons: map[string]string{
					capLinkState: notRunning,
					capWireshark: "no graphical display: neither DISPLAY nor " +
						"WAYLAND_DISPLAY is set",
				},
			},
		},
		"with an X display but without Wireshark": {
			adjust: func(_ *Options, d *deps) {
				d.getenv = func(key string) string {
					if key == "DISPLAY" {
						return ":0"
					}

					return ""
				}
				d.lookPath = func(string) (string, error) { return "", errors.New("not found") }
			},
			want: clabinternaltuibackend.Capabilities{
				Lifecycle: true, Netem: true, Capture: true, Share: true,
				Reasons: map[string]string{
					capLinkState: notRunning,
					capWireshark: "wireshark is not installed",
				},
			},
		},
		"read-only": {
			adjust: func(o *Options, _ *deps) { o.ReadOnly = true },
			want: clabinternaltuibackend.Capabilities{
				Wireshark: true,
				Reasons: map[string]string{
					capLifecycle: reasonReadOnly, capNetem: reasonReadOnly,
					capCapture: reasonReadOnly, capShare: reasonReadOnly,
					capLinkState: notRunning,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var adjust []func(o *Options, d *deps)
			if tt.adjust != nil {
				adjust = append(adjust, tt.adjust)
			}

			b := newTestBackend(t, adjust...)

			if diff := cmp.Diff(tt.want, b.Host().Caps); diff != "" {
				t.Errorf("unexpected capabilities (-want +got):\n%s", diff)
			}
		})
	}

	// The reasons handed out are a copy.
	b := newTestBackend(t)
	b.Host().Caps.Reasons[capLinkState] = "changed"

	caps := b.Host().Caps
	if got := caps.Reason(capLinkState); got != notRunning {
		t.Errorf("a change of the returned reasons reached the backend: %q", got)
	}

	b.setLinkState(true, "")

	if caps = b.Host().Caps; !caps.LinkState || len(caps.Reasons) != 0 {
		t.Errorf("unexpected capabilities with a running stream: %+v", caps)
	}

	// A capability without a reason still has an answer for the UI.
	if got := caps.Reason(capLinkState); got == "" {
		t.Error("no reason at all for a capability that is on")
	}

	b.setLinkState(false, "the stream stopped")

	if caps = b.Host().Caps; caps.LinkState || caps.Reason(capLinkState) != "the stream stopped" {
		t.Errorf("unexpected capabilities with a stopped stream: %+v", caps)
	}
}

func TestCurrentPrivilege(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// The test binary is not setuid: only root has privileges with it.
	got := currentPrivilege(self)
	if want := os.Geteuid() == 0; got.ok != want || (!want && got.reason != reasonNeedsRoot) {
		t.Errorf("currentPrivilege() = %+v for the effective user %d", got, os.Geteuid())
	}

	if exists, member, name := adminGroupMembership(); (member && !exists) ||
		(!exists && name != "") {
		t.Errorf("adminGroupMembership() = %v, %v, %q", exists, member, name)
	}
}
