package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const sysctlBase = "/proc/sys"

type DockerRuntime struct {
	Client           *dockerC.Client
	timeout          time.Duration
	Mgmt             types.MgmtNet
	debug            bool
	gracefulShutdown bool
}

func NewDockerRuntime(d bool, dur time.Duration, gracefulShutdown bool) *DockerRuntime {
	c, err := dockerC.NewClientWithOpts(dockerC.FromEnv, dockerC.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("failed to create docker client: %v", err)
	}

	return &DockerRuntime{
		Client: c,
		// TODO: undo this hard-coding
		timeout:          dur,
		debug:            d,
		gracefulShutdown: gracefulShutdown,
	}
}

func (c *DockerRuntime) SetMgmtNet(n types.MgmtNet) {
	c.Mgmt = n
}

// CreateDockerNet creates a docker network or reusing if it exists
func (c *DockerRuntime) CreateNet(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// linux bridge name that is used by docker network
	var bridgeName string

	log.Debugf("Checking if docker network '%s' exists", c.Mgmt.Network)
	netResource, err := c.Client.NetworkInspect(nctx, c.Mgmt.Network, dockerTypes.NetworkInspectOptions{})
	switch {
	case dockerC.IsErrNotFound(err):
		log.Debugf("Network '%s' does not exist", c.Mgmt.Network)
		log.Infof("Creating docker network: Name='%s', IPv4Subnet='%s', IPv6Subnet='%s', MTU='%s'",
			c.Mgmt.Network, c.Mgmt.IPv4Subnet, c.Mgmt.IPv6Subnet, c.Mgmt.MTU)

		enableIPv6 := false
		var ipamConfig []network.IPAMConfig
		if c.Mgmt.IPv4Subnet != "" {
			ipamConfig = append(ipamConfig, network.IPAMConfig{
				Subnet: c.Mgmt.IPv4Subnet,
			})
		}
		if c.Mgmt.IPv6Subnet != "" {
			ipamConfig = append(ipamConfig, network.IPAMConfig{
				Subnet: c.Mgmt.IPv6Subnet,
			})
			enableIPv6 = true
		}

		ipam := &network.IPAM{
			Driver: "default",
			Config: ipamConfig,
		}

		networkOptions := dockerTypes.NetworkCreate{
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
				"com.docker.network.driver.mtu": c.Mgmt.MTU,
			},
		}

		netCreateResponse, err := c.Client.NetworkCreate(nctx, c.Mgmt.Network, networkOptions)
		if err != nil {
			return err
		}

		if len(netCreateResponse.ID) < 12 {
			return fmt.Errorf("could not get bridge ID")
		}
		bridgeName = "br-" + netCreateResponse.ID[:12]

	case err == nil:
		log.Debugf("network '%s' was found. Reusing it...", c.Mgmt.Network)
		if len(netResource.ID) < 12 {
			return fmt.Errorf("could not get bridge ID")
		}
		switch c.Mgmt.Network {
		case "bridge":
			bridgeName = "docker0"
		default:
			bridgeName = "br-" + netResource.ID[:12]
		}

	default:
		return err
	}
	c.Mgmt.Bridge = bridgeName

	log.Debugf("Docker network '%s', bridge name '%s'", c.Mgmt.Network, bridgeName)

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
		log.Warnf("failed to enable LLDP on docker bridge: %v", err)
	}

	log.Debugf("Disabling TX checksum offloading for the %s bridge interface...", bridgeName)
	err = utils.EthtoolTXOff(bridgeName)
	if err != nil {
		log.Warnf("failed to disable TX checksum offloading for the %s bridge interface: %v", bridgeName, err)
	}
	return nil
}

// DeleteNet deletes a docker bridge
func (c *DockerRuntime) DeleteNet(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	nres, err := c.Client.NetworkInspect(ctx, c.Mgmt.Network, dockerTypes.NetworkInspectOptions{})
	if err != nil {
		return err
	}
	numEndpoints := len(nres.Containers)
	if numEndpoints > 0 {
		if c.debug {
			log.Debugf("network '%s' has %d active endpoints, deletion skipped", c.Mgmt.Network, numEndpoints)
			for _, endp := range nres.Containers {
				log.Debugf("'%s' is connected to %s", endp.Name, c.Mgmt.Network)
			}
		}
		return nil
	}
	err = c.Client.NetworkRemove(nctx, c.Mgmt.Network)
	if err != nil {
		return err
	}
	return nil
}

