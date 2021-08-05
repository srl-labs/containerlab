// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

var interfaceFormat = map[string]string{
	"srl":      "e1-%d",
	"ceos":     "eth%d",
	"crpd":     "eth%d",
	"sonic-vs": "eth%d",
	"linux":    "eth%d",
	"bridge":   "veth%d",
	"vr-sros":  "eth%d",
	"vr-vmx":   "eth%d",
	"vr-xrv9k": "eth%d",
	"vr-veos":  "eth%d",
}
var supportedKinds = []string{"srl", "ceos", "linux", "bridge", "sonic-vs", "crpd", "vr-sros", "vr-vmx", "vr-xrv9k"}

// JvB: Supported topology alternatives to generate, first=default
var supportedTopos = []string{"clos", "petersen"}

const (
	defaultSRLType     = "ixrd2"
	defaultNodePrefix  = "node"
	defaultGroupPrefix = "tier"
)

var errDuplicatedValue = errors.New("duplicated value definition")
var errSyntax = errors.New("syntax error")

var image []string
var kind string
var gen_topo string  // JvB added to support other topologies besides 'clos'
var nodesFlag []string
var license []string
var nodePrefix string
var groupPrefix string
var file string
var deploy bool
var petersenSkipFactor uint

type nodesDef struct {
	numNodes uint
	kind     string
	typ      string
}

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "generate a Clos (or other type) topology file, based on provided flags",
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
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

		b, err := generateTopologyConfig(name, mgmtNetName, mgmtIPv4Subnet.String(), mgmtIPv6Subnet.String(), images, licenses, nodeDefs...)
		if err != nil {
			return err
		}
		log.Debugf("generated topo: %s", string(b))
		if file != "" {
			err = saveTopoFile(file, b)
			if err != nil {
				return err
			}
		}
		if deploy {
			reconfigure = true
			if file == "" {
				file = fmt.Sprintf("%s.clab.yml", name)
				err = saveTopoFile(file, b)
				if err != nil {
					return err
				}
			}
			topo = file
			return deployCmd.RunE(deployCmd, nil)
		}
		if file == "" {
			fmt.Println(string(b))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	generateCmd.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	generateCmd.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	generateCmd.Flags().StringSliceVarP(&image, "image", "", []string{}, "container image name, can be prefixed with the node kind. <kind>=<image_name>")
	generateCmd.Flags().StringVarP(&kind, "kind", "", "srl", fmt.Sprintf("container kind, one of %v", supportedKinds))
	generateCmd.Flags().StringVarP(&gen_topo, "topology", "", supportedTopos[0], fmt.Sprintf("Topology to generate, one of %v", supportedTopos))
	generateCmd.Flags().StringSliceVarP(&nodesFlag, "nodes", "", []string{}, "comma separated nodes definitions in format <num_nodes>:<kind>:<type>, each defining a Clos network stage")
	generateCmd.Flags().StringSliceVarP(&license, "license", "", []string{}, "path to license file, can be prefix with the node kind. <kind>=/path/to/file")
	generateCmd.Flags().StringVarP(&nodePrefix, "node-prefix", "", defaultNodePrefix, "prefix used in node names")
	generateCmd.Flags().StringVarP(&groupPrefix, "group-prefix", "", defaultGroupPrefix, "prefix used in group names")
	generateCmd.Flags().StringVarP(&file, "file", "", "", "file path to save generated topology")
	generateCmd.Flags().BoolVarP(&deploy, "deploy", "", false, "deploy a fabric based on the generated topology file")
	generateCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0, "limit the maximum number of workers creating nodes and virtual wires")
	generateCmd.Flags().UintVarP(&petersenSkipFactor, "petersen-skip-factor", "", 1, "Skip step for inner circle of Petersen topology, 1 >= K <= N/2")
}

func generateTopologyConfig(name, network, ipv4range, ipv6range string, images map[string]string, licenses map[string]string, nodes ...nodesDef) ([]byte, error) {
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
	switch gen_topo {
	case "clos":
	  generateClosTopology(config,nodes)
	case "petersen":
		generatePetersonTopology(config,nodes)
	}
	return yaml.Marshal(config)
}

