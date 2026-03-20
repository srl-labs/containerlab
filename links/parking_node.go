package links

import (
	"context"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type ParkingNode struct {
	GenericLinkNode
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
	}, nil
}

func GetParkingNode(containerName string) (*ParkingNode, error) {
	nsName := clabutils.ParkingNetnsName(containerName)

	nsPath, err := clabutils.GetNamedNetNS(nsName)
	if err != nil {
		return nil, err
	}

	return &ParkingNode{
		GenericLinkNode: GenericLinkNode{
			shortname: nsName,
			endpoints: []Endpoint{},
			nspath:    nsPath,
		},
	}, nil
}
func (p *ParkingNode) RepointSymlink(containerName string) error {
	return clabutils.LinkContainerNS(p.nspath, containerName)
}

// ParkInterface moves an endpoint from the source node into the parking namespace.
func (p *ParkingNode) ParkInterface(ctx context.Context, src Node, ep Endpoint) error {
	return src.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ep.GetIfaceName())
		if err != nil {
			return err
		}

		if err := netlink.LinkSetDown(link); err != nil {
			return err
		}

		return p.AddLinkToContainer(ctx, link, func(_ ns.NetNS) error {
			return nil
		})
	})
}

// UnparkInterface moves an endpoint from the parking namespace back to the destination node.
func (p *ParkingNode) UnparkInterface(ctx context.Context, dst Node, ep Endpoint) error {
	return p.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ep.GetIfaceName())
		if err != nil {
			return err
		}

		return dst.AddLinkToContainer(ctx, link, func(_ ns.NetNS) error {
			l, err := netlink.LinkByName(ep.GetIfaceName())
			if err != nil {
				return err
			}

			return netlink.LinkSetUp(l)
		})
	})
}

func (*ParkingNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}
