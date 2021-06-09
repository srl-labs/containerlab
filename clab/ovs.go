// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/vishvananda/netlink"
)

func (veth *vEthEndpoint) toOvsBridge() error {
	var vethNS ns.NetNS
	var err error
	// bridge is in the host netns, thus we need to get current netns
	if vethNS, err = ns.GetCurrentNS(); err != nil {
		return err
	}
	err = vethNS.Do(func(_ ns.NetNS) error {
		_, err := netlink.LinkByName(veth.OvsBridge)
		if err != nil {
			return fmt.Errorf("could not find ovs bridge %q: %v", veth.OvsBridge, err)
		}

		c := ovs.New(
			// Prepend "sudo" to all commands.
			ovs.Sudo(),
		)

		if err := c.VSwitch.AddPort(veth.OvsBridge, veth.LinkName); err != nil {
			return err
		}

		if err = netlink.LinkSetUp(veth.Link); err != nil {
			return fmt.Errorf("failed to set %q up: %v", veth.LinkName, err)
		}
		return nil
	})
	return err
}
