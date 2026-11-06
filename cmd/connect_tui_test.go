// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestFuzzyMatchEmptyQueryReturnsAll(t *testing.T) {
	got := fuzzyMatch("", []string{"a", "b", "c"})
	if len(got) != 3 {
		t.Fatalf("expected all indexes, got %v", got)
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("expected identity mapping, got %v", got)
		}
	}
}

func TestFuzzyMatchFiltersAndKeepsOriginalIndexes(t *testing.T) {
	data := []string{"clab-lab1-leaf1", "clab-lab1-spine1", "clab-other-leaf2"}
	got := fuzzyMatch("leaf", data)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %v", got)
	}
	if data[got[0]] != "clab-lab1-leaf1" {
		t.Fatalf("unexpected first match %q", data[got[0]])
	}
}

func TestHighlightMatchedWrapsMatches(t *testing.T) {
	style := lipgloss.NewStyle().Bold(true)
	got := highlightMatched("srl1", []int{0, 1, 2}, style)
	if !strings.Contains(got, "srl") {
		t.Fatalf("expected highlighted substring, got %q", got)
	}
}

func TestBuildConnectMethodsIncludesSSHAndDefaultFirst(t *testing.T) {
	defaults := []string{"/opt/srlinux/bin/sr_cli"}
	methods := buildConnectMethods(defaults, "nokia_srlinux")
	if len(methods) < 3 {
		t.Fatalf("expected at least 3 methods, got %d", len(methods))
	}
	if methods[0].Label != "/opt/srlinux/bin/sr_cli" {
		t.Fatalf("expected default shell first, got %q", methods[0].Label)
	}
	if !methods[1].SSH {
		t.Fatalf("expected ssh as second method, got %+v", methods[1])
	}
}

func TestNodePickerViewRenders(t *testing.T) {
	entries := []connectNodeEntry{
		{
			name:  "clab-lab1-srl1",
			kind:  "nokia_srlinux",
			lab:   "lab1",
			image: "nokia/srlinux:25.10",
			state: "running",
			ipv4:  "172.20.20.2/24",
			shell: "/opt/srlinux/bin/sr_cli",
		},
		{
			name:  "clab-lab1-client1",
			kind:  "linux",
			lab:   "lab1",
			image: "srl-labs/network-multitool",
			state: "running",
			ipv4:  "172.20.20.3/24",
			shell: "/bin/bash",
		},
	}

	m := newNodePicker(entries)
	// Simulate a window size so the layout code has dimensions to work with.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := updated.(nodePickerModel).View()

	if !strings.Contains(view, "clab-lab1-srl1") {
		t.Fatalf("expected node name in view, got:\n%s", view)
	}
	if !strings.Contains(view, "node details") {
		t.Fatalf("expected details panel in view, got:\n%s", view)
	}
}

func TestFoldInternalContainers(t *testing.T) {
	containers := []clabruntime.GenericContainer{
		{
			Names:  []string{"clab-lab1-srl1"},
			Labels: map[string]string{clabconstants.LongName: "clab-lab1-srl1"},
		},
		{
			Names: []string{"clab-lab1-srl1-netns"},
			Labels: map[string]string{
				clabconstants.LongName:         "clab-lab1-srl1-netns",
				clabconstants.InternalNode:     "true",
				clabconstants.RootNodeLongName: "clab-lab1-srl1",
			},
			NetworkName:     "clab",
			NetworkSettings: clabruntime.GenericMgmtIPs{IPv4addr: "172.20.20.2", IPv4pLen: 24},
			Ports:           []*clabtypes.GenericPortBinding{{HostPort: 2222}},
		},
		{
			Names:           []string{"clab-lab1-client1"},
			Labels:          map[string]string{clabconstants.LongName: "clab-lab1-client1"},
			NetworkSettings: clabruntime.GenericMgmtIPs{IPv4addr: "172.20.20.3", IPv4pLen: 24},
		},
	}

	got := foldInternalContainers(containers)
	if len(got) != 2 {
		t.Fatalf("expected internal container to be removed, got %d containers", len(got))
	}

	srl := got[0]
	if srl.Names[0] != "clab-lab1-srl1" {
		t.Fatalf("unexpected first container %q", srl.Names[0])
	}
	if srl.NetworkName != "clab" || srl.NetworkSettings.IPv4addr != "172.20.20.2" {
		t.Errorf("holder network settings not copied: %q %+v", srl.NetworkName, srl.NetworkSettings)
	}
	if len(srl.Ports) != 1 || srl.Ports[0].HostPort != 2222 {
		t.Errorf("holder ports not copied: %+v", srl.Ports)
	}

	if got[1].NetworkSettings.IPv4addr != "172.20.20.3" {
		t.Errorf("unrelated container changed: %+v", got[1].NetworkSettings)
	}
}
