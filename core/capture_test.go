package core

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// captureLabRuntime records the capture requests core sends.
type captureLabRuntime struct {
	clablabruntime.LabRuntime

	nodeRequest    clablabruntime.NodeRequest
	captureRequest clablabruntime.CaptureRequest
}

func (r *captureLabRuntime) CaptureInterfaces(
	_ context.Context,
	req clablabruntime.NodeRequest,
) ([]clablabruntime.CaptureInterface, error) {
	r.nodeRequest = req

	return []clablabruntime.CaptureInterface{{Node: "r1", Interface: "eth1"}}, nil
}

func (r *captureLabRuntime) Capture(
	_ context.Context,
	req clablabruntime.CaptureRequest,
	w io.Writer,
) error {
	r.captureRequest = req
	_, err := io.WriteString(w, "pcapng")

	return err
}

func (r *captureLabRuntime) InspectInterfaces(
	_ context.Context,
	req clablabruntime.NodeRequest,
) ([]*clabtypes.ContainerInterfaces, error) {
	r.nodeRequest = req

	return []*clabtypes.ContainerInterfaces{{ContainerName: "lab1-r1"}}, nil
}

func TestCaptureHelpersFillLabAndIdentity(t *testing.T) {
	t.Parallel()

	lr := &captureLabRuntime{}
	c := newAccessTestCLab(lr)
	ctx := context.Background()

	interfaces, err := c.CaptureInterfaces(ctx, "R1")
	if err != nil || len(interfaces) != 1 {
		t.Fatalf("CaptureInterfaces() = %+v, %v", interfaces, err)
	}
	if lr.nodeRequest.Name != "lab1" || lr.nodeRequest.Nodes[0] != "R1" ||
		!lr.nodeRequest.Identity.Equal(testIdentity) {
		t.Fatalf("interfaces request = %+v", lr.nodeRequest)
	}

	var out bytes.Buffer
	if err := c.Capture(ctx, clablabruntime.CaptureRequest{
		Targets: []clablabruntime.CaptureTarget{{Node: "r1", Interface: "eth1"}},
	}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "pcapng" || lr.captureRequest.Name != "lab1" ||
		!lr.captureRequest.Identity.Equal(testIdentity) {
		t.Fatalf("capture request = %+v, output %q", lr.captureRequest, out.String())
	}

	ifaces, err := c.LabRuntimeInterfaces(ctx, "r1")
	if err != nil || len(ifaces) != 1 || lr.nodeRequest.Nodes[0] != "r1" {
		t.Fatalf("LabRuntimeInterfaces() = %+v, %v, request %+v", ifaces, err, lr.nodeRequest)
	}
	if _, err := c.LabRuntimeInterfaces(ctx, ""); err != nil || lr.nodeRequest.Nodes != nil {
		t.Fatalf("all nodes request = %+v, %v", lr.nodeRequest, err)
	}
}

// newLocalCaptureCLab loads the capture test topology. Its host and mgmt-net links register
// endpoints on link nodes the whole process shares, so the tests using it do not run in
// parallel and release the endpoints when they end.
func newLocalCaptureCLab(t *testing.T) *CLab {
	t.Helper()

	c, err := NewContainerLab(WithTopoPath("test_data/capture.clab.yml", nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.resetResolvedLinks)

	return c
}

func TestLocalCaptureInterfacesComeFromTheTopologyLinks(t *testing.T) {
	c := newLocalCaptureCLab(t)

	got, err := c.CaptureInterfaces(context.Background(), "srl1", "client")
	if err != nil {
		t.Fatal(err)
	}
	want := []clablabruntime.CaptureInterface{
		{Node: "client", Interface: "eth1", PeerNode: "srl1", PeerInterface: "e1-2"},
		{Node: "client", Interface: "eth2", PeerNode: "host", PeerInterface: "client-eth2"},
		{Node: "client", Interface: "eth3", PeerNode: "mgmt-net", PeerInterface: "client-eth3"},
		{
			Node: "srl1", Interface: "e1-1", Alias: "ethernet-1/1",
			PeerNode: "srl2", PeerInterface: "e1-1",
		},
		{Node: "srl1", Interface: "e1-2", PeerNode: "client", PeerInterface: "eth1"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("CaptureInterfaces() =\n%+v\nwant\n%+v", got, want)
	}

	if _, err := c.CaptureInterfaces(context.Background(), "r9"); err == nil ||
		!strings.Contains(err.Error(), `node "r9" is not part of lab "capture"`) {
		t.Fatalf("unknown node: error = %v", err)
	}
}

func TestLocalCaptureTargetsResolveByNameOrAlias(t *testing.T) {
	c := newLocalCaptureCLab(t)

	targets, err := c.resolveLocalCaptureTargets([]clablabruntime.CaptureTarget{
		{Node: "srl2", Interface: "e1-1"},
		{Node: "srl1", Interface: "ethernet-1/1"},
		{Node: "clab-capture-client", Interface: "eth2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, target := range targets {
		if target.Exec == nil {
			t.Fatalf("target %s:%s has no namespace exec", target.Node, target.Interface)
		}
		got = append(got, target.Node+":"+target.Interface)
	}
	if want := []string{"srl2:e1-1", "srl1:e1-1", "client:eth2"}; !slices.Equal(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}

	for _, tc := range []struct {
		name    string
		targets []clablabruntime.CaptureTarget
		want    string
	}{
		{
			name:    "management interface",
			targets: []clablabruntime.CaptureTarget{{Node: "client", Interface: "eth0"}},
			want: `interface "eth0" of node "client" is not a capturable link interface; ` +
				"capturable: eth1, eth2, eth3",
		},
		{
			name: "same interface twice",
			targets: []clablabruntime.CaptureTarget{
				{Node: "srl1", Interface: "e1-1"}, {Node: "srl1", Interface: "ethernet-1/1"},
			},
			want: `interface "e1-1" of node "srl1" is requested more than once`,
		},
		{
			name:    "unknown node",
			targets: []clablabruntime.CaptureTarget{{Node: "r9", Interface: "eth1"}},
			want:    `node "r9" is not part of lab "capture"`,
		},
	} {
		if _, err := c.resolveLocalCaptureTargets(tc.targets); err == nil ||
			!strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestLocalCaptureNeedsTheTopology(t *testing.T) {
	t.Parallel()

	c := &CLab{Config: &Config{Name: "lab1"}, globalRuntimeName: "docker"}
	err := c.Capture(context.Background(), clablabruntime.CaptureRequest{
		Targets: []clablabruntime.CaptureTarget{{Node: "r1", Interface: "eth1"}},
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `the topology of lab "lab1" is not available`) {
		t.Fatalf("error = %v", err)
	}
	if err := c.Capture(context.Background(), clablabruntime.CaptureRequest{},
		io.Discard); err == nil || !strings.Contains(err.Error(), "at least one capture target") {
		t.Fatalf("no targets: error = %v", err)
	}
}

func TestLabRuntimeWithoutCaptureFails(t *testing.T) {
	t.Parallel()

	unsupported := newAccessTestCLab(&recordingDestroyLabRuntime{})
	if err := unsupported.Capture(
		context.Background(),
		clablabruntime.CaptureRequest{},
		io.Discard,
	); err == nil || !strings.Contains(err.Error(), "does not support packet capture") {
		t.Fatalf("lab runtime without capture: error = %v", err)
	}
}
