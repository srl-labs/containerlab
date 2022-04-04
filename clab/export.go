// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"text/template"

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
	Peer      string `json:"peer,omitempty"`
}

// GenerateExports generate various export files and writes it to a lab location
func (c *CLab) GenerateExports() error {
	topoDataFPath := filepath.Join(c.Dir.Lab, "topology-data.json")
	f, err := os.Create(topoDataFPath)
	if err != nil {
		return err
	}
	//return c.exportTopologyData(f)
	return c.exportTopologyDataWithTemplate(f, "auto.tmpl", "/etc/containerlab/templates/export/auto.tmpl")
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
			Peer:      "z",
		}
		intmap["z"] = NodeInterface{
			Node:      l.B.Node.ShortName,
			Interface: l.B.EndpointName,
			MAC:       l.B.MAC,
			Peer:      "a",
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

// This struct will hold a combination of CLab structure, which is mostly derived from topology.yaml,
// and map of NodeConfig types, which expands Node definitions with dynamically created values
type TopologyExport struct {
	Name        string                       `json:"name"`
	Type        string                       `json:"type"`
	Clab        *CLab                        `json:"clab,omitempty"`
	NodeConfigs map[string]*types.NodeConfig `json:"nodeconfigs,omitempty"`
}

// generates and writes topology data file to w using a template
func (c *CLab) exportTopologyDataWithTemplate(w io.Writer, n string, p string) error {
	t, err := template.New(n).Funcs(template.FuncMap{
		"marshal": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
		"marshal_indent": func(v interface{}, prefix string, indent string) string {
			a, _ := json.MarshalIndent(v, prefix, indent)
			return string(a)
		},
	}).ParseFiles(p)

	if err != nil {
		return err
	}

	e := TopologyExport{
		Name:        c.Config.Name,
		Type:        "clab",
		Clab:        c,
		NodeConfigs: make(map[string]*types.NodeConfig),
	}

	for _, n := range c.Nodes {
		e.NodeConfigs[n.Config().ShortName] = n.Config()
	}

	err = t.Execute(w, e)
	if err != nil {
		return err
	}
	return err
}
