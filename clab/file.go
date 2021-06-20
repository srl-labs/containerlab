// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
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

	err = yaml.UnmarshalStrict(yamlFile, c.Config)
	if err != nil {
		return err
	}

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

// CreateNodeDirStructure create the directory structure and files for the lab nodes
func (c *CLab) CreateNodeDirStructure(node *types.NodeConfig) (err error) {
	c.m.RLock()
	defer c.m.RUnlock()

	// create node directory in the lab directory
	// skip creation of node directory for linux/bridge kinds
	// since they don't keep any state normally
	if node.Kind != "linux" && node.Kind != "bridge" {
		utils.CreateDirectory(node.LabDir, 0777)
	}

	switch node.Kind {
	case "srl":
		if err := c.createSRLFiles(node); err != nil {
			return err
		}
	case "ceos":
		if err := c.createCEOSFiles(node); err != nil {
			return err
		}
	case "crpd":
		if err := c.createCRPDFiles(node); err != nil {
			return err
		}
	case "vr-sros":
		if err := c.createVrSROSFiles(node); err != nil {
			return err
		}
	}
	return nil
}
