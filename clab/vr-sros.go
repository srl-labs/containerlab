package clab

import (
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func initSROSNode(c *CLab, nodeCfg NodeConfig, node *types.NodeBase, user string, envs map[string]string) error {
	var err error
	node.Config, err = c.configInit(&nodeCfg, node.Kind)
	if err != nil {
		return err
	}
	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)
	node.User = user

	// vr-sros type sets the vrnetlab/sros variant (https://github.com/hellt/vrnetlab/sros)
	node.NodeType = c.typeInit(&nodeCfg, node.Kind)

	// initialize license file
	lp, err := c.licenseInit(&nodeCfg, node)
	if err != nil {
		return err
	}
	node.License = lp

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    vrDefConnMode,
		"DOCKER_NET_V4_ADDR": c.Config.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": c.Config.Mgmt.IPv6Subnet,
	}
	node.Env = mergeStringMaps(defEnv, envs)

	// mount tftpboot dir
	node.Binds = append(node.Binds, fmt.Sprint(path.Join(node.LabDir, "tftpboot"), ":/tftpboot"))
	if node.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		node.Binds = append(node.Binds, "/dev:/dev")
	}

	node.Cmd = fmt.Sprintf("--trace --connection-mode %s --hostname %s --variant \"%s\"", node.Env["CONNECTION_MODE"],
		node.ShortName,
		node.NodeType,
	)
	return err
}

func (c *CLab) createVrSROSFiles(node *types.NodeBase) error {
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(node.LabDir, "tftpboot"), 0777)

	if node.License != "" {
		// copy license file to node specific lab directory
		src := node.License
		dst := path.Join(node.LabDir, "/tftpboot/license.txt")
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)

		cfg := path.Join(node.LabDir, "tftpboot", "config.txt")
		if node.Config != "" {
			err := node.GenerateConfig(cfg, defaultConfigTemplates[node.Kind])
			if err != nil {
				log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
			}
		} else {
			log.Debugf("Config file exists for node %s", node.ShortName)
		}
	}
	return nil
}
