// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// VxLAN is a structure to describe vxlan endpoint
// adopted from https://github.com/redhat-nfvpe/koko/blob/bd156c82bf25837545fb109c69c7b91c3457b318/api/koko_api.go#L46
type VxLAN struct {
	Name     string // interface name
	ParentIf string // parent interface name
	ID       int    // VxLan ID
	Remote   net.IP // VxLan destination address
	MTU      int    // VxLan Interface MTU (with VxLan encap), used mirroring
	UDPPort  int    // VxLan UDP port (src/dest, no range, single value)
}

// AddVxLanInterface creates VxLan interface by given vxlan object.
func AddVxLanInterface(vxlan VxLAN) (err error) {
	var parentIf netlink.Link
	var vxlanIf netlink.Link
	log.Infof("Adding VxLAN link %s to remote address %s via %s with VNI %v", vxlan.Name, vxlan.Remote, vxlan.ParentIf, vxlan.ID)

	// before creating vxlan interface, check if it doesn't exist already
	if vxlanIf, err = netlink.LinkByName(vxlan.Name); err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			return fmt.Errorf("failed to check if VxLAN interface %s exists: %v", vxlan.Name, err)
		}
	}
	if vxlanIf != nil {
		return fmt.Errorf("interface %s already exists", vxlan.Name)
	}

	if parentIf, err = netlink.LinkByName(vxlan.ParentIf); err != nil {
		return fmt.Errorf("failed to get VxLAN parent interface %s: %v", vxlan.ParentIf, err)
	}

	vxlanconf := netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:   vxlan.Name,
			TxQLen: 1000,
		},
		VxlanId:      vxlan.ID,
		VtepDevIndex: parentIf.Attrs().Index,
		Group:        vxlan.Remote,
		Port:         vxlan.UDPPort,
		Learning:     true,
		L2miss:       true,
		L3miss:       true,
	}
	if vxlan.MTU != 0 {
		vxlanconf.LinkAttrs.MTU = vxlan.MTU
	}
	err = netlink.LinkAdd(&vxlanconf)

	if err != nil {
		return fmt.Errorf("failed to add VxLAN interface %s: %v", vxlan.Name, err)
	}

	if err := netlink.LinkSetUp(&vxlanconf); err != nil {
		return fmt.Errorf("failed to set %q up: %v",
			vxlanconf.Name, err)
	}
	return nil
}
