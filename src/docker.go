package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	sysctl "github.com/lorenzosaino/go-sysctl"
	log "github.com/sirupsen/logrus"
)

// Docker struct
type Docker struct {
	cli *docker.Client
}

// NewDocker initializes the docker client
func NewDocker() (d *Docker, err error) {
	d = new(Docker)
	d.cli, err = docker.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Docker) createBridge() (err error) {

	ipamIPv4Config := network.IPAMConfig{
		Subnet:  DockerInfo.Ipv4Subnet,
		Gateway: DockerInfo.Ipv4Gateway,
	}
	ipamIPv6Config := network.IPAMConfig{
		Subnet:  DockerInfo.Ipv6Subnet,
		Gateway: DockerInfo.Ipv6Gateway,
	}
	var ipamConfig []network.IPAMConfig
	ipamConfig = append(ipamConfig, ipamIPv4Config)
	ipamConfig = append(ipamConfig, ipamIPv6Config)

	var ipam *network.IPAM
	ipam = &network.IPAM{
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
	_, err = d.cli.NetworkCreate(context.Background(), DockerInfo.Bridge, networkOptions)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Debug(fmt.Sprintf("Container network %s already exists", DockerInfo.Bridge))
		} else {
			return err
		}
	}
	log.Debug("Disable RPF check on the docker host part1")
	if err = sysctl.Set("net.ipv4.conf.default.rp_filter", "0"); err != nil {
		return err
	}
	log.Debug("Disable RPF check on the docker host part2")
	if err = sysctl.Set("net.ipv4.conf.all.rp_filter", "0"); err != nil {
		return err
	}
	log.Debug("Enable LLDP on the docker bridge")
	var cmd *exec.Cmd
	file := "/sys/class/net/" + DockerInfo.Bridge + "/bridge/group_fwd_mask"
	cmd = exec.Command("sudo", "echo", "16384", ">", file)
	_, err = cmd.CombinedOutput()

	log.Debug("Disable Checksum Offloading on the docker bridge")
	cmd = exec.Command("sudo", "ethtool", "--offload", DockerInfo.Bridge, "rx", "off", "tx", "off")

	return nil
}

func (d *Docker) deleteBridge() (err error) {
	err = d.cli.NetworkRemove(context.Background(), DockerInfo.Bridge)
	if err != nil {
		return err

	}
	return nil
}

func (d *Docker) createContainer(name string, node *Node) (err error) {
	log.Debug("Create and Start creating container: ", name)
	ctx := context.Background()

	c, err := d.cli.ContainerCreate(ctx,
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
		}, &container.HostConfig{
			Binds:       node.Binds,
			Sysctls:     node.Sysctls,
			Privileged:  true,
			NetworkMode: container.NetworkMode(DockerInfo.Bridge),
		}, nil, "lab"+"_"+Prefix+"_"+name)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Container create response: %v", c))

	err = d.cli.ContainerStart(ctx,
		c.ID,
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		},
	)
	if err != nil {
		return err
	}

	s, err := d.cli.ContainerInspect(ctx, c.ID)
	if err != nil {
		return err
	}
	node.Pid = s.State.Pid
	log.Debug("Container pid: ", node.Pid)

	return nil
}

func (d *Docker) deleteContainer(name string, node *Node) (err error) {
	log.Debug("Delete and remove container: ", name)
	ctx := context.Background()

	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	var cid string

	for _, container := range containers {
		for _, n := range container.Names {
			if strings.Contains(n, "lab"+"_"+Prefix+"_"+name) {
				cid = container.ID
				break
			}
		}
	}

	if cid != "" {
		err = d.cli.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return err
		}
	}

	return nil
}
