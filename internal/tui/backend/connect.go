// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package backend

import (
	"fmt"
	"strings"
)

// DefaultConnectMethods assembles the ways to open a session on a node, in the order they
// are offered: the default shell of the node's kind, SSH using the lab's ssh config, and
// the common shells as fallbacks.
func DefaultConnectMethods(defaultShell []string, kind string) []ConnectMethod {
	if len(defaultShell) == 0 {
		defaultShell = []string{"/bin/sh"}
	}

	seen := map[string]bool{}

	var methods []ConnectMethod

	add := func(m ConnectMethod) {
		key := m.Label
		if m.SSH {
			key = "ssh"
		}

		if seen[key] {
			return
		}

		seen[key] = true

		methods = append(methods, m)
	}

	add(ConnectMethod{
		Label:       strings.Join(defaultShell, " "),
		Description: fmt.Sprintf("default shell for kind %q", kind),
		Shell:       defaultShell,
	})
	add(ConnectMethod{
		Label:       "ssh",
		Description: "ssh using the lab's ssh config",
		SSH:         true,
	})
	add(ConnectMethod{Label: "/bin/bash", Description: "bash", Shell: []string{"/bin/bash"}})
	add(ConnectMethod{
		Label:       "/bin/bash -l",
		Description: "bash login shell",
		Shell:       []string{"/bin/bash", "-l"},
	})
	add(ConnectMethod{Label: "/bin/sh", Description: "POSIX sh", Shell: []string{"/bin/sh"}})
	add(ConnectMethod{Label: "/bin/ash", Description: "alpine ash", Shell: []string{"/bin/ash"}})

	return methods
}
