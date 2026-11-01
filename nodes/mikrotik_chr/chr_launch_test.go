// Copyright 2026 narrowin
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package mikrotik_chr

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// launcherFunction isolates a shell function without running the disk, network,
// QEMU or REST operations at the top level of the embedded launcher.
// It assumes nested closing braces are indented; a nested brace at column zero
// would be mistaken for the end of the function.
func launcherFunction(t *testing.T, name string) string {
	t.Helper()

	_, body, ok := strings.Cut(string(launcherScript), "\n"+name+"() {")
	if !ok {
		t.Fatalf("launcher function %q not found", name)
	}
	body, _, ok = strings.Cut(body, "\n}\n")
	if !ok {
		t.Fatalf("launcher function %q has no closing brace", name)
	}

	return name + "() {" + body + "\n}\n"
}

func TestLauncherShellSyntax(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(string(launcherScript))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("invalid launcher shell syntax: %v\n%s", err, out)
	}
}

func TestQEMUDataNIC(t *testing.T) {
	tests := map[string]struct {
		pciBridges int
		index      int
		bridge     string
		bus        string
		addr       string
	}{
		"first-data-port": {
			pciBridges: 1,
			index:      1,
			bridge:     "1",
			bus:        "1",
			addr:       "0x1",
		},
		"old-limit": {
			pciBridges: 1,
			index:      28,
			bus:        "2",
			addr:       "0x3",
		},
		"ether30": {
			pciBridges: 1,
			index:      29,
			bus:        "2",
			addr:       "0x4",
		},
		"last-slot": {
			pciBridges: 1,
			index:      25,
			bus:        "1",
			addr:       "0x19",
		},
		"second-bridge": {
			pciBridges: 1,
			index:      26,
			bridge:     "2",
			bus:        "2",
			addr:       "0x1",
		},
		"second-last-slot": {
			pciBridges: 1,
			index:      50,
			bus:        "2",
			addr:       "0x19",
		},
		"third-bridge": {
			pciBridges: 1,
			index:      51,
			bridge:     "3",
			bus:        "3",
			addr:       "0x1",
		},
		"highest-port": {
			pciBridges: 1,
			index:      63,
			bus:        "3",
			addr:       "0xd",
		},
		"arm64-first-port": {
			index: 1,
		},
		"arm64-high-port": {
			index: 32,
		},
	}

	// qemu_data_nic takes the -device value from data_nic_device, which also re-plugs NICs.
	fn := launcherFunction(t, "data_nic_device") + launcherFunction(t, "qemu_data_nic")
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mac := fmt.Sprintf("52:54:00:12:34:%02x", tc.index)
			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c",
				fn+"PCI_BRIDGES=\"$1\"; qemu_data_nic \"$2\" \"$3\"",
				"chr-launch-test", fmt.Sprint(tc.pciBridges), fmt.Sprint(tc.index), mac)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("qemu_data_nic failed: %v\n%s", err, out)
			}

			want := fmt.Sprintf(
				"-netdev tap,id=d%d,ifname=tap%d,script=no,downscript=no ",
				tc.index,
				tc.index,
			)
			if tc.bus == "" {
				want += fmt.Sprintf(
					"-device virtio-net,id=nic%d,netdev=d%d,mac=%s",
					tc.index,
					tc.index,
					mac,
				)
			} else {
				want += fmt.Sprintf(
					"-device virtio-net,id=nic%d,netdev=d%d,mac=%s,bus=chrpci%s,addr=%s,romfile=,vectors=2",
					tc.index,
					tc.index,
					mac,
					tc.bus,
					tc.addr,
				)
			}
			if tc.bridge != "" {
				want = fmt.Sprintf("-device pci-bridge,id=chrpci%s,chassis_nr=%s,bus=pci.0 ",
					tc.bridge, tc.bridge) + want
			}
			if diff := cmp.Diff(strings.Fields(want), strings.Fields(string(out))); diff != "" {
				t.Errorf("QEMU arguments mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckNICIndex(t *testing.T) {
	tests := map[string]struct {
		pciBridges int
		index      int
		errText    string // empty when the index is accepted
	}{
		"bridges-zero":      {pciBridges: 1, index: 0},
		"bridges-old-limit": {pciBridges: 1, index: 28},
		"bridges-ether33":   {pciBridges: 1, index: 32},
		"bridges-max":       {pciBridges: 1, index: 63},
		"bridges-over":      {pciBridges: 1, index: 64, errText: "beyond the tested limit"},
		"bridges-mac-wrap":  {pciBridges: 1, index: 256, errText: "beyond the tested limit"},
		"no-bridges-max":    {pciBridges: 0, index: 29},
		"no-bridges-full":   {pciBridges: 0, index: 30, errText: "no PCI slot for the disk"},
		"no-bridges-high":   {pciBridges: 0, index: 256, errText: "no PCI slot for the disk"},
	}

	fn := launcherFunction(t, "check_nic_index")
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := exec.CommandContext(
				t.Context(),
				"/bin/sh",
				"-c",
				"log() { printf '%s\\n' \"$*\" >&2; }\n"+fn+
					"PCI_BRIDGES=\"$1\"; check_nic_index \"$2\"",
				"chr-launch-test",
				fmt.Sprint(tc.pciBridges),
				fmt.Sprint(tc.index),
			)
			out, err := cmd.CombinedOutput()
			if tc.errText == "" {
				if err != nil {
					t.Fatalf("index %d rejected: %v\n%s", tc.index, err, out)
				}
				return
			}
			if err == nil || !strings.Contains(string(out), tc.errText) {
				t.Errorf(
					"index %d: want error containing %q, got %v: %s",
					tc.index,
					tc.errText,
					err,
					out,
				)
			}
		})
	}
}
