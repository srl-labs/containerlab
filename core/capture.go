package core

import (
	"context"
	"io"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// CaptureInterfaces lists the lab link interfaces Capture accepts for the given nodes, or for
// every running node, with the peer at the other end of each link. With Docker and Podman, the
// interfaces come from the topology's links; lab runtimes list them through their packet
// capturer.
func (c *CLab) CaptureInterfaces(
	ctx context.Context,
	nodeNames ...string,
) ([]clablabruntime.CaptureInterface, error) {
	if c.LabRuntime == nil {
		return c.localCaptureInterfacesOf(ctx, nodeNames)
	}

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
// and identity default to the CLab's. With Docker and Podman, containerlab reads the interfaces
// itself through packet sockets in the nodes' network namespaces, which needs root; lab
// runtimes capture through their packet capturer.
func (c *CLab) Capture(ctx context.Context, req clablabruntime.CaptureRequest, w io.Writer) error {
	if c.LabRuntime == nil {
		return c.captureLocal(ctx, req, w)
	}

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
	capturer, err := labRuntimeCapability[clablabruntime.PacketCapturer](c, "packet capture")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	return capturer, nil
}
