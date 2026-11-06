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

func TestBuildConnectMethodsKeepsFallbackDefaultFirst(t *testing.T) {
	methods := buildConnectMethods([]string{"/bin/sh"}, "linux")

	if methods[0].Label != "/bin/sh" {
		t.Fatalf("expected default shell first, got %q", methods[0].Label)
	}
	if !strings.Contains(methods[0].Description, "default shell") {
		t.Errorf("expected default shell description, got %q", methods[0].Description)
	}
	if !methods[1].SSH {
		t.Errorf("expected ssh as second method, got %+v", methods[1])
	}

	for _, m := range methods[1:] {
		if m.Label == "/bin/sh" {
			t.Errorf("default shell listed twice: %+v", methods)
		}
	}
}

func TestPickersTreatJAndKAsSearchInput(t *testing.T) {
	keys := func(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

	entries := []connectNodeEntry{{name: "clab-lab1-juniper"}, {name: "clab-lab1-spine-k"}}
	var node tea.Model = newNodePicker(entries)
	node, _ = node.Update(keys('j'))
	node, _ = node.Update(keys('u'))

	nodePicker := node.(nodePickerModel)
	if got := nodePicker.input.Value(); got != "ju" {
		t.Errorf("node picker query = %q, want %q", got, "ju")
	}
	if nodePicker.cursor != 0 {
		t.Errorf("node picker cursor moved to %d", nodePicker.cursor)
	}

	var method tea.Model = newMethodPicker(nil,
		buildConnectMethods([]string{"/bin/sh"}, "linux"))
	method, _ = method.Update(keys('k'))

	if got := method.(methodPickerModel).input.Value(); got != "k" {
		t.Errorf("method picker query = %q, want %q", got, "k")
	}
}

func TestNodePickerRowsDoNotWrap(t *testing.T) {
	entries := []connectNodeEntry{
		{
			name:  "clab-a-rather-long-lab-name-client1",
			kind:  "linux",
			image: "srl-labs/network-multitool:latest",
			ipv6:  "3fff:172:20:20::8/64",
		},
		{
			name:  "clab-a-rather-long-lab-name-srl1",
			kind:  "nokia_srlinux",
			image: "nokia/srlinux:25.10",
		},
	}

	for _, width := range []int{60, 80, 100, 120, 140, 200} {
		var m tea.Model = newNodePicker(entries)
		m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: 40})

		picker := m.(nodePickerModel)
		layout := picker.layout()
		if layout.width() > width {
			t.Errorf("width %d: list is %d wide", width, layout.width())
		}

		view := picker.View()
		if got := lipgloss.Width(view); got > width {
			t.Errorf("width %d: view is %d wide:\n%s", width, got, view)
		}

		for _, entry := range entries {
			if !viewHasRow(view, entry.name[:12], entry.kind) {
				t.Errorf("width %d: %s row is wrapped:\n%s", width, entry.kind, view)
			}
		}
	}
}

// viewHasRow reports whether name and kind share a single line of view.
func viewHasRow(view, name, kind string) bool {
	for line := range strings.Lines(view) {
		if strings.Contains(line, name) && strings.Contains(line, kind) {
			return true
		}
	}
	return false
}

func TestPickersClearScreenOnceDone(t *testing.T) {
	var node tea.Model = newNodePicker([]connectNodeEntry{{name: "clab-lab1-srl1"}})
	node, _ = node.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := node.View(); got != "" {
		t.Errorf("node picker left %q on the screen", got)
	}

	var method tea.Model = newMethodPicker(nil,
		buildConnectMethods([]string{"/bin/sh"}, "linux"))
	method, _ = method.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := method.View(); got != "" {
		t.Errorf("method picker left %q on the screen", got)
	}
}

