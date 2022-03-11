// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const (
	varFileSuffix = "_vars"
)

// TopoFile type is a struct which defines parameters of the topology file
type TopoFile struct {
	path     string // topo file path
	dir      string // topo file dir path
	fullName string // file name with extension
	name     string // file name without extension
}

// GetTopology parses the topology file into c.Conf structure
// as well as populates the TopoFile structure with the topology file related information
func (c *CLab) GetTopology(topo, varsFile string) error {
	fileBase := filepath.Base(topo)
	// load the topology file/template
	topologyTemplate, err := template.New(fileBase).
		Funcs(gomplate.CreateFuncs(context.Background(), new(data.Data))).
		ParseFiles(topo)
	if err != nil {
		return err
	}
	// read template variables
	templateVars, err := readTemplateVariables(topo, varsFile)
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
	if !strings.HasPrefix(fileBase, ".") {
		// create a hidden file that will contain the rendered topology
		err = utils.CreateFile(fmt.Sprintf(".%s.yaml", fileBase[:len(fileBase)-len(filepath.Ext(topo))]), buf.String())
		if err != nil {
			return err
		}
	}
	log.Debugf("topology:\n%s\n", buf.String())

	// expand env vars if any
	yamlFile := []byte(os.ExpandEnv(buf.String()))
	err = yaml.UnmarshalStrict(yamlFile, c.Config)
	if err != nil {
		return err
	}

	c.Config.Topology.ImportEnvs()

	topoAbsPath, err := filepath.Abs(topo)
	if err != nil {
		return err
	}

	topoDir := filepath.Dir(topoAbsPath)

	c.TopoFile = &TopoFile{
		path:     topoAbsPath,
		dir:      topoDir,
		fullName: fileBase,
		name:     strings.TrimSuffix(fileBase, path.Ext(fileBase)),
	}
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
	data, err := ioutil.ReadFile(varsFile)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, &templateVars)
	return templateVars, err
}
