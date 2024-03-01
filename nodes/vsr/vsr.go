// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vsr

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"vsr", "nokia_vsr"}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(Vsr)
	}, nil)
}

type Vsr struct {
	nodes.DefaultNode
}

func (n *Vsr) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// if user was not initialized to a value, use root
	if n.Cfg.User == "" {
		n.Cfg.User = "0:0"
	}

	if n.Cfg.License != "" {
		// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
		n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(
			filepath.Join(n.Cfg.LabDir, "license.key"), ":/nokia/license/license.txt:ro"))
	}

	// mount config directory
	cfgPath := filepath.Join(n.Cfg.LabDir, "config")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(cfgPath, ":/home/sros/flash3/:rw"))

	return nil
}

func (s *Vsr) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	return s.createVSRFiles()
}

func (s *Vsr) createVSRFiles() error {
	log.Debugf("Creating directory structure for VSR container: %s", s.Cfg.ShortName)
	var src string

	if s.Cfg.License != "" {
		// copy license file to node specific directory in lab
		src = s.Cfg.License
		licPath := filepath.Join(s.Cfg.LabDir, "license.key")
		if err := utils.CopyFile(src, licPath, 0644); err != nil {
			return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, licPath, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, licPath)
	}

	utils.CreateDirectory(path.Join(s.Cfg.LabDir, "config"), 0777)

	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (s *Vsr) CheckInterfaceName() error {
	ifRe := regexp.MustCompile(`net\d+`)
	for _, e := range s.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("nokia vsr linux interface name %q doesn't match the required pattern. VSR interfaces should be named as net<x>", e.GetIfaceName())
		}
	}

	return nil
}
