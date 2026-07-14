// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	linkEndpointTypeVethPartCount   = 2
	linkEndpointTypeBridgePartCount = 3
)

func vethCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "veth",
		Short: "veth operations",
	}

	vethCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a veth interface and attach its sides to the specified containers",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return vethCreate(o)
		},
	}

	c.AddCommand(vethCreateCmd)
	vethCreateCmd.Flags().StringVarP(
		&o.ToolsVeth.AEndpoint,
		"a-endpoint",
		"a",
		o.ToolsVeth.AEndpoint,
		"veth endpoint A in the format of <containerA-name>:<interface-name> "+
			"or <endpointA-type>:<endpoint-name>:<interface-name>",
	)
	vethCreateCmd.Flags().StringVarP(
		&o.ToolsVeth.BEndpoint,
		"b-endpoint",
		"b",
		o.ToolsVeth.BEndpoint,
		"veth endpoint B in the format of <containerB-name>:<interface-name> "+
			"or <endpointB-type>:<endpoint-name>:<interface-name>",
	)
	vethCreateCmd.Flags().IntVarP(
		&o.ToolsVeth.MTU,
		"mtu",
		"m", o.ToolsVeth.MTU,
		"link MTU",
	)

	return c, nil
}

func vethCreate(o *Options) error {
	var err error

	parsedAEnd, err := parseVethEndpoint(o.ToolsVeth.AEndpoint)
	if err != nil {
		return err
	}

	parsedBEnd, err := parseVethEndpoint(o.ToolsVeth.BEndpoint)
	if err != nil {
		return err
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rtName, _, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return err
	}

	// create placeholder nodes to make links resolve work
	err = createPlaceholderNodes(c, parsedAEnd, parsedBEnd, rtName)
	if err != nil {
		return err
	}

	// now create link brief as if the link was passed via topology file
	linkBrief := &clablinks.LinkBriefRaw{
		Endpoints: []string{
			fmt.Sprintf("%s:%s", parsedAEnd.Node, parsedAEnd.Iface),
			fmt.Sprintf("%s:%s", parsedBEnd.Node, parsedBEnd.Iface),
		},
		LinkCommonParams: clablinks.LinkCommonParams{
			MTU: o.ToolsVeth.MTU,
		},
	}

	linkRaw, err := linkBrief.ToTypeSpecificRawLink()
	if err != nil {
		return err
	}

	// we need to copy nodes.Nodes to links.Nodes since two interfaces
	// are not identical, but a subset
	resolveNodes := make(map[string]clablinks.Node, len(c.Nodes))
	for k, v := range c.Nodes {
		resolveNodes[k] = v
	}

	link, err := linkRaw.Resolve(&clablinks.ResolveParams{Nodes: resolveNodes})
	if err != nil {
		return err
	}

	nodeNames := []string{parsedAEnd.Node}
	if parsedBEnd.Node != parsedAEnd.Node {
		nodeNames = append(nodeNames, parsedBEnd.Node)
	}
	if err := c.DeployNodes(ctx, nodeNames, uint(len(nodeNames))); err != nil {
		return err
	}
	if err := c.DeployLinks(ctx, []clablinks.Link{link}); err != nil {
		return err
	}

	log.Info("veth interface successfully created!")

	return nil
}

// createPlaceholderNodes creates non-owning nodes for resources that already exist.
func createPlaceholderNodes(c *clabcore.CLab, aEnd, bEnd parsedEndpoint, rt string) error {
	for _, epDefinition := range []parsedEndpoint{aEnd, bEnd} {
		if _, exists := c.Nodes[epDefinition.Node]; exists {
			continue
		}
		if err := c.AddPlaceholderNode(&clabtypes.NodeConfig{
			ShortName: epDefinition.Node,
			LongName:  epDefinition.Node,
			Kind:      placeholderNodeKind(epDefinition.Kind),
			Runtime:   rt,
		}); err != nil {
			return err
		}
	}

	return nil
}

// parsedEndpoint is a parsed veth endpoint definition.
type parsedEndpoint struct {
	Node  string
	Iface string
	Kind  clablinks.LinkEndpointType
}

// parseVethEndpoint parses the veth endpoint definition as passed in the veth create command.
func parseVethEndpoint(s string) (parsedEndpoint, error) {
	s = strings.TrimSpace(s)

	ep := parsedEndpoint{}

	arr := strings.Split(s, ":")

	var kind clablinks.LinkEndpointType

	switch len(arr) {
	case linkEndpointTypeVethPartCount:
		ep.Kind = clablinks.LinkEndpointTypeVeth

		if arr[0] == "host" {
			ep.Kind = clablinks.LinkEndpointTypeHost
		}

		ep.Node = arr[0]
		ep.Iface = arr[1]

	case linkEndpointTypeBridgePartCount:
		if _, ok := clabutils.StringInSlice([]string{"ovs-bridge", "bridge"}, arr[0]); !ok {
			return ep, fmt.Errorf(
				"only bride and ovs-bridge can be used as a first block in the link definition. "+
					"Got: %s",
				arr[0],
			)
		}

		switch arr[0] {
		case "bridge", "ovs-bridge":
			kind = clablinks.LinkEndpointTypeBridge
		case "bridge-ns":
			kind = clablinks.LinkEndpointTypeBridgeNS
		default:
			kind = clablinks.LinkEndpointTypeVeth
		}

		ep.Kind = kind

		ep.Node = arr[1]
		ep.Iface = arr[2]

	default:
		return ep, errors.New("malformed veth endpoint reference")
	}

	return ep, nil
}

// placeholderNodeKind maps a veth endpoint type to the node kind used as a
// non-owning placeholder for an already-existing resource.
func placeholderNodeKind(t clablinks.LinkEndpointType) string {
	switch t {
	case clablinks.LinkEndpointTypeHost:
		return "host"
	case clablinks.LinkEndpointTypeBridge, clablinks.LinkEndpointTypeBridgeNS:
		return "bridge"
	default:
		return "ext-container"
	}
}
