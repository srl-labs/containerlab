package clab

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
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

// CreateBridge creates a docker bridge
func (c *cLab) CreateBridge(ctx context.Context) (err error) {
	log.Info("Creating docker bridge")

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
		//Scope:          "local",
		EnableIPv6: true,
		IPAM:       ipam,
		Internal:   false,
		Attachable: false,
		//Ingress:        false,
		//ConfigOnly:     false,
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
			netResource, err := c.DockerClient.NetworkInspect(nctx, c.Conf.DockerInfo.Bridge) //, types.NetworkInspectOptions{})
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
	var b []byte
	b, err = exec.Command("sudo", "sysctl", "-w", "net.ipv4.conf.all.rp_filter=0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disable Checksum Offloading on docker bridge: %v", err)
	}
	log.Debugf("%s", string(b))
	//if err = sysctl.Set("net.ipv4.conf.default.rp_filter", "0"); err != nil {
	//	return err
	//}
	log.Debug("Disable RPF check on the docker host part2")
	b, err = exec.Command("sudo", "sysctl", "-w", "net.ipv4.conf.all.rp_filter=0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disable Checksum Offloading on docker bridge: %v", err)
	}
	log.Debugf("%s", string(b))
	//if err = sysctl.Set("net.ipv4.conf.all.rp_filter", "0"); err != nil {
	//	return err
	//}
	log.Debug("Enable LLDP on the docker bridge")
	file := "/sys/class/net/" + bridgeName + "/bridge/group_fwd_mask"
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

// DeleteBridge deletes a docker bridge
func (c *cLab) DeleteBridge(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = c.DockerClient.NetworkRemove(nctx, c.Conf.DockerInfo.Bridge)
	if err != nil {
		return err
	}
	return nil
}

// CreateContainer creates a docker container
func (c *cLab) CreateContainer(ctx context.Context, shortDutName string, node *Node) (err error) {
	log.Info("Create container:", shortDutName)

	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	labels := map[string]string{
		"containerlab":         "lab-" + c.Conf.Prefix,
		"lab-" + c.Conf.Prefix: shortDutName,
	}
	if node.Kind != "" {
		labels["kind"] = node.Kind
	}
	if node.NodeType != "" {
		labels["type"] = node.NodeType
	}
	if node.Group != "" {
		labels["group"] = node.Group
	}
	cont, err := c.DockerClient.ContainerCreate(nctx,
		&container.Config{
			Image:        node.Image,
			Cmd:          strings.Fields(node.Cmd),
			Env:          node.Env,
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     shortDutName,
			Volumes:      node.Volumes,
			Tty:          true,
			User:         node.User,
			Labels:       labels,
		}, &container.HostConfig{
			Binds:       node.Binds,
			Sysctls:     node.Sysctls,
			Privileged:  true,
			NetworkMode: container.NetworkMode(c.Conf.DockerInfo.Bridge),
		}, nil, node.LongName)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Container create response: %v", c))

	node.Cid = cont.ID

	err = c.StartContainer(ctx, node.LongName, node)
	if err != nil {
		return err
	}
	err = c.InspectContainer(ctx, node.LongName, node)
	if err != nil {
		return err
	}
	return createContainerNS(node.Pid, node.LongName)
}

// StartContainer starts a docker container
func (c *cLab) StartContainer(ctx context.Context, name string, node *Node) (err error) {
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

// DeleteContainer deletes a docker container
func (c *cLab) DeleteContainer(ctx context.Context, name string, node *Node) (err error) {
	if node.Kind != " bridge" {
		log.Debug("Delete and remove container: ", name)

		containers, err := c.DockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
		if err != nil {
			return err
		}
		var cid string

		for _, container := range containers {
			for _, n := range container.Names {
				if strings.Contains(n, node.LongName) {
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
	}
	return nil
}

// InspectContainer inspects a docker container
func (c *cLab) InspectContainer(ctx context.Context, id string, node *Node) (err error) {
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s, err := c.DockerClient.ContainerInspect(nctx, id)
	if err != nil {
		return err
	}
	node.Pid = s.State.Pid
	if _, ok := s.NetworkSettings.Networks[c.Conf.DockerInfo.Bridge]; ok {
		node.MgmtIPv4 = s.NetworkSettings.Networks[c.Conf.DockerInfo.Bridge].IPAddress
		node.MgmtIPv6 = s.NetworkSettings.Networks[c.Conf.DockerInfo.Bridge].GlobalIPv6Address
		node.MgmtMac = s.NetworkSettings.Networks[c.Conf.DockerInfo.Bridge].MacAddress
	}

	log.Debug("Container pid: ", node.Pid)
	log.Debug("Container mgmt IPv4: ", node.MgmtIPv4)
	log.Debug("Container mgmt IPv6: ", node.MgmtIPv6)
	log.Debug("Container mgmt MAC: ", node.MgmtMac)
	return nil
}

// ListContainers lists all containers with labels []string
func (c *cLab) ListContainers(ctx context.Context, labels []string) ([]types.Container, error) {
	filter := filters.NewArgs()
	for _, l := range labels {
		filter.Add("label", l)
	}
	return c.DockerClient.ContainerList(ctx, types.ContainerListOptions{
		Filters: filter,
	})
}

// Exec executes cmd on container identified with id and returns stdout, stderr bytes and an error
func (c *cLab) Exec(ctx context.Context, id string, cmd []string) ([]byte, []byte, error) {
	cont, err := c.DockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	execID, err := c.DockerClient.ContainerExecCreate(ctx, id, types.ExecConfig{
		User:         "root",
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          cmd,
	})
	if err != nil {
		log.Errorf("failed to create exec in container %s: %v", cont.Name, err)
		return nil, nil, err
	}
	log.Debugf("%s exec created %v", cont.Name, id)
	rsp, err := c.DockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecConfig{
		User:         "root",
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          cmd,
	})
	if err != nil {
		log.Errorf("failed exec in container %s: %v", cont.Name, err)
		return nil, nil, err
	}
	defer rsp.Close()
	log.Debugf("%s exec attached %v", cont.Name, id)

	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, rsp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return outBuf.Bytes(), errBuf.Bytes(), err
		}
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	return outBuf.Bytes(), errBuf.Bytes(), nil
}
func createContainerNS(pid int, containerName string) error {
	CreateDirectory("/run/netns/", 0755)
	src := "/proc/" + strconv.Itoa(pid) + "/ns/net"
	dst := "/run/netns/" + containerName
	cmd := exec.Command("sudo", "ln", "-s", src, dst)
	return runCmd(cmd)
}
