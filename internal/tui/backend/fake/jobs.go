// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package fake

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// linesPerTick is the number of output lines a simulated job produces per step.
	linesPerTick = 3
	// captureBytesPerTick is how fast a simulated capture grows.
	captureBytesPerTick = 48 * 1024

	levelInfo = "INFO"
)

// job is a simulated operation: it plays back its script a few lines per step and applies
// its effect once the script is through.
type job struct {
	info   clabinternaltuibackend.JobInfo
	out    []clabinternaltuibackend.LogLine
	script []string
	apply  func()
	// endless jobs run until they are cancelled, like a packet capture.
	endless bool
}

// labBusy reports whether a job is running for the lab. The lock must be held.
func (b *Backend) labBusy(lab string) bool {
	for _, j := range b.jobs {
		if j.info.Lab == lab && !j.info.State.Done() && !j.endless {
			return true
		}
	}

	return false
}

// startJob registers a job. Its script starts playing on the next step.
func (b *Backend) startJob(
	kind clabinternaltuibackend.JobKind, lab, title, command string,
	script []string, apply func(),
) (clabinternaltuibackend.JobID, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.labBusy(lab) && kind != clabinternaltuibackend.JobCapture {
		return 0, clabinternaltuibackend.ErrLabBusy
	}

	b.nextJob++

	j := &job{
		info: clabinternaltuibackend.JobInfo{
			ID:      b.nextJob,
			Kind:    kind,
			Lab:     lab,
			Title:   title,
			Command: command,
			State:   clabinternaltuibackend.JobRunning,
			Started: b.opts.Now(),
		},
		script:  script,
		apply:   apply,
		endless: kind == clabinternaltuibackend.JobCapture,
	}

	b.jobs = append(b.jobs, j)
	b.signal(clabinternaltuibackend.TopicJobs | clabinternaltuibackend.TopicLabs)

	return j.info.ID, nil
}

// advanceJobs lets every running job make a step. The lock must be held.
func (b *Backend) advanceJobs() {
	for _, j := range b.jobs {
		if j.info.State.Done() {
			continue
		}

		if j.info.State == clabinternaltuibackend.JobCancelling {
			b.finish(j, clabinternaltuibackend.JobCancelled)

			continue
		}

		if j.endless {
			j.info.Bytes += captureBytesPerTick

			b.signal(clabinternaltuibackend.TopicJobs)

			continue
		}

		n := min(linesPerTick, len(j.script))
		for _, text := range j.script[:n] {
			b.emit(j, text)
		}

		j.script = j.script[n:]

		b.signal(clabinternaltuibackend.TopicJobOutput | clabinternaltuibackend.TopicJobs)

		if len(j.script) == 0 {
			if j.apply != nil {
				j.apply()
			}

			b.finish(j, clabinternaltuibackend.JobSucceeded)
		}
	}
}

func (b *Backend) emit(j *job, text string) {
	j.out = append(j.out, clabinternaltuibackend.LogLine{
		Time:  b.opts.Now(),
		Level: levelInfo,
		Text:  text,
	})
	j.info.Lines = len(j.out)
}

func (b *Backend) finish(j *job, state clabinternaltuibackend.JobState) {
	j.info.State = state
	j.info.Finished = b.opts.Now()

	b.signal(clabinternaltuibackend.TopicJobs | clabinternaltuibackend.TopicLabs |
		clabinternaltuibackend.TopicInterfaces | clabinternaltuibackend.TopicLinks)
}

// Jobs implements clabinternaltuibackend.Backend.
func (b *Backend) Jobs() []clabinternaltuibackend.JobInfo {
	b.mu.Lock()
	defer b.mu.Unlock()

	infos := make([]clabinternaltuibackend.JobInfo, 0, len(b.jobs))
	for _, j := range slices.Backward(b.jobs) {
		infos = append(infos, j.info)
	}

	return infos
}

func (b *Backend) job(id clabinternaltuibackend.JobID) *job {
	for _, j := range b.jobs {
		if j.info.ID == id {
			return j
		}
	}

	return nil
}

// Job implements clabinternaltuibackend.Backend.
func (b *Backend) Job(id clabinternaltuibackend.JobID) (clabinternaltuibackend.JobInfo, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if j := b.job(id); j != nil {
		return j.info, true
	}

	return clabinternaltuibackend.JobInfo{}, false
}

