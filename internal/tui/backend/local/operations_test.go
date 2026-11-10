// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// everyOperation calls each operation that changes state with a request that would run on
// the fabric lab of the test data, and returns the errors by operation.
func everyOperation(t *testing.T, b *testBackend) map[string]error {
	t.Helper()

	topo := absTestdata(t, "fabric.clab.yml")
	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}
	errs := map[string]error{}

	_, errs["deploy"] = b.Deploy(
		clabinternaltuibackend.DeployRequest{Lab: "fabric", TopoFile: topo},
	)
	_, errs["destroy"] = b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "fabric"})
	_, errs["redeploy"] = b.Redeploy(clabinternaltuibackend.RedeployRequest{Lab: "fabric"})
	_, errs["node action"] = b.NodeAction(clabinternaltuibackend.NodeActionRequest{
		Lab: "fabric", Action: clabinternaltuibackend.NodeRestart,
	})
	_, errs["save"] = b.Save(clabinternaltuibackend.SaveRequest{Lab: "fabric"})
	_, errs["set netem"] = b.SetNetem(clabinternaltuibackend.NetemRequest{
		Node: leaf1, Interface: "e1-1",
	})
	_, errs["reset netem"] = b.ResetNetem(leaf1, "e1-1")
	_, errs["share"] = b.Share(clabinternaltuibackend.ShareRequest{
		Lab: "fabric", Tool: clabinternaltuibackend.ShareSSHX,
		Action: clabinternaltuibackend.ShareAttach,
	})
	_, errs["capture"] = b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab: "fabric", File: filepath.Join(t.TempDir(), "x.pcapng"),
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1"}},
		},
	})

	return errs
}

func TestReadOnlyRefusesEveryOperation(t *testing.T) {
	b := newTestBackend(t, func(o *Options, _ *deps) { o.ReadOnly = true })
	b.setLabs(fabricContainers(t)...)

	for op, err := range everyOperation(t, b) {
		if !errors.Is(err, clabinternaltuibackend.ErrUnavailable) ||
			!strings.Contains(err.Error(), "read-only") {
			t.Errorf("%s: err = %v, want it to be unavailable in read-only mode", op, err)
		}
	}

	if b.run.count() != 0 || len(b.Jobs()) != 0 {
		t.Error("a read-only backend ran a command")
	}

	caps := b.Host().Caps
	if caps.Lifecycle || caps.Netem || caps.Capture || caps.Share {
		t.Errorf("a read-only backend offers operations: %+v", caps)
	}

	if !b.Host().Privileged || caps.Reason(capLifecycle) != reasonReadOnly {
		t.Errorf("unexpected host: %+v", b.Host())
	}
}

func TestOperationsWithoutPrivileges(t *testing.T) {
	const reason = "root privileges required: run with sudo or use a setuid containerlab binary"

	if reasonNeedsRoot != reason {
		t.Errorf("reason = %q", reasonNeedsRoot)
	}

	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.priv = privilege{reason: reasonNeedsRoot}
	})
	b.setLabs(fabricContainers(t)...)

	for op, err := range everyOperation(t, b) {
		// Saving goes through the Docker API and needs no privileges.
		if op == "save" {
			if err != nil {
				t.Errorf("save: %v", err)
			}

			continue
		}

		if !errors.Is(err, clabinternaltuibackend.ErrUnavailable) ||
			!strings.Contains(err.Error(), reason) {
			t.Errorf("%s: err = %v, want it to be unavailable for lack of privileges", op, err)
		}
	}

	if b.run.count() != 1 {
		t.Errorf("%d commands were run, want the save command only", b.run.count())
	}

	host := b.Host()
	want := clabinternaltuibackend.Capabilities{
		Wireshark: true,
		Reasons: map[string]string{
			capLifecycle: reason, capNetem: reason, capCapture: reason, capShare: reason,
			capLinkState: reason,
		},
	}

	if diff := cmp.Diff(want, host.Caps); diff != "" || host.Privileged {
		t.Errorf("privileged=%v, capabilities (-want +got):\n%s", host.Privileged, diff)
	}
}

