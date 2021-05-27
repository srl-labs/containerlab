package containerd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/docker/go-units"
	"github.com/google/shlex"
	"github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const containerdNamespace = "clab"

type ContainerdRuntime struct {
	client           *containerd.Client
	timeout          time.Duration
	Mgmt             types.MgmtNet
	debug            bool
	gracefulShutdown bool
}

func NewContainerdRuntime(d bool, dur time.Duration, gracefulShutdown bool) *ContainerdRuntime {
	c, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.Fatalf("failed to create containerd client: %v", err)
	}

	return &ContainerdRuntime{
		client:           c,
		debug:            d,
		timeout:          dur,
		gracefulShutdown: gracefulShutdown,
	}
}

func (c *ContainerdRuntime) SetMgmtNet(n types.MgmtNet) {
	c.Mgmt = n
}

func (c *ContainerdRuntime) CreateNet(ctx context.Context) error {
	//log.Fatalf("CreateNet() - Not implemented yet")
	// TODO: need to implement
	return nil
}
func (c *ContainerdRuntime) DeleteNet(context.Context) error {
	log.Fatalf("DeleteNet() - Not implemented yet")
	return nil
}

func (c *ContainerdRuntime) PullImageIfRequired(ctx context.Context, imagename string) error {

	canonicalimage := utils.GetCanonicalImageName(imagename)

	log.Debugf("Looking up %s container image", canonicalimage)
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	images, err := c.client.ListImages(ctx)
	if err != nil {
		return err
	}

	// If Image doesn't exist, we need to pull it
	if len(images) > 0 {
		log.Debugf("Image %s present, skip pulling", canonicalimage)
		return nil
	}

	_, err = c.client.Pull(ctx, canonicalimage, containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerdRuntime) CreateContainer(ctx context.Context, node *types.Node) error {
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)

	img, err := c.client.GetImage(ctx, node.Image)
	if err != nil {
		return err
	}

	cmd, err := shlex.Split(node.Cmd)
	if err != nil {
		return err
	}
	// TODO: MAC address
	// TODO: Network interface
	// TODO: Portbinding

	opts := []oci.SpecOpts{
		oci.WithImageConfig(img),
		oci.WithEnv(utils.ConvertEnvs(node.Env)),
		oci.WithProcessArgs(cmd...),
		oci.WithHostname(node.ShortName),
		oci.WithUser(node.User),
		WithSysctls(node.Sysctls),
		oci.WithPrivileged,
	}

	switch node.NetworkMode {
	case "host":
		opts = append(opts,
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithHostHostsFile,
			oci.WithHostResolvconf)
	default:
		// TODO: NETWORK

	}

	_, err = c.client.NewContainer(
		ctx,
		node.ShortName,
		containerd.WithNewSnapshot(node.ShortName+"-snapshot", img),
		containerd.WithNewSpec(opts...),
		containerd.WithAdditionalContainerLabels(node.Labels),
	)
	if err != nil {
		return err
	}

	log.Debugf("Container '%s' created", node.ShortName)
	log.Debugf("Start container: %s", node.LongName)

	err = c.StartContainer(ctx, node.ShortName)
	if err != nil {
		return err
	}

	log.Debugf("Container started: %s", node.LongName)

	node.NSPath, err = c.GetNSPath(ctx, node.ShortName)
	if err != nil {
		return err
	}
	return nil
}

func WithSysctls(sysctls map[string]string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *specs.Spec) error {
		if s.Linux == nil {
			s.Linux = &specs.Linux{}
		}
		if s.Linux.Sysctl == nil {
			s.Linux.Sysctl = make(map[string]string)
		}
		for k, v := range sysctls {
			s.Linux.Sysctl[k] = v
		}
		return nil
	}
}

func (c *ContainerdRuntime) StartContainer(ctx context.Context, containername string) error {
	container, err := c.client.LoadContainer(ctx, containername)
	if err != nil {
		return err
	}
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		log.Fatalf("Failed to start container %s", containername)
		return err
	}
	err = task.Start(ctx)
	return err
}
func (c *ContainerdRuntime) StopContainer(ctx context.Context, containername string, dur *time.Duration) error {
	ctask, err := c.getContainerTask(ctx, containername)
	if err != nil {
		return err
	}
	taskstatus, err := ctask.Status(ctx)
	if err != nil {
		return err
	}
	switch taskstatus.Status {
	case "running", "paused":
		err = ctask.Kill(ctx, syscall.SIGQUIT)
		if err != nil {
			return err
		}
	}
	existStatus, err := ctask.Delete(ctx)
	if err != nil {
		return err
	}
	log.Debugf("Container %s stopped with exit code %d", containername, existStatus.ExitCode())
	return nil
}

