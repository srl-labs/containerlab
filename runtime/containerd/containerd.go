package containerd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/go-units"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

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
	//defer c.Close()

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

func (c *ContainerdRuntime) CreateNet(context.Context) error {
	log.Fatalf("CreateNet() - Not implemented yet")
	return nil
}
func (c *ContainerdRuntime) DeleteNet(context.Context) error {
	log.Fatalf("DeleteNet() - Not implemented yet")
	return nil
}

func (c *ContainerdRuntime) PullImageIfRequired(ctx context.Context, imagename string) error {

	log.Debugf("Looking up %s container image", imagename)
	ctx = namespaces.WithNamespace(ctx, "clab")
	images, err := c.client.ListImages(ctx)
	if err != nil {
		return err
	}

	// If Image doesn't exist, we need to pull it
	if len(images) > 0 {
		log.Debugf("Image %s present, skip pulling", imagename)
		return nil
	}

	_, err = c.client.Pull(ctx, imagename+":latest", containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerdRuntime) CreateContainer(context.Context, *types.Node) error {
	log.Fatalf("CreateContainer() - Not implemented yet")
	return nil
}
func (c *ContainerdRuntime) StartContainer(context.Context, string) error {
	log.Fatalf("StartContainer() - Not implemented yet")
	return nil
}
func (c *ContainerdRuntime) StopContainer(ctx context.Context, containername string, duration *time.Duration) error {
	log.Fatalf("StopContainer() - Not implemented yet")
	return nil
}
func (c *ContainerdRuntime) ListContainers(ctx context.Context, labels []string) ([]types.GenericContainer, error) {

	log.Debug("listing containers")
	ctx = namespaces.WithNamespace(ctx, "clab")
	containerlist, err := c.client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	return c.produceGenericContainerList(ctx, containerlist)
}

// Transform docker-specific to generic container format
func (c *ContainerdRuntime) produceGenericContainerList(ctx context.Context, input []containerd.Container) ([]types.GenericContainer, error) {
	var result []types.GenericContainer

	ctx = namespaces.WithNamespace(ctx, "clab")

	for _, i := range input {

		ctr := types.GenericContainer{}

		_, err := i.Image(ctx)
		if err != nil {
			return nil, err
		}
		info, err := i.Info(ctx)
		if err != nil {
			return nil, err
		}

		ctr.Names = []string{info.Labels["name"]}
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
			ctr.Status = "unknown"
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
func (c *ContainerdRuntime) GetNSPath(context.Context, string) (string, error) {
	log.Fatalf("GetNSPath() - Not implemented yet")
	return "", nil
}
func (c *ContainerdRuntime) Exec(context.Context, string, []string) ([]byte, []byte, error) {
	log.Fatalf("Exec() - Not implemented yet")
	return []byte(""), []byte(""), nil
}
func (c *ContainerdRuntime) ExecNotWait(context.Context, string, []string) error {
	log.Fatalf("ExecNotWait() - Not implemented yet")
	return nil
}
func (c *ContainerdRuntime) DeleteContainer(context.Context, string) error {
	log.Fatalf("DeleteContainer() - Not implemented yet")
	return nil
}
