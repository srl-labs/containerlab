//go:build linux && podman
// +build linux,podman

package podman

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/images"
	"github.com/containers/podman/v3/pkg/bindings/network"
	dockerTypes "github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	runtimeName    = "podman"
	defaultTimeout = 120 * time.Second
)

type PodmanRuntime struct {
	config *runtime.RuntimeConfig
	mgmt   *types.MgmtNet
}

func init() {
	runtime.Register(runtimeName, func() runtime.ContainerRuntime {
		return &PodmanRuntime{
			config: &runtime.RuntimeConfig{},
			mgmt:   &types.MgmtNet{},
		}
	})
}

// Init is used to initialize our runtime struct by calling all methods received from the caller
// Invokes methods such as WithConfig, WithMgmtNet etc to populate the fields
func (r *PodmanRuntime) Init(opts ...runtime.RuntimeOption) error {
	for _, f := range opts {
		f(r)
	}
	return nil
}

func (r *PodmanRuntime) Mgmt() *types.MgmtNet { return r.mgmt }

func (r *PodmanRuntime) WithConfig(cfg *runtime.RuntimeConfig) {
	log.Debugf("Podman method WithConfig was called with cfg params: %+v", cfg)
	// Check for nil pointers on input
	if cfg == nil {
		log.Errorf("Method WithConfig has received a nil pointer")
		return
	}
	r.config = cfg
	if r.config.Timeout <= 0 {
		r.config.Timeout = defaultTimeout
	}
}

// WithMgmtNet assigns struct mgmt net parameters to the runtime struct
func (r *PodmanRuntime) WithMgmtNet(net *types.MgmtNet) {
	// Check for nil pointers on input
	if net == nil {
		log.Errorf("Method WithMgmtNet has received a nil pointer")
		return
	}
	log.Debugf("Podman method WithMgmtNet was called with net params: %+v", net)
	r.mgmt = net
	if r.mgmt.Bridge == "" && r.mgmt.Network != "" {
		// set bridge name = network name
		// albeit we don't use it as of right now when creating a bridge
		r.mgmt.Bridge = r.mgmt.Network
	}

}

// WithKeepMgmtNet defines that we shouldn't delete mgmt network(s)
func (r *PodmanRuntime) WithKeepMgmtNet() {
	r.config.KeepMgmtNet = true
}

// CreateNet used to create a new bridge for clab mgmt network
func (r *PodmanRuntime) CreateNet(ctx context.Context) error {
	// TODO add custom bridge name + bridge options
	// TODO: looks like the current version of CreateOptions does not support dual-stack / multiple subnets
	// May need to refactor this.
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	log.Debugf("Trying to create a management network with params %+v", r.mgmt)
	// check the network existence first
	b, err := network.Exists(ctx, r.mgmt.Network, &network.ExistsOptions{})
	if err != nil {
		return err
	}
	// Create if the network doesn't exist
	if !b {
		netopts, err := r.netOpts(ctx)
		if err != nil {
			return err
		}
		_, err = network.Create(ctx, &netopts)
	}
	return err
}

// DeleteNet deletes a clab mgmt bridge
func (r *PodmanRuntime) DeleteNet(ctx context.Context) error {
	// Skip if "keep mgmt" is set
	log.Debugf("Method DeleteNet was called with runtime inputs %+v and net settings %+v", r, r.mgmt)
	if r.config.KeepMgmtNet {
		return nil
	}
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	log.Debugf("Trying to delete mgmt network %v", r.mgmt.Network)
	_, err = network.Remove(ctx, r.mgmt.Network, &network.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("Error while trying to remove a mgmt network %w", err)
	}
	return nil
}

func (r *PodmanRuntime) PullImageIfRequired(ctx context.Context, image string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	// Check the existence
	ex, err := images.Exists(ctx, image, &images.ExistsOptions{})
	if err != nil {
		return err
	}
	// Pull the image if it doesn't exist
	if !ex {
		_, err = images.Pull(ctx, image, &images.PullOptions{})
	}
	return err
}

