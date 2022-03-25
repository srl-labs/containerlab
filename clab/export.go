// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/srl-labs/containerlab/types"
)

// GenerateExports generate various export files and writes it to a lab location
func (c *CLab) GenerateExports() error {

	topologyGraphFPath := filepath.Join(c.Dir.Lab, "topology-graph.json")
	_, err = os.Create(topologyGraphFPath)
	if err != nil {
		return err
	}

	return c.generateTopologyGraph(f)
}

// generateTopologyGraph generates and writes topology graph file to w
func (c *CLab) generateTopologyGraph(w io.Writer) error {

	invT :=
		`{
      "nodes": [
        {{- range $nodes}}
      ],
      "links": [
      ]
}
`
	type inv struct {
		// clab nodes aggregated by their kind
		Nodes map[string][]*types.NodeConfig
		// clab nodes aggregated by user-defined groups
		Groups map[string][]*types.NodeConfig
	}

	i := inv{
		Nodes:  make(map[string][]*types.NodeConfig),
		Groups: make(map[string][]*types.NodeConfig),
	}

	for _, n := range c.Nodes {
		i.Nodes[n.Config().Kind] = append(i.Nodes[n.Config().Kind], n.Config())
		if n.Config().Labels["ansible-group"] != "" {
			i.Groups[n.Config().Labels["ansible-group"]] = append(i.Groups[n.Config().Labels["ansible-group"]], n.Config())
		}
	}

	// sort nodes by name as they are not sorted originally
	for _, nodes := range i.Nodes {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	// sort nodes-per-group by name as they are not sorted originally
	for _, nodes := range i.Groups {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	t, err := template.New("graph").Parse(invT)
	if err != nil {
		return err
	}
	err = t.Execute(w, i)
	if err != nil {
		return err
	}
	return err

}
