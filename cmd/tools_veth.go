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
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
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

		parsedAEnd, err := parseVethEndpoint(AEnd)
		if err != nil {
			return err
		}

		parsedBEnd, err := parseVethEndpoint(BEnd)
		if err != nil {
			return err
		}

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

		rtName, _, err := clab.RuntimeInitializer(rt)
		if err != nil {
			return err
		}

		// create fake nodes to make links resolve work
		err = createNodes(ctx, c, parsedAEnd, parsedBEnd, rtName)
		if err != nil {
			return err
		}

		// now create link brief as if the link was passed via topology file
		linkBrief := &links.LinkBriefRaw{
			Endpoints: []string{
				fmt.Sprintf("%s:%s", parsedAEnd.Node, parsedAEnd.Iface),
				fmt.Sprintf("%s:%s", parsedBEnd.Node, parsedBEnd.Iface),
			},
			LinkCommonParams: links.LinkCommonParams{
				MTU: MTU,
			},
		}

		linkRaw, err := linkBrief.ToTypeSpecificRawLink()
		if err != nil {
			return err
		}

		// we need to copy nodes.Nodes to links.Nodes since two interfaces
		// are not identical, but a subset
		resolveNodes := make(map[string]links.Node, len(c.Nodes))
		for k, v := range c.Nodes {
			resolveNodes[k] = v
		}

		link, err := linkRaw.Resolve(&links.ResolveParams{Nodes: resolveNodes})
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
func createNodes(_ context.Context, c *clab.CLab, AEnd, BEnd parsedEndpoint, rt string) error {
	for _, epDefinition := range []parsedEndpoint{AEnd, BEnd} {
		switch epDefinition.Kind {
		case links.LinkEndpointTypeHost:
			err := createFakeNode(c, "host", &types.NodeConfig{
				ShortName: epDefinition.Node,
				LongName:  epDefinition.Node,
				Runtime:   rt,
			})
			if err != nil {
				return err
			}

		case links.LinkEndpointTypeBridge:
			err := createFakeNode(c, "bridge", &types.NodeConfig{
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
			err := createFakeNode(c, "linux", &types.NodeConfig{
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
	Kind  links.LinkEndpointType
}

// parseVethEndpoint parses the veth endpoint definition as passed in the veth create command.
func parseVethEndpoint(s string) (parsedEndpoint, error) {
	s = strings.TrimSpace(s)

	ep := parsedEndpoint{}

	arr := strings.Split(s, ":")

	var kind links.LinkEndpointType

	switch len(arr) {
	case 2:
		ep.Kind = links.LinkEndpointTypeVeth

		if arr[0] == "host" {
			ep.Kind = links.LinkEndpointTypeHost
		}

		ep.Node = arr[0]
		ep.Iface = arr[1]

	case 3:
		if _, ok := utils.StringInSlice([]string{"ovs-bridge", "bridge"}, arr[0]); !ok {
			return ep, fmt.Errorf("node type %s is not supported, supported nodes are %q", arr[0], supportedKinds)
		}

		switch arr[0] {
		case "bridge", "ovs-bridge":
			kind = links.LinkEndpointTypeBridge
		default:
			kind = links.LinkEndpointTypeVeth
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
func createFakeNode(c *clab.CLab, kind string, nodeCfg *types.NodeConfig) error {
	name := nodeCfg.ShortName
	// construct node
	n, err := kind_registry.KindRegistryInstance.NewNodeOfKind(kind)
	if err != nil {
		return fmt.Errorf("error constructing node %s: %v", name, err)
	}

	// Init
	err = n.Init(nodeCfg, nodes.WithRuntime(c.Runtimes[nodeCfg.Runtime]))
	if err != nil {
		return fmt.Errorf("failed to initialize node %s: %v", name, err)
	}

	// fake node is always assumed to be deployed in case of tools veth command
	n.SetState(state.Deployed)

	c.Nodes[name] = n

	return nil
}
