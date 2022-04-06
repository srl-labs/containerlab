// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

const defaultTopologyExportTemplate = "/etc/containerlab/templates/export/auto.tmpl"

// GenerateExports generate various export files and writes it to a lab location
func (c *CLab) GenerateExports() error {
	topoDataFPath := filepath.Join(c.Dir.Lab, "topology-data.json")
	f, err := os.Create(topoDataFPath)
	if err != nil {
		return err
	}

	p := defaultTopologyExportTemplate
	n := filepath.Base(p)
	err = c.exportTopologyDataWithTemplate(f, n, p)
	if err != nil {
		log.Warningf("Cannot parse export template %s", p)
		log.Warningf("Details: %s", err)
		err = c.exportTopologyDataWithDefaultTemplate(f)
		if err != nil {
			return err
		}
	}
	return err
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
	t, err := template.New(n).
		Funcs(gomplate.CreateFuncs(context.Background(), new(data.Data))).
		Funcs(template.FuncMap{
			"ToJSON": func(v interface{}) string {
				a, _ := json.Marshal(v)
				return string(a)
			},
			"ToJSONPretty": func(v interface{}, prefix string, indent string) string {
				a, _ := json.MarshalIndent(v, prefix, indent)
				return string(a)
			},
		}).
		ParseFiles(p)

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

// generates and writes topology data file to w using a default built-in template
func (c *CLab) exportTopologyDataWithDefaultTemplate(w io.Writer) error {
	tdef :=
		`{
  "name": "{{ .Name }}",
  "type": "{{ .Type }}"
}`

	t, err := template.New("default").Parse(tdef)
	if err != nil {
		return err
	}

	e := TopologyExport{
		Name: c.Config.Name,
		Type: "clab",
	}

	err = t.Execute(w, e)
	if err != nil {
		return err
	}
	return err
}
