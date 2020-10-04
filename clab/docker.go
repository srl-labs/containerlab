package clab

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	log "github.com/sirupsen/logrus"
)

const sysctlBase = "/proc/sys"

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
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	netCreateResponse, err = c.DockerClient.NetworkCreate(nctx, c.Conf.DockerInfo.Bridge, networkOptions)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Debugf("Container network %s already exists", c.Conf.DockerInfo.Bridge)
			nctx, cancel := context.WithTimeout(ctx, c.timeout)
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

	log.Debug("Disable RPF check on the docker host")
	err = setSysctl("net/ipv4/conf/all/rp_filter", 0)
	if err != nil {
		return fmt.Errorf("failed to disable RP filter on docker host for the 'all' scope: %v", err)
	}
	err = setSysctl("net/ipv4/conf/default/rp_filter", 0)
	if err != nil {
		return fmt.Errorf("failed to disable RP filter on docker host for the 'default' scope: %v", err)
	}

	log.Debugf("Enable LLDP on the docker bridge %s", bridgeName)
	file := "/sys/class/net/" + bridgeName + "/bridge/group_fwd_mask"

	err = ioutil.WriteFile(file, []byte(strconv.Itoa(16384)), 0640)
	if err != nil {
		return fmt.Errorf("failed to enable LLDP on docker bridge: %v", err)
	}

	log.Debug("Disable Checksum Offloading on the docker bridge")
	var b []byte
	b, err = exec.Command("sudo", "ethtool", "--offload", bridgeName, "rx", "off", "tx", "off").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disable Checksum Offloading on docker bridge: %v", err)
	}
	log.Debugf("%s", string(b))
	return nil
}

// DeleteBridge deletes a docker bridge
func (c *cLab) DeleteBridge(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	err = c.DockerClient.NetworkRemove(nctx, c.Conf.DockerInfo.Bridge)
	if err != nil {
		return err
	}
	return nil
}

// CreateContainer creates a docker container
func (c *cLab) CreateContainer(ctx context.Context, node *Node) (err error) {
	log.Infof("Create container: %s", node.ShortName)

	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	labels := map[string]string{
		"containerlab":         "lab-" + c.Conf.Prefix,
		"lab-" + c.Conf.Prefix: node.ShortName,
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

	err = c.PullImageIfRequired(nctx, node)
	if err != nil {
		return err
	}

	cont, err := c.DockerClient.ContainerCreate(nctx,
		&container.Config{
			Image:        node.Image,
			Cmd:          strings.Fields(node.Cmd),
			Env:          node.Env,
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     node.ShortName,
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
	log.Debugf("Container '%s' create response: %v", node.ShortName, cont)
	log.Debugf("Start container: %s", node.LongName)

	err = c.StartContainer(ctx, cont.ID)
	if err != nil {
		return err
	}
	nctx, cancelFn := context.WithTimeout(ctx, c.timeout)
	defer cancelFn()
	cJson, err := c.DockerClient.ContainerInspect(nctx, cont.ID)
	if err != nil {
		return err
	}
	if len(c.Links) >= 0 {
		return linkContainerNS(cJson.State.Pid, node.LongName)
	}
	return nil
}

func (c *cLab) PullImageIfRequired(ctx context.Context, node *Node) (err error) {
	filter := filters.NewArgs()
	filter.Add("reference", node.Image)

	ilo := types.ImageListOptions{
		All:     false,
		Filters: filter,
	}

	log.Debugf("Looking up %s Docker image", node.Image)

	images, err := c.DockerClient.ImageList(ctx, ilo)
	if err != nil {
		return err
	}

	// If Image doesn't exist, we need to pull it
	if len(images) > 0 {
		log.Debugf("Image %s present, skip pulling", node.Image)
		return nil
	}

	// might need canonical name e.g.
	//    -> alpine == docker.io/library/alpine
	//    -> foo/bar == docker.io/foo/bar
	//    -> docker.elastic.co/elasticsearch/elasticsearch == docker.elastic.co/elasticsearch/elasticsearch
	canonicalImageName := node.Image
	slashCount := strings.Count(node.Image, "/")

	switch slashCount {
	case 0:
		canonicalImageName = "docker.io/library/" + node.Image
	case 1:
		canonicalImageName = "docker.io/" + node.Image
	}

	log.Infof("Pulling %s Docker image", node.Image)
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
func (c *cLab) StartContainer(ctx context.Context, id string) error {
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
func (c *cLab) ListContainers(ctx context.Context, labels []string) ([]types.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
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

// DeleteContainers deletes all containers with label=containerlab=lab-$prefix
func (c *cLab) DeleteContainers(ctx context.Context, prefix string) error {
	conts, err := c.ListContainers(ctx, []string{"containerlab=lab-" + prefix})
	if err != nil {
		return err
	}
	if len(conts) == 0 {
		log.Info("no containers found")
		return nil
	}
	for _, cont := range conts {
		log.Infof("Delete container %s", cont.Names)
		err = c.DockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return err
		}
	}
	return nil
}

// linkContainerNS creates a symlink for containers network namespace
// so that it can be managed by iproute2 utility
func linkContainerNS(pid int, containerName string) error {
	CreateDirectory("/run/netns/", 0755)
	src := "/proc/" + strconv.Itoa(pid) + "/ns/net"
	dst := "/run/netns/" + containerName
	err := os.Symlink(src, dst)
	if err != nil {
		return err
	}
	return nil
}

// setSysctl writes sysctl data by writing to a specific file
func setSysctl(sysctl string, newVal int) error {
	return ioutil.WriteFile(path.Join(sysctlBase, sysctl), []byte(strconv.Itoa(newVal)), 0640)
}
