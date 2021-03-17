package clab

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
)

const sysctlBase = "/proc/sys"

// CreateDockerNet creates a docker network or reusing if it exists
func (c *CLab) CreateDockerNet(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// linux bridge name that is used by docker network
	var bridgeName string

	log.Debugf("Checking if docker network '%s' exists", c.Config.Mgmt.Network)
	netResource, err := c.DockerClient.NetworkInspect(nctx, c.Config.Mgmt.Network)
	switch {
	case client.IsErrNetworkNotFound(err):
		log.Debugf("Network '%s' does not exist", c.Config.Mgmt.Network)
		log.Infof("Creating docker network: Name='%s', IPv4Subnet='%s', IPv6Subnet='%s'", c.Config.Mgmt.Network, c.Config.Mgmt.IPv4Subnet, c.Config.Mgmt.IPv6Subnet)

		enableIPv6 := false
		var ipamConfig []network.IPAMConfig
		if c.Config.Mgmt.IPv4Subnet != "" {
			ipamConfig = append(ipamConfig, network.IPAMConfig{
				Subnet: c.Config.Mgmt.IPv4Subnet,
			})
		}
		if c.Config.Mgmt.IPv6Subnet != "" {
			ipamConfig = append(ipamConfig, network.IPAMConfig{
				Subnet: c.Config.Mgmt.IPv6Subnet,
			})
			enableIPv6 = true
		}

		ipam := &network.IPAM{
			Driver: "default",
			Config: ipamConfig,
		}

		networkOptions := types.NetworkCreate{
			CheckDuplicate: true,
			Driver:         "bridge",
			EnableIPv6:     enableIPv6,
			IPAM:           ipam,
			Internal:       false,
			Attachable:     false,
			Labels: map[string]string{
				"containerlab": "",
			},
			Options: map[string]string{
				"com.docker.network.driver.mtu": c.Config.Mgmt.MTU,
			},
		}

		netCreateResponse, err := c.DockerClient.NetworkCreate(nctx, c.Config.Mgmt.Network, networkOptions)
		if err != nil {
			return err
		}

		if len(netCreateResponse.ID) < 12 {
			return fmt.Errorf("could not get bridge ID")
		}
		bridgeName = "br-" + netCreateResponse.ID[:12]

	case err == nil:
		log.Debugf("network '%s' was found. Reusing it...", c.Config.Mgmt.Network)
		if len(netResource.ID) < 12 {
			return fmt.Errorf("could not get bridge ID")
		}
		bridgeName = "br-" + netResource.ID[:12]
	default:
		return err
	}

	log.Debugf("Docker network '%s', bridge name '%s'", c.Config.Mgmt.Network, bridgeName)

	log.Debug("Disable RPF check on the docker host")
	err = setSysctl("net/ipv4/conf/all/rp_filter", 0)
	if err != nil {
		return fmt.Errorf("failed to disable RP filter on docker host for the 'all' scope: %v", err)
	}
	err = setSysctl("net/ipv4/conf/default/rp_filter", 0)
	if err != nil {
		return fmt.Errorf("failed to disable RP filter on docker host for the 'default' scope: %v", err)
	}

	log.Debugf("Enable LLDP on the linux bridge %s", bridgeName)
	file := "/sys/class/net/" + bridgeName + "/bridge/group_fwd_mask"

	err = ioutil.WriteFile(file, []byte(strconv.Itoa(16384)), 0640)
	if err != nil {
		return fmt.Errorf("failed to enable LLDP on docker bridge: %v", err)
	}

	log.Debugf("Disabling TX checksum offloading for the %s bridge interface...", bridgeName)
	err = EthtoolTXOff(bridgeName)
	if err != nil {
		return fmt.Errorf("failed to disable TX checksum offloading for the %s bridge interface: %v", bridgeName, err)
	}
	return nil
}

// DeleteBridge deletes a docker bridge
func (c *CLab) DeleteBridge(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	nres, err := c.DockerClient.NetworkInspect(ctx, c.Config.Mgmt.Network)
	if err != nil {
		return err
	}
	numEndpoints := len(nres.Containers)
	if numEndpoints > 0 {
		log.Warnf("network '%s' has %d active endpoints", c.Config.Mgmt.Network, numEndpoints)
		if c.debug {
			for _, endp := range nres.Containers {
				log.Debugf("'%s' is connected to %s", endp.Name, c.Config.Mgmt.Network)
			}
		}
		return nil
	}
	err = c.DockerClient.NetworkRemove(nctx, c.Config.Mgmt.Network)
	if err != nil {
		return err
	}
	return nil
}

