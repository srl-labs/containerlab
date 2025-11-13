// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

const (
	SIOCETHTOOL     = 0x8946     // linux/sockios.h
	ETHTOOL_GTXCSUM = 0x00000016 // linux/ethtool.h
	ETHTOOL_STXCSUM = 0x00000017 // linux/ethtool.h
	IFNAMSIZ        = 16         // linux/if.h
)

// IFReqData linux/if.h 'struct ifreq'.
type IFReqData struct {
	Name [IFNAMSIZ]byte
	Data uintptr
}

// EthtoolValue linux/ethtool.h 'struct ethtool_value'.
type EthtoolValue struct {
	Cmd  uint32
	Data uint32
}

func ioctlEthtool(fd int, argp uintptr) error {
	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(SIOCETHTOOL), argp)
	if errno != 0 {
		return errno
	}
	return nil
}

// NSEthtoolTXOff EthtoolTXOff wrapper that can be handed straight to Node.ExecFunc().
func NSEthtoolTXOff(cntName, ifaceName string) func(ns.NetNS) error {
	return func(ns.NetNS) error {
		// disabling offload on given interface
		err := EthtoolTXOff(ifaceName)
		if err != nil {
			log.Infof(
				"failed to disable TX checksum offload for %s interface for %s container",
				ifaceName,
				cntName,
			)
		}
		return nil
	}
}

// EthtoolTXOff disables TX checksum offload on specified interface.
func EthtoolTXOff(name string) error {
	if len(name)+1 > IFNAMSIZ {
		return fmt.Errorf("name too long")
	}

	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(socket)

	// Request current value
	value := EthtoolValue{Cmd: ETHTOOL_GTXCSUM}
	request := IFReqData{Data: uintptr(unsafe.Pointer(&value))} // skipcq: GSC-G103
	copy(request.Name[:], name)

	if err := ioctlEthtool(socket, uintptr(unsafe.Pointer(&request))); err != nil { // skipcq: GSC-G103
		return err
	}
	if value.Data == 0 { // if already off, don't try to change
		return nil
	}

	value = EthtoolValue{ETHTOOL_STXCSUM, 0}
	return ioctlEthtool(socket, uintptr(unsafe.Pointer(&request))) // skipcq: GSC-G103
}

// VethPeerIndex gets the interface index of the veth peer for
// the given interface name in the container.
func VethPeerIndex(ifaceName string, peerIndex *int) func(ns.NetNS) error {
	return func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return fmt.Errorf("failed to find %s in container: %w", ifaceName, err)
		}

		// confirm it's a veth.
		veth, ok := link.(*netlink.Veth)
		if !ok {
			return fmt.Errorf("%s is not a veth interface", ifaceName)
		}

		*peerIndex, err = netlink.VethPeerIndex(veth)
		if err != nil {
			return fmt.Errorf("failed to get peer interface index: %w", err)
		}

		return nil
	}
}

// DisableTxOffloadByIndex disables TX checksum offload on an interface by its index.
func DisableTxOffloadByIndex(index int) error {
	link, err := netlink.LinkByIndex(index)
	if err != nil {
		return fmt.Errorf("failed to find interface with index %d: %w", index, err)
	}

	ifaceName := link.Attrs().Name
	log.Debugf("Disabling TX offload on interface %s (index %d)", ifaceName, index)

	if err := EthtoolTXOff(ifaceName); err != nil {
		return fmt.Errorf("failed to disable TX offload on %s: %w", ifaceName, err)
	}

	log.Debug("Successfully disabled TX offload on the host ns interface", "interface", ifaceName)

	return nil
}
