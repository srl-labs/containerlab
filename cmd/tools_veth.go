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
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
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

	// create fake nodes to make links resolve work
	err = createNodes(ctx, c, parsedAEnd, parsedBEnd, rtName)
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

	// deploy the endpoints of the Link
	for _, ep := range link.GetEndpoints() {
		ep.Deploy(ctx)
	}

	log.Info("veth interface successfully created!")

	return nil
}

// createNodes creates fake nodes in c.Nodes map to make link resolve work.
// It checks which endpoint type is set by a user and creates a node that matches the type.
func createNodes(_ context.Context, c *clabcore.CLab, aEnd, bEnd parsedEndpoint, rt string) error {
	for _, epDefinition := range []parsedEndpoint{aEnd, bEnd} {
		switch epDefinition.Kind {
		case clablinks.LinkEndpointTypeHost:
			err := createFakeNode(c, "host", &clabtypes.NodeConfig{
				ShortName: epDefinition.Node,
				LongName:  epDefinition.Node,
				Runtime:   rt,
			})
			if err != nil {
				return err
			}

		case clablinks.LinkEndpointTypeBridge,
			clablinks.LinkEndpointTypeBridgeNS:
			err := createFakeNode(c, "bridge", &clabtypes.NodeConfig{
				ShortName: epDefinition.Node,
				LongName:  epDefinition.Node,
				Runtime:   rt,
			})
			if err != nil {
				return err
			}
		default:
			// default endpoint type is veth
			// so we create a fake linux node for it and fetch
			// its namespace path.
			// techinically we don't care which node this is, as long as it uses
			// standard veth interface attachment process.
			err := createFakeNode(c, "linux", &clabtypes.NodeConfig{
				ShortName: epDefinition.Node,
				LongName:  epDefinition.Node,
				Runtime:   rt,
			})
			if err != nil {
				return err
			}
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

// createFakeNode creates a fake node in c.Nodes map using the provided node kind and its config.
func createFakeNode(c *clabcore.CLab, kind string, nodeCfg *clabtypes.NodeConfig) error {
	name := nodeCfg.ShortName
	// construct node
	n, err := c.Reg.NewNodeOfKind(kind)
	if err != nil {
		return fmt.Errorf("error constructing node %s: %v", name, err)
	}

	// Init
	err = n.Init(nodeCfg, clabnodes.WithRuntime(c.Runtimes[nodeCfg.Runtime]))
	if err != nil {
		return fmt.Errorf("failed to initialize node %s: %v", name, err)
	}

	// fake node is always assumed to be deployed in case of tools veth command
	n.SetState(clabnodesstate.Deployed)

	c.Nodes[name] = n

	return nil
}
