package clab

func initLinuxNode(c *CLab, nodeCfg NodeConfig, node *Node, user string, envs map[string]string) error {
	var err error

	node.Config, err = c.configInit(&nodeCfg, node.Kind)
	if err != nil {
		return err
	}
	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)
	node.Cmd = c.cmdInit(&nodeCfg, node.Kind)
	node.User = user

	node.Sysctls = make(map[string]string)
	node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"

	node.Env = envs

	return err
}
