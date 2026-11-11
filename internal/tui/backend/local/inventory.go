// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"cmp"
	"context"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

const stateRunning = "running"

// labRecord is a deployed lab as the runtime reports it. A record is never changed once it
// is built: a reload of the inventory replaces all of them.
type labRecord struct {
	// lab holds what the inventory knows of the lab. The sampled values, the busy flag and
	// the number of defined nodes are added when the lab is handed to the UI.
	lab clabinternaltuibackend.Lab
	// name is what the runtime calls the lab. A lab runtime can run labs of the same name
	// in several namespaces, which are then listed under names that tell them apart.
	name string
	// namespace is the Kubernetes namespace of a lab that a lab runtime runs, empty for
	// the labs of this host.
	namespace string
	// topoReal is the topology file with symbolic links resolved, empty when it is missing.
	topoReal string
	// containers holds the containers of the nodes by their name.
	containers map[string]*clabruntime.GenericContainer
	// tools holds the tool containers attached to the lab, ie. terminal sharing ones.
	tools []*clabruntime.GenericContainer
}

// inventoryEnv is what the inventory needs to know about the host.
type inventoryEnv struct {
	hostname string
	// exists reports whether a file exists.
	exists func(path string) bool
	// sshxLink returns the known links of a terminal sharing container.
	sshxLink func(containerID string) sshxLink
}

// buildLabs groups containers into labs. The containers are expected to have their
// network namespace holders folded in already.
func buildLabs(containers []clabruntime.GenericContainer, env *inventoryEnv) []*labRecord {
	// A lab runtime tells its labs apart by their namespace and name.
	type labKey struct{ namespace, name string }

	byName := map[labKey]*labRecord{}

	for i := range containers {
		ctr := &containers[i]

		name := ctr.Labels[clabconstants.Containerlab]
		if name == "" {
			continue
		}

		key := labKey{namespace: workloadNamespace(ctr), name: name}

		rec := byName[key]
		if rec == nil {
			rec = &labRecord{
				lab: clabinternaltuibackend.Lab{
					Name: name, Namespace: key.namespace, Deployed: true,
				},
				name:       name,
				namespace:  key.namespace,
				containers: map[string]*clabruntime.GenericContainer{},
			}
			byName[key] = rec
		}

		// Tool containers serve a lab without being part of its topology.
		if ctr.Labels[clabconstants.ToolType] != "" {
			rec.tools = append(rec.tools, ctr)

			continue
		}

		node := nodeFromContainer(ctr)
		rec.containers[node.Container] = ctr
		rec.lab.Nodes = append(rec.lab.Nodes, node)
	}

	labs := make([]*labRecord, 0, len(byName))

	for _, rec := range byName {
		// A lab is its nodes: tool containers that outlived them do not make one.
		if len(rec.lab.Nodes) == 0 {
			continue
		}

		rec.finish(env)
		labs = append(labs, rec)
	}

	tellApart(labs)

	slices.SortFunc(labs, func(a, b *labRecord) int {
		return naturalCompare(a.lab.Name, b.lab.Name)
	})

	return labs
}

// workloadNamespace returns the Kubernetes namespace of a node that a lab runtime runs,
// empty for a container of this host.
func workloadNamespace(ctr *clabruntime.GenericContainer) string {
	if ctr.Workload == nil {
		return ""
	}

	return ctr.Workload.Namespace
}

// tellApart renames the labs that share their name with a lab of another namespace: the
// UI knows a lab by its name. Such a lab is listed as name@namespace, and so are its
// containers.
func tellApart(labs []*labRecord) {
	count := map[string]int{}
	for _, rec := range labs {
		count[rec.name]++
	}

	for _, rec := range labs {
		if count[rec.name] < 2 || rec.namespace == "" {
			continue
		}

		lab := &rec.lab
		lab.Name = rec.name + "@" + rec.namespace

		containers := make(map[string]*clabruntime.GenericContainer, len(rec.containers))

		for i := range lab.Nodes {
			node := &lab.Nodes[i]
			ctr := rec.containers[node.Container]

			node.Lab = lab.Name
			node.Container = lab.Name + "-" + node.Name
			containers[node.Container] = ctr
		}

		rec.containers = containers
	}
}