// CreateContainer creates a docker container
func (c *DockerRuntime) CreateContainer(ctx context.Context, node *types.NodeBase) (err error) {
	log.Infof("Creating container: %s", node.ShortName)

	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd, err := shlex.Split(node.Cmd)
	if err != nil {
		return err
	}

	containerConfig := &container.Config{
		Image:        node.Image,
		Cmd:          cmd,
		Env:          utils.ConvertEnvs(node.Env),
		AttachStdout: true,
		AttachStderr: true,
		Hostname:     node.ShortName,
		Tty:          true,
		User:         node.User,
		Labels:       node.Labels,
		ExposedPorts: node.PortSet,
		MacAddress:   node.MacAddress,
	}
	containerHostConfig := &container.HostConfig{
		Binds:        node.Binds,
		PortBindings: node.PortBindings,
		Sysctls:      node.Sysctls,
		Privileged:   true,
		NetworkMode:  container.NetworkMode(c.Mgmt.Network),
	}

	containerNetworkingConfig := &network.NetworkingConfig{}

	switch node.NetworkMode {
	case "host":
		containerHostConfig.NetworkMode = container.NetworkMode("host")
	default:
		containerHostConfig.NetworkMode = container.NetworkMode(c.Mgmt.Network)

		containerNetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			c.Mgmt.Network: {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: node.MgmtIPv4Address,
					IPv6Address: node.MgmtIPv6Address,
				},
			},
		}
	}

	cont, err := c.Client.ContainerCreate(
		nctx,
		containerConfig,
		containerHostConfig,
		containerNetworkingConfig,
		nil,
		node.LongName,
	)
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

	node.NSPath, err = c.GetNSPath(ctx, cont.ID)
	if err != nil {
		return err
	}
	return utils.LinkContainerNS(node.NSPath, node.LongName)

}

// GetNSPath inspects a container by its name/id and returns an netns path using the pid of a container
func (c *DockerRuntime) GetNSPath(ctx context.Context, containerId string) (string, error) {
	nctx, cancelFn := context.WithTimeout(ctx, c.timeout)
	defer cancelFn()
	cJSON, err := c.Client.ContainerInspect(nctx, containerId)
	if err != nil {
		return "", err
	}
	return "/proc/" + strconv.Itoa(cJSON.State.Pid) + "/ns/net", nil
}

func (c *DockerRuntime) PullImageIfRequired(ctx context.Context, imageName string) error {
	filter := filters.NewArgs()
	filter.Add("reference", imageName)

	ilo := dockerTypes.ImageListOptions{
		All:     false,
		Filters: filter,
	}

	log.Debugf("Looking up %s Docker image", imageName)

	images, err := c.Client.ImageList(ctx, ilo)
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
	//    -> foo.bar/baz == foo.bar/bar
	//    -> docker.elastic.co/elasticsearch/elasticsearch == docker.elastic.co/elasticsearch/elasticsearch
	canonicalImageName := imageName
	slashCount := strings.Count(imageName, "/")

	switch slashCount {
	case 0:
		canonicalImageName = "docker.io/library/" + imageName
	case 1:
		// split on slash to get first element of the name
		nameSplit := strings.Split(imageName, "/")
		// case of foo.bar/baz
		if strings.Contains(nameSplit[0], ".") {
			canonicalImageName = imageName
		} else {
			canonicalImageName = "docker.io/" + imageName
		}
	}

	log.Infof("Pulling %s Docker image", canonicalImageName)
	reader, err := c.Client.ImagePull(ctx, canonicalImageName, dockerTypes.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	// must read from reader, otherwise image is not properly pulled
	_, _ = io.Copy(ioutil.Discard, reader)
	log.Infof("Done pulling %s", canonicalImageName)

	return nil
}

// StartContainer starts a docker container
func (c *DockerRuntime) StartContainer(ctx context.Context, id string) error {
	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.Client.ContainerStart(nctx,
		id,
		dockerTypes.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		},
	)
}

