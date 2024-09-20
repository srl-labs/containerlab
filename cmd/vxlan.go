// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

var (
	vxlanRemote  string
	cntLink      string
	parentDev    string
	vxlanMTU     int
	vxlanID      int
	delPrefix    string
	vxlanUDPPort int
)

func init() {
	toolsCmd.AddCommand(vxlanCmd)
	vxlanCmd.AddCommand(vxlanCreateCmd)
	vxlanCmd.AddCommand(vxlanDeleteCmd)

	vxlanCreateCmd.Flags().IntVarP(&vxlanID, "id", "i", 10, "VxLAN ID (VNI)")
	vxlanCreateCmd.Flags().StringVarP(&vxlanRemote, "remote", "", "", "address of the remote VTEP")
	vxlanCreateCmd.Flags().StringVarP(&parentDev, "dev", "", "",
		"parent (source) interface name for VxLAN")
	vxlanCreateCmd.Flags().StringVarP(&cntLink, "link", "l", "",
		"link to which 'attach' vxlan tunnel with tc redirect")
	vxlanCreateCmd.Flags().IntVarP(&vxlanMTU, "mtu", "m", 0, "VxLAN MTU")
	vxlanCreateCmd.Flags().IntVarP(&vxlanUDPPort, "port", "p", 14789, "VxLAN Destination UDP Port")

	_ = vxlanCreateCmd.MarkFlagRequired("remote")
	_ = vxlanCreateCmd.MarkFlagRequired("link")

	vxlanDeleteCmd.Flags().StringVarP(&delPrefix, "prefix", "p", "vx-",
		"delete all containerlab created VxLAN interfaces which start with this prefix")
}

// vxlanCmd represents the vxlan command container.
var vxlanCmd = &cobra.Command{
	Use:   "vxlan",
	Short: "VxLAN interface commands",
}

var vxlanCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create vxlan interface",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if _, err := netlink.LinkByName(cntLink); err != nil {
			return fmt.Errorf("failed to lookup link %q: %v",
				cntLink, err)
		}
		// if vxlan device was not set specifically, we will use
		// the device that is reported by `ip route get $remote`
		if parentDev == "" {
			r, err := utils.GetRouteForIP(net.ParseIP(vxlanRemote))
			if err != nil {
				return fmt.Errorf("failed to find a route to VxLAN remote address %s", vxlanRemote)
			}

			parentDev = r.Interface.Name
		}

		vxlraw := &links.LinkVxlanRaw{
			Remote:          vxlanRemote,
			VNI:             vxlanID,
			ParentInterface: parentDev,
			LinkCommonParams: links.LinkCommonParams{
				MTU: vxlanMTU,
			},
			UDPPort:  vxlanUDPPort,
			LinkType: links.LinkTypeVxlanStitch,
			Endpoint: *links.NewEndpointRaw(
				"host",
				cntLink,
				"",
			),
		}

		rp := &links.ResolveParams{
			Nodes: map[string]links.Node{
				"host": links.GetHostLinkNode(),
			},
			VxlanIfaceNameOverwrite: cntLink,
		}

		link, err := vxlraw.Resolve(rp)
		if err != nil {
			return err
		}

		var vxl *links.VxlanStitched
		var ok bool
		if vxl, ok = link.(*links.VxlanStitched); !ok {
			return fmt.Errorf("not a VxlanStitched link")
		}

		err = vxl.DeployWithExistingVeth(ctx)
		if err != nil {
			return err
		}

		return nil
	},
}

var vxlanDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete vxlan interface",
	RunE: func(_ *cobra.Command, _ []string) error {
		var ls []netlink.Link
		var err error

		ls, err = utils.GetLinksByNamePrefix(delPrefix)
		if err != nil {
			return err
		}
		for _, l := range ls {
			if l.Type() != "vxlan" {
				continue
			}
			log.Infof("Deleting VxLAN link %s", l.Attrs().Name)
			err := netlink.LinkDel(l)
			if err != nil {
				log.Warnf("error when deleting link %s: %v", l.Attrs().Name, err)
			}
		}
		return nil
	},
}
