// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// listedTargets is the number of targets a confirmation names before summing up the rest.
const listedTargets = 6

// sessionEndedMsg tells that an interactive session returned the terminal.
type sessionEndedMsg struct {
	node    string
	err     error
	started time.Time
}

func nodePointers(nodes []clabinternaltuibackend.Node) []*clabinternaltuibackend.Node {
	out := make([]*clabinternaltuibackend.Node, len(nodes))
	for i := range nodes {
		out[i] = &nodes[i]
	}

	return out
}

// allowed reports whether a group of operations is available, telling the user why when
// it is not.
func (m *Model) allowed(ok bool, capability string) bool {
	if ok {
		return true
	}

	m.notifyf(clabinternaltuitheme.LevelWarning, "%s", m.host.Caps.Reason(capability))

	return false
}

// started reports the outcome of asking the backend for a job. Jobs worth watching open
// their output, the quick ones only leave a notification.
func (m *Model) started(id clabinternaltuibackend.JobID, err error, show bool) {
	if err != nil {
		m.notifyf(clabinternaltuitheme.LevelError, "%v", err)

		return
	}

	m.jobs = m.be.Jobs()
	m.watched[id] = true

	if show {
		m.push(newJobPage(m, id))

		return
	}

	if job, ok := m.be.Job(id); ok {
		m.notifyf(clabinternaltuitheme.LevelInfo, "started: %s", job.Title)
	}
}

func (m *Model) deployLab(lab *clabinternaltuibackend.Lab) tea.Cmd {
	if !m.allowed(m.host.Caps.Lifecycle, "lifecycle") {
		return nil
	}

	if lab.TopoFile == "" || lab.TopoMissing {
		m.notifyf(
			clabinternaltuitheme.LevelWarning,
			"the topology file of %s is not available",
			lab.Name,
		)

		return nil
	}

	command := func(v formValues) string {
		line := "containerlab deploy -t " + clabinternaltuiui.HomePath(lab.TopoFile, m.opts.Home)
		if v.Bool("reconfigure") {
			line += " --reconfigure"
		}

		return line
	}

	submit := func(v formValues) tea.Cmd {
		id, err := m.be.Deploy(clabinternaltuibackend.DeployRequest{
			Lab: lab.Name, TopoFile: lab.TopoFile, Reconfigure: v.Bool("reconfigure"),
		})
		m.started(id, err, true)

		return nil
	}

	form := newForm("Deploy "+lab.Name, "Deploy", submit).withPreview(command)

	if lab.Deployed {
		form.text(
			"The lab is deployed. Deploying again applies the changes made to the",
			"topology file, leaving the nodes that did not change alone.",
		).toggle("reconfigure", "start over: regenerate the lab directory (--reconfigure)")
	} else {
		form.text(fmt.Sprintf("Deploy the %d nodes of this topology.", lab.DefinedNodes))
	}

	return m.ask(form)
}

func (m *Model) destroyLab(lab *clabinternaltuibackend.Lab) tea.Cmd {
	if !lab.Deployed {
		m.notifyf(clabinternaltuitheme.LevelInfo, "%s is not deployed", lab.Name)

		return nil
	}

	if !m.allowed(m.host.Caps.Lifecycle, "lifecycle") {
		return nil
	}

	command := func(v formValues) string {
		line := "containerlab destroy --name " + lab.Name
		if v.Bool("cleanup") {
			line += " --cleanup"
		}

		if v.Bool("graceful") {
			line += " --graceful"
		}

		return line
	}

	submit := func(v formValues) tea.Cmd {
		id, err := m.be.Destroy(clabinternaltuibackend.DestroyRequest{
			Lab: lab.Name, TopoFile: lab.TopoFile,
			Cleanup: v.Bool("cleanup"), Graceful: v.Bool("graceful"),
		})
		m.started(id, err, true)

		return nil
	}

	return m.ask(newForm("Destroy "+lab.Name, "Destroy", submit).
		dangerous().
		text(fmt.Sprintf("%d nodes will be removed. Configuration that was not saved is lost.",
			len(lab.Nodes))).
		toggle("cleanup", "also remove the lab directory (--cleanup)").
		toggle("graceful", "stop the nodes gracefully first (--graceful)").
		withPreview(command))
}

func (m *Model) redeployLab(lab *clabinternaltuibackend.Lab) tea.Cmd {
	if !lab.Deployed {
		return m.deployLab(lab)
	}

	if !m.allowed(m.host.Caps.Lifecycle, "lifecycle") {
		return nil
	}

	command := func(v formValues) string {
		line := "containerlab redeploy -t " + clabinternaltuiui.HomePath(lab.TopoFile, m.opts.Home)
		if v.Bool("cleanup") {
			line += " --cleanup"
		}

		return line
	}

	submit := func(v formValues) tea.Cmd {
		id, err := m.be.Redeploy(clabinternaltuibackend.RedeployRequest{
			Lab: lab.Name, TopoFile: lab.TopoFile, Cleanup: v.Bool("cleanup"),
		})
		m.started(id, err, true)

		return nil
	}

	return m.ask(newForm("Redeploy "+lab.Name, "Redeploy", submit).
		dangerous().
		text(
			fmt.Sprintf("The %d nodes are destroyed and deployed again.", len(lab.Nodes)),
			"Configuration that was not saved is lost.",
		).
		toggle("cleanup", "also remove the lab directory (--cleanup)").
		withPreview(command))
}