func TestSaveNeedsPrivilegesWithOtherRuntimes(t *testing.T) {
	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.rtName = "podman"
		d.priv = privilege{reason: reasonNeedsRoot}
	})
	b.setLabs(fabricContainers(t)...)

	_, err := b.Save(clabinternaltuibackend.SaveRequest{Lab: "fabric"})
	if !errors.Is(err, clabinternaltuibackend.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestEveryOperationRunsItsCommand(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(fabricContainers(t)...)

	topo := absTestdata(t, "fabric.clab.yml")
	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}
	dir := filepath.Dir(topo)

	// One job per lab at a time: each job is ended before the next one starts.
	steps := []struct {
		run       func() (clabinternaltuibackend.JobID, error)
		wantKind  clabinternaltuibackend.JobKind
		wantTitle string
		wantArgs  string
		wantDir   string
	}{
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.Redeploy(
					clabinternaltuibackend.RedeployRequest{Lab: "fabric", Cleanup: true},
				)
			},
			wantKind:  clabinternaltuibackend.JobRedeploy,
			wantTitle: "redeploy fabric",
			wantArgs:  "redeploy -r docker -t " + topo + " --cleanup --format json",
			wantDir:   dir,
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.Destroy(
					clabinternaltuibackend.DestroyRequest{Lab: "fabric", Graceful: true},
				)
			},
			wantKind:  clabinternaltuibackend.JobDestroy,
			wantTitle: "destroy fabric",
			wantArgs:  "destroy -r docker --name fabric --graceful",
			wantDir:   dir,
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "fabric", Action: clabinternaltuibackend.NodeStop,
					Nodes: []string{"leaf1", "leaf2"},
				})
			},
			wantKind:  clabinternaltuibackend.JobStop,
			wantTitle: "stop fabric (2 nodes)",
			wantArgs:  "stop -r docker --name fabric --node leaf1,leaf2",
			wantDir:   dir,
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "fabric", Action: clabinternaltuibackend.NodeStart,
				})
			},
			wantKind:  clabinternaltuibackend.JobStart,
			wantTitle: "start fabric (4 nodes)",
			wantArgs:  "start -r docker --name fabric",
			wantDir:   dir,
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.Save(clabinternaltuibackend.SaveRequest{
					Lab: "fabric", Nodes: []string{"leaf1"}, CopyTo: "/backup",
				})
			},
			wantKind:  clabinternaltuibackend.JobSave,
			wantTitle: "save fabric",
			wantArgs:  "save -r docker --name fabric --node-filter leaf1 --copy /backup",
			wantDir:   dir,
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.SetNetem(clabinternaltuibackend.NetemRequest{
					Node: leaf1, Interface: "e1-1",
					Impairment: clabinternaltuibackend.Impairment{
						Delay: 50 * time.Millisecond, LossPct: 0.5,
					},
				})
			},
			wantKind:  clabinternaltuibackend.JobNetem,
			wantTitle: "netem leaf1:e1-1",
			// The container is filled in from the inventory.
			wantArgs: "tools netem set -r docker -n clab-fabric-leaf1 -i e1-1 --delay 50ms " +
				"--jitter 0s --loss 0.5 --rate 0 --corruption 0",
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.ResetNetem(leaf1, "e1-1")
			},
			wantKind:  clabinternaltuibackend.JobNetem,
			wantTitle: "netem reset leaf1:e1-1",
			wantArgs:  "tools netem reset -r docker -n clab-fabric-leaf1 -i e1-1",
		},
		{
			run: func() (clabinternaltuibackend.JobID, error) {
				return b.Share(clabinternaltuibackend.ShareRequest{
					Lab: "fabric", Tool: clabinternaltuibackend.ShareGoTTY,
					Action: clabinternaltuibackend.ShareAttach, Port: 9090,
					Username: "admin", Password: "s3cret",
				})
			},
			wantKind:  clabinternaltuibackend.JobShare,
			wantTitle: "gotty attach fabric",
			wantArgs: "tools gotty attach -r docker -l fabric --port 9090 --username admin " +
				"--password s3cret",
			wantDir: dir,
		},
	}

	for i, step := range steps {
		id, err := step.run()
		if err != nil {
			t.Fatalf("%s: %v", step.wantTitle, err)
		}

		proc := b.run.proc(t, i)
		if diff := cmp.Diff(argv(step.wantArgs), proc.spec.args); diff != "" {
			t.Errorf("%s: unexpected arguments (-want +got):\n%s", step.wantTitle, diff)
		}

		if proc.spec.dir != step.wantDir {
			t.Errorf("%s: runs in %q, want %q", step.wantTitle, proc.spec.dir, step.wantDir)
		}

		info, _ := b.Job(id)
		if info.Kind != step.wantKind || info.Title != step.wantTitle || info.Lab != "fabric" {
			t.Errorf("%s: unexpected job %+v", step.wantTitle, info)
		}

		if strings.Contains(info.Command, "s3cret") {
			t.Errorf("the command shown gives the password away: %s", info.Command)
		}

		// The lab is busy while the job runs.
		for _, lab := range b.Labs() {
			if lab.Name == "fabric" && !lab.Busy {
				t.Errorf("%s: the lab is not marked busy", step.wantTitle)
			}
		}

		proc.exitWith(0)
		b.awaitJob(t, id)
	}

	if want := "containerlab tools gotty attach -r docker -l fabric --port 9090 " +
		"--username admin --password ******"; b.Jobs()[0].Command != want {
		t.Errorf("command = %q, want %q", b.Jobs()[0].Command, want)
	}
}