// stateKey returns what the sampled values and the interface state of a node are filed
// under: the name of its container, or the ID a lab runtime knows the node by, which is
// what the events of that runtime carry.
func (rec *labRecord) stateKey(container string) string {
	if ctr := rec.containers[container]; ctr != nil && rec.namespace != "" {
		return ctr.ID
	}

	return container
}

// runtimeNode returns the name a lab runtime knows a node by. That is not always the name
// in the topology: a name the cluster cannot carry is changed.
func (rec *labRecord) runtimeNode(node *clabinternaltuibackend.Node) string {
	if ctr := rec.containers[node.Container]; ctr != nil {
		if i := strings.LastIndexByte(ctr.ID, '/'); i >= 0 && rec.namespace != "" {
			return ctr.ID[i+1:]
		}
	}

	return node.Name
}

// finish derives the properties of the lab from its containers.
func (rec *labRecord) finish(env *inventoryEnv) {
	lab := &rec.lab

	slices.SortFunc(lab.Nodes, func(a, b clabinternaltuibackend.Node) int {
		return cmp.Or(naturalCompare(a.Name, b.Name),
			strings.Compare(a.Container, b.Container))
	})

	for i := range lab.Nodes {
		labels := lab.Nodes[i].Labels

		if lab.TopoFile == "" {
			lab.TopoFile = labels[clabconstants.TopoFile]
		}

		if lab.LabDir == "" && labels[clabconstants.NodeLabDir] != "" {
			// The label names the directory of the node, which lives in the lab directory.
			lab.LabDir = filepath.Dir(labels[clabconstants.NodeLabDir])
		}

		if lab.Owner == "" {
			lab.Owner = labels[clabconstants.Owner]
		}
	}

	// A lab that was not deployed from a file is listed with the place its definition is
	// kept at in the cluster, which is no file.
	if strings.Contains(lab.TopoFile, "://") {
		lab.TopoFile = ""
	}

	// Without its topology file a lab can still be inspected and destroyed, not more.
	lab.TopoMissing = lab.TopoFile == "" || !env.exists(lab.TopoFile)
	if !lab.TopoMissing {
		rec.topoReal = realPath(lab.TopoFile)
	}

	slices.SortFunc(rec.tools, func(a, b *clabruntime.GenericContainer) int {
		return strings.Compare(containerName(a), containerName(b))
	})

	for _, tool := range rec.tools {
		if share, ok := shareFromContainer(tool, env); ok {
			lab.Shares = append(lab.Shares, share)
		}
	}
}

// nodeFromContainer maps a container to a node, the way the inspect command reports it:
// the state and status texts are the runtime's, the addresses come without prefix length.
func nodeFromContainer(ctr *clabruntime.GenericContainer) clabinternaltuibackend.Node {
	lab := ctr.Labels[clabconstants.Containerlab]
	container := containerName(ctr)

	name := ctr.Labels[clabconstants.NodeName]
	if name == "" {
		name = strings.TrimPrefix(container, "clab-"+lab+"-")
	}

	return clabinternaltuibackend.Node{
		NodeRef: clabinternaltuibackend.NodeRef{Lab: lab, Name: name, Container: container},
		ID:      ctr.ShortID,
		Kind:    ctr.Labels[clabconstants.NodeKind],
		Type:    ctr.Labels[clabconstants.NodeType],
		Group:   ctr.Labels[clabconstants.NodeGroup],
		Image:   ctr.Image,
		State:   ctr.State,
		Status:  ctr.Status,
		IPv4:    ctr.NetworkSettings.IPv4addr,
		IPv6:    ctr.NetworkSettings.IPv6addr,
		Ports:   portsFromBindings(ctr.Ports),
		Owner:   ctr.Labels[clabconstants.Owner],
		Pid:     ctr.Pid,
		Labels:  maps.Clone(ctr.Labels),
	}
}

// containerName returns the name of a container without the slash some runtimes put in
// front of it.
func containerName(ctr *clabruntime.GenericContainer) string {
	if len(ctr.Names) == 0 {
		return ctr.ShortID
	}

	return strings.TrimPrefix(ctr.Names[0], "/")
}

