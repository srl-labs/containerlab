package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dockerC "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func TestWriteContainerLogs(t *testing.T) {
	for _, tty := range []bool{false, true} {
		var query string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/json"):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Id":     "abc",
					"Name":   "/clab-lab1-r1",
					"Config": map[string]any{"Tty": tty},
					"State":  map[string]any{"Running": true},
				})
			case strings.HasSuffix(r.URL.Path, "/logs"):
				query = r.URL.RawQuery
				if tty {
					_, _ = w.Write([]byte("tty line\n"))
					return
				}
				_, _ = stdcopy.NewStdWriter(w, stdcopy.Stdout).Write([]byte("out line\n"))
				_, _ = stdcopy.NewStdWriter(w, stdcopy.Stderr).Write([]byte("err line\n"))
			default:
				http.NotFound(w, r)
			}
		}))

		cli, err := dockerC.NewClientWithOpts(
			dockerC.WithHost(srv.URL),
			dockerC.WithVersion("1.45"),
		)
		if err != nil {
			t.Fatal(err)
		}
		d := &DockerRuntime{Client: cli}

		tail := int64(5)
		var stdout, stderr strings.Builder
		if err := d.WriteContainerLogs(context.Background(), "clab-lab1-r1",
			clabruntime.ContainerLogOptions{Tail: &tail, Follow: true, Timestamps: true},
			&stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		srv.Close()

		for _, param := range []string{"tail=5", "follow=1", "timestamps=1", "stdout=1", "stderr=1"} {
			if !strings.Contains(query, param) {
				t.Fatalf("logs query %q lacks %s", query, param)
			}
		}
		if tty {
			if stdout.String() != "tty line\n" || stderr.String() != "" {
				t.Fatalf("tty logs: stdout %q stderr %q", stdout.String(), stderr.String())
			}
			continue
		}
		if stdout.String() != "out line\n" || stderr.String() != "err line\n" {
			t.Fatalf("demuxed logs: stdout %q stderr %q", stdout.String(), stderr.String())
		}
	}
}
