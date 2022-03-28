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

type topoData struct {
	Name string `json:"name"`
	Type string `json:"type"`
	//	Config topoConfig                   `json:"configuration,omitempty"`
	Nodes map[string]*types.NodeConfig `json:"nodes,omitempty"`
	Links []Link                       `json:"links,omitempty"`
}

type topoConfig struct {
	Name string `json:"name,omitempty"`
}

// GenerateExports generate various export files and writes it to a lab location
func (c *CLab) GenerateExports() error {
	topoDataFPath := filepath.Join(c.Dir.Lab, "topology-data.json")
	f, err := os.Create(topoDataFPath)
	if err != nil {
		return err
	}
	return c.exportTopologyData(f)
}

// generates and writes topology data file to w
func (c *CLab) exportTopologyData(w io.Writer) error {
	d := topoData{
		Name:  c.Config.Name,
		Type:  "clab",
		Nodes: make(map[string]*types.NodeConfig),
		Links: make([]Link, 0, len(c.Links)),
	}

	for _, n := range c.Nodes {
		cfg := n.Config()
		// Empty NodeConfig.Endpoints slice to avoid cyclic references incompatible with json.Marshal()
		cfg.Endpoints = make([]types.Endpoint, 0, 0)
		d.Nodes[n.Config().ShortName] = cfg
	}

	for _, l := range c.Links {
		d.Links = append(d.Links, Link{
			Source:         l.A.Node.ShortName,
			SourceEndpoint: l.A.EndpointName,
			Target:         l.B.Node.ShortName,
			TargetEndpoint: l.B.EndpointName,
		})
	}

	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	w.Write(b)

	return nil
}
