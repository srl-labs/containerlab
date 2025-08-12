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
	containerlabcore "github.com/srl-labs/containerlab/core"
	containerlablinks "github.com/srl-labs/containerlab/links"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	containerlabruntime "github.com/srl-labs/containerlab/runtime"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var (
	AEnd = ""
	BEnd = ""
	MTU  = containerlablinks.DefaultLinkMTU
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
	Use:     "create",
	Short:   "Create a veth interface and attach its sides to the specified containers",
	PreRunE: containerlabutils.CheckAndGetRootPrivs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		parsedAEnd, err := parseVethEndpoint(AEnd)
		if err != nil {
			return err
		}

		parsedBEnd, err := parseVethEndpoint(BEnd)
		if err != nil {
			return err
		}

		opts := []containerlabcore.ClabOption{
			containerlabcore.WithTimeout(timeout),
			containerlabcore.WithRuntime(
				runtime,
				&containerlabruntime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: gracefulShutdown,
				},
			),
			containerlabcore.WithDebug(debug),
		}
		c, err := containerlabcore.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		rtName, _, err := containerlabcore.RuntimeInitializer(runtime)
		if err != nil {
			return err
		}

		// create fake nodes to make links resolve work
		err = createNodes(ctx, c, parsedAEnd, parsedBEnd, rtName)
		if err != nil {
			return err
		}

		// now create link brief as if the link was passed via topology file
		linkBrief := &containerlablinks.LinkBriefRaw{
			Endpoints: []string{
				fmt.Sprintf("%s:%s", parsedAEnd.Node, parsedAEnd.Iface),
				fmt.Sprintf("%s:%s", parsedBEnd.Node, parsedBEnd.Iface),
			},
			LinkCommonParams: containerlablinks.LinkCommonParams{
				MTU: MTU,
			},
		}

		linkRaw, err := linkBrief.ToTypeSpecificRawLink()
		if err != nil {
			return err
		}

		// we need to copy nodes.Nodes to links.Nodes since two interfaces
		// are not identical, but a subset
		resolveNodes := make(map[string]containerlablinks.Node, len(c.Nodes))
		for k, v := range c.Nodes {
			resolveNodes[k] = v
		}

		link, err := linkRaw.Resolve(&containerlablinks.ResolveParams{Nodes: resolveNodes})
		if err != nil {
			return err
		}

		// deploy the endpoints of the Link
		for _, ep := range link.GetEndpoints() {
			ep.Deploy(ctx)
		}

		log.Info("veth interface successfully created!")
		return nil
	},
}

// createNodes creates fake nodes in c.Nodes map to make link resolve work.
// It checks which endpoint type is set by a user and creates a node that matches the type.
func createNodes(_ context.Context, c *containerlabcore.CLab, aEnd, bEnd parsedEndpoint, rt string) error {
	for _, epDefinition := range []parsedEndpoint{aEnd, bEnd} {
		switch epDefinition.Kind {
		case containerlablinks.LinkEndpointTypeHost:
			err := createFakeNode(c, "host", &containerlabtypes.NodeConfig{
				ShortName: epDefinition.Node,
				LongName:  epDefinition.Node,
				Runtime:   rt,
			})
			if err != nil {
				return err
			}

		case containerlablinks.LinkEndpointTypeBridge,
			containerlablinks.LinkEndpointTypeBridgeNS:
			err := createFakeNode(c, "bridge", &containerlabtypes.NodeConfig{
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
			err := createFakeNode(c, "linux", &containerlabtypes.NodeConfig{
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
	Kind  containerlablinks.LinkEndpointType
}

// parseVethEndpoint parses the veth endpoint definition as passed in the veth create command.
func parseVethEndpoint(s string) (parsedEndpoint, error) {
	s = strings.TrimSpace(s)

	ep := parsedEndpoint{}

	arr := strings.Split(s, ":")

	var kind containerlablinks.LinkEndpointType

	switch len(arr) {
	case 2:
		ep.Kind = containerlablinks.LinkEndpointTypeVeth

		if arr[0] == "host" {
			ep.Kind = containerlablinks.LinkEndpointTypeHost
		}

		ep.Node = arr[0]
		ep.Iface = arr[1]

	case 3:
		if _, ok := containerlabutils.StringInSlice([]string{"ovs-bridge", "bridge"}, arr[0]); !ok {
			return ep, fmt.Errorf("only bride and ovs-bridge can be used as a first block in the link definition. Got: %s", arr[0])
		}

		switch arr[0] {
		case "bridge", "ovs-bridge":
			kind = containerlablinks.LinkEndpointTypeBridge
		case "bridge-ns":
			kind = containerlablinks.LinkEndpointTypeBridgeNS
		default:
			kind = containerlablinks.LinkEndpointTypeVeth
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
func createFakeNode(c *containerlabcore.CLab, kind string, nodeCfg *containerlabtypes.NodeConfig) error {
	name := nodeCfg.ShortName
	// construct node
	n, err := c.Reg.NewNodeOfKind(kind)
	if err != nil {
		return fmt.Errorf("error constructing node %s: %v", name, err)
	}

	// Init
	err = n.Init(nodeCfg, containerlabnodes.WithRuntime(c.Runtimes[nodeCfg.Runtime]))
	if err != nil {
		return fmt.Errorf("failed to initialize node %s: %v", name, err)
	}

	// fake node is always assumed to be deployed in case of tools veth command
	n.SetState(containerlabnodesstate.Deployed)

	c.Nodes[name] = n

	return nil
}
