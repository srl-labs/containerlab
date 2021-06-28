// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package srl

import (
	"context"
	"crypto/rand"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	srlDefaultType = "ixr6"
)

var (
	srlSysctl = map[string]string{
		"net.ipv4.ip_forward":              "0",
		"net.ipv6.conf.all.disable_ipv6":   "0",
		"net.ipv6.conf.all.accept_dad":     "0",
		"net.ipv6.conf.default.accept_dad": "0",
		"net.ipv6.conf.all.autoconf":       "0",
		"net.ipv6.conf.default.autoconf":   "0",
	}

	srlTypes = map[string]string{
		"ixr6":  "7250IXR6.yml",
		"ixr10": "7250IXR10.yml",
		"ixrd1": "7220IXRD1.yml",
		"ixrd2": "7220IXRD2.yml",
		"ixrd3": "7220IXRD3.yml",
	}

	srlEnv = map[string]string{"SRLINUX": "1"}

	//go:embed srl.cfg
	cfgTemplate string

	//go:embed topology/*
	topologies embed.FS
)

func init() {
	nodes.Register(nodes.NodeKindSRL, func() nodes.Node {
		return new(srl)
	})
}

type srl struct {
	cfg *types.NodeConfig
}

func (s *srl) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	if s.cfg.Config == "" {
		s.cfg.Config = nodes.DefaultConfigTemplates[s.cfg.Kind]
	}
	if s.cfg.License == "" {
		return fmt.Errorf("no license found for node '%s' of kind '%s'", s.cfg.ShortName, s.cfg.Kind)
	}
	if s.cfg.NodeType == "" {
		s.cfg.NodeType = srlDefaultType
	}
	if _, found := srlTypes[s.cfg.NodeType]; !found {
		keys := make([]string, 0, len(srlTypes))
		for key := range srlTypes {
			keys = append(keys, key)
		}
		log.Fatalf("wrong node type. '%s' doesn't exist. should be any of %s", s.cfg.NodeType, strings.Join(keys, ", "))
	}

	// the addition touch is needed to support non docker runtimes
	s.cfg.Cmd = "sudo bash -c 'touch /.dockerenv && /opt/srlinux/bin/sr_linux'"

	s.cfg.Env = utils.MergeStringMaps(srlEnv, s.cfg.Env)

	// if user was not initialized to a value, use root
	if s.cfg.User == "" {
		s.cfg.User = "0:0"
	}
	for k, v := range srlSysctl {
		s.cfg.Sysctls[k] = v
	}

	// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(filepath.Join(s.cfg.LabDir, "license.key"), ":/opt/srlinux/etc/license.key:ro"))

	// mount config directory
	cfgPath := filepath.Join(s.cfg.LabDir, "config")
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(cfgPath, ":/etc/opt/srlinux/:rw"))

	// mount srlinux.conf
	srlconfPath := filepath.Join(s.cfg.LabDir, "srlinux.conf")
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(srlconfPath, ":/home/admin/.srlinux.conf:rw"))

	// mount srlinux topology
	topoPath := filepath.Join(s.cfg.LabDir, "topology.yml")
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(topoPath, ":/tmp/topology.yml:ro"))

	return nil
}

func (s *srl) Config() *types.NodeConfig { return s.cfg }

func (s *srl) PreDeploy(configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	// retrieve node certificates
	nodeCerts, err := cert.RetrieveNodeCertData(s.cfg, labCADir)
	// if not available on disk, create cert in next step
	if err != nil {
		// create CERT
		certTpl, err := template.New("node-cert").Parse(cert.NodeCSRTempl)
		if err != nil {
			log.Errorf("failed to parse Node CSR Template: %v", err)
		}
		certInput := cert.CertInput{
			Name:     s.cfg.ShortName,
			LongName: s.cfg.LongName,
			Fqdn:     s.cfg.Fqdn,
			Prefix:   configName,
		}
		nodeCerts, err = cert.GenerateCert(
			path.Join(labCARoot, "root-ca.pem"),
			path.Join(labCARoot, "root-ca-key.pem"),
			certTpl,
			certInput,
			path.Join(labCADir, certInput.Name),
		)
		if err != nil {
			log.Errorf("failed to generate certificates for node %s: %v", s.cfg.ShortName, err)
		}
		log.Debugf("%s CSR: %s", s.cfg.ShortName, string(nodeCerts.Csr))
		log.Debugf("%s Cert: %s", s.cfg.ShortName, string(nodeCerts.Cert))
		log.Debugf("%s Key: %s", s.cfg.ShortName, string(nodeCerts.Key))
	}
	s.cfg.TLSCert = string(nodeCerts.Cert)
	s.cfg.TLSKey = string(nodeCerts.Key)

	return createSRLFiles(s.cfg)
}

func (s *srl) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	return r.CreateContainer(ctx, s.cfg)
}

func (s *srl) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	return nil
}
func (s *srl) Destroy(ctx context.Context, r runtime.ContainerRuntime) error {
	// return r.DeleteContainer(ctx, s.cfg)
	return nil
}

func (s *srl) WithMgmtNet(*types.MgmtNet) {}

func (s *srl) SaveConfig(ctx context.Context, r runtime.ContainerRuntime) error {
	return nil
}

//

func createSRLFiles(nodeCfg *types.NodeConfig) error {
	log.Debugf("Creating directory structure for SRL container: %s", nodeCfg.ShortName)
	var src string
	var dst string

	// copy license file to node specific directory in lab
	src = nodeCfg.License
	dst = path.Join(nodeCfg.LabDir, "license.key")
	if err := utils.CopyFile(src, dst); err != nil {
		return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, dst, err)
	}
	log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)

	// generate SRL topology file
	err := generateSRLTopologyFile(nodeCfg.NodeType, nodeCfg.LabDir, nodeCfg.Index)
	if err != nil {
		return err
	}

	// generate a config file
	// if the node has a `config:` statement, the file specified in that section
	// will be used as a template in nodeGenerateConfig()
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, "config"), 0777)
	dst = path.Join(nodeCfg.LabDir, "config", "config.json")
	err = nodeCfg.GenerateConfig(dst, cfgTemplate)
	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", nodeCfg.ShortName, err)
	}

	return err
}

type mac struct {
	MAC string
}

func generateSRLTopologyFile(nodeType, labDir string, index int) error {
	dst := path.Join(labDir, "topology.yml")

	tpl, err := template.ParseFS(topologies, "topology/"+srlTypes[nodeType])
	if err != nil {
		return errors.Wrap(err, "failed to get srl topology file")
	}

	// generate random bytes to use in the 2-3rd bytes of a base mac
	// this ensures that different srl nodes will have different macs for their ports
	buf := make([]byte, 2)
	_, err = rand.Read(buf)
	if err != nil {
		return err
	}
	m := fmt.Sprintf("02:%02x:%02x:00:00:00", buf[0], buf[1])

	mac := mac{
		MAC: m,
	}
	log.Debug(mac, dst)
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, mac)
}
