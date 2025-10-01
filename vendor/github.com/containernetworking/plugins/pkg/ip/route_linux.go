// Copyright 2015-2017 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"net"

	"github.com/vishvananda/netlink"
)

// AddRoute adds a universally-scoped route to a device.
func AddRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipn,
		Gw:        gw,
	})
}

// AddHostRoute adds a host-scoped route to a device.
func AddHostRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_HOST,
		Dst:       ipn,
		Gw:        gw,
	})
}

// AddDefaultRoute sets the default route on the given gateway.
func AddDefaultRoute(gw net.IP, dev netlink.Link) error {
	var defNet *net.IPNet
	if gw.To4() != nil {
		_, defNet, _ = net.ParseCIDR("0.0.0.0/0")
	} else {
		_, defNet, _ = net.ParseCIDR("::/0")
	}
	return AddRoute(defNet, gw, dev)
}

// IsIPNetZero check if the IPNet is "0.0.0.0/0" or "::/0"
// This is needed as go-netlink replaces nil Dst with a '0' IPNet since
// https://github.com/vishvananda/netlink/commit/acdc658b8613655ddb69f978e9fb4cf413e2b830
func IsIPNetZero(ipnet *net.IPNet) bool {
	if ipnet == nil {
		return true
	}
	if ones, _ := ipnet.Mask.Size(); ones != 0 {
		return false
	}
	return ipnet.IP.Equal(net.IPv4zero) || ipnet.IP.Equal(net.IPv6zero)
}
