// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const (
	defaultNodePrefix  = "node"
	defaultGroupPrefix = "tier"
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
	reg         *nodes.NodeRegistry
)

type nodesDef struct {
	numNodes uint
	kind     string
	typ      string
}

// generateCmd represents the generate command.
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "generate a Clos topology file, based on provided flags",
	RunE: func(_ *cobra.Command, _ []string) error {
		if common.Name == "" {
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

		b, err := generateTopologyConfig(common.Name, mgmtNetName, mgmtIPv4Subnet.String(),
			mgmtIPv6Subnet.String(), images, licenses, nodeDefs...)
		if err != nil {
			return err
		}
		log.Debugf("generated topo: %s", string(b))
		if file != "" {
			err = utils.CreateFile(file, string(b))
			if err != nil {
				return err
			}
		}
		if deploy {
			err = common.CheckAndGetRootPrivs(nil, nil)
			if err != nil {
				return err
			}
			reconfigure = true
			if file == "" {
				file = fmt.Sprintf("%s.clab.yml", common.Name)
				err = utils.CreateFile(file, string(b))
				if err != nil {
					return err
				}
			}
			common.Topo = file
			return deployCmd.RunE(deployCmd, nil)
		}
		if file == "" {
			fmt.Println(string(b))
		}
		return nil
	},
}

func init() {
	c := &clab.CLab{}
	c.Reg = nodes.NewNodeRegistry()
	c.RegisterNodes()

	reg = c.Reg

	generateNodesAttributes := reg.GetGenerateNodeAttributes()
	supportedKinds := []string{}

	// prepare list of generateable node kinds
	for k, v := range generateNodesAttributes {
		if v.IsGenerateable() {
			supportedKinds = append(supportedKinds, k)
		}
	}

	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	generateCmd.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	generateCmd.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	generateCmd.Flags().StringSliceVarP(&image, "image", "", []string{},
		"container image name, can be prefixed with the node kind. <kind>=<image_name>")
	generateCmd.Flags().StringVarP(&kind, "kind", "", "srl",
		fmt.Sprintf("container kind, one of %v", supportedKinds))
	generateCmd.Flags().StringSliceVarP(&nodesFlag, "nodes", "", []string{},
		"comma separated nodes definitions in format <num_nodes>:<kind>:<type>, each defining a Clos network stage")
	generateCmd.Flags().StringSliceVarP(&license, "license", "", []string{},
		"path to license file, can be prefix with the node kind. <kind>=/path/to/file")
	generateCmd.Flags().StringVarP(&nodePrefix, "node-prefix", "", defaultNodePrefix, "prefix used in node names")
	generateCmd.Flags().StringVarP(&groupPrefix, "group-prefix", "", defaultGroupPrefix, "prefix used in group names")
	generateCmd.Flags().StringVarP(&file, "file", "", "", "file path to save generated topology")
	generateCmd.Flags().BoolVarP(&deploy, "deploy", "", false,
		"deploy a fabric based on the generated topology file")
	generateCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers creating nodes and virtual wires")
}

func generateTopologyConfig(name, network, ipv4range, ipv6range string,
	images, licenses map[string]string, nodes ...nodesDef,
) ([]byte, error) {
	numStages := len(nodes)
	config := &clab.Config{
		Name: name,
		Mgmt: new(types.MgmtNet),
		Topology: &types.Topology{
			Kinds: make(map[string]*types.NodeDefinition),
			Nodes: make(map[string]*types.NodeDefinition),
		},
	}
	config.Mgmt.Network = network
	if ipv4range != "<nil>" {
		config.Mgmt.IPv4Subnet = ipv4range
	}
	if ipv6range != "<nil>" {
		config.Mgmt.IPv6Subnet = ipv6range
	}
	for k, img := range images {
		config.Topology.Kinds[k] = &types.NodeDefinition{Image: img}
	}
	for k, lic := range licenses {
		if knd, ok := config.Topology.Kinds[k]; ok {
			knd.License = lic
			config.Topology.Kinds[k] = knd
			continue
		}
		config.Topology.Kinds[k] = &types.NodeDefinition{License: lic}
	}
	if numStages == 1 {
		for j := uint(0); j < nodes[0].numNodes; j++ {
			node1 := fmt.Sprintf("%s1-%d", nodePrefix, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = &types.NodeDefinition{
					Group: fmt.Sprintf("%s-1", groupPrefix),
					Kind:  nodes[0].kind,
					Type:  nodes[0].typ,
				}
			}
		}
	}

	generateNodesAttributes := reg.GetGenerateNodeAttributes()

	for i := 0; i < numStages-1; i++ {
		interfaceOffset := uint(0)
		if i > 0 {
			interfaceOffset = nodes[i-1].numNodes
		}
		for j := uint(0); j < nodes[i].numNodes; j++ {
			node1 := fmt.Sprintf("%s%d-%d", nodePrefix, i+1, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = &types.NodeDefinition{
					Group: fmt.Sprintf("%s-%d", groupPrefix, i+1),
					Kind:  nodes[i].kind,
					Type:  nodes[i].typ,
				}
			}
			for k := uint(0); k < nodes[i+1].numNodes; k++ {
				node2 := fmt.Sprintf("%s%d-%d", nodePrefix, i+2, k+1)
				if _, ok := config.Topology.Nodes[node2]; !ok {
					config.Topology.Nodes[node2] = &types.NodeDefinition{
						Group: fmt.Sprintf("%s-%d", groupPrefix, i+2),
						Kind:  nodes[i+1].kind,
						Type:  nodes[i+1].typ,
					}
				}

				// create a raw veth link
				l := &links.LinkVEthRaw{
					Endpoints: []*links.EndpointRaw{
						links.NewEndpointRaw(node1, fmt.Sprintf(
							generateNodesAttributes[nodes[i].kind].GetInterfaceFormat(), k+1+interfaceOffset), ""),
						links.NewEndpointRaw(node2, fmt.Sprintf(
							generateNodesAttributes[nodes[i+1].kind].GetInterfaceFormat(), j+1), ""),
					},
				}

				// encapsulate the brief rawlink in a linkdefinition
				ld := &links.LinkDefinition{
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
		items := strings.SplitN(l, "=", 2)
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
		case 2:
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
		items := strings.SplitN(n, ":", 3)
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
		// num_nodes notation
		// kind is assumed to be `srl` or set with --kind
		case 1:
			if kind == "" {
				log.Errorf("no kind specified for nodes '%s'", n)
				return nil, errSyntax
			}
			def.kind = kind
		// num_nodes:kind notation
		case 2:
			if kind == "" {
				log.Errorf("no kind specified for nodes '%s'", n)
				return nil, errSyntax
			}
			def.kind = items[1]

		// num_nodes:kind:type notation
		case 3:
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