func TestOperationsAfterAJobSignalAReload(t *testing.T) {
	b := newTestBackend(t)

	id, err := b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "lab1"})
	if err != nil {
		t.Fatal(err)
	}

	b.run.proc(t, 0).exitWith(0)
	b.awaitJob(t, id)

	// Whatever a job did, the inventory and the topology files are looked at again.
	eventually(t, "a reload to be requested", func() bool { return len(b.reloadCh) == 1 })
	eventually(t, "a scan to be requested", func() bool { return len(b.files.wanted) == 1 })

	eventually(t, "the links to be signaled", func() bool {
		return b.drainTopics().Has(clabinternaltuibackend.TopicLinks)
	})
}

func TestRequestsThatCannotRun(t *testing.T) {
	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}
	stopped := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "client1"}

	tests := map[string]struct {
		run     func(b *testBackend) (clabinternaltuibackend.JobID, error)
		wantErr string
	}{
		"deploy without a file": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Deploy(clabinternaltuibackend.DeployRequest{Lab: "unknown"})
			},
			wantErr: "no topology file known",
		},
		"deploy of a missing file": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Deploy(clabinternaltuibackend.DeployRequest{TopoFile: "/gone/x.clab.yml"})
			},
			wantErr: "does not exist",
		},
		"destroy of nothing": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Destroy(clabinternaltuibackend.DestroyRequest{})
			},
			wantErr: "no lab given",
		},
		"redeploy of a lab whose file is gone": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Redeploy(clabinternaltuibackend.RedeployRequest{Lab: "orphan"})
			},
			wantErr: "does not exist",
		},
		// Without the topology file the command finds no nodes and does nothing, which
		// must not pass for success.
		"node action on a lab whose file is gone": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "orphan", Action: clabinternaltuibackend.NodeStop,
				})
			},
			wantErr: `cannot stop nodes of lab "orphan": its topology file ` +
				"/gone/orphan.clab.yml is missing",
		},
		"node action on a lab that is not deployed": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "nope", Action: clabinternaltuibackend.NodeStop,
				})
			},
			wantErr: "not deployed",
		},
		"unknown node action": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "fabric", Action: "pause",
				})
			},
			wantErr: "unknown node action",
		},
		"save of a lab whose file is gone": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Save(clabinternaltuibackend.SaveRequest{Lab: "orphan"})
			},
			wantErr: "is missing",
		},
		"netem without an interface": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.SetNetem(clabinternaltuibackend.NetemRequest{Node: leaf1})
			},
			wantErr: "no interface given",
		},
		"netem on an unknown node": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.ResetNetem(clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "x"}, "e1")
			},
			wantErr: "not found",
		},
		"netem on a stopped node": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.ResetNetem(stopped, "eth1")
			},
			wantErr: "not running",
		},
		"jitter without delay": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.SetNetem(clabinternaltuibackend.NetemRequest{
					Node: leaf1, Interface: "e1-1",
					Impairment: clabinternaltuibackend.Impairment{Jitter: time.Millisecond},
				})
			},
			wantErr: "jitter cannot be set without a delay",
		},
		"loss beyond everything": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.SetNetem(clabinternaltuibackend.NetemRequest{
					Node: leaf1, Interface: "e1-1",
					Impairment: clabinternaltuibackend.Impairment{LossPct: 100.5},
				})
			},
			wantErr: "packet loss must be between",
		},
		"negative delay": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.SetNetem(clabinternaltuibackend.NetemRequest{
					Node: leaf1, Interface: "e1-1",
					Impairment: clabinternaltuibackend.Impairment{Delay: -time.Second},
				})
			},
			wantErr: "cannot be negative",
		},
		"negative corruption": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.SetNetem(clabinternaltuibackend.NetemRequest{
					Node: leaf1, Interface: "e1-1",
					Impairment: clabinternaltuibackend.Impairment{CorruptionPct: -1},
				})
			},
			wantErr: "packet corruption must be between",
		},
		"share with an unknown tool": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Share(clabinternaltuibackend.ShareRequest{
					Lab: "fabric", Tool: "tmate", Action: clabinternaltuibackend.ShareAttach,
				})
			},
			wantErr: "unknown terminal sharing tool",
		},
		"share with an unknown action": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Share(clabinternaltuibackend.ShareRequest{
					Lab: "fabric", Tool: clabinternaltuibackend.ShareSSHX, Action: "toggle",
				})
			},
			wantErr: "unknown terminal sharing action",
		},
		"share for a lab that is not deployed": {
			run: func(b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Share(clabinternaltuibackend.ShareRequest{
					Lab: "nope", Tool: clabinternaltuibackend.ShareSSHX,
					Action: clabinternaltuibackend.ShareAttach,
				})
			},
			wantErr: "not deployed",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t)

			containers := fabricContainers(t)
			containers[3].State = "exited"
			containers[3].Status = "Exited (0) 1 minute ago"
			containers = append(containers, nodeContainer("orphan", "n1", "/gone/orphan.clab.yml"))
			b.setLabs(containers...)

			_, err := tt.run(b)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want one about %q", err, tt.wantErr)
			}

			if b.run.count() != 0 {
				t.Error("a command was run for a request that cannot run")
			}
		})
	}
}

