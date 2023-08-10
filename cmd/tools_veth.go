// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	AEnd = ""
	BEnd = ""
	MTU  = clab.DefaultVethLinkMTU
)

func init() {
	toolsCmd.AddCommand(vethCmd)
	vethCmd.AddCommand(vethCreateCmd)
	vethCreateCmd.Flags().StringVarP(&AEnd, "a-endpoint", "a", "",
		"veth endpoint A in the format of <containerA-name>:<interface-name> or <endpointA-type>:<endpoint-name>:<interface-name>")
	vethCreateCmd.Flags().StringVarP(&BEnd, "b-endpoint", "b", "",
		"veth endpoint B in the format of <containerB-name>:<interface-name> or <endpointB-type>:<endpoint-name>:<interface-name>")
	vethCreateCmd.Flags().IntVarP(&MTU, "mtu", "m", MTU, "link MTU")
}

var vethCmd = &cobra.Command{
	Use:   "veth",
	Short: "veth operations",
}

var vethCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a veth interface and attach its sides to the specified containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
			clab.WithDebug(debug),
		}
		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var vethAEndpoint *vethEndpoint
		var vethBEndpoint *vethEndpoint

		if vethAEndpoint, err = parseVethEndpoint(AEnd); err != nil {
			return err
		}
		if vethBEndpoint, err = parseVethEndpoint(BEnd); err != nil {
			return err
		}

		aNode := &types.NodeConfig{
			LongName:  vethAEndpoint.node,
			ShortName: vethAEndpoint.node,
			Kind:      vethAEndpoint.kind,
			NSPath:    "__host", // NSPath defaults to __host to make attachment to host. For attachment to containers the NSPath will be overwritten
		}

		bNode := &types.NodeConfig{
			LongName:  vethBEndpoint.node,
			ShortName: vethBEndpoint.node,
			Kind:      vethBEndpoint.kind,
			NSPath:    "__host",
		}

		if aNode.Kind == "container" {
			aNode.NSPath, err = c.GlobalRuntime().GetNSPath(ctx, aNode.LongName)
			if err != nil {
				return err
			}
		}
		if bNode.Kind == "container" {
			bNode.NSPath, err = c.GlobalRuntime().GetNSPath(ctx, bNode.LongName)
			if err != nil {
				return err
			}
		}
		// generate mac for endpoint A
		aMac, err := utils.GenMac(links.ClabOUI)
		if err != nil {
			return err
		}
		endpointA := types.Endpoint{
			Node:         aNode,
			EndpointName: vethAEndpoint.iface,
			MAC:          aMac.String(),
		}

		// generate mac for endpoint B
		bMac, err := utils.GenMac(links.ClabOUI)
		if err != nil {
			return err
		}
		endpointB := types.Endpoint{
			Node:         bNode,
			EndpointName: vethBEndpoint.iface,
			MAC:          bMac.String(),
		}

		link := &types.Link{
			A:   &endpointA,
			B:   &endpointB,
			MTU: MTU,
		}

		if err := c.CreateVirtualWiring(link); err != nil {
			return err
		}
		log.Info("veth interface successfully created!")
		return nil
	},
}

func parseVethEndpoint(s string) (*vethEndpoint, error) {
	supportedKinds := []string{"ovs-bridge", "bridge", "host"}
	ve := &vethEndpoint{}
	arr := strings.Split(s, ":")
	if (len(arr) != 2) && (len(arr) != 3) {
		return ve, errors.New("malformed veth endpoint reference")
	}
	switch len(arr) {
	case 2:
		ve.kind = "container"
		if arr[0] == "host" {
			ve.kind = "host"
		}
		ve.node = arr[0]
		ve.iface = arr[1]
	case 3:
		if _, ok := utils.StringInSlice(supportedKinds, arr[0]); !ok {
			return nil, fmt.Errorf("node type %s is not supported, supported nodes are %q", arr[0], supportedKinds)
		}
		ve.kind = arr[0]
		ve.node = arr[1]
		ve.iface = arr[2]
	}
	return ve, nil
}

type vethEndpoint struct {
	kind  string // kind of the node to attach to: ovs-bridge, bridge, host or implicitly container
	node  string
	iface string
}
