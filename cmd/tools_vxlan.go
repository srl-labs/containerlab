// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"net"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clablinks "github.com/srl-labs/containerlab/links"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

func vxlanCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "vxlan",
		Short: "VxLAN interface commands",
	}

	vxlanCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "create vxlan interface",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return vxlanCreate(cobraCmd, o)
		},
	}

	c.AddCommand(vxlanCreateCmd)
	vxlanCreateCmd.Flags().UintVarP(
		&o.ToolsVxlan.ID,
		"id",
		"i",
		o.ToolsVxlan.ID,
		"VxLAN ID (VNI)")
	vxlanCreateCmd.Flags().StringVarP(
		&o.ToolsVxlan.Remote,
		"remote",
		"",
		o.ToolsVxlan.Remote,
		"address of the remote VTEP",
	)
	vxlanCreateCmd.Flags().StringVarP(
		&o.ToolsVxlan.ParentDevice,
		"dev",
		"",
		o.ToolsVxlan.ParentDevice,
		"parent (source) interface name for VxLAN",
	)
	vxlanCreateCmd.Flags().StringVarP(
		&o.ToolsVxlan.Link,
		"link",
		"l",
		o.ToolsVxlan.Link,
		"link to which 'attach' vxlan tunnel with tc redirect",
	)
	vxlanCreateCmd.Flags().UintVarP(
		&o.ToolsVxlan.MTU,
		"mtu",
		"m",
		o.ToolsVxlan.MTU,
		"VxLAN MTU",
	)
	vxlanCreateCmd.Flags().UintVarP(
		&o.ToolsVxlan.Port,
		"port",
		"p",
		o.ToolsVxlan.Port,
		"VxLAN Destination UDP Port",
	)

	err := vxlanCreateCmd.MarkFlagRequired("remote")
	if err != nil {
		return nil, err
	}

	err = vxlanCreateCmd.MarkFlagRequired("link")
	if err != nil {
		return nil, err
	}

	vxlanDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "delete vxlan interface",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return vxlanDelete(o)
		},
	}

	c.AddCommand(vxlanDeleteCmd)
	vxlanDeleteCmd.Flags().StringVarP(
		&o.ToolsVxlan.DeletionPrefix,
		"prefix",
		"p",
		o.ToolsVxlan.DeletionPrefix,
		"delete all containerlab created VxLAN interfaces which start with this prefix",
	)

	return c, nil
}

func vxlanCreate(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	if _, err := netlink.LinkByName(o.ToolsVxlan.Link); err != nil {
		return fmt.Errorf("failed to lookup link %q: %v",
			o.ToolsVxlan.Link, err)
	}
	// if vxlan device was not set specifically, we will use
	// the device that is reported by `ip route get $remote`
	if o.ToolsVxlan.ParentDevice == "" {
		r, err := clabutils.GetRouteForIP(net.ParseIP(o.ToolsVxlan.Remote))
		if err != nil {
			return fmt.Errorf(
				"failed to find a route to VxLAN remote address %s", o.ToolsVxlan.Remote,
			)
		}

		o.ToolsVxlan.ParentDevice = r.Interface.Name
	}

	vxlraw := &clablinks.LinkVxlanRaw{
		Remote:          o.ToolsVxlan.Remote,
		VNI:             int(o.ToolsVxlan.ID),
		ParentInterface: o.ToolsVxlan.ParentDevice,
		LinkCommonParams: clablinks.LinkCommonParams{
			MTU: int(o.ToolsVxlan.MTU),
		},
		UDPPort:  int(o.ToolsVxlan.Port),
		LinkType: clablinks.LinkTypeVxlanStitch,
		Endpoint: *clablinks.NewEndpointRaw(
			"host",
			o.ToolsVxlan.Link,
			"",
		),
	}

	rp := &clablinks.ResolveParams{
		Nodes: map[string]clablinks.Node{
			"host": clablinks.GetHostLinkNode(),
		},
		VxlanIfaceNameOverwrite: o.ToolsVxlan.Link,
	}

	link, err := vxlraw.Resolve(rp)
	if err != nil {
		return err
	}

	vxl, ok := link.(*clablinks.VxlanStitched)
	if !ok {
		return fmt.Errorf("not a VxlanStitched link")
	}

	err = vxl.DeployWithExistingVeth(ctx)
	if err != nil {
		return err
	}

	return nil
}

func vxlanDelete(o *Options) error {
	ls, err := clabutils.GetLinksByNamePrefix(o.ToolsVxlan.DeletionPrefix)
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
}
