// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata")

type goldenCase struct {
	name  string
	graph Graph
	opts  Options
	state func(Graph) State
	sel   Selection
}

func goldenCases() []goldenCase {
	none := Selection{Edge: -1}
	normal := Options{Zoom: ZoomNormal}

	return []goldenCase{
		{"pair", pair(), normal, allUp, none},
		{"clos-2x4", clos(2, 4), normal, allUp, none},
		{"three-tier", threeTier(), normal, allUp, none},
		{"ring-6", ring(6), normal, allUp, none},
		{"mesh-4", fullMesh(4), normal, allUp, none},
		{"star-8", star(8), normal, allUp, none},
		{"parallel", parallel(), normal, allUp, none},
		{"long-edge", longEdge(), normal, allUp, none},
		{"two-components", twoComponents(), normal, allUp, none},
		{"fat-tree-100", fatTree(), normal, allUp, none},
		{"fan-out-bus", fanOut(12), normal, allUp, none},
		{"service-provider", serviceProvider(), normal, allUp, none},
		{"metro-ring", metroRing(true), normal, allUp, none},
		{"ring-8", metroRing(false), normal, allUp, none},
		{"campus", campus(), normal, allUp, none},
		{"three-tier-compact", threeTier(), Options{Zoom: ZoomCompact}, allUp, none},
		{"leaf-spine-4x12-compact", leafSpine(4, 12), Options{Zoom: ZoomCompact}, allUp, none},
		{"three-tier-detail", withKinds(threeTier()), Options{Zoom: ZoomDetail}, loadNotes, none},
		{"three-tier-ascii", threeTier(), Options{Zoom: ZoomNormal, ASCII: true}, allUp, none},
		{"three-tier-live", threeTier(), normal, liveState, Selection{Node: "leaf1", Edge: 0}},
		{"three-tier-names", threeTier(), normal, ifaceNames, none},
		{"three-tier-names-detail", threeTier(), Options{Zoom: ZoomDetail}, ifaceNames, none},
		{"pair-names", pair(), Options{Zoom: ZoomDetail}, ifaceNames, none},
		{"service-provider-names", serviceProvider(), normal, ifaceNames, none},
		{"metro-ring-names", metroRing(true), normal, ifaceNames, none},
		{"fan-out-bus-names", fanOut(12), normal, ifaceNames, none},
		{
			"three-tier-names-ascii",
			threeTier(),
			Options{Zoom: ZoomDetail, ASCII: true},
			ifaceNames,
			none,
		},
		{
			"pair-room",
			pair(),
			Options{Zoom: ZoomNormal, Room: map[[2]string]int{{"srl2", "srl1"}: 25}},
			namesAndRates,
			none,
		},
		{"three-tier-rows", threeTier(), Options{Zoom: ZoomNormal, RowGap: 2}, namesAndRates, none},
		{
			"two-components-room",
			twoComponents(),
			Options{Zoom: ZoomNormal, RowGap: 2, Margin: 6},
			namesAndRates,
			none,
		},
	}
}

// namesAndRates names the ends of every edge and writes a rate on each.
func namesAndRates(g Graph) State {
	st := ifaceNames(g)
	st.EdgeLabels = map[int]string{}

	for id := range st.EndLabels {
		st.EdgeLabels[id] = "1.2M"
	}

	return st
}

// liveState is a fabric in use: one link down, one unknown, rates on some edges.
func liveState(g Graph) State {
	st := allUp(g)
	st.Links[4] = LinkDown
	st.Links[7] = LinkUnknown
	st.EdgeLabels = map[int]string{0: "2.6M", 8: "1.2M", 9: "880k", 5: "14k"}

	return st
}

// loadNotes gives some nodes a load figure, as the detail zoom shows it.
func loadNotes(g Graph) State {
	st := allUp(g)
	st.NodeNotes = map[string]string{
		"spine1": "14% 712M", "leaf1": "3% 1.1G", "leaf2": "100% 1024M", "client1": "1% 24M",
	}

	return st
}

func TestGolden(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			l := Compute(tc.graph, tc.opts)
			got := strings.Join(l.Paint(tc.state(tc.graph), tc.sel).Lines(), "\n") + "\n"
			path := filepath.Join("testdata", tc.name+".golden")

			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run the test with -update to create it)", err)
			}

			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("drawing differs from %s (-want +got):\n%s", path, diff)
			}
		})
	}
}
