// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

// keyActions opens the menu of the actions that apply to the selection.
const keyActions = "a"

// action is something the user can do with what is selected. The key handling, the hints
// at the bottom of the screen and the action menu are all made from the same list of
// actions, so that they cannot disagree.
type action struct {
	// key is the key that runs the action.
	key string
	// name is the short name shown next to the key in the hints.
	name string
	// desc says what the action does, for the action menu.
	desc string
	// off tells why the action is not available, empty when it is.
	off string
	run func() tea.Cmd
}

// actor is implemented by pages that offer actions on their selection.
type actor interface {
	// actions returns a title for the selection and what can be done with it.
	actions(m *Model) (title string, list []action)
}

// runAction runs the action bound to a key. An action that is not available tells why
// instead. It reports whether the key belongs to an action at all.
func (m *Model) runAction(list []action, key string) (bool, tea.Cmd) {
	for i := range list {
		if list[i].key != key {
			continue
		}

		if list[i].off != "" {
			m.notifyf(clabinternaltuitheme.LevelWarning, "%s: %s", list[i].name, list[i].off)

			return true, nil
		}

		return true, list[i].run()
	}

	return false, nil
}

// actionHints returns the hints of the actions that are available.
func actionHints(list []action) []hint {
	hints := make([]hint, 0, len(list))

	for i := range list {
		if list[i].off == "" {
			hints = append(hints, hint{key: list[i].key, desc: list[i].name})
		}
	}

	return hints
}

// actionMenu lists the actions to choose from. The ones that are not available are shown
// with the reason, so that the menu also answers why something cannot be done.
func (m *Model) actionMenu(title string, list []action) tea.Cmd {
	if len(list) == 0 {
		return nil
	}

	items := make([]menuItem, 0, len(list))

	for i := range list {
		a := list[i]
		items = append(items, menuItem{
			shortcut: a.key,
			label:    a.name,
			desc:     a.desc,
			disabled: a.off,
			run:      a.run,
		})
	}

	return m.choose(title, items)
}

// firstOf returns the first of the reasons that is not empty.
func firstOf(reasons ...string) string {
	for _, r := range reasons {
		if r != "" {
			return r
		}
	}

	return ""
}

// unless returns the reason when the condition does not hold.
func unless(ok bool, reason string) string {
	if ok {
		return ""
	}

	return reason
}

// capReason returns why a group of operations is unavailable, empty when it is not.
func (m *Model) capReason(ok bool, capability string) string {
	if ok {
		return ""
	}

	return m.host.Caps.Reason(capability)
}

// showMap opens the topology map of a lab.
func (m *Model) showMap(lab string) tea.Cmd {
	m.scope = lab
	m.goTo(tabMap)

	return nil
}

// labActions returns what can be done with a lab.
func (m *Model) labActions(lab *clabinternaltuibackend.Lab) (string, []action) {
	if lab == nil {
		return "", nil
	}

	lifecycle := m.capReason(m.host.Caps.Lifecycle, "lifecycle")
	deployed := unless(lab.Deployed, "the lab is not deployed")
	hasFile := unless(lab.TopoFile != "" && !lab.TopoMissing, "the topology file is not available")

	deploy := "deploy the lab"
	if lab.Deployed {
		deploy = "apply the changes of the topology file"
	}

	return lab.Name, []action{
		{
			key: "t", name: "map", desc: "show the topology",
			run: func() tea.Cmd { return m.showMap(lab.Name) },
		},
		{
			key: "D", name: "deploy", desc: deploy, off: firstOf(hasFile, lifecycle),
			run: func() tea.Cmd { return m.deployLab(lab) },
		},
		{
			key: "X", name: "destroy", desc: "stop and remove the nodes of the lab",
			off: firstOf(deployed, lifecycle),
			run: func() tea.Cmd { return m.destroyLab(lab) },
		},
		{
			key: "R", name: "redeploy", desc: "destroy the lab and deploy it again",
			off: firstOf(deployed, hasFile, lifecycle),
			run: func() tea.Cmd { return m.redeployLab(lab) },
		},
		{
			key:  "W",
			name: "save",
			desc: "save the running configuration of all nodes",
			off:  deployed,
			run:  func() tea.Cmd { return m.saveNodes(lab, nil) },
		},
		{
			key: "x", name: "run", desc: "run a command in all nodes", off: deployed,
			run: func() tea.Cmd { return m.runCommandIn(lab.Name, nodePointers(lab.Nodes)) },
		},
		{
			key: "U", name: "start", desc: "start all nodes", off: firstOf(deployed, lifecycle),
			run: func() tea.Cmd { return m.labNodeAction(lab, clabinternaltuibackend.NodeStart) },
		},
		{
			key: "O", name: "stop", desc: "stop all nodes", off: firstOf(deployed, lifecycle),
			run: func() tea.Cmd { return m.labNodeAction(lab, clabinternaltuibackend.NodeStop) },
		},
		{
			key: "H", name: "share", desc: "share a terminal with sshx or gotty", off: deployed,
			run: func() tea.Cmd { return m.shareMenu(lab) },
		},
		{
			key: "y", name: "copy path", desc: "copy the path of the topology file",
			off: unless(lab.TopoFile != "", "the path is not known"),
			run: func() tea.Cmd { return m.copy("topology path", lab.TopoFile) },
		},
	}
}

