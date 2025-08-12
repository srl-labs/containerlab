// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hellt/envsubst"

	"github.com/charmbracelet/log"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const (
	varFileSuffix = "_vars"
)

// LoadTopologyFromFile loads a topology by the topo file path
// parses the topology file into c.Conf structure
// as well as populates the TopoFile structure with the topology file related information.
func (c *CLab) LoadTopologyFromFile(topo, varsFile string) error {
	var err error

	c.TopoPaths, err = containerlabtypes.NewTopoPaths(topo, varsFile)
	if err != nil {
		return err
	}

	// load the topology file/template
	topologyTemplate, err := template.New(c.TopoPaths.TopologyFilenameBase()).Funcs(containerlabutils.CreateFuncs()).
		ParseFiles(c.TopoPaths.TopologyFilenameAbsPath())
	if err != nil {
		return err
	}

	// read template variables
	templateVars, err := readTemplateVariables(c.TopoPaths.TopologyFilenameAbsPath(), varsFile)
	if err != nil {
		return err
	}

	log.Debugf("template variables: %v", templateVars)
	// execute template
	buf := new(bytes.Buffer)

	err = topologyTemplate.Execute(buf, templateVars)
	if err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	log.Debugf("topology:\n%s\n", buf.String())

	// expand env vars if any
	// do not replace vars initialized with defaults
	// and do not replace vars that are not set
	yamlFile, err := envsubst.BytesRestrictedNoReplace(buf.Bytes(), false, false, true, true)
	if err != nil {
		return err
	}
	err = yaml.UnmarshalStrict(yamlFile, c.Config)
	if err != nil {
		return fmt.Errorf("%w\nConsult with release notes to see if any fields were changed/removed", err)
	}

	c.Config.Topology.ImportEnvs()

	return nil
}

func readTemplateVariables(topo, varsFile string) (interface{}, error) {
	var templateVars interface{}
	// variable file is not explicitly set
	if varsFile == "" {
		ext := filepath.Ext(topo)
		for _, vext := range []string{".yaml", ".yml", ".json"} {
			varsFile = fmt.Sprintf("%s%s%s", topo[0:len(topo)-len(ext)], varFileSuffix, vext)
			_, err := os.Stat(varsFile)
			switch {
			case os.IsNotExist(err):
				continue
			case err != nil:
				return nil, err
			}
			// file with current extension found, go read it.
			goto READFILE
		}
		// no var file found, assume the topology is not a template
		// or a template that doesn't require external variables
		return nil, nil
	}
READFILE:
	data, err := os.ReadFile(varsFile)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, &templateVars)
	return templateVars, err
}