// targetList names the targets of an action, summing up the tail of a long list.
func targetList(names []string) string {
	if len(names) <= listedTargets {
		return strings.Join(names, ", ")
	}

	return strings.Join(names[:listedTargets], ", ") +
		fmt.Sprintf(" and %d more", len(names)-listedTargets)
}

// byLab groups nodes by the lab they belong to, keeping the labs in a stable order.
func byLab(nodes []*clabinternaltuibackend.Node) (labs []string, grouped map[string][]string) {
	grouped = map[string][]string{}

	for _, n := range nodes {
		if _, seen := grouped[n.Lab]; !seen {
			labs = append(labs, n.Lab)
		}

		grouped[n.Lab] = append(grouped[n.Lab], n.Name)
	}

	sort.Strings(labs)

	return labs, grouped
}

func nodeNames(nodes []*clabinternaltuibackend.Node) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}

	return names
}

// nodeAction starts, stops or restarts nodes after asking for confirmation.
func (m *Model) nodeAction(
	action clabinternaltuibackend.NodeAction, nodes []*clabinternaltuibackend.Node,
) tea.Cmd {
	if !m.allowed(m.host.Caps.Lifecycle, "lifecycle") {
		return nil
	}

	labs, grouped := byLab(nodes)
	verb := strings.ToUpper(string(action)[:1]) + string(action)[1:]

	submit := func(formValues) tea.Cmd {
		for _, lab := range labs {
			id, err := m.be.NodeAction(clabinternaltuibackend.NodeActionRequest{
				Lab: lab, Action: action, Nodes: grouped[lab],
			})
			m.started(id, err, false)
		}

		return nil
	}

	form := newForm(fmt.Sprintf("%s %d node(s)", verb, len(nodes)), verb, submit).
		text(targetList(nodeNames(nodes))).
		withPreview(func(formValues) string {
			return fmt.Sprintf("containerlab %s --name %s --node %s",
				action, labs[0], strings.Join(grouped[labs[0]], ","))
		})

	if action != clabinternaltuibackend.NodeStart {
		form.dangerous()
	}

	return m.ask(form)
}

// labNodeAction applies a node action to every node of a lab.
func (m *Model) labNodeAction(
	lab *clabinternaltuibackend.Lab, action clabinternaltuibackend.NodeAction,
) tea.Cmd {
	if !lab.Deployed {
		m.notifyf(clabinternaltuitheme.LevelInfo, "%s is not deployed", lab.Name)

		return nil
	}

	return m.nodeAction(action, nodePointers(lab.Nodes))
}

// saveNodes saves the configuration of nodes of a lab, of all of them when none is named.
func (m *Model) saveNodes(lab *clabinternaltuibackend.Lab, nodes []string) tea.Cmd {
	if !lab.Deployed {
		m.notifyf(clabinternaltuitheme.LevelInfo, "%s is not deployed", lab.Name)

		return nil
	}

	what := "all nodes of " + lab.Name
	if len(nodes) > 0 {
		what = targetList(nodes)
	}

	submit := func(formValues) tea.Cmd {
		id, err := m.be.Save(clabinternaltuibackend.SaveRequest{Lab: lab.Name, Nodes: nodes})
		m.started(id, err, false)

		return nil
	}

	return m.ask(newForm("Save configuration", "Save", submit).
		text("Save the running configuration of " + what + ".").
		withPreview(func(formValues) string {
			line := "containerlab save --name " + lab.Name
			if len(nodes) > 0 {
				line += " --node-filter " + strings.Join(nodes, ",")
			}

			return line
		}))
}

func (m *Model) saveSelected(nodes []*clabinternaltuibackend.Node) tea.Cmd {
	labs, grouped := byLab(nodes)
	if len(labs) != 1 {
		m.notifyf(clabinternaltuitheme.LevelWarning, "save the nodes of one lab at a time")

		return nil
	}

	lab := m.lab(labs[0])
	if lab == nil {
		return nil
	}

	return m.saveNodes(lab, grouped[labs[0]])
}

// connectDefault opens a session on a node with its preferred shell, or over SSH.
func (m *Model) connectDefault(n *clabinternaltuibackend.Node, ssh bool) tea.Cmd {
	if !n.Readiness().Running() {
		m.notifyf(clabinternaltuitheme.LevelWarning, "%s is not running", n.Name)

		return nil
	}

	for _, method := range m.be.ConnectMethods(n) {
		if method.SSH == ssh {
			return m.connect(n, method)
		}
	}

	m.notifyf(clabinternaltuitheme.LevelWarning, "no way to connect to %s", n.Name)

	return nil
}

