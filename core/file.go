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
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
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

	c.TopoPaths, err = clabtypes.NewTopoPaths(topo, varsFile)
	if err != nil {
		return err
	}

	// load the topology file/template
	topologyTemplate, err := template.New(c.TopoPaths.TopologyFilenameBase()).
		Funcs(clabutils.CreateFuncs()).
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
		return fmt.Errorf(
			"%w\nConsult with release notes to see if any fields were changed/removed", err,
		)
	}

	c.Config.Topology.ImportEnvs()

	return nil
}

func readTemplateVariables(topo, varsFile string) (any, error) {
	var templateVars any

	if varsFile == "" {
		ext := filepath.Ext(topo)

		for _, vext := range []string{".yaml", ".yml", ".json"} {
			maybeVarsFile := fmt.Sprintf("%s%s%s", topo[0:len(topo)-len(ext)], varFileSuffix, vext)

			_, err := os.Stat(maybeVarsFile)
			switch {
			case os.IsNotExist(err):
				continue
			case err != nil:
				return nil, err
			}

			varsFile = maybeVarsFile

			break
		}

		if varsFile == "" {
			// no var file found, assume the topology is not a template
			// or a template that doesn't require external variables
			return nil, nil
		}
	}

	data, err := os.ReadFile(varsFile)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &templateVars)
	if err != nil {
		return nil, err
	}

	return templateVars, nil
}
