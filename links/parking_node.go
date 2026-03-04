package links

import (
	"context"
	"fmt"
	"path/filepath"

	clabutils "github.com/srl-labs/containerlab/utils"
)

type ParkingNode struct {
	GenericLinkNode
	containerName string
}

func NewParkingNode(containerName string) (*ParkingNode, error) {
	parkName := clabutils.ParkingNetnsName(containerName)

	parkPath, err := clabutils.CreateOrGetNamedNetNS(parkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get an existing or create a new parking netns %q: %w", parkName, err)
	}

	return &ParkingNode{
		GenericLinkNode: GenericLinkNode{
			shortname: parkName,
			endpoints: []Endpoint{},
			nspath:    parkPath,
		},
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
		GenericLinkNode: GenericLinkNode{
			shortname: nsName,
			endpoints: []Endpoint{},
			nspath:    nsPath,
		},
		containerName: containerName,
	}, nil
}

func (p *ParkingNode) NSPath() string {
	return p.nspath
}

func (p *ParkingNode) RepointSymlink() error {
	return clabutils.LinkContainerNS(p.nspath, p.containerName)
}

// moveBackEndpoints is intended to restore interface if a restoration has errored.
// ie, move back to parking if restoring parked interfaces back to the ctr failed.
func (p *ParkingNode) moveBackEndpoints(ctx context.Context, endpoints []Endpoint) {
	for _, ep := range endpoints {
		_ = ep.MoveTo(ctx, p, nil)
	}
	_ = p.RepointSymlink()
}

func (p *ParkingNode) RestoreInterfaces(ctx context.Context, dst Node) error {
	endpoints := dst.GetEndpoints()

	// make sure the ifaces belong to parkingnode
	for _, ep := range endpoints {
		ep.SetNode(p)
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

func (*ParkingNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}
