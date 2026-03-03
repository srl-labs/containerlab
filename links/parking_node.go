package links

import (
	"context"
	"fmt"
	"path/filepath"

	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type ParkingNode struct {
	node          *GenericLinkNode
	containerName string
}

func NewParkingNode(containerName string) (*ParkingNode, error) {
	parkName := clabutils.ParkingNetnsName(containerName)

	parkPath, err := clabutils.CreateOrGetNamedNetNS(parkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get an existing or create a new parking netns %q: %w", parkName, err)
	}

	return &ParkingNode{
		node:          NewGenericLinkNode(parkName, parkPath),
		containerName: containerName,
	}, nil
}

func GetParkingNode(containerName string) (*ParkingNode, error) {
	nsName := clabutils.ParkingNetnsName(containerName)
	nsPath := filepath.Join("/run/netns", nsName)

	if !clabutils.FileOrDirExists(nsPath) {
		return nil, fmt.Errorf("parking netns %q does not exist. failed to get parking node.", nsName)
	}

	return &ParkingNode{
		node:          NewGenericLinkNode(nsName, nsPath),
		containerName: containerName,
	}, nil
}

func (p *ParkingNode) NSPath() string {
	return p.node.nspath
}

func (p *ParkingNode) RepointSymlink() error {
	return clabutils.LinkContainerNS(p.node.nspath, p.containerName)
}

// moveBackEndpoints is intended to restore interface if a restoration has errored.
// ie, move back to parking if restoring parked interfaces back to the ctr failed.
func (p *ParkingNode) moveBackEndpoints(ctx context.Context, endpoints []Endpoint) {
	for _, ep := range endpoints {
		_ = ep.MoveTo(ctx, p.node, nil)
	}
	_ = p.RepointSymlink()
}

func (p *ParkingNode) ParkInterfaces(ctx context.Context, src Node) error {
	endpoints := src.GetEndpoints()
	moved := make([]Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		if err := ep.MoveTo(ctx, p.node, &MoveOptions{PreMove: netlink.LinkSetDown}); err != nil {
			for _, m := range moved {
				_ = m.MoveTo(ctx, src, nil)
				_ = m.SetUp(ctx)
			}
			return err
		}
		moved = append(moved, ep)
	}

	if err := p.RepointSymlink(); err != nil {
		return fmt.Errorf("failed to repoint symlink for %q: %w", p.containerName, err)
	}

	return nil
}

func (p *ParkingNode) RestoreInterfaces(ctx context.Context, dst Node) error {
	endpoints := dst.GetEndpoints()

	// make sure the ifaces belong to parkingnode
	for _, ep := range endpoints {
		ep.SetNode(p.node)
	}

	moved := make([]Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		// if any failure, move back the moved endpoints.
		if err := ep.MoveTo(ctx, dst, nil); err != nil {
			p.moveBackEndpoints(ctx, moved)
			return err
		}

		if err := ep.SetUp(ctx); err != nil {
			p.moveBackEndpoints(ctx, moved)
			return err
		}

		moved = append(moved, ep)
	}

	return nil
}
