// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// Dash stands in for a value that is absent.
const Dash = "—"

const (
	kilo = 1000.0
	kibi = 1024.0
	// oneDecimalBelow is the value under which a scaled number keeps one decimal.
	oneDecimalBelow = 10.0
)

// Rate formats a traffic rate given in bits per second in at most four cells, ie. 1.2M.
func Rate(bps float64) string {
	return scaled(bps, kilo, []string{"", "k", "M", "G", "T"})
}

// Bytes formats an amount of memory in at most four cells, ie. 712M.
func Bytes(n uint64) string {
	return scaled(float64(n), kibi, []string{"", "K", "M", "G", "T"})
}

func scaled(v, base float64, units []string) string {
	if v < 1 {
		return "0"
	}

	unit := 0
	for v >= base && unit < len(units)-1 {
		v /= base
		unit++
	}

	if unit > 0 && v < oneDecimalBelow {
		return strconv.FormatFloat(v, 'f', 1, 64) + units[unit]
	}

	return strconv.FormatFloat(v, 'f', 0, 64) + units[unit]
}

// Percent formats a percentage without decimals.
func Percent(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64) + "%"
}

// Duration formats a duration compactly: 42s, 3m12s, 2h05m or 4d.
func Duration(d time.Duration) string {
	d = d.Round(time.Second)

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60) //nolint:mnd
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60) //nolint:mnd
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24) //nolint:mnd // hours per day
	}
}

// Ports renders port bindings the way `docker ps` does: published ports as
// host→container/protocol, exposed ones as container/protocol. Bindings that differ only
// by the host address (ie. IPv4 and IPv6) are shown once.
func Ports(ports []clabinternaltuibackend.Port) string {
	sorted := slices.Clone(ports)
	slices.SortStableFunc(sorted, func(a, b clabinternaltuibackend.Port) int {
		return cmp.Or(
			cmp.Compare(a.ContainerPort, b.ContainerPort),
			cmp.Compare(a.HostPort, b.HostPort),
		)
	})

	out := make([]string, 0, len(sorted))

	for _, p := range sorted {
		port := strconv.Itoa(p.ContainerPort)
		if p.HostPort != 0 {
			port = strconv.Itoa(p.HostPort) + "→" + port
		}

		if p.Protocol != "" {
			port += "/" + p.Protocol
		}

		out = append(out, port)
	}

	return strings.Join(slices.Compact(out), ", ")
}

// ShortImage returns a shortened image name: the digest is dropped and only the last two
// path components are kept.
func ShortImage(image string) string {
	if i := strings.Index(image, "@"); i != -1 {
		image = image[:i]
	}

	const keep = 2

	parts := strings.Split(image, "/")
	if len(parts) <= keep {
		return image
	}

	return strings.Join(parts[len(parts)-keep:], "/")
}

// Netem summarizes the impairments set on an interface, ie. "50ms ±5ms 1% loss".
func Netem(i *clabinternaltuibackend.Impairment) string {
	if i.IsZero() {
		return Dash
	}

	var parts []string

	if i.Delay > 0 {
		delay := i.Delay.String()
		if i.Jitter > 0 {
			delay += " ±" + i.Jitter.String()
		}

		parts = append(parts, delay)
	}

	if i.LossPct > 0 {
		parts = append(parts, trimFloat(i.LossPct)+"% loss")
	}

	if i.RateKbit > 0 {
		parts = append(parts, Rate(float64(i.RateKbit)*kilo)+"bit")
	}

	if i.CorruptionPct > 0 {
		parts = append(parts, trimFloat(i.CorruptionPct)+"% corrupt")
	}

	return strings.Join(parts, " ")
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// HomePath shortens a path inside the user's home directory with a tilde.
func HomePath(path, home string) string {
	if home == "" || path == "" {
		return path
	}

	if path == home {
		return "~"
	}

	if rest, ok := strings.CutPrefix(path, strings.TrimSuffix(home, "/")+"/"); ok {
		return "~/" + rest
	}

	return path
}
