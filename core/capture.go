package core

import (
	"context"
	"fmt"
	"io"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// CaptureInterfaces lists the lab link interfaces Capture accepts for the given nodes, or for
// every running node, with the peer at the other end of each link. Only lab runtimes with a
// packet capturer (c9s) support it for now.
func (c *CLab) CaptureInterfaces(
	ctx context.Context,
	nodeNames ...string,
) ([]clablabruntime.CaptureInterface, error) {
	capturer, err := c.packetCapturer()
	if err != nil {
		return nil, err
	}

	return capturer.CaptureInterfaces(ctx, clablabruntime.NodeRequest{
		Name:     c.Config.Name,
		Nodes:    nodeNames,
		Identity: c.labRuntimeIdentity,
	})
}

// Capture streams pcapng from lab link interfaces to w until req.Duration passed,
// req.PacketLimit packets were written or ctx ends; an ended ctx is not an error. The lab name
// and identity default to the CLab's. Only lab runtimes with a packet capturer (c9s) support it
// for now.
func (c *CLab) Capture(ctx context.Context, req clablabruntime.CaptureRequest, w io.Writer) error {
	capturer, err := c.packetCapturer()
	if err != nil {
		return err
	}

	if req.Name == "" {
		req.Name = c.Config.Name
	}
	if req.Identity.IsZero() {
		req.Identity = c.labRuntimeIdentity
	}

	return capturer.Capture(ctx, req, w)
}

// LabRuntimeInterfaces lists the link interfaces of the lab's nodes, or of one node, with their
// state, in the shape ListContainersInterfaces reports for local containers.
func (c *CLab) LabRuntimeInterfaces(
	ctx context.Context,
	nodeName string,
) ([]*clabtypes.ContainerInterfaces, error) {
	inspector, err := labRuntimeCapability[clablabruntime.InterfaceInspector](
		c, "inspecting interfaces")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	req := clablabruntime.NodeRequest{Name: c.Config.Name, Identity: c.labRuntimeIdentity}
	if nodeName != "" {
		req.Nodes = []string{nodeName}
	}

	return inspector.InspectInterfaces(ctx, req)
}

func (c *CLab) packetCapturer() (clablabruntime.PacketCapturer, error) {
	if c.LabRuntime == nil {
		runtimeName := c.globalRuntimeName
		if runtimeName == "" {
			runtimeName = "docker"
		}

		return nil, fmt.Errorf("packet capture is not supported by runtime %q yet; "+
			"it is available with the %q lab runtime", runtimeName,
			clablabruntime.ClabernetesRuntimeName)
	}

	capturer, err := labRuntimeCapability[clablabruntime.PacketCapturer](c, "packet capture")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	return capturer, nil
}
