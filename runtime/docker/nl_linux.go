//go:build linux
// +build linux

package docker

import "github.com/vishvananda/netlink"

func getIPv4Family() int {
    return netlink.FAMILY_V4
}
