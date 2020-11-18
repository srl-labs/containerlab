/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
	"github.com/srl-wim/container-lab/clab"
	"gopkg.in/yaml.v2"
)

var interfaceFormat = map[string]string{
	"srl":    "e1-%d",
	"ceos":   "eth%d",
	"linux":  "eth$d",
	"bridge": "veth%d",
}
var supportedKinds = []string{"srl", "ceos", "linux", "bridge", "sonic", "crpd"}

const (
	defaultSRLType     = "ixr6"
	defaultNodePrefix  = "node"
	defaultGroupPrefix = "tier"
)

var errDuplicatedValue = errors.New("duplicated value definition")
var errSyntax = errors.New("syntax error")

var image []string
var kind string
var nodes []string
var license []string
var nodePrefix string
var groupPrefix string
var file string
var deploy bool

type nodesDef struct {
	numNodes uint
	kind     string
	typ      string
}

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "generate a Clos topology file, based on provided flags",
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

		nodeDefs, err := parseNodesFlag(kind, nodes...)
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
				file = fmt.Sprintf("%s.yaml", name)
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
	generateCmd.Flags().StringSliceVarP(&nodes, "nodes", "", []string{}, "comma separated nodes definitions in format <num_nodes>:<kind>:<type>, each defining a Clos network stage")
	generateCmd.Flags().StringSliceVarP(&license, "license", "", []string{}, "path to license file, can be prefix with the node kind. <kind>=/path/to/file")
	generateCmd.Flags().StringVarP(&nodePrefix, "node-prefix", "", defaultNodePrefix, "prefix used in node names")
	generateCmd.Flags().StringVarP(&groupPrefix, "group-prefix", "", defaultGroupPrefix, "prefix used in group names")
	generateCmd.Flags().StringVarP(&file, "file", "", "", "file path to save generated topology")
	generateCmd.Flags().BoolVarP(&deploy, "deploy", "", false, "deploy a fabric based on the generated topology file")
}

func generateTopologyConfig(name, network, ipv4range, ipv6range string, images map[string]string, licenses map[string]string, nodes ...nodesDef) ([]byte, error) {
	numStages := len(nodes)
	config := &clab.Config{
		Name: name,
		Topology: clab.Topology{
			Kinds: make(map[string]clab.NodeConfig),
			Nodes: make(map[string]clab.NodeConfig),
		},
	}
	config.Mgmt.Network = network
	if ipv4range != "<nil>" {
		config.Mgmt.Ipv4Subnet = ipv4range
	}
	if ipv6range != "<nil>" {
		config.Mgmt.Ipv6Subnet = ipv6range
	}
	for k, img := range images {
		config.Topology.Kinds[k] = clab.NodeConfig{Image: img}
	}
	for k, lic := range licenses {
		if knd, ok := config.Topology.Kinds[k]; ok {
			knd.License = lic
			config.Topology.Kinds[k] = knd
			continue
		}
		config.Topology.Kinds[k] = clab.NodeConfig{License: lic}
	}
	for i := 0; i < numStages-1; i++ {
		interfaceOffset := uint(0)
		if i > 0 {
			interfaceOffset = nodes[i-1].numNodes
		}
		for j := uint(0); j < nodes[i].numNodes; j++ {
			node1 := fmt.Sprintf("%s%d-%d", nodePrefix, i+1, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = clab.NodeConfig{
					Group: fmt.Sprintf("%s-%d", groupPrefix, i+1),
					Kind:  nodes[i].kind,
					Type:  nodes[i].typ,
				}
			}
			for k := uint(0); k < nodes[i+1].numNodes; k++ {
				node2 := fmt.Sprintf("%s%d-%d", nodePrefix, i+2, k+1)
				if _, ok := config.Topology.Nodes[node2]; !ok {
					config.Topology.Nodes[node2] = clab.NodeConfig{
						Group: fmt.Sprintf("%s-%d", groupPrefix, i+2),
						Kind:  nodes[i+1].kind,
						Type:  nodes[i+1].typ,
					}
				}
				config.Topology.Links = append(config.Topology.Links, clab.LinkConfig{
					Endpoints: []string{
						node1 + ":" + fmt.Sprintf(interfaceFormat[nodes[i].kind], k+1+interfaceOffset),
						node2 + ":" + fmt.Sprintf(interfaceFormat[nodes[i+1].kind], j+1),
					},
				})
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
