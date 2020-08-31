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
	netCreateResponse, err = d.cli.NetworkCreate(context.Background(), DockerInfo.Bridge, networkOptions)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Debugf("Container network %s already exists", DockerInfo.Bridge)
			netResource, err := d.cli.NetworkInspect(context.TODO(), DockerInfo.Bridge, types.NetworkInspectOptions{})
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
	log.Debugf("container network %s : bridge name: %s", DockerInfo.Bridge, bridgeName)
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

func (d *Docker) deleteBridge() (err error) {
	err = d.cli.NetworkRemove(context.Background(), DockerInfo.Bridge)
	if err != nil {
		return err

	}
	return nil
}

func (d *Docker) createContainer(name string, node *Node) (err error) {
	log.Debug("Create container: ", name)
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
		}, nil, "lab"+"-"+Prefix+"-"+name)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Container create response: %v", c))

	node.Cid = c.ID

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
	node.MgmtIPv4 = s.NetworkSettings.Networks["srlinux_bridge"].IPAddress
	node.MgmtIPv6 = s.NetworkSettings.Networks["srlinux_bridge"].GlobalIPv6Address
	node.MgmtMac = s.NetworkSettings.Networks["srlinux_bridge"].MacAddress

	log.Debug("Container pid: ", node.Pid)
	log.Debug("Container pid: ", node.MgmtIPv4)

	return nil
}

func (d *Docker) startContainer(name string, node *Node) (err error) {
	log.Debug("Start container: ", name)
	ctx := context.Background()

	err = d.cli.ContainerStart(ctx,
		node.Cid,
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		},
	)
	if err != nil {
		return err
	}

	s, err := d.cli.ContainerInspect(ctx, node.Cid)
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
			if strings.Contains(n, "lab"+"-"+Prefix+"-"+name) {
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
