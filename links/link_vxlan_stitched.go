package links

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/vishvananda/netlink"
)

type VxlanStitched struct {
	LinkCommonParams
	vxlanLink *LinkVxlan
	vethLink  *LinkVEth
	// the veth does not distinguish between endpoints. but we
	// need to know which endpoint is the one used for
	// stitching therefore we get that endpoint separately
	vethStitchEp Endpoint
}

// NewVxlanStitched constructs a new VxlanStitched object.
func NewVxlanStitched(vxlan *LinkVxlan, veth *LinkVEth, vethStitchEp Endpoint) *VxlanStitched {
	// init the VxlanStitched struct
	vxlanStitched := &VxlanStitched{
		LinkCommonParams: vxlan.LinkCommonParams,
		vxlanLink:        vxlan,
		vethLink:         veth,
		vethStitchEp:     vethStitchEp,
	}

	return vxlanStitched
}

// DeployWithExistingVeth provisions the stitched vxlan link whilst the
// veth interface does already exist, hence it is not created as part of this
// deployment.
func (l *VxlanStitched) DeployWithExistingVeth(ctx context.Context) error {
	return l.internalDeploy(ctx, nil, true)
}

// Deploy provisions the stitched vxlan link with all its underlying sub-links.
func (l *VxlanStitched) Deploy(ctx context.Context, ep Endpoint) error {
	return l.internalDeploy(ctx, ep, false)
}

func (l *VxlanStitched) internalDeploy(ctx context.Context, ep Endpoint, skipVethCreation bool) error {
	// deploy the vxlan link
	err := l.vxlanLink.Deploy(ctx, ep)
	if err != nil {
		return err
	}

	// the veth creation might be skipped if it already exists
	if !skipVethCreation {
		err = l.vethLink.Deploy(ctx, ep)
		if err != nil {
			return err
		}
	}

	// unidirectionally stitch the vxlan endpoint to the veth endpoint
	err = stitch(l.vxlanLink.localEndpoint, l.vethStitchEp)
	if err != nil {
		return err
	}

	// unidirectionally stitch the veth endpoint to the vxlan endpoint
	err = stitch(l.vethStitchEp, l.vxlanLink.localEndpoint)
	if err != nil {
		return err
	}

	return nil
}

// Remove deprovisions the stitched vxlan link.
func (l *VxlanStitched) Remove(ctx context.Context) error {
	// remove the veth link piece
	err := l.vethLink.Remove(ctx)
	if err != nil {
		log.Debug(err)
	}
	// remove the vxlan link piece
	err = l.vxlanLink.Remove(ctx)
	if err != nil {
		log.Debug(err)
	}
	// set the links DeploymentState to Removed
	l.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

// GetEndpoints returns the endpoints that are part of the link.
func (l *VxlanStitched) GetEndpoints() []Endpoint {
	return []Endpoint{l.vxlanLink.localEndpoint, l.vxlanLink.remoteEndpoint}
}

// GetType returns the LinkType enum.
func (*VxlanStitched) GetType() LinkType {
	return LinkTypeVxlanStitch
}

// stitch provisions the tc rules to stitch two endpoints together in a unidirectional fashion
// it should take the veth and the vxlan endpoints of the root namespace.
func stitch(ep1, ep2 Endpoint) error {
	var err error
	// collection of netlink links for the given endpoints
	netlinkLinks := make([]netlink.Link, 0, 2)
	log.Infof("configuring ingress mirroring with tc in the direction of %s -> %s", ep1, ep2)

	// retrieve the respective netlink Links
	for _, endpointName := range []string{ep1.GetIfaceName(), ep2.GetIfaceName()} {
		var l netlink.Link
		if l, err = netlink.LinkByName(endpointName); err != nil {
			return fmt.Errorf("failed to lookup %q: %v", endpointName, err)
		}
		netlinkLinks = append(netlinkLinks, l)
	}

	// tc qdisc add dev $SRC_IFACE ingress
	qdisc := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: netlinkLinks[0].Attrs().Index,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	if err = netlink.QdiscAdd(qdisc); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	// tc filter add dev $SRC_IFACE parent fffff:
	// protocol all
	// u32 match u32 0 0
	// action mirred egress mirror dev $DST_IFACE
	filter := &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: netlinkLinks[0].Attrs().Index,
			Parent:    netlink.MakeHandle(0xffff, 0),
			Protocol:  syscall.ETH_P_ALL,
		},
		Sel: &netlink.TcU32Sel{
			Keys: []netlink.TcU32Key{
				{
					Mask: 0x0,
					Val:  0,
				},
			},
			Flags: netlink.TC_U32_TERMINAL,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs: netlink.ActionAttrs{
					Action: netlink.TC_ACT_PIPE,
				},
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      netlinkLinks[1].Attrs().Index,
			},
		},
	}
	// finally add the tc filter
	return netlink.FilterAdd(filter)
}
