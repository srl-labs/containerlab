//go:build linux && podman
// +build linux,podman

package podman

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/bindings/network"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/srl-labs/containerlab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	RuntimeName    = "podman"
	defaultTimeout = 120 * time.Second
)

type PodmanRuntime struct {
	config *runtime.RuntimeConfig
	mgmt   *types.MgmtNet
}

func init() {
	runtime.Register(RuntimeName, func() runtime.ContainerRuntime {
		return &PodmanRuntime{
			config: &runtime.RuntimeConfig{},
			mgmt:   &types.MgmtNet{},
		}
	})
}

// Init is used to initialize our runtime struct by calling all methods received from the caller
// Invokes methods such as WithConfig, WithMgmtNet etc to populate the fields.
func (r *PodmanRuntime) Init(opts ...runtime.RuntimeOption) error {
	for _, f := range opts {
		f(r)
	}
	r.config.VerifyLinkParams = links.NewVerifyLinkParams()
	r.config.VerifyLinkParams.RunBridgeExistsCheck = false

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

// WithMgmtNet assigns struct mgmt net parameters to the runtime struct.
func (r *PodmanRuntime) WithMgmtNet(net *types.MgmtNet) {
	// Check for nil pointers on input
	if net == nil {
		log.Errorf("Method WithMgmtNet has received a nil pointer")
		return
	}
	log.Debugf("Podman method WithMgmtNet was called with net params: %+v", net)
	r.mgmt = net
}

// WithKeepMgmtNet defines that we shouldn't delete mgmt network(s).
func (r *PodmanRuntime) WithKeepMgmtNet() {
	r.config.KeepMgmtNet = true
}

// CreateNet used to create a new bridge for clab mgmt network.
func (r *PodmanRuntime) CreateNet(ctx context.Context) error {
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
		log.Debugf("Trying to create mgmt network with params: %+v", netopts)
		resp, err := network.Create(ctx, &netopts)
		if err != nil {
			return err
		}
		log.Debugf("Create network response was: %+v", resp)
	}
	// set bridge name = network name if explicit name was not provided
	if r.mgmt.Bridge == "" && r.mgmt.Network != "" {
		details, err := network.Inspect(ctx, r.mgmt.Network, &network.InspectOptions{})
		if err != nil {
			return err
		}
		r.mgmt.Bridge = details.NetworkInterface
	}
	return err
}

// DeleteNet deletes a clab mgmt bridge.
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
	log.Debugf("trying to delete mgmt network %v", r.mgmt.Network)
	_, err = network.Remove(ctx, r.mgmt.Network, &network.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("error while trying to remove a mgmt network %w", err)
	}
	return nil
}

func (r *PodmanRuntime) PullImage(ctx context.Context, image string, pullPolicy types.PullPolicyValue) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	// avoid short-hand image names
	// https://www.redhat.com/sysadmin/container-image-short-names
	canonicalImage := utils.GetCanonicalImageName(image)

	// Check the existence
	ex, err := images.Exists(ctx, canonicalImage, &images.ExistsOptions{})
	if err != nil {
		return err
	}

	if pullPolicy == types.PullPolicyNever {
		if ex {
			// image present, all good
			log.Debugf("Image %s present, skip pulling", image)
			return nil
		} else {
			// image not found but pull policy = never
			return fmt.Errorf("image %s not found locally, but image-pull-policy is %s", image, pullPolicy)
		}
	}
	if pullPolicy == types.PullPolicyIfNotPresent && ex == true {
		// pull policy == IfNotPresent and image is present
		log.Debugf("Image %s present, skip pulling", image)
		return nil
	}

	// Pull the image if it doesn't exist
	if !ex || pullPolicy == types.PullPolicyAlways {
		_, err = images.Pull(ctx, canonicalImage, &images.PullOptions{})
	}
	return err
}

// CreateContainer creates a container, but does not start it.
func (r *PodmanRuntime) CreateContainer(ctx context.Context, cfg *types.NodeConfig) (string, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return "", err
	}
	sg, err := r.createContainerSpec(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("error while trying to create a container spec for node %q: %w", cfg.LongName, err)
	}
	res, err := containers.CreateWithSpec(ctx, &sg, &containers.CreateOptions{})
	log.Debugf("Created a container with ID %v, warnings %v and error %v", res.ID, res.Warnings, err)
	return res.ID, err
}

// StartContainer starts a previously created container by ID or its name and executes post-start actions method.
func (r *PodmanRuntime) StartContainer(ctx context.Context, cID string, node runtime.Node) (interface{}, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	cfg := node.Config()

	err = containers.Start(ctx, cID, &containers.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("error while starting a container %q: %w", cfg.LongName, err)
	}
	err = r.postStartActions(ctx, cID, cfg)
	if err != nil {
		return nil, fmt.Errorf("error while starting a container %q: %w", cfg.LongName, err)
	}
	return nil, nil
}

func (r *PodmanRuntime) PauseContainer(ctx context.Context, cID string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	return containers.Pause(ctx, cID, &containers.PauseOptions{})
}

func (r *PodmanRuntime) UnpauseContainer(ctx context.Context, cID string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	return containers.Unpause(ctx, cID, &containers.UnpauseOptions{})
}

func (r *PodmanRuntime) StopContainer(ctx context.Context, cID string) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	err = containers.Stop(ctx, cID, &containers.StopOptions{})
	if err != nil {
		return err
	}
	return nil
}

