package core

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

// attachingLabRuntime records the attach request of an attached exec on a lab runtime.
type attachingLabRuntime struct {
	clablabruntime.LabRuntime

	request clablabruntime.AttachRequest
	exitErr error
}

func (r *attachingLabRuntime) Inspect(
	context.Context,
	clablabruntime.InspectRequest,
) (*clablabruntime.LabState, error) {
	return &clablabruntime.LabState{
		Name:      "lab1",
		Namespace: "lab-ns",
		Nodes:     []clablabruntime.NodeState{{Name: "node1"}, {Name: "node2"}},
	}, nil
}

func (r *attachingLabRuntime) Attach(
	_ context.Context,
	req clablabruntime.AttachRequest,
	streams clablabruntime.AttachStreams,
) error {
	r.request = req

	if _, err := io.Copy(streams.Stdout, streams.Stdin); err != nil {
		return err
	}

	return r.exitErr
}

func TestExecAttachedWithALabRuntime(t *testing.T) {
	t.Parallel()

	runtime := &attachingLabRuntime{exitErr: &clablabruntime.CommandExitError{Code: 3}}
	c := &CLab{
		Config:             &Config{Name: "lab1", Topology: clabtypes.NewTopology()},
		TopoPaths:          &clabtypes.TopoPaths{},
		LabRuntime:         runtime,
		globalRuntimeName:  clablabruntime.ClabernetesRuntimeName,
		labRuntimeIdentity: testIdentity,
	}

	var stdout strings.Builder
	err := c.ExecAttached(context.Background(), "sh -c 'cat > /tmp/f'", true,
		clablabruntime.AttachStreams{Stdin: strings.NewReader("config"), Stdout: &stdout},
		WithListFromCliArgs([]string{clabconstants.NodeName + "=node1"}))

	var exitErr *clablabruntime.CommandExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Fatalf("ExecAttached() error = %v, want the command's exit code 3", err)
	}
	want := clablabruntime.AttachRequest{
		Name:      "lab1",
		Namespace: "lab-ns",
		NodeName:  "node1",
		Command:   []string{"sh", "-c", "cat > /tmp/f"},
		TTY:       true,
		Identity:  testIdentity,
	}
	if d := cmp.Diff(want, runtime.request); d != "" {
		t.Fatalf("attach request mismatch (-want +got):\n%s", d)
	}
	if stdout.String() != "config" {
		t.Fatalf("stdin reached the command as %q", stdout.String())
	}

	err = c.ExecAttached(context.Background(), "sh", false, clablabruntime.AttachStreams{})
	if err == nil || !strings.Contains(err.Error(), "matched 2") {
		t.Fatalf("ExecAttached() on two nodes error = %v", err)
	}
}

// attachingRuntime adds the optional attacher to the container runtime mock.
type attachingRuntime struct {
	*clabmocksmockruntime.MockContainerRuntime

	containerName string
	cmd           []string
	tty           bool
}

func (r *attachingRuntime) AttachContainer(
	_ context.Context,
	containerName string,
	cmd []string,
	tty bool,
	streams clablabruntime.AttachStreams,
) error {
	r.containerName = containerName
	r.cmd = cmd
	r.tty = tty
	_, err := io.Copy(streams.Stdout, streams.Stdin)

	return err
}

func TestExecAttachedWithALocalRuntime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mock := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	runtime := &attachingRuntime{MockContainerRuntime: mock}

	mock.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, filters []*clabtypes.GenericFilter) (
			[]clabruntime.GenericContainer, error,
		) {
			containers := []clabruntime.GenericContainer{
				{Names: []string{"clab-lab1-r1"}, Runtime: runtime},
				{Names: []string{"clab-lab1-r2"}, Runtime: runtime},
			}
			for _, filter := range filters {
				if filter.Field == clabconstants.NodeName {
					return containers[:1], nil
				}
			}

			return containers, nil
		}).
		AnyTimes()

	c := &CLab{
		Config:   &Config{Name: "lab1"},
		Runtimes: map[string]clabruntime.ContainerRuntime{"docker": runtime},
	}

	var stdout strings.Builder
	if err := c.ExecAttached(context.Background(), "bash", true,
		clablabruntime.AttachStreams{Stdin: strings.NewReader("exit\n"), Stdout: &stdout},
		WithListFromCliArgs([]string{clabconstants.NodeName + "=r1"})); err != nil {
		t.Fatal(err)
	}
	if runtime.containerName != "clab-lab1-r1" || len(runtime.cmd) != 1 ||
		runtime.cmd[0] != "bash" || !runtime.tty || stdout.String() != "exit\n" {
		t.Fatalf("container %q cmd %q tty %t stdout %q",
			runtime.containerName, runtime.cmd, runtime.tty, stdout.String())
	}

	err := c.ExecAttached(context.Background(), "bash", false, clablabruntime.AttachStreams{})
	if err == nil || !strings.Contains(err.Error(), "matched 2") {
		t.Fatalf("ExecAttached() on two containers error = %v", err)
	}
}

func TestExecAttachedWithoutAnAttachingRuntime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mock := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	mock.EXPECT().GetName().Return("docker").AnyTimes()
	mock.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(
		[]clabruntime.GenericContainer{{Names: []string{"clab-lab1-r1"}, Runtime: mock}}, nil)

	c := &CLab{
		Config:   &Config{Name: "lab1"},
		Runtimes: map[string]clabruntime.ContainerRuntime{"docker": mock},
	}

	err := c.ExecAttached(context.Background(), "bash", false, clablabruntime.AttachStreams{})
	if err == nil || !strings.Contains(err.Error(), "does not support attached exec") {
		t.Fatalf("ExecAttached() error = %v", err)
	}
}
