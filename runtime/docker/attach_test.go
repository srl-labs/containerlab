package docker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerC "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// fakeExecAPI serves the exec endpoints of the Docker API. The exec session echoes its stdin
// and exits with exitCode.
type fakeExecAPI struct {
	exitCode int
	// notStarted makes the exec fail to start, like a missing executable
	notStarted bool
	// running reports the command as still running after the session ended
	running bool
	// endless writes output until the client closes the connection
	endless bool

	mu      sync.Mutex
	options container.ExecOptions
	resized []string
}

func (f *fakeExecAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/containers/clab-lab1-r1/exec"):
		f.mu.Lock()
		_ = json.NewDecoder(r.Body).Decode(&f.options)
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"Id": "exec1"})
	case strings.HasSuffix(r.URL.Path, "/exec/exec1/start"):
		_, _ = io.Copy(io.Discard, r.Body)
		f.echo(w)
	case strings.HasSuffix(r.URL.Path, "/exec/exec1/resize"):
		f.mu.Lock()
		f.resized = append(f.resized, r.URL.Query().Get("h")+"x"+r.URL.Query().Get("w"))
		f.mu.Unlock()
	case strings.HasSuffix(r.URL.Path, "/exec/exec1/json"):
		pid := 42
		if f.notStarted {
			pid = 0
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ExitCode": f.exitCode, "Pid": pid, "Running": f.running,
		})
	default:
		http.NotFound(w, r)
	}
}

// echo upgrades the connection like the exec start endpoint and writes back what it reads until
// the client closes its write side, multiplexed unless the session has a TTY.
func (f *fakeExecAPI) echo(w http.ResponseWriter) {
	conn, rw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	_, _ = rw.WriteString("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	_ = rw.Flush()

	if f.notStarted {
		_, _ = conn.Write(
			[]byte("OCI runtime exec failed: exec: \"nope\": executable file not found\r\n"),
		)
		return
	}

	if f.endless {
		for {
			if _, err := conn.Write([]byte("x")); err != nil {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}

	f.mu.Lock()
	tty, attachStdin := f.options.Tty, f.options.AttachStdin
	f.mu.Unlock()

	var input []byte
	if attachStdin {
		input, _ = io.ReadAll(rw)
	}

	if tty {
		_, _ = conn.Write(input)
		return
	}
	_, _ = stdcopy.NewStdWriter(conn, stdcopy.Stdout).Write(input)
	_, _ = stdcopy.NewStdWriter(conn, stdcopy.Stderr).Write([]byte("err"))
}

func newFakeExecRuntime(t *testing.T, api *fakeExecAPI) *DockerRuntime {
	t.Helper()

	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	cli, err := dockerC.NewClientWithOpts(
		dockerC.WithHost("tcp://"+srv.Listener.Addr().String()),
		dockerC.WithVersion("1.45"),
	)
	if err != nil {
		t.Fatal(err)
	}

	return &DockerRuntime{Client: cli}
}

func TestAttachContainer(t *testing.T) {
	api := &fakeExecAPI{}
	d := newFakeExecRuntime(t, api)

	var stdout, stderr strings.Builder
	err := d.AttachContainer(context.Background(), "clab-lab1-r1",
		[]string{"sh", "-c", "cat > /tmp/f"}, false, clablabruntime.AttachStreams{
			Stdin:  strings.NewReader("config"),
			Stdout: &stdout,
			Stderr: &stderr,
		})
	if err != nil {
		t.Fatal(err)
	}

	if !api.options.AttachStdin || !api.options.AttachStdout || !api.options.AttachStderr ||
		api.options.Tty || strings.Join(api.options.Cmd, "|") != "sh|-c|cat > /tmp/f" {
		t.Fatalf("exec options = %+v", api.options)
	}
	if stdout.String() != "config" || stderr.String() != "err" {
		t.Fatalf("stdout %q stderr %q", stdout.String(), stderr.String())
	}
}

func TestAttachContainerWithATTY(t *testing.T) {
	api := &fakeExecAPI{exitCode: 3}
	d := newFakeExecRuntime(t, api)

	resize := make(chan clablabruntime.TerminalSize, 1)
	resize <- clablabruntime.TerminalSize{Width: 101, Height: 33}

	stdin, input := io.Pipe()
	var stdout strings.Builder
	done := make(chan error, 1)
	go func() {
		done <- d.AttachContainer(context.Background(), "clab-lab1-r1", []string{"sh"}, true,
			clablabruntime.AttachStreams{Stdin: stdin, Stdout: &stdout, Resize: resize})
	}()

	// the size sent before the session starts is its initial size, later sizes resize it
	resize <- clablabruntime.TerminalSize{Width: 120, Height: 40}
	waitForResizes(t, api, []string{"40x120"})

	_, _ = input.Write([]byte("exit 3\n"))
	_ = input.Close()

	var exitErr *clablabruntime.CommandExitError
	if err := <-done; !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Fatalf("AttachContainer() error = %v, want the command's exit code 3", err)
	}
	if !api.options.Tty || !api.options.AttachStdout || !api.options.AttachStderr {
		t.Fatalf("exec options = %+v", api.options)
	}
	if api.options.ConsoleSize == nil || *api.options.ConsoleSize != [2]uint{33, 101} {
		t.Fatalf("console size = %v, want [33 101]", api.options.ConsoleSize)
	}
	if stdout.String() != "exit 3\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func waitForResizes(t *testing.T, api *fakeExecAPI, want []string) {
	t.Helper()

	var resized []string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		api.mu.Lock()
		resized = slices.Clone(api.resized)
		api.mu.Unlock()
		if len(resized) >= len(want) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !slices.Equal(resized, want) {
		t.Fatalf("terminal resizes = %q, want %q", resized, want)
	}
}

func TestAttachContainerWithoutOutputStreams(t *testing.T) {
	api := &fakeExecAPI{exitCode: 2}
	d := newFakeExecRuntime(t, api)

	err := d.AttachContainer(context.Background(), "clab-lab1-r1", []string{"false"}, false,
		clablabruntime.AttachStreams{})

	// without attached output Docker would end the session before the command exits
	var exitErr *clablabruntime.CommandExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("AttachContainer() error = %v, want the command's exit code 2", err)
	}
	if api.options.AttachStdin || !api.options.AttachStdout || !api.options.AttachStderr {
		t.Fatalf("exec options = %+v", api.options)
	}
}

func TestAttachContainerWithACommandThatKeepsRunning(t *testing.T) {
	api := &fakeExecAPI{running: true}
	d := newFakeExecRuntime(t, api)

	// the detach keys end the input, like the end of stdin
	var stdout strings.Builder
	err := d.AttachContainer(context.Background(), "clab-lab1-r1", []string{"sh"}, true,
		clablabruntime.AttachStreams{
			Stdin:  strings.NewReader("ls\n\x10\x11exit\n"),
			Stdout: &stdout,
		})

	if err == nil || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("AttachContainer() error = %v, want the command reported as still running", err)
	}
	if stdout.String() != "ls\n" {
		t.Fatalf("the command got %q, want the input up to the detach keys", stdout.String())
	}
}