func portsFromBindings(
	bindings []*clabtypes.GenericPortBinding,
) []clabinternaltuibackend.Port {
	var ports []clabinternaltuibackend.Port

	for _, b := range bindings {
		if b == nil {
			continue
		}

		ports = append(ports, clabinternaltuibackend.Port{
			HostIP:        b.HostIP,
			HostPort:      b.HostPort,
			ContainerPort: b.ContainerPort,
			Protocol:      b.Protocol,
		})
	}

	slices.SortFunc(ports, func(a, b clabinternaltuibackend.Port) int {
		return cmp.Or(
			cmp.Compare(a.ContainerPort, b.ContainerPort),
			strings.Compare(a.Protocol, b.Protocol),
			cmp.Compare(a.HostPort, b.HostPort),
			strings.Compare(a.HostIP, b.HostIP),
		)
	})

	return slices.Compact(ports)
}

// realPath resolves the symbolic links of a path. It returns an empty string for a path
// that cannot be resolved.
func realPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}

	return resolved
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// naturalCompare orders strings with the numbers in them compared by value, so that leaf2
// sorts before leaf10. Strings that only differ in how their numbers are written, ie. by
// leading zeros, are ordered as plain strings.
func naturalCompare(a, b string) int {
	return cmp.Or(compareNumbersByValue(a, b), strings.Compare(a, b))
}

func compareNumbersByValue(a, b string) int {
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			an, bn := digits(a), digits(b)
			a, b = a[len(an):], b[len(bn):]

			// Without leading zeros the longer number is the larger one.
			an, bn = strings.TrimLeft(an, "0"), strings.TrimLeft(bn, "0")
			if c := cmp.Or(cmp.Compare(len(an), len(bn)), strings.Compare(an, bn)); c != 0 {
				return c
			}

			continue
		}

		if a[0] != b[0] {
			return cmp.Compare(a[0], b[0])
		}

		a, b = a[1:], b[1:]
	}

	return cmp.Compare(len(a), len(b))
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// digits returns the run of digits s starts with.
func digits(s string) string {
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}

	return s[:n]
}

// inventory holds the deployed labs as of the last reload.
type inventory struct {
	mu   sync.RWMutex
	labs []*labRecord
}

// replace stores a reloaded inventory. It reports whether anything changed, and whether
// the set of labs or their topology files did.
func (inv *inventory) replace(labs []*labRecord) (changed, membership bool) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	previous := inv.labs
	inv.labs = labs

	if len(previous) != len(labs) {
		return true, true
	}

	for i := range labs {
		if !reflect.DeepEqual(previous[i].lab, labs[i].lab) {
			changed = true
		}

		if previous[i].lab.Name != labs[i].lab.Name ||
			previous[i].lab.TopoFile != labs[i].lab.TopoFile ||
			previous[i].lab.TopoMissing != labs[i].lab.TopoMissing {
			membership = true
		}
	}

	return changed || membership, membership
}

// records returns the deployed labs. The records are shared and must not be changed.
func (inv *inventory) records() []*labRecord {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	return inv.labs
}

// record finds a deployed lab by its name.
func (inv *inventory) record(name string) *labRecord {
	for _, rec := range inv.records() {
		if rec.lab.Name == name {
			return rec
		}
	}

	return nil
}

// find resolves a node reference to the lab, the node and its container. A reference that
// names the container is resolved by it, any other by the lab and node names.
func (inv *inventory) find(
	ref clabinternaltuibackend.NodeRef,
) (*labRecord, *clabinternaltuibackend.Node, *clabruntime.GenericContainer) {
	for _, rec := range inv.records() {
		if ref.Lab != "" && rec.lab.Name != ref.Lab {
			continue
		}

		for i := range rec.lab.Nodes {
			node := &rec.lab.Nodes[i]

			if ref.Container != "" && node.Container != ref.Container {
				continue
			}

			if ref.Container == "" && node.Name != ref.Name {
				continue
			}

			return rec, node, rec.containers[node.Container]
		}
	}

	return nil, nil, nil
}

// runningKeys returns what the state of the running nodes is filed under.
func (inv *inventory) runningKeys() map[string]struct{} {
	keys := map[string]struct{}{}

	for _, rec := range inv.records() {
		for i := range rec.lab.Nodes {
			node := &rec.lab.Nodes[i]

			// A container of this host is only read while it has a process.
			if !strings.EqualFold(node.State, stateRunning) ||
				(rec.namespace == "" && node.Pid <= 0) {
				continue
			}

			keys[rec.stateKey(node.Container)] = struct{}{}
		}
	}

	return keys
}

