// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// report builds a text of titled sections holding aligned "key: value" lines.
type report struct {
	b strings.Builder
}

func (r *report) section(title string) {
	if r.b.Len() > 0 {
		r.b.WriteByte('\n')
	}

	r.b.WriteString(title)
	r.b.WriteByte('\n')
}

// field adds a line. Values come from containers and files and are not trusted to be fit
// for a terminal. Empty values are left out.
func (r *report) field(key, value string) {
	const keyWidth = 16

	if value = oneLine(value); value == "" {
		return
	}

	fmt.Fprintf(&r.b, "  %-*s %s\n", keyWidth, key+":", value)
}

func (r *report) line(text string) {
	r.b.WriteString("  " + oneLine(text) + "\n")
}

// Inspect describes a node: what the runtime knows about its container and what the export
// of the deployed topology says about the node.
func (b *Backend) Inspect(
	ctx context.Context,
	ref clabinternaltuibackend.NodeRef,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	rec, node, ctr := b.inv.find(ref)
	if node == nil {
		return "", fmt.Errorf("node %q of lab %q not found", ref.Name, ref.Lab)
	}

	var r report

	inspectNode(&r, node)
	inspectWorkload(&r, ctr)
	inspectNetwork(&r, node, ctr)
	inspectPaths(&r, &rec.lab, node)
	inspectExport(&r, rec, node)
	inspectLabels(&r, node.Labels)

	return r.b.String(), nil
}

func inspectNode(r *report, node *clabinternaltuibackend.Node) {
	r.section("Node")
	r.field("name", node.Name)
	r.field("lab", node.Lab)
	r.field("kind", node.Kind)
	r.field("type", node.Type)
	r.field("group", node.Group)
	r.field("owner", node.Owner)

	r.section("Container")
	r.field("name", node.Container)
	r.field("id", node.ID)
	r.field("image", node.Image)
	r.field("state", node.State)
	r.field("status", node.Status)

	if node.Pid > 0 {
		r.field("pid", fmt.Sprint(node.Pid))
	}
}

func inspectNetwork(
	r *report,
	node *clabinternaltuibackend.Node,
	ctr *clabruntime.GenericContainer,
) {
	net := &ctr.NetworkSettings

	r.section("Management network")

	// A lab runtime names the namespace of the node where a network is named otherwise.
	if ctr.Workload == nil {
		r.field("network", ctr.NetworkName)
	}

	r.field("ipv4", withPrefixLen(net.IPv4addr, net.IPv4pLen))
	r.field("ipv4 gateway", net.IPv4Gw)
	r.field("ipv6", withPrefixLen(net.IPv6addr, net.IPv6pLen))
	r.field("ipv6 gateway", net.IPv6Gw)

	if len(node.Ports) == 0 {
		return
	}

	r.section("Ports")

	for _, p := range node.Ports {
		r.line(formatPort(&p))
	}
}

// withPrefixLen renders an address in CIDR notation when the prefix length is known.
func withPrefixLen(addr string, prefixLen int) string {
	if addr == "" || prefixLen <= 0 {
		return addr
	}

	return fmt.Sprintf("%s/%d", addr, prefixLen)
}

// formatPort renders a port the way container runtimes list them.
func formatPort(p *clabinternaltuibackend.Port) string {
	port := fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
	if p.HostPort == 0 {
		return port
	}

	host := p.HostIP
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	return fmt.Sprintf("%s:%d -> %s", host, p.HostPort, port)
}

func inspectPaths(
	r *report,
	lab *clabinternaltuibackend.Lab,
	node *clabinternaltuibackend.Node,
) {
	r.section("Files")

	topoFile := lab.TopoFile
	if lab.TopoMissing && topoFile != "" {
		topoFile += " (missing)"
	}

	r.field("topology file", topoFile)
	r.field("lab directory", lab.LabDir)
	r.field("node directory", node.Labels[clabconstants.NodeLabDir])
}

// inspectExport adds what the topology export of the lab says about the node. The export
// is optional, a lab deployed with a custom export template may not have a usable one.
func inspectExport(r *report, rec *labRecord, node *clabinternaltuibackend.Node) {
	if rec.lab.LabDir == "" {
		return
	}

	data, err := readTopologyData(filepath.Join(rec.lab.LabDir, topologyDataFile))
	if err != nil {
		return
	}

	exported, ok := data.Nodes[node.Name]
	if !ok {
		return
	}

	r.section("Topology")
	r.field("index", string(exported.Index))
	r.field("long name", exported.LongName)
	r.field("fqdn", exported.Fqdn)
	r.field("mgmt network", exported.MgmtNet)
	r.field("mgmt interface", exported.MgmtIntf)
	r.field("mgmt ipv4", withPrefixLen(exported.MgmtIPv4, exported.MgmtIPv4Len))
	r.field("mgmt ipv6", withPrefixLen(exported.MgmtIPv6, exported.MgmtIPv6Len))
	r.field("mac address", exported.MAC)

	for _, p := range exported.PortBindings {
		r.field("port binding", formatPort(&clabinternaltuibackend.Port{
			HostIP:        p.HostIP,
			HostPort:      p.HostPort,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		}))
	}

	var links []string

	for i := range data.Links {
		if text, ok := describeLink(&data.Links[i], node.Name); ok {
			links = append(links, text)
		}
	}

	if len(links) == 0 {
		return
	}

	r.section("Links")

	for _, link := range links {
		r.line(link)
	}
}

// describeLink renders a link from the point of view of one of its nodes.
func describeLink(link *topoDataLink, node string) (string, bool) {
	near, far := link.Endpoints.A, link.Endpoints.Z
	if near == nil {
		return "", false
	}

	if far != nil && near.Node != node {
		near, far = far, near
	}

	if near.Node != node {
		return "", false
	}

	if far == nil {
		return near.Interface, true
	}

	return fmt.Sprintf("%s <-> %s:%s", near.Interface, far.Node, far.Interface), true
}

func inspectLabels(r *report, labels map[string]string) {
	if len(labels) == 0 {
		return
	}

	r.section("Labels")

	for _, key := range slices.Sorted(maps.Keys(labels)) {
		r.line(key + "=" + labels[key])
	}
}