func TestConnectCmdHasLabelFlag(t *testing.T) {
	o := GetOptions()

	c, err := connectCmd(o)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Flags().Parse([]string{"--label", "clab-node-kind=linux"}); err != nil {
		t.Fatal(err)
	}
	if len(o.Filter.LabelFilter) != 1 || o.Filter.LabelFilter[0] != "clab-node-kind=linux" {
		t.Errorf("unexpected label filter %v", o.Filter.LabelFilter)
	}
}

func TestFormatConnectPorts(t *testing.T) {
	tests := map[string]struct {
		bindings []*clabtypes.GenericPortBinding
		want     string
	}{
		"none": {want: ""},
		"exposed only, sorted numerically": {
			bindings: []*clabtypes.GenericPortBinding{
				{ContainerPort: 8080, Protocol: "tcp"},
				{ContainerPort: 22, Protocol: "tcp"},
				{ContainerPort: 443, Protocol: "tcp"},
			},
			want: "22/tcp, 443/tcp, 8080/tcp",
		},
		"published on IPv4 and IPv6 is shown once": {
			bindings: []*clabtypes.GenericPortBinding{
				{HostIP: "0.0.0.0", HostPort: 18080, ContainerPort: 80, Protocol: "tcp"},
				{HostIP: "::", HostPort: 18080, ContainerPort: 80, Protocol: "tcp"},
			},
			want: "18080→80/tcp",
		},
		"published and exposed": {
			bindings: []*clabtypes.GenericPortBinding{
				{ContainerPort: 443, Protocol: "tcp"},
				{HostPort: 18080, ContainerPort: 80, Protocol: "tcp"},
			},
			want: "18080→80/tcp, 443/tcp",
		},
		"no protocol": {
			bindings: []*clabtypes.GenericPortBinding{{ContainerPort: 22}},
			want:     "22",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := formatConnectPorts(tc.bindings); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNodeDetailWrapsLongPortLists(t *testing.T) {
	short := connectNodeEntry{name: "clab-lab1-client1"}
	long := short
	long.ports = "22/tcp, 80/tcp, 443/tcp, 1180/tcp, 8080/tcp, 11443/tcp"

	detail := renderNodeDetail(&long)
	if got, limit := lipgloss.Width(
		detail,
	), lipgloss.Width(
		renderNodeDetail(&short),
	)+connectPortsWidth; got > limit {
		t.Errorf("details panel is %d wide, want at most %d:\n%s", got, limit, detail)
	}
	for _, port := range []string{"22/tcp", "11443/tcp"} {
		if !strings.Contains(detail, port) {
			t.Errorf("port %q is missing:\n%s", port, detail)
		}
	}
}

func TestPickersResetCursorOnNewQuery(t *testing.T) {
	typeRune := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	down := tea.KeyMsg{Type: tea.KeyDown}

	var node tea.Model = newNodePicker([]connectNodeEntry{
		{name: "clab-lab1-client1"}, {name: "clab-lab1-client2"}, {name: "clab-lab1-client3"},
	})
	node, _ = node.Update(down)
	node, _ = node.Update(down)
	if got := node.(nodePickerModel).cursor; got != 2 {
		t.Fatalf("node picker cursor = %d, want 2", got)
	}

	// messages that leave the query alone, ie. the input's cursor blink, keep the position
	node, _ = node.Update(struct{}{})
	if got := node.(nodePickerModel).cursor; got != 2 {
		t.Errorf("node picker cursor moved to %d without a new query", got)
	}

	node, _ = node.Update(typeRune)
	if got := node.(nodePickerModel).cursor; got != 0 {
		t.Errorf("node picker cursor = %d after a new query, want 0", got)
	}

	var method tea.Model = newMethodPicker(nil,
		buildConnectMethods([]string{"/bin/sh"}, "linux"))
	method, _ = method.Update(down)
	method, _ = method.Update(typeRune)
	if got := method.(methodPickerModel).cursor; got != 0 {
		t.Errorf("method picker cursor = %d after a new query, want 0", got)
	}
}
