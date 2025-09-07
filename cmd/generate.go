// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const (
	defaultNodePrefix            = "node"
	defaultGroupPrefix           = "tier"
	nodeFlagNumPartCount         = 1
	nodeFlagNumKindPartCount     = 2
	nodeFlagNumKindTypePartCount = 3
)

var (
	errDuplicatedValue = errors.New("duplicated value definition")
	errSyntax          = errors.New("syntax error")
)

var (
	image       []string
	kind        string
	nodesFlag   []string
	license     []string
	nodePrefix  string
	groupPrefix string
	file        string
	deploy      bool
)

type nodesDef struct {
	numNodes uint
	kind     string
	typ      string
}

func generateCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	clab := &clabcore.CLab{}
	clab.Reg = clabnodes.NewNodeRegistry()
	clab.RegisterNodes()

	generateNodesAttributes := clab.Reg.GetGenerateNodeAttributes()

	var supportedKinds []string

	// prepare list of generateable node kinds
	for k, v := range generateNodesAttributes {
		if v.IsGenerateable() {
			supportedKinds = append(supportedKinds, k)
		}
	}

	c := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Short:   "generate a Clos topology file, based on provided flags",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return generate(cobraCmd, o, clab.Reg)
		},
	}

	c.Flags().StringVarP(
		&o.Deploy.ManagementNetworkName,
		"network",
		"",
		o.Deploy.ManagementNetworkName,
		"management network name",
	)
	c.Flags().IPNetVarP(
		&o.Deploy.ManagementIPv4Subnet,
		"ipv4-subnet",
		"4",
		o.Deploy.ManagementIPv4Subnet,
		"management network IPv4 subnet range",
	)
	c.Flags().IPNetVarP(
		&o.Deploy.ManagementIPv6Subnet,
		"ipv6-subnet",
		"6",
		o.Deploy.ManagementIPv6Subnet,
		"management network IPv6 subnet range",
	)
	c.Flags().StringSliceVarP(
		&image,
		"image",
		"",
		[]string{},
		"container image name, can be prefixed with the node kind. <kind>=<image_name>",
	)
	c.Flags().StringVarP(
		&kind,
		"kind",
		"",
		"srl",
		fmt.Sprintf("container kind, one of %v", supportedKinds),
	)
	c.Flags().StringSliceVarP(
		&nodesFlag,
		"nodes",
		"",
		[]string{},
		"comma separated nodes definitions in format <num_nodes>:<kind>:<type>, "+
			"each defining a Clos network stage",
	)
	c.Flags().StringSliceVarP(
		&license,
		"license",
		"",
		[]string{},
		"path to license file, can be prefix with the node kind. <kind>=/path/to/file",
	)
	c.Flags().StringVarP(
		&nodePrefix,
		"node-prefix",
		"",
		defaultNodePrefix,
		"prefix used in node names",
	)
	c.Flags().StringVarP(
		&groupPrefix,
		"group-prefix",
		"",
		defaultGroupPrefix,
		"prefix used in group names",
	)
	c.Flags().StringVarP(
		&file,
		"file",
		"",
		"",
		"file path to save generated topology",
	)
	c.Flags().BoolVarP(
		&deploy,
		"deploy",
		"",
		false,
		"deploy a fabric based on the generated topology file",
	)
	c.Flags().UintVarP(
		&o.Deploy.MaxWorkers,
		"max-workers",
		"",
		o.Deploy.MaxWorkers,
		"limit the maximum number of workers creating nodes and virtual wires",
	)
	// Add the owner flag to generate command
	c.Flags().StringVarP(
		&o.Deploy.LabOwner,
		"owner",
		"",
		o.Deploy.LabOwner,
		"lab owner name (only for users in clab_admins group)",
	)

	return c, nil
}

