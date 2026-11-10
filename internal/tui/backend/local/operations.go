// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
)

// The operations in this file change labs. Each one runs the containerlab command that
// does the work as a child process: the commands obtain the root privileges they need on
// their own, which keeps this process unprivileged, and a failing command cannot take the
// UI down with it.

// allow reports whether an operation that changes state may run. needsRoot tells whether
// the command behind it requires root privileges.
func (b *Backend) allow(needsRoot bool) error {
	if b.ctx.Err() != nil {
		return errors.New("the backend is closed")
	}

	if b.opts.ReadOnly {
		return fmt.Errorf("%w: %s", clabinternaltuibackend.ErrUnavailable, reasonReadOnly)
	}

	if needsRoot && !b.priv.ok {
		return fmt.Errorf("%w: %s", clabinternaltuibackend.ErrUnavailable, b.priv.reason)
	}

	return nil
}

// runJob starts a job and returns its ID.
func (b *Backend) runJob(spec *jobSpec) (clabinternaltuibackend.JobID, error) {
	j, err := b.jobs.start(spec)
	if err != nil {
		return 0, err
	}

	return j.info.ID, nil
}

// topologyFile returns the absolute path of the topology file an operation on a lab works
// with: the given one, or else the file the lab is known by.
func (b *Backend) topologyFile(lab, file string) (string, error) {
	if file == "" {
		if rec := b.inv.record(lab); rec != nil {
			file = rec.lab.TopoFile
		} else if known, ok := b.files.byName(lab); ok {
			file = known.path
		}
	}

	if file == "" {
		return "", fmt.Errorf("no topology file known for lab %q", lab)
	}

	abs, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}

	if !fileExists(abs) {
		return "", fmt.Errorf("topology file %s does not exist", abs)
	}

	return abs, nil
}

// deployedLab returns a deployed lab whose topology file still exists. The commands that
// work on the nodes of a lab find them through that file: without it they find no nodes
// and succeed without doing anything, which would pass for success.
func (b *Backend) deployedLab(name, purpose string) (*labRecord, error) {
	rec := b.inv.record(name)
	if rec == nil {
		return nil, fmt.Errorf("lab %q is not deployed", name)
	}

	if rec.lab.TopoMissing {
		return nil, fmt.Errorf("cannot %s lab %q: its topology file %s is missing",
			purpose, name, describePath(rec.lab.TopoFile))
	}

	return rec, nil
}

func describePath(path string) string {
	if path == "" {
		return "(unknown)"
	}

	return path
}

// Deploy deploys a topology file, or reconciles the lab with it if it is deployed already.
func (b *Backend) Deploy(
	req clabinternaltuibackend.DeployRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	topo, err := b.topologyFile(req.Lab, req.TopoFile)
	if err != nil {
		return 0, err
	}

	req.TopoFile = topo

	lab := req.Lab
	if lab == "" {
		lab = readTopoFile(topo).name
	}

	return b.runJob(&jobSpec{
		kind:  clabinternaltuibackend.JobDeploy,
		lab:   lab,
		title: "deploy " + lab,
		topo:  topo,
		args:  deployArgs(b.rtName, &req),
	})
}

// Destroy removes a deployed lab.
func (b *Backend) Destroy(
	req clabinternaltuibackend.DestroyRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	if req.Lab == "" && req.TopoFile == "" {
		return 0, errors.New("no lab given")
	}

	// The file is only needed to run the command next to it, a missing one is fine.
	topo, _ := b.topologyFile(req.Lab, req.TopoFile)
	if req.Lab == "" {
		if topo == "" {
			return 0, fmt.Errorf("topology file %s does not exist", req.TopoFile)
		}

		req.TopoFile = topo
	}

	return b.runJob(&jobSpec{
		kind:  clabinternaltuibackend.JobDestroy,
		lab:   req.Lab,
		title: "destroy " + labOrFile(req.Lab, topo),
		topo:  topo,
		args:  destroyArgs(b.rtName, &req),
	})
}

