package clab

import (
	"fmt"
	"path"
)

func initSROSNode(c *CLab, nodeCfg NodeConfig, node *Node, user string, envs map[string]string) error {
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