// JobOutput implements clabinternaltuibackend.Backend.
func (b *Backend) JobOutput(
	id clabinternaltuibackend.JobID,
	from int,
) []clabinternaltuibackend.LogLine {
	b.mu.Lock()
	defer b.mu.Unlock()

	j := b.job(id)
	if j == nil || from >= len(j.out) {
		return nil
	}

	return slices.Clone(j.out[max(0, from):])
}

// CancelJob implements clabinternaltuibackend.Backend.
func (b *Backend) CancelJob(id clabinternaltuibackend.JobID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	j := b.job(id)
	if j == nil || j.info.State.Done() {
		return
	}

	if j.endless {
		// Stopping a capture is its normal end: the file is complete.
		b.finish(j, clabinternaltuibackend.JobSucceeded)

		return
	}

	j.info.State = clabinternaltuibackend.JobCancelling

	b.signal(clabinternaltuibackend.TopicJobs)
}

// rebuildLab regenerates a lab from its spec after a change, carrying the sampled history
// of the nodes that keep running over to the new entries. The lock must be held.
func (b *Backend) rebuildLab(index int) {
	old := b.labs[index]
	oldIfaces := map[string][]clabinternaltuibackend.Interface{}

	for i := range old.Nodes {
		oldIfaces[old.Nodes[i].Container] = b.ifaces[old.Nodes[i].Container]
		delete(b.ifaces, old.Nodes[i].Container)
	}

	lab := b.buildLab(index)

	for i := range lab.Nodes {
		n := &lab.Nodes[i]

		if prev := old.Node(n.Name); prev != nil && prev.State == n.State {
			n.CPUSeries, n.RxSeries, n.TxSeries = prev.CPUSeries, prev.RxSeries, prev.TxSeries
		}

		for k := range b.ifaces[n.Container] {
			iface := &b.ifaces[n.Container][k]

			for _, prev := range oldIfaces[n.Container] {
				if prev.Name == iface.Name {
					iface.RxSeries, iface.TxSeries = prev.RxSeries, prev.TxSeries
					iface.RxBytes, iface.TxBytes = prev.RxBytes, prev.TxBytes
					iface.Netem = prev.Netem
				}
			}
		}
	}

	b.labs[index] = lab
	b.simulate()
}

func (b *Backend) setNodes(index int, names []string, state, status string) {
	spec := &b.specs[index]

	for i := range spec.nodes {
		if len(names) == 0 || slices.Contains(names, spec.nodes[i].name) {
			spec.nodes[i].state, spec.nodes[i].status = state, status
		}
	}
}

func (b *Backend) index(lab string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	i := b.specIndex(lab)
	if i < 0 {
		return 0, fmt.Errorf("lab %q not found", lab)
	}

	return i, nil
}

// Deploy implements clabinternaltuibackend.Backend.
func (b *Backend) Deploy(
	req clabinternaltuibackend.DeployRequest,
) (clabinternaltuibackend.JobID, error) {
	i, err := b.index(req.Lab)
	if err != nil {
		return 0, err
	}

	spec := &b.specs[i]
	script := []string{
		"Containerlab started version=0.83.0",
		"Parsing & checking topology file=" + filepath.Base(spec.topoFile),
		"Creating docker network name=clab IPv4 subnet=172.20.20.0/24 MTU=1500",
		"Creating lab directory path=" + filepath.Join(
			filepath.Dir(spec.topoFile),
			"clab-"+spec.name,
		),
	}

	for n := range spec.nodes {
		script = append(script, "Creating container name="+spec.nodes[n].name)
	}

	for l := range spec.links {
		link := &spec.links[l]
		script = append(script, fmt.Sprintf("Created link: %s:%s ▪┄┄▪ %s:%s",
			link.a, link.aif, link.z, link.zif))
	}

	script = append(script,
		"Adding host entries path=/etc/hosts",
		"Adding SSH config for nodes path=/etc/ssh/ssh_config.d/clab-"+spec.name+".conf",
	)

	command := "containerlab deploy -t " + spec.topoFile
	if req.Reconfigure {
		command += " --reconfigure"
	}

	return b.startJob(clabinternaltuibackend.JobDeploy, req.Lab, "deploy "+req.Lab, command, script,
		func() {
			spec.deployed = true

			b.setNodes(i, nil, stateRunning, "Up 1 second")
			b.rebuildLab(i)
		})
}