// nodeActions returns what can be done with nodes. current is the node under the cursor,
// nodes the ones an action applies to: the marked ones, or just the current one.
func (m *Model) nodeActions(
	current *clabinternaltuibackend.Node, nodes []*clabinternaltuibackend.Node,
) (string, []action) {
	if current == nil || len(nodes) == 0 {
		return "", nil
	}

	lifecycle := m.capReason(m.host.Caps.Lifecycle, "lifecycle")
	running := unless(current.Readiness().Running(), current.Name+" is not running")

	anyRunning, anyStopped := false, false

	for _, n := range nodes {
		if n.Readiness().Running() {
			anyRunning = true
		} else {
			anyStopped = true
		}
	}

	title := current.Name
	if len(nodes) > 1 {
		title = fmt.Sprintf("%d nodes", len(nodes))
	}

	return title, []action{
		{
			key: "s", name: "shell", desc: "open the command line of the node", off: running,
			run: func() tea.Cmd { return m.connectDefault(current, false) },
		},
		{
			key: "S", name: "ssh", desc: "open an SSH session", off: running,
			run: func() tea.Cmd { return m.connectDefault(current, true) },
		},
		{
			key: "c", name: "connect", desc: "choose a shell or SSH to connect with", off: running,
			run: func() tea.Cmd { return m.connectMenu(current) },
		},
		{
			key: "l", name: "logs", desc: "follow the log",
			run: func() tea.Cmd { return m.openLogs(current.NodeRef) },
		},
		{
			key: "d", name: "describe", desc: "show every detail of the node",
			run: func() tea.Cmd { return m.describe(current.NodeRef) },
		},
		{
			key: "x", name: "run", desc: "run a command in the node",
			off: unless(anyRunning, "no running node selected"),
			run: func() tea.Cmd { return m.runCommandIn(current.Lab, nodes) },
		},
		{
			key: "R", name: "restart", desc: "restart the node",
			off: firstOf(lifecycle, unless(anyRunning, "not running")),
			run: func() tea.Cmd { return m.nodeAction(clabinternaltuibackend.NodeRestart, nodes) },
		},
		{
			key: "O", name: "stop", desc: "stop the node",
			off: firstOf(lifecycle, unless(anyRunning, "not running")),
			run: func() tea.Cmd { return m.nodeAction(clabinternaltuibackend.NodeStop, nodes) },
		},
		{
			key: "U", name: "start", desc: "start the node",
			off: firstOf(lifecycle, unless(anyStopped, "already running")),
			run: func() tea.Cmd { return m.nodeAction(clabinternaltuibackend.NodeStart, nodes) },
		},
		{
			key: "W", name: "save", desc: "save the running configuration to the lab directory",
			off: unless(anyRunning, "no running node selected"),
			run: func() tea.Cmd { return m.saveSelected(nodes) },
		},
		{
			key: "t", name: "map", desc: "show the node in the topology",
			run: func() tea.Cmd { return m.showMap(current.Lab) },
		},
		{
			key: "y", name: "copy", desc: "copy the name, an address or another value",
			run: func() tea.Cmd { return m.copyNodeMenu(current) },
		},
	}
}

// interfaceActions returns what can be done with interfaces.
func (m *Model) interfaceActions(ifaces []*clabinternaltuibackend.Interface) (string, []action) {
	if len(ifaces) == 0 {
		return "", nil
	}

	title := ifaces[0].Node.Name + ":" + ifaces[0].Name
	if len(ifaces) > 1 {
		title = fmt.Sprintf("%d interfaces", len(ifaces))
	}

	impaired := false

	for _, iface := range ifaces {
		if !iface.Netem.IsZero() {
			impaired = true
		}
	}

	netem := m.capReason(m.host.Caps.Netem, "netem")

	return title, []action{
		{
			key: "n", name: "impair", desc: "add delay, jitter, loss, a rate limit or corruption",
			off: netem,
			run: func() tea.Cmd { return m.netemForm(ifaces) },
		},
		{
			key: "N", name: "unimpair", desc: "remove the impairments",
			off: firstOf(netem, unless(impaired, "no impairment is set")),
			run: func() tea.Cmd { return m.netemReset(ifaces) },
		},
		{
			key: "C", name: "capture", desc: "capture packets into Wireshark or a file",
			off: m.capReason(m.host.Caps.Capture, "capture"),
			run: func() tea.Cmd { return m.captureForm(ifaces) },
		},
		{
			key: "m", name: "copy mac", desc: "copy the MAC address",
			off: unless(ifaces[0].MAC != "", "the address is not known"),
			run: func() tea.Cmd { return m.copy("MAC address", ifaces[0].MAC) },
		},
	}
}
