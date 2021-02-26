package clab

import (
	"fmt"
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// BindIfacesWithTC creates TC ingress mirred redirection between a pair of interfaces
// to make them virtually form a kind of a veth pair
// see https://netdevops.me/2021/transparently-redirecting-packets/frames-between-interfaces/#4-tc-to-the-rescue
// used https://github.com/redhat-nfvpe/koko/blob/bd156c82bf25837545fb109c69c7b91c3457b318/api/koko_api.go#L224
// and https://gist.github.com/mcastelino/7d85f4164ffdaf48242f9281bb1d0f9b#gistcomment-2734521
func BindIfacesWithTC(if1, if2 string) (err error) {
	if err := SetIngressMirror(if1, if2); err != nil {
		return err
	}
	if err := SetIngressMirror(if2, if1); err != nil {
		return err
	}
	return err
}

// SetIngressMirror sets TC to mirror ingress from given port
// as MirrorIngress.
func SetIngressMirror(src, dst string) (err error) {
	var linkSrc, linkDest netlink.Link
	log.Infof("configuring ingress mirroring with tc in the direction of %s -> %s", src, dst)

	if linkSrc, err = netlink.LinkByName(src); err != nil {
		return fmt.Errorf("failed to lookup %q: %v",
			src, err)
	}

	if linkDest, err = netlink.LinkByName(dst); err != nil {
		return fmt.Errorf("failed to lookup %q: %v",
			dst, err)
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
