// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

// daemonsTemplate is the daemons file shipped by FRR 10.7.1 with the
// enable/disable toggles turned into template actions. The per-daemon
// *_options lines are kept verbatim, as they carry the VTY bind addresses the
// daemons are started with.
//
//go:embed daemons.tmpl
var daemonsTemplate string

// configurableDaemons are the daemons that the FRR daemons file can switch on
// and off. Keep in sync with daemons.tmpl.
var configurableDaemons = []string{
	"bgpd", "ospfd", "ospf6d", "ripd", "ripngd", "isisd", "pimd", "pim6d",
	"ldpd", "nhrpd", "eigrpd", "babeld", "sharpd", "pbrd", "bfdd", "fabricd",
	"vrrpd", "pathd",
}

// alwaysOnDaemons are started by FRR regardless of the daemons file. Listing
// them in the topology is allowed but has no effect, so that users need not
// know which daemons are special.
var alwaysOnDaemons = []string{"zebra", "staticd", "mgmtd", "watchfrr"}

// renderDaemons produces the contents of FRR's /etc/frr/daemons file.
//
// An empty list enables every configurable daemon, so that a lab works without
// the user having to think about which daemons their config needs. A non-empty
// list enables exactly the daemons named in it.
func renderDaemons(daemons []string) (string, error) {
	enabled := make(map[string]bool, len(configurableDaemons))

	if len(daemons) == 0 {
		for _, d := range configurableDaemons {
			enabled[d] = true
		}
	} else {
		for _, d := range daemons {
			switch {
			case slices.Contains(configurableDaemons, d):
				enabled[d] = true
			case slices.Contains(alwaysOnDaemons, d):
				// Always started by FRR, nothing to switch on.
			default:
				return "", fmt.Errorf(
					"unknown FRR daemon %q, expected one of: %s",
					d,
					strings.Join(slices.Concat(configurableDaemons, alwaysOnDaemons), ", "),
				)
			}
		}
	}

	tpl, err := template.New("daemons").Parse(daemonsTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse FRR daemons template: %w", err)
	}

	var buf strings.Builder

	err = tpl.Execute(&buf, struct{ Daemons map[string]bool }{Daemons: enabled})
	if err != nil {
		return "", fmt.Errorf("failed to render FRR daemons file: %w", err)
	}

	return buf.String(), nil
}