// Destroy implements clabinternaltuibackend.Backend.
func (b *Backend) Destroy(
	req clabinternaltuibackend.DestroyRequest,
) (clabinternaltuibackend.JobID, error) {
	i, err := b.index(req.Lab)
	if err != nil {
		return 0, err
	}

	spec := &b.specs[i]
	script := []string{"Destroying lab name=" + spec.name}

	for n := range spec.nodes {
		script = append(
			script,
			"Removed container name="+containerName(spec.name, spec.nodes[n].name),
		)
	}

	script = append(script, "Removing host entries path=/etc/hosts")

	command := "containerlab destroy --name " + spec.name
	if req.Cleanup {
		command += " --cleanup"
	}

	return b.startJob(
		clabinternaltuibackend.JobDestroy,
		req.Lab,
		"destroy "+req.Lab,
		command,
		script,
		func() {
			spec.deployed = false

			b.setNodes(i, nil, "", "")
			b.rebuildLab(i)
		},
	)
}

// Redeploy implements clabinternaltuibackend.Backend.
func (b *Backend) Redeploy(
	req clabinternaltuibackend.RedeployRequest,
) (clabinternaltuibackend.JobID, error) {
	i, err := b.index(req.Lab)
	if err != nil {
		return 0, err
	}

	spec := &b.specs[i]
	script := []string{"Destroying lab name=" + spec.name}

	for n := range spec.nodes {
		script = append(
			script,
			"Removed container name="+containerName(spec.name, spec.nodes[n].name),
		)
	}

	script = append(script, "Containerlab started version=0.83.0")

	for n := range spec.nodes {
		script = append(script, "Creating container name="+spec.nodes[n].name)
	}

	command := "containerlab redeploy -t " + spec.topoFile
	if req.Cleanup {
		command += " --cleanup"
	}

	return b.startJob(clabinternaltuibackend.JobRedeploy, req.Lab, "redeploy "+req.Lab, command,
		script, func() {
			spec.deployed = true

			b.setNodes(i, nil, stateRunning, "Up 1 second")
			b.rebuildLab(i)
		})
}

// NodeAction implements clabinternaltuibackend.Backend.
func (b *Backend) NodeAction(
	req clabinternaltuibackend.NodeActionRequest,
) (clabinternaltuibackend.JobID, error) {
	i, err := b.index(req.Lab)
	if err != nil {
		return 0, err
	}

	names := req.Nodes
	if len(names) == 0 {
		for n := range b.specs[i].nodes {
			names = append(names, b.specs[i].nodes[n].name)
		}
	}

	script := make([]string, 0, len(names))
	for _, name := range names {
		script = append(script, fmt.Sprintf("%s node name=%s", req.Action, name))
	}

	state, status := stateRunning, "Up 1 second"
	kind := clabinternaltuibackend.JobKind(req.Action)

	if req.Action == clabinternaltuibackend.NodeStop {
		state, status = stateExited, "Exited (0) 1 second ago"
	}

	command := fmt.Sprintf("containerlab %s --name %s --node %s",
		req.Action, req.Lab, strings.Join(names, ","))
	title := fmt.Sprintf("%s %s (%d nodes)", req.Action, req.Lab, len(names))

	return b.startJob(kind, req.Lab, title, command, script, func() {
		b.setNodes(i, names, state, status)
		b.rebuildLab(i)
	})
}

// Save implements clabinternaltuibackend.Backend.
func (b *Backend) Save(
	req clabinternaltuibackend.SaveRequest,
) (clabinternaltuibackend.JobID, error) {
	i, err := b.index(req.Lab)
	if err != nil {
		return 0, err
	}

	spec := &b.specs[i]

	var script []string

	for n := range spec.nodes {
		node := &spec.nodes[n]
		if node.state != stateRunning ||
			(len(req.Nodes) > 0 && !slices.Contains(req.Nodes, node.name)) {
			continue
		}

		script = append(script, "Saved node configuration node="+node.name)
	}

	if len(script) == 0 {
		script = []string{"no running nodes to save"}
	}

	return b.startJob(clabinternaltuibackend.JobSave, req.Lab, "save "+req.Lab,
		"containerlab save --name "+req.Lab, script, nil)
}

func (b *Backend) setImpairment(
	node clabinternaltuibackend.NodeRef, iface string, netem *clabinternaltuibackend.Impairment,
) {
	list := slices.Clone(b.ifaces[node.Container])

	for i := range list {
		if list[i].Name == iface {
			list[i].Netem = netem
		}
	}

	b.ifaces[node.Container] = list
}

