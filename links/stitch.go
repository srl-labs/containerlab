package links

import (
	"fmt"
	"os"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/vishvananda/netlink"
)

// stitch provisions the tc rules to stitch two endpoints together in a unidirectional fashion
// it should take the veth and the vxlan endpoints of the root namespace.
func stitch(ep1, ep2 Endpoint) error {
	var err error
	// collection of netlink links for the given endpoints
	netlinkLinks := make([]netlink.Link, 0, 2)
	log.Debugf("configuring ingress mirroring with tc in the direction of %s -> %s", ep1, ep2)

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
	// action mirred egress redirect dev $DST_IFACE
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
					Action: netlink.TC_ACT_STOLEN,
				},
				MirredAction: netlink.TCA_EGRESS_REDIR,
				Ifindex:      netlinkLinks[1].Attrs().Index,
			},
		},
	}
	// finally add the tc filter
	return netlink.FilterAdd(filter)
}
