// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
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
	MTU  = links.DefaultLinkMTU
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
			var node, iface string
			var kind links.LinkEndpointType
			if node, iface, kind, err = parseVethEndpoint(definition); err != nil {
				return err
			}
			var nspath string

			var leType links.LinkEndpointType
			switch kind {
			case links.LinkEndpointTypeHost:
				leType = links.LinkEndpointTypeHost
				nsp, err := ns.GetCurrentNS()
				if err != nil {
					return err
				}
				nspath = nsp.Path()
			case links.LinkEndpointTypeBridge:
				leType = links.LinkEndpointTypeBridge
				nsp, err := ns.GetCurrentNS()
				if err != nil {
					return err
				}
				nspath = nsp.Path()
			default:
				leType = links.LinkEndpointTypeVeth
				nspath, err = c.GlobalRuntime().GetNSPath(ctx, node)
				if err != nil {
					return err
				}
			}

			linkNode := links.NewShorthandLinkNode(node, nspath, leType)

			r := links.NewEndpointRaw(node, iface, "")
			ep, err := r.Resolve(&links.ResolveParams{Nodes: map[string]links.Node{node: linkNode}}, link)
			if err != nil {
				return err
			}
			link.Endpoints = append(link.Endpoints, ep)
			linkNode.AddLink(link)
		}

		err = link.Deploy(ctx)
		if err != nil {
			return err
		}

		log.Info("veth interface successfully created!")
		return nil
	},
}

func parseVethEndpoint(s string) (node, iface string, kind links.LinkEndpointType, err error) {
	arr := strings.Split(s, ":")
	if (len(arr) != 2) && (len(arr) != 3) {
		return node, iface, kind, errors.New("malformed veth endpoint reference")
	}
	switch len(arr) {
	case 2:
		kind = links.LinkEndpointTypeVeth
		if arr[0] == "host" {
			kind = links.LinkEndpointTypeHost
		}
		node = arr[0]
		iface = arr[1]
	case 3:
		if _, ok := utils.StringInSlice([]string{"ovs-bridge", "bridge"}, arr[0]); !ok {
			return node, iface, kind, fmt.Errorf("node type %s is not supported, supported nodes are %q", arr[0], supportedKinds)
		}

		switch arr[0] {
		case "bridge", "ovs-bridge":
			kind = links.LinkEndpointTypeBridge
		default:
			kind = links.LinkEndpointTypeVeth
		}

		node = arr[1]
		iface = arr[2]
	}
	return node, iface, kind, err
}