// CreateContainer creates a docker container
func (c *CLab) CreateContainer(ctx context.Context, node *Node) (err error) {
	log.Infof("Creating container: %s", node.ShortName)

	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd, err := shlex.Split(node.Cmd)
	if err != nil {
		return err
	}

	cont, err := c.DockerClient.ContainerCreate(nctx,
		&container.Config{
			Image:        node.Image,
			Cmd:          cmd,
			Env:          convertEnvs(node.Env),
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     node.ShortName,
			Tty:          true,
			User:         node.User,
			Labels:       node.Labels,
			ExposedPorts: node.PortSet,
		}, &container.HostConfig{
			Binds:        node.Binds,
			PortBindings: node.PortBindings,
			Sysctls:      node.Sysctls,
			Privileged:   true,
			NetworkMode:  container.NetworkMode(c.Config.Mgmt.Network),
		}, &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				c.Config.Mgmt.Network: {
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: node.MgmtIPv4Address,
						IPv6Address: node.MgmtIPv6Address,
					},
				},
			},
		}, node.LongName)
	if err != nil {
		return err
	}
	log.Debugf("Container '%s' create response: %v", node.ShortName, cont)
	log.Debugf("Start container: %s", node.LongName)

	err = c.StartContainer(ctx, cont.ID)
	if err != nil {
		return err
	}
	log.Debugf("Container started: %s", node.LongName)
	nctx, cancelFn := context.WithTimeout(ctx, c.timeout)
	defer cancelFn()
	cJSON, err := c.DockerClient.ContainerInspect(nctx, cont.ID)
	if err != nil {
		return err
	}
	node.NSPath = "/proc/" + strconv.Itoa(cJSON.State.Pid) + "/ns/net"
	return linkContainerNS(node.NSPath, node.LongName)
}

func (c *CLab) PullImageIfRequired(ctx context.Context, imageName string) error {
	filter := filters.NewArgs()
	filter.Add("reference", imageName)

	ilo := types.ImageListOptions{
		All:     false,
		Filters: filter,
	}

	log.Debugf("Looking up %s Docker image", imageName)

	images, err := c.DockerClient.ImageList(ctx, ilo)
	if err != nil {
		return err
	}

	// If Image doesn't exist, we need to pull it
	if len(images) > 0 {
		log.Debugf("Image %s present, skip pulling", imageName)
		return nil
	}

	// might need canonical name e.g.
	//    -> alpine == docker.io/library/alpine
	//    -> foo/bar == docker.io/foo/bar
	//    -> docker.elastic.co/elasticsearch/elasticsearch == docker.elastic.co/elasticsearch/elasticsearch
	canonicalImageName := imageName
	slashCount := strings.Count(imageName, "/")

	switch slashCount {
	case 0:
		canonicalImageName = "docker.io/library/" + imageName
	case 1:
		canonicalImageName = "docker.io/" + imageName
	}

	log.Infof("Pulling %s Docker image", canonicalImageName)
	reader, err := c.DockerClient.ImagePull(ctx, canonicalImageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	// must read from reader, otherwise image is not properly pulled
	io.Copy(ioutil.Discard, reader)
	log.Infof("Done pulling %s", canonicalImageName)

	return nil
}

// StartContainer starts a docker container
func (c *CLab) StartContainer(ctx context.Context, id string) error {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.DockerClient.ContainerStart(nctx,
		id,
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		},
	)
}

// ListContainers lists all containers with labels []string
func (c *CLab) ListContainers(ctx context.Context, labels []string) ([]types.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	filter := filters.NewArgs()
	for _, l := range labels {
		filter.Add("label", l)
	}
	return c.DockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
}

// Exec executes cmd on container identified with id and returns stdout, stderr bytes and an error
func (c *CLab) Exec(ctx context.Context, id string, cmd []string) ([]byte, []byte, error) {
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

// ExecNotWait executes cmd on container identified with id but doesn't wait for output nor attaches stodout/err
func (c *CLab) ExecNotWait(ctx context.Context, id string, cmd []string) error {
	execConfig := types.ExecConfig{Tty: false, AttachStdout: false, AttachStderr: false, Cmd: cmd}
	respID, err := c.DockerClient.ContainerExecCreate(context.Background(), id, execConfig)
	if err != nil {
		return err
	}
	_, err = c.DockerClient.ContainerExecAttach(context.Background(), respID.ID, execConfig)
	if err != nil {
		return err
	}
	return nil
}

// DeleteContainer tries to stop a container then remove it
func (c *CLab) DeleteContainer(ctx context.Context, name string) error {
	var err error
	force := !c.gracefulShutdown
	if c.gracefulShutdown {
		log.Infof("Stopping container: %s", name)
		err = c.DockerClient.ContainerStop(ctx, name, &c.timeout)
		if err != nil {
			log.Errorf("could not stop container '%s': %v", name, err)
			force = true
		}
	}
	log.Debugf("Removing container: %s", name)
	err = c.DockerClient.ContainerRemove(ctx, name, types.ContainerRemoveOptions{Force: force})
	if err != nil {
		return err
	}
	log.Infof("Removed container: %s", name)
	return nil
}

// linkContainerNS creates a symlink for containers network namespace
// so that it can be managed by iproute2 utility
func linkContainerNS(nspath string, containerName string) error {
	CreateDirectory("/run/netns/", 0755)
	dst := "/run/netns/" + containerName
	err := os.Symlink(nspath, dst)
	if err != nil {
		return err
	}
	return nil
}

// setSysctl writes sysctl data by writing to a specific file
func setSysctl(sysctl string, newVal int) error {
	return ioutil.WriteFile(path.Join(sysctlBase, sysctl), []byte(strconv.Itoa(newVal)), 0640)
}