// ListContainers returns a list of all available containers in the system in a containerlab-specific struct.
func (r *PodmanRuntime) ListContainers(ctx context.Context, filters []*types.GenericFilter) ([]runtime.GenericContainer, error) {
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

func (r *PodmanRuntime) Exec(ctx context.Context, cID string, execCmd *exec.ExecCmd) (*exec.ExecResult, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	execCreateConf := handlers.ExecCreateConfig{
		ExecConfig: dockerTypes.ExecConfig{
			User:         "root",
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          execCmd.GetCmd(),
		},
	}
	execID, err := containers.ExecCreate(ctx, cID, &execCreateConf)
	if err != nil {
		log.Errorf("failed to create exec in container %q: %v", cID, err)
		return nil, err
	}
	var sOut, sErr podmanWriterCloser
	execSAAOpts := new(containers.ExecStartAndAttachOptions).WithOutputStream(&sOut).WithErrorStream(
		&sErr).WithAttachOutput(true).WithAttachError(true)

	err = containers.ExecStartAndAttach(ctx, execID, execSAAOpts)
	if err != nil {
		log.Errorf("failed to start/attach exec in container %q: %v", cID, err)
		return nil, err
	}
	// perform inspection to retrieve the exitcode
	inspectOut, err := containers.ExecInspect(ctx, execID, nil)
	if err != nil {
		return nil, err
	}
	log.Debugf("Exec attached to the container %q and got stdout %q and stderr %q", cID, sOut.Bytes(), sErr.Bytes())

	// fill the execution result
	execResult := exec.NewExecResult(execCmd)
	execResult.SetStdErr(sErr.Bytes())
	execResult.SetStdOut(sOut.Bytes())
	execResult.SetReturnCode(inspectOut.ExitCode)

	return execResult, nil
}

func (r *PodmanRuntime) ExecNotWait(ctx context.Context, cID string, exec *exec.ExecCmd) error {
	ctx, err := r.connect(ctx)
	if err != nil {
		return err
	}
	execCreateConf := handlers.ExecCreateConfig{
		ExecConfig: dockerTypes.ExecConfig{
			Tty:          false,
			AttachStderr: false,
			AttachStdout: false,
			Cmd:          exec.GetCmd(),
		},
	}
	execID, err := containers.ExecCreate(ctx, cID, &execCreateConf)
	if err != nil {
		log.Errorf("failed to create exec in container %q: %v", cID, err)
		return err
	}
	execSAAOpts := new(containers.ExecStartAndAttachOptions)
	err = containers.ExecStartAndAttach(ctx, execID, execSAAOpts)
	return err
}

// DeleteContainer removes a given container from the system (if it exists).
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
	depend := true
	_, err = containers.Remove(ctx, contName, &containers.RemoveOptions{Force: &force, Depend: &depend})
	return err
}

// Config returns the runtime configuration options.
func (r *PodmanRuntime) Config() runtime.RuntimeConfig {
	return *r.config
}

// GetName returns runtime name as a string.
func (r *PodmanRuntime) GetName() string {
	return RuntimeName
}

// GetHostsPath returns fs path to a file which is mounted as /etc/hosts into a given container.
func (r *PodmanRuntime) GetHostsPath(ctx context.Context, cID string) (string, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return "", err
	}
	inspect, err := containers.Inspect(ctx, cID, &containers.InspectOptions{})
	if err != nil {
		return "", err
	}
	hostsPath := inspect.HostsPath
	log.Debugf("Method GetHostsPath was called with a resulting path %q", hostsPath)
	return hostsPath, nil
}

// GetContainerStatus retrieves the ContainerStatus of the named container.
func (r *PodmanRuntime) GetContainerStatus(ctx context.Context, cID string) runtime.ContainerStatus {
	ctx, err := r.connect(ctx)
	if err != nil {
		return runtime.NotFound
	}
	icd, err := containers.Inspect(ctx, cID, nil)
	if err != nil {
		return runtime.NotFound
	}
	if icd.State.Running {
		return runtime.Running
	}
	return runtime.Stopped
}

// IsHealthy returns true is the container is reported as being healthy, false otherwise.
func (r *PodmanRuntime) IsHealthy(ctx context.Context, cID string) (bool, error) {
	ctx, err := r.connect(ctx)
	if err != nil {
		return false, err
	}
	icd, err := containers.Inspect(ctx, cID, nil)
	if err != nil {
		return false, err
	}
	return icd.State.Health.Status == "healthy", nil
}

func (*PodmanRuntime) WriteToStdinNoWait(ctx context.Context, cID string, data []byte) error {
	log.Infof("WriteToStdinNoWait is not yet implemented for Podman runtime")
	return nil
}

func (r *PodmanRuntime) CheckConnection(ctx context.Context) error {
	_, err := r.connect(ctx)
	if err != nil {
		return fmt.Errorf("could not connect to Podman runtime: %w", err)
	}

	return nil
}

func (r *PodmanRuntime) GetRuntimeSocket() (string, error) {
	socket := "/run/podman/podman.sock"

	// For rootless podman, check if XDG_RUNTIME_DIR is set
	if os.Getenv("XDG_RUNTIME_DIR") != "" {
		userID := os.Getenv("UID")
		if userID == "" {
			userID = strconv.Itoa(os.Getuid())
		}
		nonRootSocket := fmt.Sprintf("/run/user/%s/podman/podman.sock", userID)
		if _, err := os.Stat(nonRootSocket); err == nil {
			socket = nonRootSocket
		}
	}
	return socket, nil
}

func (*PodmanRuntime) GetRuntimeBinary() (string, error) {
	runtimePath, err := exec.LookPath("podman")
	if err != nil {
		return "", fmt.Errorf("failed to get podman runtime binary path: %w", err)
	}
	return runtimePath, nil
}
