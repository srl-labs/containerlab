package core

import (
	"bytes"
	"context"
	"encoding/binary"
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

// TestC9sClusterCapture captures through the library against a deployed c9s lab, as an identity
// that holds only the clabernetes-lab-editor role in the lab namespace. It runs only when
// CLAB_C9S_TEST_LAB names the lab and CLAB_C9S_TEST_CAPTURE_USER the identity (tests/14-clabernetes
// sets both). The lab must have linux nodes "client" (10.10.10.1) and "server" (10.10.10.2)
// linked by eth1. CLAB_C9S_TEST_FORBIDDEN_USER names an identity without a binding.
func TestC9sClusterCapture(t *testing.T) {
	lab := os.Getenv("CLAB_C9S_TEST_LAB")
	user := os.Getenv("CLAB_C9S_TEST_CAPTURE_USER")
	if lab == "" || user == "" {
		t.Skip("CLAB_C9S_TEST_LAB or CLAB_C9S_TEST_CAPTURE_USER is not set")
	}

	newCLab := func(t *testing.T, user string) *CLab {
		t.Helper()

		c, err := NewContainerLab(
			WithRuntime(clablabruntime.ClabernetesRuntimeName, nil),
			WithTopologyName(lab),
			WithLabRuntimeIdentity(clablabruntime.Identity{User: user}),
		)
		if err != nil {
			t.Fatal(err)
		}

		return c
	}
	c := newCLab(t, user)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("version check", func(t *testing.T) {
		if err := c.CheckLabRuntimeSupport(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("capture interfaces", func(t *testing.T) {
		interfaces, err := c.CaptureInterfaces(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := []clablabruntime.CaptureInterface{
			{Node: "client", Interface: "eth1", PeerNode: "server", PeerInterface: "eth1"},
			{Node: "server", Interface: "eth1", PeerNode: "client", PeerInterface: "eth1"},
		}
		if len(interfaces) != 2 || interfaces[0] != want[0] || interfaces[1] != want[1] {
			t.Fatalf("interfaces = %+v", interfaces)
		}
	})

	t.Run("capture both ends of a link", func(t *testing.T) {
		pinged := make(chan error, 1)
		go func() {
			time.Sleep(time.Second)
			pinged <- c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
				NodeName: "client",
				Command:  []string{"ping", "-c", "4", "-i", "0.3", "10.10.10.2"},
			}, clablabruntime.AttachStreams{Stdout: io.Discard})
		}()

		var out bytes.Buffer
		err := c.Capture(ctx, clablabruntime.CaptureRequest{
			Targets: []clablabruntime.CaptureTarget{
				{Node: "client", Interface: "eth1"}, {Node: "server", Interface: "eth1"},
			},
			Duration: 5 * time.Second,
		}, &out)
		if err != nil {
			t.Fatal(err)
		}
		if err := <-pinged; err != nil {
			t.Fatalf("ping: %v", err)
		}

		names, icmp := summarizePCAPNG(t, out.Bytes())
		if len(names) != 2 || names[0] != "client:eth1" || names[1] != "server:eth1" {
			t.Fatalf("interfaces = %v", names)
		}
		// every echo request and reply crosses both ends
		if icmp[0] < 8 || icmp[1] < 8 {
			t.Fatalf("ICMP packets per interface = %v, want at least 8 on each", icmp)
		}
	})

	t.Run("restart as lab editor", func(t *testing.T) {
		if err := c.RestartNodes(ctx, []string{"server"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("forbidden identity", func(t *testing.T) {
		forbidden := os.Getenv("CLAB_C9S_TEST_FORBIDDEN_USER")
		if forbidden == "" {
			t.Skip("CLAB_C9S_TEST_FORBIDDEN_USER is not set")
		}

		err := newCLab(t, forbidden).Capture(ctx, clablabruntime.CaptureRequest{
			Targets:  []clablabruntime.CaptureTarget{{Node: "client", Interface: "eth1"}},
			Duration: time.Second,
		}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("capture as %s = %v, want forbidden", forbidden, err)
		}
	})
}

// summarizePCAPNG returns the interface names of a little-endian pcapng stream and its IPv4 ICMP
// packets per interface ID.
func summarizePCAPNG(t *testing.T, data []byte) ([]string, map[uint32]int) {
	t.Helper()

	var names []string
	icmp := map[uint32]int{}
	for len(data) >= 12 {
		blockType := binary.LittleEndian.Uint32(data[0:4])
		length := int(binary.LittleEndian.Uint32(data[4:8]))
		if length < 12 || length > len(data) {
			t.Fatalf("invalid block length %d", length)
		}
		body := data[8 : length-4]
		switch blockType {
		case 1: // Interface Description Block
			options := body[8:]
			for len(options) >= 4 {
				code := binary.LittleEndian.Uint16(options[0:2])
				size := int(binary.LittleEndian.Uint16(options[2:4]))
				if code == 0 {
					break
				}
				if code == 2 {
					names = append(names, string(options[4:4+size]))
				}
				options = options[4+size+(4-size%4)%4:]
			}
		case 6: // Enhanced Packet Block
			id := binary.LittleEndian.Uint32(body[0:4])
			frame := body[20 : 20+binary.LittleEndian.Uint32(body[12:16])]
			if len(frame) > 23 && frame[12] == 0x08 && frame[13] == 0x00 && frame[23] == 1 {
				icmp[id]++
			}
		}
		data = data[length:]
	}

	return names, icmp
}
