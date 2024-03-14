// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/a8m/envsubst"
	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const (
	varFileSuffix = "_vars"
)

// loadTopology parses the topology file into c.Conf structure
// as well as populates the TopoFile structure with the topology file related information.
func (c *CLab) loadTopologyFromFile(topo, varsFile string) error {
	var err error

	c.TopoPaths, err = types.NewTopoPaths(topo)
	if err != nil {
		return err
	}

	// load the topology file/template
	topologyTemplate, err := template.New(c.TopoPaths.TopologyFilenameBase()).
		Funcs(gomplate.CreateFuncs(context.Background(), new(data.Data))).
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

	// create a hidden file that will contain the rendered topology
	if !strings.HasPrefix(c.TopoPaths.TopologyFilenameBase(), ".") {
		backupFPath := c.TopoPaths.TopologyBakFileAbsPath()
		err = utils.CreateFile(backupFPath, buf.String())
		if err != nil {
			log.Warnf("Could not write rendered topology: %v", err)
		}
	}
	log.Debugf("topology:\n%s\n", buf.String())

	// expand env vars if any
	yamlFile, err := envsubst.Bytes(buf.Bytes())
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
			// file with current extention found, go read it.
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