// CreateContainer creates a container based on the given NodeConfig and starts it as well
func (r *PodmanRuntime) CreateContainer(ctx context.Context, cfg *types.NodeConfig) (interface{}, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	cID, err := r.createPodmanContainer(ctx, cfg)
	if err != nil {
		return nil, err
	}
	err = r.StartContainer(ctx, cID)
	if err != nil {
		return nil, fmt.Errorf("error during a container create/start operation: %w", err)
	}

	// Add NSpath to the node config struct
	cfg.NSPath, err = r.GetNSPath(ctx, cID)
	if err != nil {
		return nil, err
	}
	// And setup netns alias. Not really needed with podman
	// But currently (Oct 2021) clab depends on the specific naming scheme of veth aliases.
	err = utils.LinkContainerNS(cfg.NSPath, cfg.LongName)
	if err != nil {
		return nil, err
	}
	// TX checksum disabling will be done here since the mgmt bridge
	// may not exist in netlink before a container is attached to it
	err = r.disableTXOffload(ctx)
	return nil, err
}

func (r *PodmanRuntime) StartContainer(ctx context.Context, cID string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	err = containers.Start(ctx, cID, &containers.StartOptions{})
	return err
}

func (r *PodmanRuntime) StopContainer(ctx context.Context, cID string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	err = containers.Stop(ctx, cID, &containers.StopOptions{})
	return nil
}

// ListContainers returns a list of all available containers in the system in a containerlab-specific struct
func (r *PodmanRuntime) ListContainers(ctx context.Context, filters []*types.GenericFilter) ([]types.GenericContainer, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	listOpts := new(containers.ListOptions).WithAll(true).WithFilters(r.buildFilterString(filters))
	cList, err := containers.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return r.produceGenericContainerList(ctx, cList)
}

func (r *PodmanRuntime) GetNSPath(ctx context.Context, cID string) (string, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return "", err
	}
	inspect, err := containers.Inspect(ctx, cID, &containers.InspectOptions{})
	if err != nil {
		return "", err
	}
	nspath := inspect.NetworkSettings.SandboxKey
	log.Debugf("Method GetNSPath was called with a resulting nspath %q", nspath)
	return nspath, nil
}

func (r *PodmanRuntime) Exec(ctx context.Context, cID string, cmd []string) (stdout []byte, stderr []byte, err error) {
	ctx, err = r.connect(ctx)
	if err != nil {
		return nil, nil, err
	}
	execCreateConf := handlers.ExecCreateConfig{
		ExecConfig: dockerTypes.ExecConfig{
			User:         "root",
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          cmd},
	}
	execID, err := containers.ExecCreate(ctx, cID, &execCreateConf)
	if err != nil {
		log.Errorf("failed to create exec in container %q: %v", cID, err)
		return nil, nil, err
	}
	var sOut, sErr podmanWriterCloser
	var execSAAOpts = new(containers.ExecStartAndAttachOptions).WithOutputStream(&sOut).WithErrorStream(&sErr).WithAttachOutput(true).WithAttachError(true)
	err = containers.ExecStartAndAttach(ctx, execID, execSAAOpts)
	if err != nil {
		log.Errorf("failed to start/attach exec in container %q: %v", cID, err)
		return nil, nil, err
	}
	log.Debugf("Exec attached to the container %q and got stdout %q and stderr %q", cID, sOut.Bytes(), sErr.Bytes())
	return sOut.Bytes(), sErr.Bytes(), nil
}

func (r *PodmanRuntime) ExecNotWait(ctx context.Context, cID string, cmd []string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	execCreateConf := handlers.ExecCreateConfig{
		ExecConfig: dockerTypes.ExecConfig{
			Tty:          false,
			AttachStderr: false,
			AttachStdout: false,
			Cmd:          cmd,
		},
	}
	execID, err := containers.ExecCreate(ctx, cID, &execCreateConf)
	if err != nil {
		log.Errorf("failed to create exec in container %q: %v", cID, err)
		return err
	}
	var execSAAOpts = new(containers.ExecStartAndAttachOptions)
	err = containers.ExecStartAndAttach(ctx, execID, execSAAOpts)
	return nil
}

// DeleteContainer removes a given container from the system (if it exists)
func (r *PodmanRuntime) DeleteContainer(ctx context.Context, contName string) error {
	force := !r.config.GracefulShutdown
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	if !force {
		// Try to stop the containers first in case of graceful shutdown
		err = containers.Stop(ctx, contName, &containers.StopOptions{})
		if err != nil {
			log.Warnf("Unable to stop %q gracefully: %v", contName, err)
		}
	}
	// and do a force removal in the end
	force = true
	err = containers.Remove(ctx, contName, &containers.RemoveOptions{Force: &force})
	return err
}

// Config returns the runtime configuration options
func (r *PodmanRuntime) Config() runtime.RuntimeConfig {
	return *r.config
}

// GetName returns runtime name as a string
func (r *PodmanRuntime) GetName() string {
	return runtimeName
}