// connectMenu lets the user pick how to connect to a node.
func (m *Model) connectMenu(n *clabinternaltuibackend.Node) tea.Cmd {
	if !n.Readiness().Running() {
		m.notifyf(clabinternaltuitheme.LevelWarning, "%s is not running", n.Name)

		return nil
	}

	methods := m.be.ConnectMethods(n)
	items := make([]menuItem, 0, len(methods))

	for _, method := range methods {
		items = append(items, menuItem{
			label: method.Label,
			desc:  method.Description,
			run:   func() tea.Cmd { return m.connect(n, method) },
		})
	}

	return m.choose("Connect to "+n.Name, numbered(items))
}

// connect hands the terminal over to an interactive session on a node. The UI is
// suspended while the session runs and resumes when it ends.
func (m *Model) connect(
	n *clabinternaltuibackend.Node, method clabinternaltuibackend.ConnectMethod,
) tea.Cmd {
	cmd, err := m.be.Interactive(n.NodeRef, method)
	if err != nil {
		m.notifyf(clabinternaltuitheme.LevelError, "cannot connect to %s: %v", n.Name, err)

		return nil
	}

	node, started := n.Name, m.now()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionEndedMsg{node: node, err: err, started: started}
	})
}

func (m *Model) sessionEnded(msg sessionEndedMsg) {
	took := clabinternaltuiui.Duration(m.now().Sub(msg.started))

	if msg.err != nil {
		m.notifyf(clabinternaltuitheme.LevelWarning, "session on %s ended after %s: %v",
			msg.node, took, msg.err)

		return
	}

	m.notifyf(clabinternaltuitheme.LevelInfo, "session on %s closed after %s", msg.node, took)
}

// copy puts a value on the clipboard of the user's terminal. The escape sequence used for
// it travels through SSH, so this also works on a remote lab host.
func (m *Model) copy(what, value string) tea.Cmd {
	if value == "" {
		m.notifyf(clabinternaltuitheme.LevelWarning, "no %s to copy", what)

		return nil
	}

	m.notifyf(clabinternaltuitheme.LevelSuccess, "copied %s: %s", what, firstLine(value))

	return tea.SetClipboard(value)
}

// copyNodeMenu lets the user pick which value of a node to copy.
func (m *Model) copyNodeMenu(n *clabinternaltuibackend.Node) tea.Cmd {
	values := []struct{ key, what, value string }{
		{"n", "name", n.Name},
		{"c", "container name", n.Container},
		{"4", "IPv4 address", stripPrefixLen(n.IPv4)},
		{"6", "IPv6 address", stripPrefixLen(n.IPv6)},
		{"k", "kind", n.Kind},
		{"i", "image", n.Image},
		{"d", "container ID", n.ID},
		{"s", "ssh command", "ssh " + n.Container},
	}

	items := make([]menuItem, 0, len(values))

	for _, v := range values {
		item := menuItem{shortcut: v.key, label: v.what, desc: v.value}
		if v.value == "" {
			item.disabled = "not set"
		} else {
			item.run = func() tea.Cmd { return m.copy(v.what, v.value) }
		}

		items = append(items, item)
	}

	return m.choose("Copy from "+n.Name, items)
}

// shareMenu offers the terminal sharing actions of a lab.
func (m *Model) shareMenu(lab *clabinternaltuibackend.Lab) tea.Cmd {
	if !lab.Deployed {
		m.notifyf(clabinternaltuitheme.LevelInfo, "%s is not deployed", lab.Name)

		return nil
	}

	var items []menuItem

	for _, share := range lab.Shares {
		items = append(items, menuItem{
			label: "copy the " + string(share.Tool) + " link",
			desc:  share.Link,
			run:   func() tea.Cmd { return m.copy(string(share.Tool)+" link", share.Link) },
		})
	}

	share := func(
		tool clabinternaltuibackend.ShareTool, action clabinternaltuibackend.ShareAction,
	) func() tea.Cmd {
		return func() tea.Cmd {
			if !m.allowed(m.host.Caps.Share, "share") {
				return nil
			}

			id, err := m.be.Share(clabinternaltuibackend.ShareRequest{
				Lab: lab.Name, Tool: tool, Action: action,
			})
			m.started(id, err, false)

			return nil
		}
	}

	for _, tool := range []clabinternaltuibackend.ShareTool{
		clabinternaltuibackend.ShareSSHX, clabinternaltuibackend.ShareGoTTY,
	} {
		name := string(tool)
		items = append(items,
			menuItem{
				label: name + ": share",
				desc:  "give others a terminal in the lab through a link",
				run:   share(tool, clabinternaltuibackend.ShareAttach),
			},
			menuItem{
				label: name + ": stop sharing",
				run:   share(tool, clabinternaltuibackend.ShareDetach),
			},
		)
	}

	return m.choose("Share "+lab.Name, numbered(items))
}
