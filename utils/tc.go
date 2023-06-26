package utils

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func SetDelay(nsPath string, iface string, delay int64) error {
	link, err := GetNamespaceInterface(nsPath, iface)
	if err != nil {
		return err
	}

	pid, err := PidFromNSPath(nsPath)
	if err != nil {
		return err
	}

	pidfd, err := pidfdOpen(pid, 0)
	if err != nil {
		return err
	}

	tcnl, err := tc.Open(&tc.Config{
		NetNS: int(pidfd),
	})
	if err != nil {
		return err
	}

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(link.Attrs().Index),
			Handle:  core.BuildHandle(0xFFFF, 0x0000),
			Parent:  0xFFFFFFF1,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "netem",
			Netem: &tc.Netem{
				Latency64: &delay,
			},
		},
	}

	err = tcnl.Qdisc().Replace(&qdisc)
	if err != nil {
		return err
	}

	return nil
}

func SetJitter(nsPath string, iface string, jitter int64) error {
	link, err := GetNamespaceInterface(nsPath, iface)
	if err != nil {
		return err
	}

	pid, err := PidFromNSPath(nsPath)
	if err != nil {
		return err
	}

	pidfd, err := pidfdOpen(pid, 0)
	if err != nil {
		return err
	}

	tcnl, err := tc.Open(&tc.Config{
		NetNS: int(pidfd),
	})
	if err != nil {
		return err
	}

	lat := int64(640000000)
	jit := int64(320000000)

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(link.Attrs().Index),
			Handle:  core.BuildHandle(0xFFFF, 0x0000),
			Parent:  0xFFFFFFF1,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "netem",
			Netem: &tc.Netem{
				Latency64: &lat,
				Jitter64:  &jit,
				Qopt: tc.NetemQopt{
					Loss: uint32(math.Round(math.MaxUint32 * 0.5)),
				},
			},
		},
	}

	err = tcnl.Qdisc().Replace(&qdisc)
	if err != nil {
		return err
	}

	return nil
}

// PidFromNSPath extratcts the pid from the NSPath string
func PidFromNSPath(ns string) (int, error) {
	re := regexp.MustCompile(`.*/(?P<pid>\d+)/ns/net$`)
	matches := re.FindStringSubmatch(ns)
	if len(matches) == 0 {
		return -1, fmt.Errorf("unable to extract pid from provided NSPath %q", ns)
	}
	return strconv.Atoi(matches[1])

}

func GetNamespaceInterface(nsPath string, iface string) (netlink.Link, error) {
	netNamespace, err := ns.GetNS(nsPath)
	if err != nil {
		return nil, err
	}

	var link netlink.Link
	err = netNamespace.Do(func(_ ns.NetNS) error {
		link, err = netlink.LinkByName(iface)
		if err != nil {
			return fmt.Errorf("failed to resolve link: %v", err)
		}
		return nil
	})

	return link, nil
}

type pidFD int // file descriptor that refers to a process
const syscallPidfdOpen = 434

func pidfdOpen(pid int, flags uint) (pidFD, error) {
	fd, _, errno := syscall.Syscall(syscallPidfdOpen, uintptr(pid), uintptr(flags), 0)
	if errno != 0 {
		return 0, errno
	}
	return pidFD(fd), nil
}
