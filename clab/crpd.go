package clab

import (
	"fmt"
	"path"
)

func initCrpdNode(c *CLab, nodeCfg NodeConfig, node *Node, user string, envs map[string]string) error {
	var err error

	node.Config, err = c.configInit(&nodeCfg, node.Kind)
	if err != nil {
		return err
	}
	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)
	node.User = user

	// initialize license file
	lp, err := c.licenseInit(&nodeCfg, node)
	if err != nil {
		return err
	}
	node.License = lp

	// mount config and log dirs
	node.Binds = append(node.Binds, fmt.Sprint(path.Join(node.LabDir, "config"), ":/config"))
	node.Binds = append(node.Binds, fmt.Sprint(path.Join(node.LabDir, "log"), ":/var/log"))
	// mount sshd_config
	node.Binds = append(node.Binds, fmt.Sprint(path.Join(node.LabDir, "config/sshd_config"), ":/etc/ssh/sshd_config"))

	return err
}
