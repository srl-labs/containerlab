package core

import (
	"bytes"
	"context"
	"io"
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

func TestCaptureIsNotSupportedByLocalRuntimesYet(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"docker", "podman"} {
		c := &CLab{Config: &Config{Name: "lab1"}, globalRuntimeName: name}
		err := c.Capture(context.Background(), clablabruntime.CaptureRequest{}, io.Discard)
		if err == nil || !strings.Contains(err.Error(),
			`packet capture is not supported by runtime "`+name+`" yet`) {
			t.Fatalf("%s: error = %v", name, err)
		}
		if _, err := c.CaptureInterfaces(context.Background()); err == nil {
			t.Fatalf("%s: CaptureInterfaces() succeeded", name)
		}
	}

	unsupported := newAccessTestCLab(&recordingDestroyLabRuntime{})
	if err := unsupported.Capture(
		context.Background(),
		clablabruntime.CaptureRequest{},
		io.Discard,
	); err == nil || !strings.Contains(err.Error(), "does not support packet capture") {
		t.Fatalf("lab runtime without capture: error = %v", err)
	}
}