func labOrFile(lab, file string) string {
	if lab != "" {
		return lab
	}

	return filepath.Base(file)
}

// Redeploy destroys a lab and deploys it again from its topology file.
func (b *Backend) Redeploy(
	req clabinternaltuibackend.RedeployRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	topo, err := b.topologyFile(req.Lab, req.TopoFile)
	if err != nil {
		return 0, err
	}

	req.TopoFile = topo

	return b.runJob(&jobSpec{
		kind:  clabinternaltuibackend.JobRedeploy,
		lab:   req.Lab,
		title: "redeploy " + labOrFile(req.Lab, topo),
		topo:  topo,
		args:  redeployArgs(b.rtName, &req),
	})
}

// NodeAction starts, stops or restarts nodes of a deployed lab.
func (b *Backend) NodeAction(
	req clabinternaltuibackend.NodeActionRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	switch req.Action {
	case clabinternaltuibackend.NodeStart, clabinternaltuibackend.NodeStop,
		clabinternaltuibackend.NodeRestart:
	default:
		return 0, fmt.Errorf("unknown node action %q", req.Action)
	}

	rec, err := b.deployedLab(req.Lab, string(req.Action)+" nodes of")
	if err != nil {
		return 0, err
	}

	count := len(req.Nodes)
	if count == 0 {
		count = len(rec.lab.Nodes)
	}

	return b.runJob(&jobSpec{
		kind:  clabinternaltuibackend.JobKind(req.Action),
		lab:   req.Lab,
		title: fmt.Sprintf("%s %s (%d nodes)", req.Action, req.Lab, count),
		topo:  rec.lab.TopoFile,
		args:  nodeActionArgs(b.rtName, &req),
	})
}

// Save saves the configuration of nodes of a deployed lab. The command exits with success
// also when nodes could not be saved, so the errors it logs decide about the job.
func (b *Backend) Save(
	req clabinternaltuibackend.SaveRequest,
) (clabinternaltuibackend.JobID, error) {
	// Saving goes through the runtime API. Only runtimes other than Docker make
	// containerlab ask for root privileges for every command.
	if err := b.allow(b.rtName != clabruntimedocker.RuntimeName); err != nil {
		return 0, err
	}

	rec, err := b.deployedLab(req.Lab, "save the nodes of")
	if err != nil {
		return 0, err
	}

	if req.CopyTo != "" {
		// The command runs in the directory of the topology file, a relative path is
		// meant relative to where the UI runs.
		if req.CopyTo, err = filepath.Abs(req.CopyTo); err != nil {
			return 0, err
		}
	}

	return b.runJob(&jobSpec{
		kind:           clabinternaltuibackend.JobSave,
		lab:            req.Lab,
		title:          "save " + req.Lab,
		topo:           rec.lab.TopoFile,
		args:           saveArgs(b.rtName, &req),
		errorLinesFail: true,
	})
}

// netemTarget resolves the node of an impairment request to its running container.
func (b *Backend) netemTarget(
	ref clabinternaltuibackend.NodeRef,
	iface string,
) (*clabinternaltuibackend.Node, error) {
	if iface == "" {
		return nil, errors.New("no interface given")
	}

	_, node, _ := b.inv.find(ref)
	if node == nil {
		return nil, fmt.Errorf("node %q of lab %q not found", ref.Name, ref.Lab)
	}

	if !node.Readiness().Running() {
		return nil, fmt.Errorf("node %q is not running", node.Name)
	}

	return node, nil
}

// SetNetem sets the impairments of an interface, replacing what was set before.
func (b *Backend) SetNetem(
	req clabinternaltuibackend.NetemRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	const maxPercent = 100

	switch {
	case req.Delay < 0 || req.Jitter < 0:
		return 0, errors.New("delay and jitter cannot be negative")
	case req.Jitter != 0 && req.Delay == 0:
		return 0, errors.New("jitter cannot be set without a delay")
	case req.LossPct < 0 || req.LossPct > maxPercent:
		return 0, errors.New("packet loss must be between 0 and 100 percent")
	case req.CorruptionPct < 0 || req.CorruptionPct > maxPercent:
		return 0, errors.New("packet corruption must be between 0 and 100 percent")
	}

	node, err := b.netemTarget(req.Node, req.Interface)
	if err != nil {
		return 0, err
	}

	req.Node = node.NodeRef

	return b.runJob(&jobSpec{
		kind:  clabinternaltuibackend.JobNetem,
		lab:   node.Lab,
		title: "netem " + node.Name + ":" + req.Interface,
		args:  netemSetArgs(b.rtName, &req),
	})
}

