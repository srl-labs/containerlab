// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"path/filepath"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

// GenerateExports generates various export files and writes it to a file in the lab directory.
func (c *CLab) GenerateExports(ctx context.Context, f io.Writer, p string) error {
	err := c.exportTopologyDataWithTemplate(ctx, f, p)
	if err != nil {
		log.Warningf("Cannot parse export template %s: %v", p, err)
		// a minimal topology data file that just provides the name of a lab that failed to generate a proper export data
		err = c.exportTopologyDataWithMinimalTemplate(f)
		if err != nil {
			return err
		}
	}
	return err
}

// TopologyExport holds a combination of CLab structure and map of NodeConfig types,
// which expands Node definitions with dynamically created values.
type TopologyExport struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Clab *CLab  `json:"clab,omitempty"`
	// SSHPubKeys is a list of string representations of SSH public keys.
	SSHPubKeys  []string                     `json:"SSHPubKeys,omitempty"`
	NodeConfigs map[string]*types.NodeConfig `json:"nodeconfigs,omitempty"`
}

//go:embed export_templates/auto.tmpl
var defaultExportTemplate string

//go:embed export_templates/full.tmpl
var fullExportTemplate string

// exportTopologyDataWithTemplate generates and writes topology data file to w using a template.
func (c *CLab) exportTopologyDataWithTemplate(_ context.Context, w io.Writer, p string) error {
	name := "export"
	if p != "" {
		name = filepath.Base(p)
	}

	t := template.New(name).
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
		})

	var err error

	switch {
	case p != "":
		_, err = t.ParseFiles(p)
	case p == "__full":
		_, err = t.Parse(fullExportTemplate)
	default:
		_, err = t.Parse(defaultExportTemplate)
	}

	if err != nil {
		return err
	}

	e := TopologyExport{
		Name:        c.Config.Name,
		Type:        "clab",
		Clab:        c,
		SSHPubKeys:  utils.MarshalSSHPubKeys(c.SSHPubKeys),
		NodeConfigs: make(map[string]*types.NodeConfig),
	}

	for _, n := range c.Nodes {
		e.NodeConfigs[n.Config().ShortName] = n.Config()
	}

	err = t.Execute(w, e)
	if err != nil {
		return err
	}
	log.Debugf("Exported topology data using %s template", p)

	return err
}

// generates and writes topology data file to w using a default built-in template.
func (c *CLab) exportTopologyDataWithMinimalTemplate(w io.Writer) error {
	tdef := `{
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
	log.Debug("Exported topology data using built-in template")
	return err
}
