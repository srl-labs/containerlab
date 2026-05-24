// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"strings"
	"testing"

	"github.com/awalterschulze/gographviz"
)

func TestDotGraphUsesQuotedGraphName(t *testing.T) {
	t.Parallel()

	graphName := dotIdentifier("sample.clab")

	g := gographviz.NewGraph()
	if err := g.SetName(graphName); err != nil {
		t.Fatalf("SetName failed: %v", err)
	}

	if err := g.SetDir(false); err != nil {
		t.Fatalf("SetDir failed: %v", err)
	}

	if err := g.AddNode(graphName, "node1", nil); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	got := g.String()
	if !strings.HasPrefix(got, `graph "sample.clab" {`) {
		t.Fatalf("unexpected dot graph header: %q", got)
	}
}