func (c *ContainerdRuntime) getContainerTask(ctx context.Context, containername string) (containerd.Task, error) {
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	cont, err := c.client.LoadContainer(ctx, containername)
	if err != nil {
		return nil, err
	}
	task, err := cont.Task(ctx, nil)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (c *ContainerdRuntime) ListContainers(ctx context.Context, filter []*types.GenericFilter) ([]types.GenericContainer, error) {
	log.Debug("listing containers")
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	// TODO add containerlab label as filter criteria

	filterstring := c.filterStringBuilder(filter)

	containerlist, err := c.client.Containers(ctx, filterstring)
	if err != nil {
		return nil, err
	}

	return c.produceGenericContainerList(ctx, containerlist)
}

func (c *ContainerdRuntime) filterStringBuilder(filter []*types.GenericFilter) string {
	filterstring := ""
	delim := ""
	for _, filterEntry := range filter {
		isExistsOperator := false

		operator := filterEntry.Operator
		switch filterEntry.Operator {
		case "=":
			operator = "=="
		case "exists":
			operator = ""
			isExistsOperator = true
		}

		switch filterEntry.FilterType {
		case "label":
			filterstring = filterstring + "labels." + filterEntry.Field
			if !isExistsOperator {
				filterstring = filterstring + operator + filterEntry.Match + delim
			}

		}
		delim = ","
	}
	log.Debug("Filterstring: " + filterstring)
	return filterstring
}

// Transform docker-specific to generic container format
func (c *ContainerdRuntime) produceGenericContainerList(ctx context.Context, input []containerd.Container) ([]types.GenericContainer, error) {
	var result []types.GenericContainer

	for _, i := range input {

		ctr := types.GenericContainer{}

		info, err := i.Info(ctx)
		if err != nil {
			return nil, err
		}

		ctr.Names = []string{i.ID()}
		ctr.ID = i.ID()
		ctr.Image = info.Image
		ctr.Labels = info.Labels
		ctr.NetworkSettings = &types.GenericMgmtIPs{Set: false}

		taskfound := true
		task, err := i.Task(ctx, nil)
		if err != nil {
			// NOTE: NotFound doesn't mean that container hasn't started.
			// In docker/CRI-containerd plugin, the task will be deleted
			// when it exits. So, the status will be "created" for this
			// case.
			if errdefs.IsNotFound(err) {
				taskfound = false
			}
		}
		if taskfound {
			status, err := task.Status(ctx)
			if err != nil {
				log.Fatalf("failed to retrieve task status")
				return nil, err
			}
			ctr.State = string(status.Status)

			switch s := status.Status; s {
			case containerd.Stopped:
				ctr.Status = fmt.Sprintf("Exited (%v) %s", status.ExitStatus, timeSinceInHuman(status.ExitTime))
			case containerd.Running:
				ctr.Status = "Up"
			default:
				ctr.Status = strings.Title(string(s))
			}

			ctr.Pid = int(task.Pid())
		} else {
			ctr.State = strings.Title(string(containerd.Unknown))
			ctr.Status = "Unknown"
			ctr.Pid = -1
		}
		result = append(result, ctr)
	}
	return result, nil
}

func timeSinceInHuman(since time.Time) string {
	return units.HumanDuration(time.Since(since)) + " ago"
}

func (c *ContainerdRuntime) ContainerInspect(context.Context, string) (*types.GenericContainer, error) {
	log.Fatalf("ContainerInspect() - Not implemented yet")
	return &types.GenericContainer{}, nil
}
func (c *ContainerdRuntime) GetNSPath(ctx context.Context, containername string) (string, error) {
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	task, err := c.getContainerTask(ctx, containername)
	if err != nil {
		return "", err
	}
	return "/proc/" + strconv.Itoa(int(task.Pid())) + "/ns/net", nil
}
func (c *ContainerdRuntime) Exec(context.Context, string, []string) ([]byte, []byte, error) {
	log.Fatalf("Exec() - Not implemented yet")
	return []byte(""), []byte(""), nil
}
func (c *ContainerdRuntime) ExecNotWait(context.Context, string, []string) error {
	log.Fatalf("ExecNotWait() - Not implemented yet")
	return nil
}
func (c *ContainerdRuntime) DeleteContainer(ctx context.Context, container *types.GenericContainer) error {
	log.Debugf("deleting container %s", container.ID)
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)

	err := c.StopContainer(ctx, container.ID, nil)
	if err != nil {
		return err
	}

	cont, err := c.client.LoadContainer(ctx, container.ID)
	if err != nil {
		return err
	}
	var delOpts []containerd.DeleteOpts
	if _, err := cont.Image(ctx); err == nil {
		delOpts = append(delOpts, containerd.WithSnapshotCleanup)
	}

	if err := cont.Delete(ctx, delOpts...); err != nil {
		return err
	}
	log.Debug("successfully deleted container %s", container.ID)
	return nil
}