// netemJob starts the job that works on the impairments of interfaces of one lab, with a
// command and a line of output for each of them.
func (b *Backend) netemJob(
	verb, action, output string,
	targets []clabinternaltuibackend.InterfaceRef,
	netem *clabinternaltuibackend.Impairment,
) (clabinternaltuibackend.JobID, error) {
	if len(targets) == 0 {
		return 0, fmt.Errorf("no interface given")
	}

	lab := targets[0].Node.Lab
	title := verb + " " + targets[0].Node.Name + ":" + targets[0].Name

	if len(targets) > 1 {
		title = fmt.Sprintf("%s %s (%d interfaces)", verb, lab, len(targets))
	}

	commands := make([]string, len(targets))
	script := make([]string, len(targets))

	for i, target := range targets {
		if target.Node.Lab != lab {
			return 0, fmt.Errorf("the interfaces belong to more than one lab")
		}

		commands[i] = fmt.Sprintf("containerlab tools netem %s -n %s -i %s",
			action, target.Node.Container, target.Name)
		script[i] = output + " node=" + target.Node.Name + " interface=" + target.Name
	}

	return b.startJob(clabinternaltuibackend.JobNetem, lab, title,
		strings.Join(commands, "; "), script,
		func() {
			for _, target := range targets {
				if netem == nil {
					b.setImpairment(target.Node, target.Name, nil)

					continue
				}

				set := *netem
				b.setImpairment(target.Node, target.Name, &set)
			}
		})
}

// SetNetem implements clabinternaltuibackend.Backend.
func (b *Backend) SetNetem(
	req clabinternaltuibackend.NetemRequest,
) (clabinternaltuibackend.JobID, error) {
	var netem *clabinternaltuibackend.Impairment
	if !req.IsZero() {
		netem = &req.Impairment
	}

	return b.netemJob("netem", "set", "Setting impairments", req.Interfaces, netem)
}

// ResetNetem implements clabinternaltuibackend.Backend.
func (b *Backend) ResetNetem(
	interfaces []clabinternaltuibackend.InterfaceRef,
) (clabinternaltuibackend.JobID, error) {
	return b.netemJob("netem reset", "reset", "Reset impairments", interfaces, nil)
}

// Share implements clabinternaltuibackend.Backend.
func (b *Backend) Share(
	req clabinternaltuibackend.ShareRequest,
) (clabinternaltuibackend.JobID, error) {
	i, err := b.index(req.Lab)
	if err != nil {
		return 0, err
	}

	spec := &b.specs[i]
	link := "https://sshx.io/s/Dm0Lab42#demoKey1"

	if req.Tool == clabinternaltuibackend.ShareGoTTY {
		port := req.Port
		if port == 0 {
			port = 8080
		}

		link = fmt.Sprintf("http://demo:%d", port)
	}

	command := fmt.Sprintf("containerlab tools %s %s -l %s", req.Tool, req.Action, req.Lab)
	title := fmt.Sprintf("%s %s %s", req.Tool, req.Action, req.Lab)

	return b.startJob(clabinternaltuibackend.JobShare, req.Lab, title, command,
		[]string{fmt.Sprintf("%s %s lab=%s", req.Tool, req.Action, req.Lab)},
		func() {
			spec.shares = slices.DeleteFunc(spec.shares, func(s clabinternaltuibackend.Share) bool {
				return s.Tool == req.Tool
			})

			if req.Action != clabinternaltuibackend.ShareDetach {
				share := clabinternaltuibackend.Share{Tool: req.Tool, Lab: req.Lab, Link: link}
				if req.ReadOnly {
					share.ReadOnlyLink = link + ",ro"
				}

				spec.shares = append(spec.shares, share)
			}

			b.labs[i].Shares = slices.Clone(spec.shares)
		})
}

// Capture implements clabinternaltuibackend.Backend.
func (b *Backend) Capture(
	req clabinternaltuibackend.CaptureRequest,
) (clabinternaltuibackend.JobID, error) {
	targets := make([]string, 0, len(req.Targets))
	for _, t := range req.Targets {
		targets = append(targets, t.Node+":"+strings.Join(t.Interfaces, ","))
	}

	if len(targets) == 0 {
		return 0, fmt.Errorf("no interfaces selected")
	}

	sink := "-"
	if !req.Wireshark {
		sink = req.File
	}

	command := fmt.Sprintf("containerlab capture --name %s --target %s -w %s",
		req.Lab, strings.Join(targets, " --target "), sink)

	return b.startJob(clabinternaltuibackend.JobCapture, req.Lab,
		"capture "+strings.Join(targets, " "), command, nil, nil)
}
