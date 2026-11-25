// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package backend

import (
	"strconv"
	"strings"
)

// Container states as reported by the runtimes.
const (
	stateRunning    = "running"
	stateCreated    = "created"
	stateRestarting = "restarting"
	stateRemoving   = "removing"
	statePaused     = "paused"
	stateExited     = "exited"
	stateDead       = "dead"
)

// Readiness derives how usable the node is from the container state and the health
// reported in its status text.
func (n *Node) Readiness() Readiness {
	status := strings.ToLower(n.Status)

	switch strings.ToLower(n.State) {
	case stateRunning:
		switch {
		case strings.Contains(status, "health: starting"):
			return Starting
		case strings.Contains(status, "unhealthy"):
			return Unhealthy
		default:
			return Ready
		}
	case stateCreated, stateRestarting:
		return Starting
	case stateRemoving:
		return Stopping
	case statePaused:
		return Paused
	case stateDead:
		return Failed
	case stateExited:
		if code, ok := exitCode(status); ok && code != 0 {
			return Failed
		}

		return Stopped
	case "":
		return Unknown
	default:
		return Stopped
	}
}

// exitCode extracts the exit code from a status text such as "Exited (137) 2 hours ago".
func exitCode(status string) (int, bool) {
	_, rest, found := strings.Cut(status, "(")
	if !found {
		return 0, false
	}

	digits, _, found := strings.Cut(rest, ")")
	if !found {
		return 0, false
	}

	code, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return 0, false
	}

	return code, true
}

// uptimeUnits maps the unit words of a runtime status text to their short form.
var uptimeUnits = []struct{ word, short string }{
	{"second", "s"},
	{"minute", "m"},
	{"hour", "h"},
	{"day", "d"},
	{"week", "w"},
	{"month", "mo"},
	{"year", "y"},
}

// Uptime returns how long the node has been up in a compact form, ie. "3h" for a status of
// "Up 3 hours (healthy)". It is empty for nodes that are not running.
func (n *Node) Uptime() string {
	rest, found := strings.CutPrefix(n.Status, "Up ")
	if !found {
		return ""
	}

	if text, _, hasHealth := strings.Cut(rest, " ("); hasHealth {
		rest = text
	}

	lower := strings.ToLower(strings.TrimSpace(rest))
	if strings.HasPrefix(lower, "less than") {
		return "1s"
	}

	lower = strings.TrimPrefix(lower, "about ")
	fields := strings.Fields(lower)

	if len(fields) < 2 { //nolint:mnd // an amount and a unit
		return ""
	}

	amount := fields[0]
	if amount == "a" || amount == "an" {
		amount = "1"
	}

	for _, u := range uptimeUnits {
		if strings.HasPrefix(fields[1], u.word) {
			return amount + u.short
		}
	}

	return ""
}
