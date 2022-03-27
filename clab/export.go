// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/srl-labs/containerlab/types"
)

type topoGraph struct {
	Nodes map[string]*types.NodeConfig `json:"nodes,omitempty"`
	//	Links []link `json:"links,omitempty"`
}

type link struct {
	Source         string `json:"source,omitempty"`
	SourceEndpoint string `json:"source_endpoint,omitempty"`
	Target         string `json:"target,omitempty"`
	TargetEndpoint string `json:"target_endpoint,omitempty"`
}

// GenerateExports generate various export files and writes it to a lab location
func (c *CLab) GenerateExports() error {
	topologyGraphFPath := filepath.Join(c.Dir.Lab, "topology-graph.json")
	f, err := os.Create(topologyGraphFPath)
	if err != nil {
		return err
	}
	return c.generateTopologyGraph(f)
}

// generateTopologyGraph generates and writes topology graph file to w
func (c *CLab) generateTopologyGraph(w io.Writer) error {
	g := topoGraph{
		Nodes: make(map[string]*types.NodeConfig),
		//Links: make([]c.Link, 0, len(c.Links)),
	}

	for _, n := range c.Nodes {
		cfg := n.Config()
		// Empty NodeConfig.Endpoints slice to avoid cyclic references incompatible with json.Marshal()
		cfg.Endpoints = make([]types.Endpoint, 0, 0)
		g.Nodes[n.Config().ShortName] = cfg
	}

	b, err := json.Marshal(g)
	if err != nil {
		return err
	}

	w.Write(b)

	return nil
}