func generate(cobraCmd *cobra.Command, o *Options, reg *clabnodes.NodeRegistry) error {
	if o.Global.TopologyName == "" {
		return errors.New("provide a lab name with --name flag")
	}

	licenses, err := parseFlag(kind, license)
	if err != nil {
		return err
	}

	log.Debugf("parsed licenses: %+v", licenses)

	images, err := parseFlag(kind, image)
	if err != nil {
		return err
	}

	log.Debugf("parsed images: %+v", images)

	nodeDefs, err := parseNodesFlag(kind, nodesFlag...)
	if err != nil {
		return err
	}

	log.Debugf("parsed nodes definitions: %+v", nodeDefs)

	b, err := generateTopologyConfig(
		o.Global.TopologyName,
		o.Deploy.ManagementNetworkName,
		o.Deploy.ManagementIPv4Subnet.String(),
		o.Deploy.ManagementIPv6Subnet.String(),
		images,
		licenses,
		reg,
		nodeDefs...)
	if err != nil {
		return err
	}

	log.Debugf("generated topo: %s", string(b))

	if file != "" {
		err = clabutils.CreateFile(file, string(b))
		if err != nil {
			return err
		}
	}

	if deploy {
		err = clabutils.CheckAndGetRootPrivs()
		if err != nil {
			return err
		}

		o.Deploy.Reconfigure = true
		if file == "" {
			file = fmt.Sprintf("%s.clab.yml", o.Global.TopologyName)

			err = clabutils.CreateFile(file, string(b))
			if err != nil {
				return err
			}
		}

		o.Global.TopologyFile = file

		// Pass owner to deploy command if specified
		if o.Deploy.LabOwner != "" {
			// This will be picked up by the deploy command
			os.Setenv("CLAB_OWNER", o.Deploy.LabOwner)
		}

		return deployFn(cobraCmd, o)
	}

	if file == "" {
		fmt.Println(string(b))
	}

	return nil
}

func generateTopologyConfig( //nolint: funlen
	name,
	network,
	ipv4range,
	ipv6range string,
	images,
	licenses map[string]string,
	reg *clabnodes.NodeRegistry,
	nodes ...nodesDef,
) ([]byte, error) {
	numStages := len(nodes)
	config := &clabcore.Config{
		Name: name,
		Mgmt: new(clabtypes.MgmtNet),
		Topology: &clabtypes.Topology{
			Kinds: make(map[string]*clabtypes.NodeDefinition),
			Nodes: make(map[string]*clabtypes.NodeDefinition),
		},
	}

	config.Mgmt.Network = network

	if ipv4range != clabconstants.UnsetNetAddr {
		config.Mgmt.IPv4Subnet = ipv4range
	}

	if ipv6range != clabconstants.UnsetNetAddr {
		config.Mgmt.IPv6Subnet = ipv6range
	}

	for k, img := range images {
		config.Topology.Kinds[k] = &clabtypes.NodeDefinition{Image: img}
	}

	for k, lic := range licenses {
		if knd, ok := config.Topology.Kinds[k]; ok {
			knd.License = lic
			config.Topology.Kinds[k] = knd

			continue
		}

		config.Topology.Kinds[k] = &clabtypes.NodeDefinition{License: lic}
	}

	if numStages == 1 {
		for j := range nodes[0].numNodes {
			node1 := fmt.Sprintf("%s1-%d", nodePrefix, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = &clabtypes.NodeDefinition{
					Group: fmt.Sprintf("%s-1", groupPrefix),
					Kind:  nodes[0].kind,
					Type:  nodes[0].typ,
				}
			}
		}
	}

	generateNodesAttributes := reg.GetGenerateNodeAttributes()

	for i := range numStages - 1 {
		interfaceOffset := uint(0)
		if i > 0 {
			interfaceOffset = nodes[i-1].numNodes
		}

		for j := range nodes[i].numNodes {
			node1 := fmt.Sprintf("%s%d-%d", nodePrefix, i+1, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = &clabtypes.NodeDefinition{
					Group: fmt.Sprintf("%s-%d", groupPrefix, i+1),
					Kind:  nodes[i].kind,
					Type:  nodes[i].typ,
				}
			}

			for k := range nodes[i+1].numNodes {
				node2 := fmt.Sprintf("%s%d-%d", nodePrefix, i+2, k+1) //nolint: mnd
				if _, ok := config.Topology.Nodes[node2]; !ok {
					config.Topology.Nodes[node2] = &clabtypes.NodeDefinition{
						Group: fmt.Sprintf("%s-%d", groupPrefix, i+2), //nolint: mnd
						Kind:  nodes[i+1].kind,
						Type:  nodes[i+1].typ,
					}
				}

				// create a raw veth link
				l := &clablinks.LinkVEthRaw{
					Endpoints: []*clablinks.EndpointRaw{
						clablinks.NewEndpointRaw(node1, fmt.Sprintf(
							generateNodesAttributes[nodes[i].kind].GetInterfaceFormat(), k+1+interfaceOffset), ""),
						clablinks.NewEndpointRaw(node2, fmt.Sprintf(
							generateNodesAttributes[nodes[i+1].kind].GetInterfaceFormat(), j+1), ""),
					},
				}

				// encapsulate the brief rawlink in a linkdefinition
				ld := &clablinks.LinkDefinition{
					Link: l.ToLinkBriefRaw(),
				}

				// add the link to the topology
				config.Topology.Links = append(config.Topology.Links, ld)
			}
		}
	}

	return yaml.Marshal(config)
}

