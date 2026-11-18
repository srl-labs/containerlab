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
		_ = json.NewEncoder(w).Encode(map[string]any{"ExitCode": f.exitCode})
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

	input, _ := io.ReadAll(rw)

	f.mu.Lock()
	tty := f.options.Tty
	f.mu.Unlock()

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout strings.Builder
	err := d.AttachContainer(ctx, "clab-lab1-r1", []string{"sh"}, true,
		clablabruntime.AttachStreams{
			Stdin:  strings.NewReader("exit 3\n"),
			Stdout: &stdout,
			Resize: resize,
		})

	var exitErr *clablabruntime.CommandExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Fatalf("AttachContainer() error = %v, want the command's exit code 3", err)
	}
	if !api.options.Tty || api.options.AttachStderr {
		t.Fatalf("exec options = %+v", api.options)
	}
	if stdout.String() != "exit 3\n" {
		t.Fatalf("stdout %q", stdout.String())
	}

	// the resize is applied in the background, possibly after the session ended
	var resized []string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		api.mu.Lock()
		resized = slices.Clone(api.resized)
		api.mu.Unlock()
		if len(resized) != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(resized) != 1 || resized[0] != "33x101" {
		t.Fatalf("terminal resizes = %q, want [33x101]", resized)
	}
}