func TestDestroyOfLabWhoseFileIsGone(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(nodeContainer("orphan", "n1", "/gone/orphan.clab.yml"))

	// Destroying is what is left to do with such a lab, and works by name.
	if _, err := b.Destroy(
		clabinternaltuibackend.DestroyRequest{Lab: "orphan", Cleanup: true},
	); err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)
	if diff := cmp.Diff(
		argv("destroy -r docker --name orphan --cleanup"),
		proc.spec.args,
	); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}

	if proc.spec.dir != "" {
		t.Errorf("the command runs in %q, a directory that does not exist", proc.spec.dir)
	}
}

func TestDeployFindsTheFileOfAKnownLab(t *testing.T) {
	file := filepath.Join(t.TempDir(), "idle.clab.yml")
	writeFile(t, file, "name: idle\n")

	b := newTestBackend(t)
	b.files.replace([]topoFile{readTopoFile(file)})

	if _, err := b.Deploy(clabinternaltuibackend.DeployRequest{Lab: "idle"}); err != nil {
		t.Fatal(err)
	}

	want := argv("deploy -r docker -t " + file + " --format json")
	if diff := cmp.Diff(want, b.run.proc(t, 0).spec.args); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}
}

func TestRelativePathsAreResolvedHere(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(fabricContainers(t)...)

	// The working directory is reported with symbolic links resolved.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(dir, "sub", "lab.clab.yml"), "name: lab\n")
	t.Chdir(dir)

	// The commands run next to the topology file, so paths relative to the directory the
	// UI was started in have to be made absolute.
	if _, err := b.Deploy(
		clabinternaltuibackend.DeployRequest{TopoFile: "sub/lab.clab.yml"},
	); err != nil {
		t.Fatal(err)
	}

	if _, err := b.Save(
		clabinternaltuibackend.SaveRequest{Lab: "fabric", CopyTo: "saved"},
	); err != nil {
		t.Fatal(err)
	}

	deploy, save := b.run.proc(t, 0), b.run.proc(t, 1)

	if got := deploy.spec.args[4]; got != filepath.Join(dir, "sub", "lab.clab.yml") {
		t.Errorf("topology file = %q", got)
	}

	if got := save.spec.args[len(save.spec.args)-1]; got != filepath.Join(dir, "saved") {
		t.Errorf("copy directory = %q", got)
	}
}
