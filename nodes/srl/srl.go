package srl

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/labstack/gommon/log"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	nodeKind = "srl"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(srl)
	})
}

type srl struct {
	cfg *types.NodeConfig
}

func (s *srl) Init(cfg *types.NodeConfig) error {
	s.cfg = cfg
	return nil
}
func (s *srl) GenerateConfig() error { return nil }

func (s *srl) Config() *types.NodeConfig { return s.cfg }

func (s *srl) PreDeploy() error {
	log.Debugf("Creating directory structure for SRL container: %s", s.cfg.ShortName)
	var src string
	var dst string

	// copy license file to node specific directory in lab
	src = s.cfg.License
	dst = path.Join(s.cfg.LabDir, "license.key")
	if err := utils.CopyFile(src, dst); err != nil {
		return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, dst, err)
	}
	log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)

	// generate SRL topology file
	err := generateSRLTopologyFile(s.cfg.Topology, s.cfg.LabDir, s.cfg.Index)
	if err != nil {
		return err
	}

	// generate a config file if the destination does not exist
	// if the node has a `config:` statement, the file specified in that section
	// will be used as a template in nodeGenerateConfig()
	utils.CreateDirectory(path.Join(s.cfg.LabDir, "config"), 0777)
	dst = path.Join(s.cfg.LabDir, "config", "config.json")
	err = s.cfg.GenerateConfig(dst, nodes.DefaultConfigTemplates[s.cfg.Kind])
	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", s.cfg.ShortName, err)
	}

	// copy env config to node specific directory in lab
	src = "/etc/containerlab/templates/srl/srl_env.conf"
	dst = s.cfg.LabDir + "/" + "srlinux.conf"
	err = utils.CopyFile(src, dst)
	if err != nil {
		return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, dst, err)
	}
	log.Debugf("CopyFile src %s -> dst %s succeeded\n", src, dst)

	return nil
}

func (s *srl) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	// certs ?
	return r.CreateContainer(ctx, s.cfg)
}
func (s *srl) PostDeploy() error { return nil }
func (s *srl) Destroy() error    { return nil }

//////////

type mac struct {
	MAC string
}

func generateSRLTopologyFile(src, labDir string, index int) error {
	dst := path.Join(labDir, "topology.yml")
	tpl, err := template.ParseFiles(src)
	if err != nil {
		return err
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

	if err = tpl.Execute(f, mac); err != nil {
		return err
	}
	return nil
}