// ListContainers lists all containers with labels []string
func (c *DockerRuntime) ListContainers(ctx context.Context, labels []string) ([]types.GenericContainer, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	filter := filters.NewArgs()
	for _, l := range labels {
		filter.Add("label", l)
	}
	ctrs, err := c.Client.ContainerList(ctx, dockerTypes.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, err
	}

	return c.produceGenericContainerList(ctrs)
}

// Transform docker-specific to generic container format
func (c *DockerRuntime) produceGenericContainerList(input []dockerTypes.Container) ([]types.GenericContainer, error) {
	var result []types.GenericContainer

	for _, i := range input {
		ctr := types.GenericContainer{
			Names:  i.Names,
			ID:     i.ID,
			Image:  i.Image,
			State:  i.State,
			Status: i.Status,
			Labels: i.Labels,
			NetworkSettings: &types.GenericMgmtIPs{
				Set: false,
			},
		}
		if ifcfg, ok := i.NetworkSettings.Networks[c.Mgmt.Network]; ok {
			ctr.NetworkSettings.IPv4addr = ifcfg.IPAddress
			ctr.NetworkSettings.IPv4pLen = ifcfg.IPPrefixLen
			ctr.NetworkSettings.IPv6addr = ifcfg.GlobalIPv6Address
			ctr.NetworkSettings.IPv6pLen = ifcfg.GlobalIPv6PrefixLen
			ctr.NetworkSettings.Set = true
		}
		result = append(result, ctr)
	}

	return result, nil
}

// Exec executes cmd on container identified with id and returns stdout, stderr bytes and an error
func (c *DockerRuntime) Exec(ctx context.Context, id string, cmd []string) ([]byte, []byte, error) {
	cont, err := c.Client.ContainerInspect(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	execID, err := c.Client.ContainerExecCreate(ctx, id, dockerTypes.ExecConfig{
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

	rsp, err := c.Client.ContainerExecAttach(ctx, execID.ID, dockerTypes.ExecStartCheck{})
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
func (c *DockerRuntime) ExecNotWait(ctx context.Context, id string, cmd []string) error {
	execConfig := dockerTypes.ExecConfig{Tty: false, AttachStdout: false, AttachStderr: false, Cmd: cmd}
	respID, err := c.Client.ContainerExecCreate(context.Background(), id, execConfig)
	if err != nil {
		return err
	}

	execStartCheck := dockerTypes.ExecStartCheck{}
	_, err = c.Client.ContainerExecAttach(context.Background(), respID.ID, execStartCheck)
	if err != nil {
		return err
	}
	return nil
}

// DeleteContainer tries to stop a container then remove it
func (c *DockerRuntime) DeleteContainer(ctx context.Context, name string) error {
	var err error
	force := !c.gracefulShutdown
	if c.gracefulShutdown {
		log.Infof("Stopping container: %s", name)
		err = c.Client.ContainerStop(ctx, name, &c.timeout)
		if err != nil {
			log.Errorf("could not stop container '%s': %v", name, err)
			force = true
		}
	}
	log.Debugf("Removing container: %s", name)
	err = c.Client.ContainerRemove(ctx, name, dockerTypes.ContainerRemoveOptions{Force: force})
	if err != nil {
		return err
	}
	log.Infof("Removed container: %s", name)
	return nil
}

// setSysctl writes sysctl data by writing to a specific file
func setSysctl(sysctl string, newVal int) error {
	return ioutil.WriteFile(path.Join(sysctlBase, sysctl), []byte(strconv.Itoa(newVal)), 0640)
}

func (c *DockerRuntime) ContainerInspect(ctx context.Context, id string) (*types.GenericContainer, error) {
	ctr, err := c.Client.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	return &types.GenericContainer{
		Pid: ctr.State.Pid,
	}, nil
}

func (c *DockerRuntime) StopContainer(context.Context, string, *time.Duration) error {
	return nil
}
