package links

import (
	"context"
	"fmt"
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type VxlanStitched struct {
	vxlanLink *LinkVxlan
	vethLink  *LinkVEth
	// the veth does not distinguist between endpoints. but we
	// need to  know which endpoint is the one used for
	// stitching therefore we get that endpoint seperately
	vethStitchEp Endpoint
}

func NewVxlanStitched(vxlan *LinkVxlan, veth *LinkVEth, vethStitchEp Endpoint) *VxlanStitched {
	// init the VxlanStitched struct
	vxlanStitched := &VxlanStitched{
		vxlanLink:    vxlan,
		vethLink:     veth,
		vethStitchEp: vethStitchEp,
	}

	return vxlanStitched
}

func (l *VxlanStitched) Deploy(ctx context.Context) error {
	err := l.vxlanLink.Deploy(ctx)
	if err != nil {
		return err
	}

	err = l.vethLink.Deploy(ctx)
	if err != nil {
		return err
	}

	err = stitch(l.vxlanLink.localEndpoint, l.vethStitchEp)
	if err != nil {
		return err
	}
	err = stitch(l.vethStitchEp, l.vxlanLink.localEndpoint)
	if err != nil {
		return err
	}

	return nil
}

func (l *VxlanStitched) Remove(_ context.Context) error {
	// TODO
	return nil
}

func (l *VxlanStitched) GetEndpoints() []Endpoint {
	return []Endpoint{l.vxlanLink.localEndpoint, l.vxlanLink.remoteEndpoint}
}

func (l *VxlanStitched) GetType() LinkType {
	return LinkTypeVxlanStitch
}

func stitch(ep1, ep2 Endpoint) error {

	var err error
	var linkSrc, linkDest netlink.Link
	log.Infof("configuring ingress mirroring with tc in the direction of %s -> %s", ep1, ep2)

	if linkSrc, err = netlink.LinkByName(ep1.GetIfaceName()); err != nil {
		return fmt.Errorf("failed to lookup %q: %v",
			ep1, err)
	}

	if linkDest, err = netlink.LinkByName(ep2.GetIfaceName()); err != nil {
		return fmt.Errorf("failed to lookup %q: %v",
			ep2, err)
	}

	// tc qdisc add dev $SRC_IFACE ingress
	qdisc := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkSrc.Attrs().Index,
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
			LinkIndex: linkSrc.Attrs().Index,
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
				Ifindex:      linkDest.Attrs().Index,
			},
		},
	}

	return netlink.FilterAdd(filter)
}
