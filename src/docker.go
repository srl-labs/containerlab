package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	sysctl "github.com/lorenzosaino/go-sysctl"
	log "github.com/sirupsen/logrus"
)

// // Docker struct
// type Docker struct {
// 	cli *docker.Client
// }

// // NewDocker initializes the docker client
// func NewDocker() (d *Docker, err error) {
// 	d = new(Docker)
// 	d.cli, err = docker.NewEnvClient()
// 	if err != nil {
// 		return nil, err
// 	}
// 	return d, nil
// }

func (c *cLab) createBridge(ctx context.Context) (err error) {

	ipamIPv4Config := network.IPAMConfig{
		Subnet:  c.Conf.DockerInfo.Ipv4Subnet,
		Gateway: c.Conf.DockerInfo.Ipv4Gateway,
	}
	ipamIPv6Config := network.IPAMConfig{
		Subnet:  c.Conf.DockerInfo.Ipv6Subnet,
		Gateway: c.Conf.DockerInfo.Ipv6Gateway,
	}
	var ipamConfig []network.IPAMConfig
	ipamConfig = append(ipamConfig, ipamIPv4Config)
	ipamConfig = append(ipamConfig, ipamIPv6Config)

	ipam := &network.IPAM{
		Driver: "default",
		Config: ipamConfig,
	}

	networkOptions := types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "bridge",
		Scope:          "local",
		EnableIPv6:     true,
		IPAM:           ipam,
		Internal:       false,
		Attachable:     false,
		Ingress:        false,
		ConfigOnly:     false,
	}

	var bridgeName string
	var netCreateResponse types.NetworkCreateResponse
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	netCreateResponse, err = c.DockerClient.NetworkCreate(nctx, c.Conf.DockerInfo.Bridge, networkOptions)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Debugf("Container network %s already exists", c.Conf.DockerInfo.Bridge)
			nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			netResource, err := c.DockerClient.NetworkInspect(nctx, c.Conf.DockerInfo.Bridge, types.NetworkInspectOptions{})
			if err != nil {
				return err
			}
			log.Debugf("container network: %+v", netResource)
			if len(netResource.ID) < 12 {
				return fmt.Errorf("could not get bridge ID")
			}
			bridgeName = "br-" + netResource.ID[:12]
		} else {
			return err
		}
	}
	if len(bridgeName) == 0 {
		if len(netCreateResponse.ID) < 12 {
			return fmt.Errorf("could not get bridge ID")
		}
		bridgeName = "br-" + netCreateResponse.ID[:12]
	}
	log.Debugf("container network %s : bridge name: %s", c.Conf.DockerInfo.Bridge, bridgeName)
	log.Debug("Disable RPF check on the docker host part1")
	if err = sysctl.Set("net.ipv4.conf.default.rp_filter", "0"); err != nil {
		return err
	}
	log.Debug("Disable RPF check on the docker host part2")
	if err = sysctl.Set("net.ipv4.conf.all.rp_filter", "0"); err != nil {
		return err
	}
	log.Debug("Enable LLDP on the docker bridge")
	file := "/sys/class/net/" + bridgeName + "/bridge/group_fwd_mask"
	var b []byte
	b, err = exec.Command("sudo", "echo", "16384", ">", file).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable LLDP on docker bridge: %v", err)
	}
	log.Debugf("%s", string(b))
	log.Debug("Disable Checksum Offloading on the docker bridge")
	b, err = exec.Command("sudo", "ethtool", "--offload", bridgeName, "rx", "off", "tx", "off").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disable Checksum Offloading on docker bridge: %v", err)
	}
	log.Debugf("%s", string(b))
	return nil
}

func (c *cLab) deleteBridge(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = c.DockerClient.NetworkRemove(nctx, c.Conf.DockerInfo.Bridge)
	if err != nil {
		return err
	}
	return nil
}

func (c *cLab) createContainer(ctx context.Context, name string, node *Node) (err error) {
	log.Debug("Create container: ", name)
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cont, err := c.DockerClient.ContainerCreate(nctx,
		&container.Config{
			Image:        node.Image,
			Cmd:          strings.Fields(node.Cmd),
			Env:          node.Env,
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     name,
			Volumes:      node.Volumes,
			Tty:          true,
			User:         node.User,
			Labels: map[string]string{
				"containerlab":         "lab-" + c.Conf.Prefix,
				"lab-" + c.Conf.Prefix: name,
				"kind":                 node.OS,
				"type":                 node.NodeType,
				"group":                node.Group,
			},
		}, &container.HostConfig{
			Binds:       node.Binds,
			Sysctls:     node.Sysctls,
			Privileged:  true,
			NetworkMode: container.NetworkMode(c.Conf.DockerInfo.Bridge),
		}, nil, "lab"+"-"+c.Conf.Prefix+"-"+name)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Container create response: %v", c))

	node.Cid = cont.ID

	err = c.startContainer(ctx, name, node)
	if err != nil {
		return err
	}

	return c.inspectContainer(ctx, name, node)
}

func (c *cLab) startContainer(ctx context.Context, name string, node *Node) (err error) {
	log.Debug("Start container: ", name)
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = c.DockerClient.ContainerStart(nctx,
		node.Cid,
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *cLab) deleteContainer(ctx context.Context, name string, node *Node) (err error) {
	log.Debug("Delete and remove container: ", name)

	containers, err := c.DockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	var cid string

	for _, container := range containers {
		for _, n := range container.Names {
			if strings.Contains(n, "lab"+"-"+c.Conf.Prefix+"-"+name) {
				cid = container.ID
				break
			}
		}
	}

	if cid != "" {
		err = c.DockerClient.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *cLab) inspectContainer(ctx context.Context, id string, node *Node) (err error) {
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s, err := c.DockerClient.ContainerInspect(nctx, id)
	if err != nil {
		return err
	}
	node.Pid = s.State.Pid
	node.MgmtIPv4 = s.NetworkSettings.Networks["srlinux_bridge"].IPAddress
	node.MgmtIPv6 = s.NetworkSettings.Networks["srlinux_bridge"].GlobalIPv6Address
	node.MgmtMac = s.NetworkSettings.Networks["srlinux_bridge"].MacAddress

	log.Debug("Container pid: ", node.Pid)
	log.Debug("Container mgmt IPv4: ", node.MgmtIPv4)
	log.Debug("Container mgmt IPv6: ", node.MgmtIPv6)
	log.Debug("Container mgmt MAC: ", node.MgmtMac)
	return nil
}