// running returns the running containers of a lab of this host, or of every such lab for
// an empty name.
func (inv *inventory) running(lab string) []sampleTarget {
	var targets []sampleTarget

	for _, rec := range inv.records() {
		if (lab != "" && rec.lab.Name != lab) || rec.namespace != "" {
			continue
		}

		for i := range rec.lab.Nodes {
			node := &rec.lab.Nodes[i]
			if !strings.EqualFold(node.State, stateRunning) || node.Pid <= 0 {
				continue
			}

			targets = append(targets, sampleTarget{
				container: node.Container,
				id:        rec.containers[node.Container].ID,
				pid:       node.Pid,
			})
		}
	}

	return targets
}

// listContainers lists the containers of all labs: the ones of the container runtime of
// this host, or the nodes of every lab the lab runtime runs.
func (b *Backend) listContainers(ctx context.Context) ([]clabruntime.GenericContainer, error) {
	if b.lrt != nil {
		return b.clab.ListLabRuntimeContainers(ctx, true)
	}

	containers, err := b.clab.ListContainers(ctx, clabcore.WithListclabLabelExists())
	if err != nil {
		return nil, err
	}

	// A node that joined the network namespace of a holder container takes its addressing.
	return clabcore.FoldInternalContainers(containers), nil
}

// reload lists the containers of all labs and replaces the inventory with the result.
func (b *Backend) reload(ctx context.Context) error {
	b.reloadMu.Lock()
	defer b.reloadMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, b.readTimeout())
	defer cancel()

	containers, err := b.listContainers(ctx)
	if err != nil {
		return err
	}

	labs := buildLabs(containers, &inventoryEnv{
		hostname: b.hostname,
		exists:   fileExists,
		sshxLink: b.shares.link,
	})

	changed, membership := b.inv.replace(labs)
	running := b.inv.runningKeys()

	b.stats.retain(running)

	var topics clabinternaltuibackend.Topic

	if changed {
		topics |= clabinternaltuibackend.TopicLabs
	}

	// The interfaces of a stopped container are gone with its network namespace.
	if b.ifaces.retain(running) || changed {
		topics |= clabinternaltuibackend.TopicInterfaces
	}

	if membership {
		// Other labs mean other topologies, and other directories to look for files in.
		topics |= clabinternaltuibackend.TopicLinks

		b.files.request()
	}

	b.mark(topics)
	b.resolveShareLinks(labs)

	return nil
}

// Labs returns the deployed labs followed by the topology files that are not deployed.
func (b *Backend) Labs() []clabinternaltuibackend.Lab {
	records := b.inv.records()
	files := b.files.list()
	busy := b.jobs.busyLabs()

	labs := make([]clabinternaltuibackend.Lab, 0, len(records)+len(files))
	matched := make([]bool, len(files))

	for _, rec := range records {
		lab := cloneLab(&rec.lab)

		for i := range lab.Nodes {
			b.stats.fill(rec.stateKey(lab.Nodes[i].Container), &lab.Nodes[i])
		}

		for i := range files {
			if files[i].belongsTo(rec) {
				matched[i] = true
				lab.DefinedNodes = max(lab.DefinedNodes, files[i].nodes)
			}
		}

		lab.Busy = busy.has(lab.Name, lab.TopoFile)
		labs = append(labs, lab)
	}

	for i := range files {
		if matched[i] {
			continue
		}

		labs = append(labs, clabinternaltuibackend.Lab{
			Name:         files[i].name,
			TopoFile:     files[i].path,
			DefinedNodes: files[i].nodes,
			Busy:         busy.has(files[i].name, files[i].path),
		})
	}

	return labs
}

// cloneLab returns a copy of a lab that shares no memory with it.
func cloneLab(lab *clabinternaltuibackend.Lab) clabinternaltuibackend.Lab {
	out := *lab
	out.Shares = slices.Clone(lab.Shares)
	out.Nodes = slices.Clone(lab.Nodes)

	for i := range out.Nodes {
		out.Nodes[i].Ports = slices.Clone(out.Nodes[i].Ports)
		out.Nodes[i].Labels = maps.Clone(out.Nodes[i].Labels)
	}

	return out
}
