package core

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

// accessLabRuntime records the node access requests core sends.
type accessLabRuntime struct {
	clablabruntime.LabRuntime

	nodeRequest   clablabruntime.NodeRequest
	attachRequest clablabruntime.AttachRequest
	logRequest    clablabruntime.LogRequest
	statsRequest  clablabruntime.InspectRequest
	port          int
	checkIdentity clablabruntime.Identity
}

func (r *accessLabRuntime) CheckSupport(_ context.Context, identity clablabruntime.Identity) error {
	r.checkIdentity = identity

	return nil
}

func (r *accessLabRuntime) LocateNode(
	_ context.Context,
	req clablabruntime.NodeRequest,
) ([]clablabruntime.NodeLocation, error) {
	r.nodeRequest = req

	return []clablabruntime.NodeLocation{{NodeName: "r1", Pod: "lab1-r1-abc"}}, nil
}

func (r *accessLabRuntime) Attach(
	_ context.Context,
	req clablabruntime.AttachRequest,
	_ clablabruntime.AttachStreams,
) error {
	r.attachRequest = req

	return nil
}

func (r *accessLabRuntime) StreamLogs(
	_ context.Context,
	req clablabruntime.LogRequest,
	w io.Writer,
) error {
	r.logRequest = req
	_, err := io.WriteString(w, "device log\n")

	return err
}

func (r *accessLabRuntime) DialPort(
	_ context.Context,
	req clablabruntime.NodeRequest,
	port int,
) (net.Conn, error) {
	r.nodeRequest = req
	r.port = port
	local, remote := net.Pipe()
	_ = remote.Close()

	return local, nil
}

func (r *accessLabRuntime) SampleStats(
	_ context.Context,
	req clablabruntime.InspectRequest,
) ([]clablabruntime.NodeStats, error) {
	r.statsRequest = req

	return []clablabruntime.NodeStats{{NodeName: "r1", CPUMilliCores: 5}}, nil
}

var testIdentity = clablabruntime.Identity{User: "clab:alice", Groups: []string{"clab:users"}}

func newAccessTestCLab(lr clablabruntime.LabRuntime) *CLab {
	return &CLab{
		Config:             &Config{Name: "lab1"},
		LabRuntime:         lr,
		globalRuntimeName:  clablabruntime.ClabernetesRuntimeName,
		labRuntimeIdentity: testIdentity,
	}
}

func TestLabRuntimeAccessHelpersFillLabAndIdentity(t *testing.T) {
	t.Parallel()

	lr := &accessLabRuntime{}
	c := newAccessTestCLab(lr)
	ctx := context.Background()

	locations, err := c.LocateLabRuntimeNodes(ctx, "R1")
	if err != nil || len(locations) != 1 {
		t.Fatalf("LocateLabRuntimeNodes() = %+v, %v", locations, err)
	}
	if lr.nodeRequest.Name != "lab1" || lr.nodeRequest.Nodes[0] != "R1" ||
		!lr.nodeRequest.Identity.Equal(testIdentity) {
		t.Fatalf("locate request = %+v", lr.nodeRequest)
	}

	if err := c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
		NodeName: "r1", TTY: true, Command: []string{"bash"},
	}, clablabruntime.AttachStreams{}); err != nil {
		t.Fatal(err)
	}
	if lr.attachRequest.Name != "lab1" || !lr.attachRequest.TTY ||
		!lr.attachRequest.Identity.Equal(testIdentity) {
		t.Fatalf("attach request = %+v", lr.attachRequest)
	}

	other := clablabruntime.Identity{User: "clab:bob"}
	if err := c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
		Name: "lab2", NodeName: "r1", Identity: other,
	}, clablabruntime.AttachStreams{}); err != nil {
		t.Fatal(err)
	}
	if lr.attachRequest.Name != "lab2" || !lr.attachRequest.Identity.Equal(other) {
		t.Fatalf("explicit attach request was overridden: %+v", lr.attachRequest)
	}

	conn, err := c.DialLabRuntimeNodePort(ctx, "r1", 22)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if lr.port != 22 || lr.nodeRequest.Nodes[0] != "r1" ||
		!lr.nodeRequest.Identity.Equal(testIdentity) {
		t.Fatalf("dial request = %+v port %d", lr.nodeRequest, lr.port)
	}

	stats, err := c.SampleLabRuntimeNodeStats(ctx)
	if err != nil || len(stats) != 1 || lr.statsRequest.Name != "lab1" ||
		!lr.statsRequest.Identity.Equal(testIdentity) {
		t.Fatalf("stats = %+v, %v, request %+v", stats, err, lr.statsRequest)
	}

	if err := c.CheckLabRuntimeSupport(ctx); err != nil ||
		!lr.checkIdentity.Equal(testIdentity) {
		t.Fatalf("CheckLabRuntimeSupport() = %v as %+v", err, lr.checkIdentity)
	}

	tail := int64(10)
	var out strings.Builder
	if err := c.NodeLogs(ctx, NodeLogOptions{
		NodeName: "r1", Tail: &tail, Follow: true, Timestamps: true, Container: "clabwire",
	}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := lr.logRequest
	if out.String() != "device log\n" || got.Name != "lab1" || got.NodeName != "r1" ||
		got.Container != "clabwire" || *got.Tail != 10 || !got.Follow || !got.Timestamps ||
		!got.Identity.Equal(testIdentity) {
		t.Fatalf("log request = %+v, output %q", got, out.String())
	}
}

