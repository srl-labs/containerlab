package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// TestC9sClusterNodeAccess drives the node access helpers against a deployed c9s lab. It runs
// only when CLAB_C9S_TEST_LAB names the lab (tests/14-clabernetes sets it) and uses the current
// kubeconfig context. The lab must have a running linux node "client" serving HTTP on port 80
// (ghcr.io/srl-labs/network-multitool). With CLAB_C9S_TEST_USER set, every call impersonates
// that user, who must be allowed to read the lab and use pods/exec, pods/portforward and
// pods/log in its namespace; CLAB_C9S_TEST_FORBIDDEN_USER names a user without these rights.
func TestC9sClusterNodeAccess(t *testing.T) {
	lab := os.Getenv("CLAB_C9S_TEST_LAB")
	if lab == "" {
		t.Skip("CLAB_C9S_TEST_LAB is not set")
	}

	identity := clablabruntime.Identity{User: os.Getenv("CLAB_C9S_TEST_USER")}
	c, err := NewContainerLab(
		WithRuntime(clablabruntime.ClabernetesRuntimeName, nil),
		WithTopologyName(lab),
		WithLabRuntimeIdentity(identity),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("locate", func(t *testing.T) {
		locations, err := c.LocateLabRuntimeNodes(ctx, "client")
		if err != nil {
			t.Fatal(err)
		}
		if len(locations) != 1 || locations[0].Pod == "" || locations[0].Container == "" ||
			locations[0].NodeUID == "" {
			t.Fatalf("locations = %+v", locations)
		}
	})

	t.Run("attach with a terminal", func(t *testing.T) {
		resize := make(chan clablabruntime.TerminalSize, 1)
		resize <- clablabruntime.TerminalSize{Width: 101, Height: 33}

		var out bytes.Buffer
		err := c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
			NodeName: "client",
			Command:  []string{"sh", "-c", "sleep 1; stty size; echo attached-$((6*7))"},
			TTY:      true,
		}, clablabruntime.AttachStreams{Stdout: &out, Resize: resize})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "33 101") ||
			!strings.Contains(out.String(), "attached-42") {
			t.Fatalf("terminal output = %q", out.String())
		}
	})

	t.Run("attach with stdin", func(t *testing.T) {
		var out bytes.Buffer
		err := c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
			NodeName: "client",
			Command:  []string{"cat"},
		}, clablabruntime.AttachStreams{
			Stdin:  strings.NewReader("hello from stdin\n"),
			Stdout: &out,
			Stderr: io.Discard,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.String() != "hello from stdin\n" {
			t.Fatalf("stdin echo = %q", out.String())
		}
	})

	t.Run("attach exit code", func(t *testing.T) {
		err := c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
			NodeName: "client",
			Command:  []string{"sh", "-c", "exit 3"},
		}, clablabruntime.AttachStreams{Stdout: io.Discard, Stderr: io.Discard})

		var exitErr *clablabruntime.CommandExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 3 {
			t.Fatalf("exit error = %v, want code 3", err)
		}
	})

	t.Run("port-forward", func(t *testing.T) {
		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return c.DialLabRuntimeNodePort(ctx, "client", 80)
				},
			},
		}
		resp, err := client.Get("http://client/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK || len(body) == 0 {
			t.Fatalf("HTTP through port-forward = %d %q", resp.StatusCode, body)
		}
	})

	t.Run("logs", func(t *testing.T) {
		tail := int64(5)
		var out bytes.Buffer
		if err := c.NodeLogs(ctx, NodeLogOptions{
			NodeName: "client", Tail: &tail, Timestamps: true,
		}, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) == 0 || len(lines) > 5 {
			t.Fatalf("log lines = %q", out.String())
		}
		if _, err := time.Parse(time.RFC3339Nano, strings.Fields(lines[0])[0]); err != nil {
			t.Fatalf("log line %q has no timestamp: %v", lines[0], err)
		}
	})

	t.Run("stats", func(t *testing.T) {
		deadline := time.Now().Add(3 * time.Minute)
		for {
			stats, err := c.SampleLabRuntimeNodeStats(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, sample := range stats {
				if sample.NodeName == "client" && sample.MemoryBytes > 0 {
					t.Logf("client stats: %+v", sample)
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("no metrics for client: %+v", stats)
			}
			time.Sleep(10 * time.Second)
		}
	})

	forbidden := os.Getenv("CLAB_C9S_TEST_FORBIDDEN_USER")
	if forbidden == "" {
		return
	}

	t.Run("forbidden identity", func(t *testing.T) {
		other, err := NewContainerLab(
			WithRuntime(clablabruntime.ClabernetesRuntimeName, nil),
			WithTopologyName(lab),
			WithLabRuntimeIdentity(clablabruntime.Identity{User: forbidden}),
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = other.LocateLabRuntimeNodes(ctx, "client")
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("impersonated user without rights: error = %v, want forbidden", err)
		}
	})
}
