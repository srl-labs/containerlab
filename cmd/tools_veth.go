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

		link := &links.LinkVEth{}

		for _, definition := range []string{AEnd, BEnd} {
			var node, iface, kind string
			if node, iface, kind, err = parseVethEndpoint(definition); err != nil {
				return err
			}

			nspathA, err := c.GlobalRuntime().GetNSPath(ctx, node)
			if err != nil {
				return err
			}

			linkNode := links.NewShorthandLinkNode(node, nspathA)

			ep := &links.EndpointVeth{
				EndpointGeneric: *links.NewEndpointGeneric(linkNode, iface, link),
			}

			linkNode.AddEndpoint(ep)
			link.Endpoints = append(link.Endpoints, ep)
			_ = kind
		}

		link.Deploy(ctx)

		log.Info("veth interface successfully created!")
		return nil
	},
}

func parseVethEndpoint(s string) (node, iface, kind string, err error) {
	supportedKinds := []string{"ovs-bridge", "bridge", "host"}

	arr := strings.Split(s, ":")
	if (len(arr) != 2) && (len(arr) != 3) {
		return node, iface, kind, errors.New("malformed veth endpoint reference")
	}
	switch len(arr) {
	case 2:
		kind = "container"
		if arr[0] == "host" {
			kind = "host"
		}
		node = arr[0]
		iface = arr[1]
	case 3:
		if _, ok := utils.StringInSlice(supportedKinds, arr[0]); !ok {
			return node, iface, kind, fmt.Errorf("node type %s is not supported, supported nodes are %q", arr[0], supportedKinds)
		}
		kind = arr[0]
		node = arr[1]
		iface = arr[2]
	}
	return node, iface, kind, err
}
