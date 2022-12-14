package border0

import (
	"context"
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/border0_api"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var Kindnames = []string{"border0"}

func Register() {
	nodes.Register(Kindnames, func() nodes.Node {
		return new(border0)
	})
}

type border0 struct {
	nodes.DefaultNode
	topologyName             string
	hostborder0yamlPath      string
	containerborder0yamlPath string
}

func (b *border0) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	b.Cfg = cfg
	for _, o := range opts {
		o(b)
	}

	b.hostborder0yamlPath = path.Join(b.Cfg.LabDir, "border0.yaml")
	b.containerborder0yamlPath = path.Join("/code", "border0.yaml")
	return nil
}

func (b *border0) PreDeploy(_ context.Context, topologyName, labCADir, labCARoot string) error {
	utils.CreateDirectory(b.Cfg.LabDir, 0777)
	b.topologyName = topologyName
	border0Mount := fmt.Sprintf("%s:%s:ro", b.hostborder0yamlPath, b.containerborder0yamlPath)
	b.Cfg.Binds = append(b.Cfg.Binds, border0Mount)
	// make sure the host file exists prior to starting the container
	err := utils.CreateFile(b.hostborder0yamlPath, "")
	if err != nil {
		return err
	}

	return nil
}

func (b *border0) PostDeploy(ctx context.Context, nodesMap map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for border0.com node %q", b.Cfg.ShortName)
	err := types.DisableTxOffload(b.Cfg)
	if err != nil {
		return fmt.Errorf("failed to disable tx checksum offload for border0 kind: %v", err)
	}

	log.Infof("Creating border0.com tunnels...")

	// create a configuration for border0.com
	config, err := border0_api.CreateBorder0Config(ctx, nodesMap, b.topologyName)
	if err != nil {
		return err
	}
	// write the config to the mounted file
	utils.CreateFile(b.hostborder0yamlPath, config)

	// bring up the tunnels
	b0Cmd := exec.NewExecCmdFromSlice([]string{"./border0", "connector", "start", "--config", b.containerborder0yamlPath})
	err = b.Runtime.ExecNotWait(ctx, b.Cfg.LongName, b0Cmd)
	if err != nil {
		return err
	}

	return nil
}

func (b *border0) Delete(ctx context.Context) error {
	// deleting container
	return b.DefaultNode.Delete(ctx)
	// TODO: maybe deleting sockets as well; we would need infos about the nodes
}