func TestAttachContainerEndsWithTheContext(t *testing.T) {
	api := &fakeExecAPI{endless: true}
	d := newFakeExecRuntime(t, api)

	resize := make(chan clablabruntime.TerminalSize, 1)
	ctx, cancel := context.WithCancel(context.Background())
	stdout := &afterReturnWriter{firstWrite: make(chan struct{})}
	go func() {
		<-stdout.firstWrite
		cancel()
	}()

	err := d.AttachContainer(ctx, "clab-lab1-r1", []string{"sh"}, true,
		clablabruntime.AttachStreams{Stdout: stdout, Resize: resize})
	stdout.returned.Store(true)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AttachContainer() error = %v, want context.Canceled", err)
	}

	// no new output or resizes reach the caller or the session after the call returned, only a
	// write that was already in progress
	resize <- clablabruntime.TerminalSize{Width: 120, Height: 40}
	time.Sleep(100 * time.Millisecond)
	if stdout.lateWrites.Load() > 1 {
		t.Fatalf("%d writes after AttachContainer returned", stdout.lateWrites.Load())
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.resized) != 0 {
		t.Fatalf("terminal resizes = %q after AttachContainer returned", api.resized)
	}
}

// afterReturnWriter signals its first write and counts the writes after returned is set.
type afterReturnWriter struct {
	firstWrite chan struct{}
	once       sync.Once
	returned   atomic.Bool
	lateWrites atomic.Int32
}

func (w *afterReturnWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.firstWrite) })
	if w.returned.Load() {
		w.lateWrites.Add(1)
	}

	return len(p), nil
}

func TestAttachContainerWithACommandThatDoesNotStart(t *testing.T) {
	api := &fakeExecAPI{exitCode: 127, notStarted: true}
	d := newFakeExecRuntime(t, api)

	var stdout strings.Builder
	err := d.AttachContainer(context.Background(), "clab-lab1-r1", []string{"nope"}, true,
		clablabruntime.AttachStreams{Stdin: strings.NewReader(""), Stdout: &stdout})

	var exitErr *clablabruntime.CommandExitError
	if err == nil || errors.As(err, &exitErr) ||
		!strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("AttachContainer() error = %v, want why the command didn't start", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q, want nothing", stdout.String())
	}
}