func parseFlag(kind string, ls []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, l := range ls {
		items := strings.SplitN(l, "=", 2) //nolint: mnd

		switch len(items) {
		case 0:
			log.Errorf("missing value for flag item '%s'", l)

			return nil, errSyntax
		case 1:
			if kind == "" {
				log.Errorf("no kind specified for flag item '%s'", l)

				return nil, errSyntax
			}

			if _, ok := result[kind]; !ok {
				result[kind] = items[0]
			} else {
				log.Errorf("duplicated flag item for kind '%s'", kind)

				return nil, errDuplicatedValue
			}
		case 2: //nolint: mnd
			if _, ok := result[items[0]]; !ok {
				result[items[0]] = items[1]
			} else {
				log.Errorf("duplicated flag item for kind '%s'", items[0])

				return nil, errDuplicatedValue
			}
		}
	}

	return result, nil
}

func parseNodesFlag(kind string, nodes ...string) ([]nodesDef, error) {
	numStages := len(nodes)
	if numStages == 0 {
		log.Error("no nodes specified using --nodes")

		return nil, errSyntax
	}

	result := make([]nodesDef, numStages)
	for idx, n := range nodes {
		def := nodesDef{}

		items := strings.SplitN(n, ":", nodeFlagNumKindTypePartCount)
		if len(items) == 0 {
			log.Errorf("wrong --nodes format '%s'", n)

			return nil, errSyntax
		}

		i, err := strconv.Atoi(items[0])
		if err != nil {
			log.Errorf("failed converting '%s' to an integer: %v", items[0], err)

			return nil, errSyntax
		}

		def.numNodes = uint(i)

		switch len(items) {
		// kind is assumed to be `srl` or set with --kind
		case nodeFlagNumPartCount:
			if kind == "" {
				log.Errorf("no kind specified for nodes '%s'", n)
				return nil, errSyntax
			}

			def.kind = kind
		case nodeFlagNumKindPartCount:
			if kind == "" {
				log.Errorf("no kind specified for nodes '%s'", n)
				return nil, errSyntax
			}

			def.kind = items[1]
		case nodeFlagNumKindTypePartCount:
			def.numNodes = uint(i)
			def.kind = kind

			if items[1] != "" {
				def.kind = items[1]
			}

			def.typ = items[2]
		}

		result[idx] = def
	}

	return result, nil
}