func TestLabRuntimeAccessHelpersReportMissingSupport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	unsupported := newAccessTestCLab(&recordingDestroyLabRuntime{})
	if _, err := unsupported.LocateLabRuntimeNodes(ctx); err == nil ||
		!strings.Contains(err.Error(), "does not support locating nodes") {
		t.Fatalf("unsupported lab runtime error = %v", err)
	}

	if err := unsupported.CheckLabRuntimeSupport(ctx); err == nil ||
		!strings.Contains(err.Error(), "does not support checking the lab runtime version") {
		t.Fatalf("unsupported version check error = %v", err)
	}

	local := &CLab{Config: &Config{Name: "lab1"}}
	if _, err := local.DialLabRuntimeNodePort(ctx, "r1", 22); err == nil ||
		!strings.Contains(err.Error(), "needs a lab runtime") {
		t.Fatalf("local runtime error = %v", err)
	}

	unnamed := newAccessTestCLab(&accessLabRuntime{})
	unnamed.Config.Name = ""
	if _, err := unnamed.SampleLabRuntimeNodeStats(ctx); err == nil {
		t.Fatal("stats without a lab name succeeded")
	}
	if err := unnamed.NodeLogs(ctx, NodeLogOptions{NodeName: "r1"}, io.Discard, io.Discard); err == nil {
		t.Fatal("logs without a lab name succeeded")
	}
}

// logWritingRuntime adds the optional log writer to the container runtime mock.
type logWritingRuntime struct {
	*clabmocksmockruntime.MockContainerRuntime

	containerName string
	opts          clabruntime.ContainerLogOptions
}

func (r *logWritingRuntime) WriteContainerLogs(
	_ context.Context,
	containerName string,
	opts clabruntime.ContainerLogOptions,
	stdout, stderr io.Writer,
) error {
	r.containerName = containerName
	r.opts = opts
	_, _ = io.WriteString(stdout, "out\n")
	_, _ = io.WriteString(stderr, "err\n")

	return nil
}

func TestNodeLogsWithALocalRuntime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mock := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	mock.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, filters []*clabtypes.GenericFilter) ([]clabruntime.GenericContainer, error) {
			labels := map[string]string{}
			for _, filter := range filters {
				labels[filter.Field] = filter.Match
			}
			if labels[clabconstants.Containerlab] != "lab1" {
				return nil, nil
			}
			if labels[clabconstants.NodeName] != "r1" {
				return nil, nil
			}

			return []clabruntime.GenericContainer{{Names: []string{"clab-lab1-r1"}}}, nil
		}).AnyTimes()

	runtime := &logWritingRuntime{MockContainerRuntime: mock}
	c := &CLab{
		Config:   &Config{Name: "lab1"},
		Runtimes: map[string]clabruntime.ContainerRuntime{"docker": runtime},
	}

	var stdout, stderr strings.Builder
	if err := c.NodeLogs(context.Background(), NodeLogOptions{NodeName: "r1", Follow: true},
		&stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if runtime.containerName != "clab-lab1-r1" || !runtime.opts.Follow || runtime.opts.Tail != nil ||
		stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Fatalf("container %q opts %+v stdout %q stderr %q",
			runtime.containerName, runtime.opts, stdout.String(), stderr.String())
	}

	err := c.NodeLogs(context.Background(), NodeLogOptions{NodeName: "missing"},
		io.Discard, io.Discard)
	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing node error = %v", err)
	}

	if err := c.NodeLogs(context.Background(), NodeLogOptions{NodeName: "r1", Container: "x"},
		io.Discard, io.Discard); err == nil {
		t.Fatal("a container override was accepted for a local runtime")
	}
}

func TestLogTail(t *testing.T) {
	t.Parallel()

	ten, negative := int64(10), int64(-1)
	for _, tt := range []struct {
		tail *int64
		want string
	}{{nil, "all"}, {&negative, "all"}, {&ten, "10"}} {
		if got := clabruntime.LogTail(tt.tail); got != tt.want {
			t.Fatalf("LogTail(%v) = %q, want %q", tt.tail, got, tt.want)
		}
	}
}
