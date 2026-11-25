// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The traffic in the header stays in place as it changes, also when no traffic flows: a
// header that comes and goes with every sample blinks.
func TestHeaderRatesKeepTheirPlace(t *testing.T) {
	m, _ := newTestModel(t, 160, 40)

	setRates := func(bps float64, sampled bool) {
		for i := range m.labs {
			for n := range m.labs[i].Nodes {
				m.labs[i].Nodes[n].HasStats = sampled
				m.labs[i].Nodes[n].RxBps = bps
				m.labs[i].Nodes[n].TxBps = bps
			}
		}
	}

	setRates(0, false)

	if header := ansi.Strip(m.header()); strings.Contains(header, arrowRx) {
		t.Errorf("rates shown before the first sample: %q", header)
	}

	var headers []string

	for _, bps := range []float64{0, 1_234_567, 0, 42} {
		setRates(bps, true)
		headers = append(headers, ansi.Strip(m.header()))
	}

	for _, header := range headers {
		if !strings.Contains(header, arrowRx) {
			t.Errorf("rates missing: %q", header)
		} else if rateColumn(header) != rateColumn(headers[0]) {
			t.Errorf("header shifts with the traffic:\n%q\n%q", headers[0], header)
		}
	}
}

// rateColumn returns the cell the rates start at in a header.
func rateColumn(header string) int {
	before, _, _ := strings.Cut(header, arrowRx)

	return ansi.StringWidth(before)
}
