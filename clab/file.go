// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// TopoFile type is a struct which defines parameters of the topology file
type TopoFile struct {
	path     string // topo file path
	fullName string // file name with extension
	name     string // file name without extension
}

// GetTopology parses the topology file into c.Conf structure
// as well as populates the TopoFile structure with the topology file related information
func (c *CLab) GetTopology(topo string) error {
	yamlFile, err := ioutil.ReadFile(topo)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Topology file contents:\n%s\n", yamlFile))

	yamlFile = []byte(os.ExpandEnv(string(yamlFile)))
	err = yaml.UnmarshalStrict(yamlFile, c.Config)
	if err != nil {
		return err
	}

	c.Config.Topology.ImportEnvs()

	topoAbsPath, err := filepath.Abs(topo)
	if err != nil {
		return err
	}

	file := path.Base(topo)
	c.TopoFile = &TopoFile{
		path:     topoAbsPath,
		fullName: file,
		name:     strings.TrimSuffix(file, path.Ext(file)),
	}
	return nil
}
