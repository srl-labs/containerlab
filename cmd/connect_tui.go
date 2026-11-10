// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuipicker "github.com/srl-labs/containerlab/internal/tui/picker"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

var errConnectNeedsTerminal = errors.New("interactive connect requires a terminal")

// connectNode describes a container the way the node picker shows it.
func connectNode(ct *clabruntime.GenericContainer) clabinternaltuipicker.Node {
	kind := ct.Labels[clabconstants.NodeKind]
	if kind == "" {
		kind = "linux"
	}

	lab := ct.Labels[clabconstants.Containerlab]
	if lab == "" {
		lab = "-"
	}

	node := clabinternaltuipicker.Node{
		Name:  strings.TrimPrefix(ct.Names[0], "/"),
		Kind:  kind,
		Lab:   lab,
		Image: clabinternaltuiui.ShortImage(ct.Image),
		State: ct.State,
		Ports: formatConnectPorts(ct.Ports),
	}

	if ip := ct.NetworkSettings.IPv4addr; ip != "" {
		node.IPv4 = fmt.Sprintf("%s/%d", ip, ct.NetworkSettings.IPv4pLen)
	}

	if ip := ct.NetworkSettings.IPv6addr; ip != "" {
		node.IPv6 = fmt.Sprintf("%s/%d", ip, ct.NetworkSettings.IPv6pLen)
	}

	return node
}

// formatConnectPorts renders port bindings the way `docker ps` does: published ports as
// host→container/protocol, exposed ones as container/protocol. Bindings that differ only
// by the host IP (ie. IPv4 and IPv6) are shown once.
func formatConnectPorts(bindings []*clabtypes.GenericPortBinding) string {
	ports := make([]clabinternaltuibackend.Port, 0, len(bindings))

	for _, b := range bindings {
		ports = append(ports, clabinternaltuibackend.Port{
			HostIP:        b.HostIP,
			HostPort:      b.HostPort,
			ContainerPort: b.ContainerPort,
			Protocol:      b.Protocol,
		})
	}

	return clabinternaltuiui.Ports(ports)
}

func connectHasTerminal() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stderr.Fd())
}

// runNodePicker shows the node picker and returns the index of the chosen container in
// the given slice.
func runNodePicker(containers []clabruntime.GenericContainer) (int, error) {
	nodes := make([]clabinternaltuipicker.Node, len(containers))
	for i := range containers {
		nodes[i] = connectNode(&containers[i])
	}

	if !connectHasTerminal() {
		// Fall back to a plain, readable listing when there's no TTY.
		var b strings.Builder

		b.WriteString("multiple nodes matched, please narrow the selection:\n")

		for i := range nodes {
			fmt.Fprintf(&b, "  %s (%s) [%s]\n", nodes[i].Name, nodes[i].Image, nodes[i].State)
		}

		b.WriteString("\nhint: run with a TTY to get an interactive fuzzy-search picker\n")
		fmt.Fprint(os.Stderr, b.String())

		return -1, errConnectNeedsTerminal
	}

	idx, err := clabinternaltuipicker.PickNode(nodes, os.Stdin, os.Stderr)
	if errors.Is(err, clabinternaltuipicker.ErrCancelled) {
		return -1, errConnectCancelled
	}

	return idx, err
}

// runMethodPicker shows the method picker for a single node and returns the selected
// connection method.
func runMethodPicker(
	ct clabruntime.GenericContainer, c *clabcore.CLab,
) (clabinternaltuibackend.ConnectMethod, error) {
	if !connectHasTerminal() {
		return clabinternaltuibackend.ConnectMethod{}, errConnectNeedsTerminal
	}

	node := connectNode(&ct)
	methods := clabinternaltuibackend.DefaultConnectMethods(resolveConnectShell(c, ct), node.Kind)

	idx, err := clabinternaltuipicker.PickMethod(&node, methods, os.Stdin, os.Stderr)
	if errors.Is(err, clabinternaltuipicker.ErrCancelled) {
		return clabinternaltuibackend.ConnectMethod{}, errConnectCancelled
	}

	if err != nil {
		return clabinternaltuibackend.ConnectMethod{}, err
	}

	return methods[idx], nil
}
