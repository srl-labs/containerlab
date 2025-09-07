// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/jsimonetti/rtnetlink/rtnl"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/vishvananda/netlink"
)

// BridgeByName returns a *netlink.Bridge referenced by its name.
func BridgeByName(name string) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not lookup %q: %v", name, err)
	}
	br, ok := l.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("%q already exists but is not a bridge", name)
	}
	return br, nil
}

// LinkContainerNS creates a symlink for containers network namespace
// so that it can be managed by iproute2 utility.
func LinkContainerNS(nspath, containerName string) error {
	CreateDirectory("/run/netns/", clabconstants.PermissionsDirDefault)
	dst := "/run/netns/" + containerName
	if _, err := os.Lstat(dst); err == nil {
		os.Remove(dst)
	}
	err := os.Symlink(nspath, dst)
	if err != nil {
		return err
	}
	return nil
}

// GenMac generates a random MAC address for a given OUI.
func GenMac(oui string) (net.HardwareAddr, error) {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)

	hwa, err := net.ParseMAC(fmt.Sprintf("%s:%02x:%02x:%02x", oui, buf[0], buf[1], buf[2]))
	return hwa, err
}

// DeleteNetnsSymlink deletes a network namespace and removes the symlink created by LinkContainerNS func.
func DeleteNetnsSymlink(n string) error {
	log.Debug("Deleting netns symlink: ", n)
	sl := fmt.Sprintf("/run/netns/%s", n)
	err := os.Remove(sl)
	if err != nil {
		log.Debug("Failed to delete netns symlink by path:", sl)
	}
	return nil
}

// LinkIPs returns IPv4/IPv6 addresses assigned to a link referred by its name.
func LinkIPs(ln string) (v4addrs, v6addrs []netlink.Addr, err error) {
	l, err := netlink.LinkByName(ln)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup link %q: %w", ln, err)
	}

	v4addrs, err = netlink.AddrList(l, netlink.FAMILY_V4)
	if err != nil {
		return nil, nil, err
	}

	v6addrs, err = netlink.AddrList(l, netlink.FAMILY_V6)
	if err != nil {
		return nil, nil, err
	}

	return
}

// FirstLinkIPs returns string representation of the first IPv4/v6 address
// found for a link referenced by name.
func FirstLinkIPs(ln string) (v4, v6 string, err error) {
	v4addrs, v6addrs, err := LinkIPs(ln)
	if err != nil {
		return
	}

	if len(v4addrs) != 0 {
		v4 = v4addrs[0].IP.String()
	}

	if len(v6addrs) != 0 {
		v6 = v6addrs[0].IP.String()
	}

	return v4, v6, err
}

// GetLinksByNamePrefix returns a list of links whose name matches a prefix.
func GetLinksByNamePrefix(prefix string) ([]netlink.Link, error) {
	// filtered list of interfaces
	if prefix == "" {
		return nil, fmt.Errorf("prefix is not specified")
	}
	var fls []netlink.Link

	ls, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, l := range ls {
		if strings.HasPrefix(l.Attrs().Name, prefix) {
			fls = append(fls, l)
		}
	}
	if len(fls) == 0 {
		return nil, fmt.Errorf("no links found by specified prefix %s", prefix)
	}
	return fls, nil
}

func GetRouteForIP(ip net.IP) (*rtnl.Route, error) {
	conn, err := rtnl.Dial(nil)
	if err != nil {
		return nil, fmt.Errorf("can't establish netlink connection: %s", err)
	}
	defer conn.Close()

	r, err := conn.RouteGet(ip)

	return r, err
}