// ResetNetem removes the impairments of an interface.
func (b *Backend) ResetNetem(
	ref clabinternaltuibackend.NodeRef,
	iface string,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	node, err := b.netemTarget(ref, iface)
	if err != nil {
		return 0, err
	}

	return b.runJob(&jobSpec{
		kind:  clabinternaltuibackend.JobNetem,
		lab:   node.Lab,
		title: "netem reset " + node.Name + ":" + iface,
		args:  netemResetArgs(b.rtName, node.Container, iface),
	})
}

// Share attaches a terminal sharing session to a deployed lab, or detaches it.
func (b *Backend) Share(
	req clabinternaltuibackend.ShareRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	switch req.Tool {
	case clabinternaltuibackend.ShareSSHX, clabinternaltuibackend.ShareGoTTY:
	default:
		return 0, fmt.Errorf("unknown terminal sharing tool %q", req.Tool)
	}

	switch req.Action {
	case clabinternaltuibackend.ShareAttach, clabinternaltuibackend.ShareDetach,
		clabinternaltuibackend.ShareReattach:
	default:
		return 0, fmt.Errorf("unknown terminal sharing action %q", req.Action)
	}

	rec := b.inv.record(req.Lab)
	if rec == nil {
		return 0, fmt.Errorf("lab %q is not deployed", req.Lab)
	}

	spec := &jobSpec{
		kind:  clabinternaltuibackend.JobShare,
		lab:   req.Lab,
		title: fmt.Sprintf("%s %s %s", req.Tool, req.Action, req.Lab),
		args:  shareArgs(b.rtName, &req),
	}

	if !rec.lab.TopoMissing {
		spec.topo = rec.lab.TopoFile
	}

	return b.runJob(spec)
}

// Capture captures packets on interfaces of nodes, into a file or into Wireshark. A
// capture changes nothing in its lab and runs next to other jobs.
func (b *Backend) Capture(
	req clabinternaltuibackend.CaptureRequest,
) (clabinternaltuibackend.JobID, error) {
	if err := b.allow(true); err != nil {
		return 0, err
	}

	targets := make([]string, 0, len(req.Targets))

	for _, target := range req.Targets {
		if target.Node == "" || len(target.Interfaces) == 0 {
			return 0, errors.New("a capture target needs a node and at least one interface")
		}

		targets = append(targets, target.Node+":"+strings.Join(target.Interfaces, ","))
	}

	if len(targets) == 0 {
		return 0, errors.New("no interfaces selected")
	}

	// The capture command finds the interfaces through the links of the topology.
	topo, err := b.topologyFile(req.Lab, req.TopoFile)
	if err != nil {
		return 0, fmt.Errorf("cannot capture: %w", err)
	}

	req.TopoFile = topo

	spec := &jobSpec{
		kind:   clabinternaltuibackend.JobCapture,
		lab:    req.Lab,
		title:  "capture " + strings.Join(targets, " "),
		topo:   topo,
		shared: true,
	}

	if req.Wireshark {
		return b.captureToWireshark(&req, spec)
	}

	if req.File == "" {
		return 0, errors.New("no capture file given")
	}

	// The command runs in the directory of the topology file, see Save.
	file, err := filepath.Abs(req.File)
	if err != nil {
		return 0, err
	}

	spec.args = captureArgs(b.rtName, &req, file)

	j, err := b.jobs.start(spec)
	if err != nil {
		return 0, err
	}

	b.spawn("capture file size", func() { b.followCaptureFile(j, file) })

	return j.info.ID, nil
}
