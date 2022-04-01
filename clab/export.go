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

type TopologyData struct {
	Name       string                       `json:"name"`
	Type       string                       `json:"type"`
	ClabConfig ClabConfig                   `json:"clabconfig,omitempty"`
	Nodes      map[string]*types.NodeConfig `json:"nodes,omitempty"`
	Links      []map[string]NodeInterface   `json:"links,omitempty"`
}

type ClabConfig struct {
	Prefix     *string        `json:"prefix,omitempty"`
	Mgmt       *types.MgmtNet `json:"mgmt,omitempty"`
	ConfigPath string         `json:"config-path,omitempty"`
}

type NodeInterface struct {
	Node      string `json:"node,omitempty"`
	Interface string `json:"interface,omitempty"`
	MAC       string `json:"mac,omitempty"`
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
	cc := ClabConfig{
		c.Config.Prefix,
		c.Config.Mgmt,
		c.Config.ConfigPath,
	}

	d := TopologyData{
		Name:       c.Config.Name,
		Type:       "clab",
		ClabConfig: cc,
		Nodes:      make(map[string]*types.NodeConfig),
		Links:      make([]map[string]NodeInterface, 0, len(c.Links)),
	}

	for _, n := range c.Nodes {
		d.Nodes[n.Config().ShortName] = n.Config()
	}

	for _, l := range c.Links {
		intmap := make(map[string]NodeInterface)
		intmap["a"] = NodeInterface{
			Node:      l.A.Node.ShortName,
			Interface: l.A.EndpointName,
			MAC:       l.A.MAC,
		}
		intmap["z"] = NodeInterface{
			Node:      l.B.Node.ShortName,
			Interface: l.B.EndpointName,
			MAC:       l.B.MAC,
		}
		d.Links = append(d.Links, intmap)
	}

	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	w.Write(b)

	return nil
}