func generateClosTopology(config *clab.Config, nodes []nodesDef) {
	numStages := len(nodes)
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
				config.Topology.Links = append(config.Topology.Links, &types.LinkConfig{
					Endpoints: []string{
						node1 + ":" + fmt.Sprintf(interfaceFormat[nodes[i].kind], k+1+interfaceOffset),
						node2 + ":" + fmt.Sprintf(interfaceFormat[nodes[i+1].kind], j+1),
					},
				})
			}
		}
	}
}

/*
 * Jvb: This generates a (generalized) Petersen graph, see
 * https://en.wikipedia.org/wiki/Petersen_graph
 *
 * Inspired by https://metacpan.org/pod/Graph::Maker::Petersen
 */
func generatePetersonTopology(config *clab.Config, nodes []nodesDef) error {
	numStages := len(nodes)
	if numStages != 1 {
     return errors.New("Petersen topology requires a single stage")
	}
	numNodes := nodes[0].numNodes
	if (numNodes < 6) || (numNodes % 2)==1 {
		 return errors.New("Petersen topology requires an even number of nodes, minimal 6")
	}
	N := numNodes / 2 // Number of nodes in outer and inner circle

  // Skip factor for inner ring, 1 <= K <= N/2
  if (petersenSkipFactor < 1) || (petersenSkipFactor > N/2) {
		 return errors.New(fmt.Sprintf("petersenSkipFactor must be >= 1 and <= %d (= N/2)", N ))
	}
	var nodeNames = make([]string, numNodes, numNodes)

	// 1. Create nodes
	for j := uint(0); j < N; j++ {
		for r := uint(0); r < 2; r++ { // outer+inner circle
		  node1 := fmt.Sprintf("%s-%d", nodePrefix, j+1 + r*N )
			nodeNames[ j + r*N ] = node1
		  if _, ok := config.Topology.Nodes[node1]; !ok {
			  config.Topology.Nodes[node1] = &types.NodeDefinition{
				  Group: fmt.Sprintf("%s-%s", groupPrefix, map[uint]string{0:"O",1:"I"} [r]),
				  Kind:  nodes[0].kind,
				  Type:  nodes[0].typ,
			  }
		  }
    }
	}

	// 2. Add links
	addLink := func(n1 uint,n2 uint,p1 uint,p2 uint) {
		config.Topology.Links = append(config.Topology.Links, &types.LinkConfig{
			Endpoints: []string{
				nodeNames[n1] + ":" + fmt.Sprintf(interfaceFormat[nodes[0].kind], p1),
				nodeNames[n2] + ":" + fmt.Sprintf(interfaceFormat[nodes[0].kind], p2),
			},
		})
	}

	for j := uint(0); j < N; j++ {

		// Each node has 3 links: 2 to nodes in the same circle, 1 outer<->inner
		addLink(j,j+N,1,1) // port1 is connection between inner and outer

		for r := uint(0); r < 2; r++ { // outer+inner circle
			 b := r*N     // base ID, 0 for outer, N for inner
			 n := j + b   // node ID, 0..2N-1
			 skip := r*petersenSkipFactor // ==0 for outer circle

			 // To avoid duplicating links, only emit when n is smaller
			 if n < (n+1+skip)%N+b {
			    addLink(n,(n+1+skip)%N+b,2,3)   // port2 to next neighbor
			 }
			 if n < (n+N-1-skip)%N+b {
			    addLink(n,(n+N-1-skip)%N+b,3,2) // port3 to prev neighbor
			 }
		}
	}
	return nil
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
		case 1:
			if kind == "" {
				log.Errorf("no kind specified for nodes '%s'", n)
				return nil, errSyntax
			}
			def.kind = kind
			if kind == "srl" {
				def.typ = defaultSRLType
			}
		case 2:
			switch items[1] {
			case "ceos", "linux", "bridge", "sonic", "crpd":
				def.kind = items[1]
			case "srl":
				def.kind = items[1]
				def.typ = defaultSRLType
			default:
				// assume second item is a type if kind set using --kind
				if kind == "" {
					log.Errorf("no kind specified for nodes '%s'", n)
					return nil, errSyntax
				}
				def.kind = kind
				def.typ = items[1]
			}
		case 3:
			// srl with #nodes, kind and type
			def.numNodes = uint(i)
			def.kind = items[1]
			def.typ = items[2]
		}
		result[idx] = def
	}
	return result, nil
}

func saveTopoFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
