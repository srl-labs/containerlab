package ignite

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	api "github.com/weaveworks/ignite/pkg/apis/ignite"
	meta "github.com/weaveworks/ignite/pkg/apis/meta/v1alpha1"
	igniteConstants "github.com/weaveworks/ignite/pkg/constants"
	"github.com/weaveworks/ignite/pkg/dmlegacy"
	"github.com/weaveworks/ignite/pkg/filter"
	"github.com/weaveworks/ignite/pkg/metadata"
	igniteNetwork "github.com/weaveworks/ignite/pkg/network"
	"github.com/weaveworks/ignite/pkg/operations"
	"github.com/weaveworks/ignite/pkg/providers"
	igniteDocker "github.com/weaveworks/ignite/pkg/providers/docker"
	"github.com/weaveworks/ignite/pkg/providers/ignite"
	igniteRuntimes "github.com/weaveworks/ignite/pkg/runtime"
	"github.com/weaveworks/ignite/pkg/util"
)

const (
	RuntimeName                   = "ignite"
	defaultContainerRuntime       = igniteRuntimes.RuntimeDocker
	defaultTimeout                = 30 * time.Second
	kvmPath                       = "/dev/kvm"
	defaultContainerNetworkPlugin = igniteNetwork.PluginDockerBridge
	udevRuleTemplate              = "SUBSYSTEM==\"net\", ACTION==\"add\", DRIVERS==\"?*\", ATTR{address}==\"%s\", ATTR{type}==\"1\", KERNEL==\"eth*\", NAME=\"%s\""
	udevRulesPath                 = "/etc/udev/rules.d/70-persistent-net.rules"
	hostnamePath                  = "/etc/hostname"
)

var runtimePaths = []string{
	"/var/lib/firecracker/vm",
	"/var/lib/firecracker/image",
	"/var/lib/firecracker/kernel",
}

type IgniteRuntime struct {
	config     runtime.RuntimeConfig
	baseVM     *api.VM
	mgmt       *types.MgmtNet
	ctrRuntime runtime.ContainerRuntime
}

func init() {
	runtime.Register(RuntimeName, func() runtime.ContainerRuntime {
		return &IgniteRuntime{
			mgmt: &types.MgmtNet{},
		}
	})
}

func (c *IgniteRuntime) Init(opts ...runtime.RuntimeOption) error {
	// check that /dev/kvm exists
	if _, err := os.Stat(kvmPath); err != nil {
		return fmt.Errorf("cannot find %q: %s", kvmPath, err)
	}

	// ensure firecracker directroy
	for _, path := range runtimePaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(path, os.ModeDir); err != nil {
				return fmt.Errorf("cannot create the required directory %q: %s", path, err)
			}
		}
	}

	// init igniteC
	util.GenericCheckErr(providers.Populate(ignite.Preload))

	// init providers
	igniteDocker.SetDockerRuntime()
	igniteDocker.SetDockerNetwork()

	providers.RuntimeName = defaultContainerRuntime
	providers.NetworkPluginName = defaultContainerNetworkPlugin
	providers.Populate(ignite.Providers)

	// build VM skeleton
	vm := providers.Client.VMs().New()

	// force runtime and network plugin to docker/CNM
	vm.Status.Runtime.Name = defaultContainerRuntime
	vm.Status.Network.Plugin = defaultContainerNetworkPlugin

	c.baseVM = vm

	rInit, ok := runtime.ContainerRuntimes[defaultContainerRuntime.String()]
	if !ok {
		return fmt.Errorf("failed to initialize %q runtime", defaultContainerRuntime)
	}
	c.ctrRuntime = rInit()

	err := c.ctrRuntime.Init(opts...)
	if err != nil {
		return err
	}

	return nil
}

func (c *IgniteRuntime) Mgmt() *types.MgmtNet { return c.mgmt }

func (*IgniteRuntime) GetName() string                 { return RuntimeName }
func (c *IgniteRuntime) Config() runtime.RuntimeConfig { return c.config }

func (c *IgniteRuntime) WithConfig(cfg *runtime.RuntimeConfig) {
	c.config.Timeout = cfg.Timeout
	c.config.Debug = cfg.Debug
	c.config.GracefulShutdown = cfg.GracefulShutdown
	if c.config.Timeout <= 0 {
		c.config.Timeout = defaultTimeout
	}
}

func (c *IgniteRuntime) WithKeepMgmtNet() {
	c.ctrRuntime.WithKeepMgmtNet()
}

func (c *IgniteRuntime) WithMgmtNet(n *types.MgmtNet) {
	c.mgmt = n
}

func (c *IgniteRuntime) CreateNet(ctx context.Context) error {
	return c.ctrRuntime.CreateNet(ctx)
}

func (c *IgniteRuntime) DeleteNet(ctx context.Context) error {
	return c.ctrRuntime.DeleteNet(ctx)
}

// PullImage pulls the provided image name if it does not exist.
// Ignite does ignore the pullPolicy though.
func (*IgniteRuntime) PullImage(_ context.Context, imageName string, _ types.PullPolicyValue) error {
	ociRef, err := meta.NewOCIImageRef(imageName)
	if err != nil {
		return fmt.Errorf("failed to parse OCI image ref %q: %s", imageName, err)
	}
	_, err = operations.FindOrImportImage(providers.Client, ociRef)
	if err != nil {
		return fmt.Errorf("failed to find OCI image ref %q: %s", ociRef, err)
	}

	return nil
}

func (c *IgniteRuntime) StartContainer(ctx context.Context, _ string, node runtime.Node) (interface{}, error) {
	vm := c.baseVM.DeepCopy()

	nodecfg := node.Config()

	// updating the node RAM if it's set
	if nodecfg.Memory != "" {
		ram, err := meta.NewSizeFromString(nodecfg.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as memory value: %s", nodecfg.Memory, err)
		}
		vm.Spec.Memory = ram
	}

	ociRef, err := meta.NewOCIImageRef(nodecfg.Sandbox)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI image ref %q: %s", nodecfg.Sandbox, err)
	}
	vm.Spec.Sandbox.OCI = ociRef

	ociRef, err = meta.NewOCIImageRef(nodecfg.Kernel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI image ref %q: %s", nodecfg.Kernel, err)
	}
	c.baseVM.Spec.Kernel.OCI = ociRef
	k, err := operations.FindOrImportKernel(providers.Client, ociRef)
	if err != nil {
		log.Error(err)
	}
	vm.SetKernel(k)

	ociRef, err = meta.NewOCIImageRef(nodecfg.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI image ref %q: %s", nodecfg.Image, err)
	}
	img, err := operations.FindOrImportImage(providers.Client, ociRef)
	if err != nil {
		return nil, fmt.Errorf("failed to find OCI image ref %q: %s", ociRef, err)
	}
	vm.SetImage(img)

	vm.Name = nodecfg.LongName
	vm.Labels = nodecfg.Labels
	metadata.SetNameAndUID(vm, providers.Client)

	var copyFiles []api.FileMapping
	for _, bind := range nodecfg.Binds {
		parts := strings.Split(bind, ":")
		if len(parts) < 2 {
			continue
		}
		copyFiles = append(copyFiles, api.FileMapping{
			HostPath: parts[0],
			VMPath:   parts[1],
		})
	}

	// Create udev rules to rename interfaces
	var extraIntfs []string
	var udevRules []string
	for _, ep := range node.GetEndpoints() {
		extraIntfs = append(extraIntfs, ep.GetIfaceName())
		udevRules = append(udevRules, fmt.Sprintf(udevRuleTemplate, ep.GetMac(), ep.GetIfaceName()))
	}

	udevFile, err := os.CreateTemp("/tmp", fmt.Sprintf("%s-udev", vm.Name))
	if err != nil {
		return nil, err
	}
	defer os.Remove(udevFile.Name())

	if _, err := udevFile.Write([]byte(strings.Join(udevRules, "\n") + "\n")); err != nil {
		return nil, err
	}
	if err := udevFile.Close(); err != nil {
		return nil, err
	}

	copyFiles = append(copyFiles, api.FileMapping{
		HostPath: udevFile.Name(),
		VMPath:   udevRulesPath,
	})

	vm.Spec.CopyFiles = copyFiles

	// Setting up env variables
	fcReqKey := igniteConstants.IGNITE_SANDBOX_ENV_VAR +
		"FIRECRACKER_GO_SDK_REQUEST_TIMEOUT_MILLISECONDS"
	fcInitKey := igniteConstants.IGNITE_SANDBOX_ENV_VAR +
		"FIRECRACKER_GO_SDK_INIT_TIMEOUT_SECONDS"
	vm.SetAnnotation(fcReqKey, "1000")
	vm.SetAnnotation(fcInitKey, "1")

	// Instructing ignite to connect extra interfaces as tc redirect
	for _, extraIntf := range extraIntfs {
		vm.SetAnnotation(igniteConstants.IGNITE_INTERFACE_ANNOTATION+extraIntf, "tc-redirect")
	}

	providers.Client.VMs().Set(vm)

	err = dmlegacy.AllocateAndPopulateOverlay(vm)
	if err != nil {
		log.Printf("Error AllocateAndPopulateOverlay: %s", err)
		return nil, err
	}

	vmChans, err := operations.StartVMNonBlocking(vm, c.config.Debug)
	if err != nil {
		return nil, err
	}

	nspath, err := c.GetNSPath(ctx, vm.PrefixedID())
	if err != nil {
		return nil, err
	}
	return vmChans, utils.LinkContainerNS(nspath, nodecfg.LongName)
}

func (*IgniteRuntime) CreateContainer(_ context.Context, node *types.NodeConfig) (string, error) {
	// this is a no-op
	return node.LongName, nil
}

func (*IgniteRuntime) PauseContainer(_ context.Context, cID string) error {
	pid, err := utils.ContainerNSToPID(cID)
	if err != nil {
		return err
	}

	return utils.PauseProcessGroup(pid)
}

func (*IgniteRuntime) UnpauseContainer(_ context.Context, cID string) error {
	pid, err := utils.ContainerNSToPID(cID)
	if err != nil {
		return err
	}

	return utils.UnpauseProcessGroup(pid)
}

func (*IgniteRuntime) StopContainer(_ context.Context, _ string) error {
	// this is a no-op, only used by ceos at this stage
	return nil
}

func (c *IgniteRuntime) ListContainers(_ context.Context, gfilters []*types.GenericFilter) ([]runtime.GenericContainer, error) {
	var result []runtime.GenericContainer

	var metaFilters []string
	for _, gf := range gfilters {
		if gf.FilterType == "label" && gf.Operator == "=" {
			metaFilters = append(metaFilters, fmt.Sprintf(
				"{{.ObjectMeta.Labels.%s}}=%s", gf.Field, gf.Match))
		} else if gf.FilterType == "name" {
			metaFilters = append(metaFilters, fmt.Sprintf("{{.ObjectMeta.Name}}=%s", gf.Match))
		}
	}

	allVMs, err := providers.Client.VMs().FindAll(filter.NewVMFilterAll("", true))
	if err != nil {
		return result, fmt.Errorf("failed to list all VMs: %s", err)
	}

	if len(metaFilters) < 1 {
		return c.produceGenericContainerList(allVMs)
	}

	metaFilter := strings.Join(metaFilters, ",")
	filters, err := filter.GenerateMultipleMetadataFiltering(metaFilter)
	if err != nil {
		return result, fmt.Errorf("failed to generate filters: %s", err)
	}

	var filteredVMs []*api.VM
	for _, vm := range allVMs {
		isExpected, err := filters.AreExpected(vm)
		if err != nil {
			continue
		}
		if isExpected {
			filteredVMs = append(filteredVMs, vm)
		}
	}

	return c.produceGenericContainerList(filteredVMs)
}

func (c *IgniteRuntime) GetContainer(_ context.Context, containerID string) (*runtime.GenericContainer, error) {
	var result *runtime.GenericContainer
	vm, err := providers.Client.VMs().Find(filter.NewVMFilter(containerID))
	if err != nil {
		return result, err
	}

	genericCtrs, err := c.produceGenericContainerList([]*api.VM{vm})
	if err != nil {
		return result, err
	}
	if len(genericCtrs) != 1 {
		return result, fmt.Errorf("unexpected number of matched containers %d", len(genericCtrs))
	}

	return &genericCtrs[0], nil
}

// Transform docker-specific to generic container format.
func (ir *IgniteRuntime) produceGenericContainerList(input []*api.VM) ([]runtime.GenericContainer, error) {
	var result []runtime.GenericContainer

	for _, i := range input {
		ctr := runtime.GenericContainer{
			Names:           []string{i.Name},
			ID:              i.GetUID().String(),
			ShortID:         i.PrefixedID(),
			Labels:          i.Labels,
			Image:           i.Spec.Image.OCI.Normalized(),
			NetworkSettings: runtime.GenericMgmtIPs{},
		}
		ctr.SetRuntime(ir)

		if i.Status.Runtime != nil && i.Status.Runtime.ID != "" && len(i.Status.Runtime.ID) > 12 {
			ctr.ShortID = i.Status.Runtime.ID[:12]
		}

		if i.Status.Running {
			ctr.State = "running"
		}

		for _, addr := range i.Status.Network.IPAddresses {
			ctr.NetworkSettings.IPv4addr = addr.String()
			// TODO: figure out what to do with this
			ctr.NetworkSettings.IPv4pLen = 24

			break
		}

		result = append(result, ctr)
	}

	return result, nil
}

func (c *IgniteRuntime) GetNSPath(ctx context.Context, ctrId string) (string, error) {
	result, err := c.ctrRuntime.GetNSPath(ctx, ctrId)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (*IgniteRuntime) Exec(_ context.Context, _ string, _ *exec.ExecCmd) (*exec.ExecResult, error) {
	log.Infof("Exec is not yet implemented for Ignite runtime")
	return nil, nil
}

func (*IgniteRuntime) ExecNotWait(_ context.Context, _ string, _ *exec.ExecCmd) error {
	log.Infof("ExecNotWait is not yet implemented for Ignite runtime")
	return nil
}

func (c *IgniteRuntime) DeleteContainer(ctx context.Context, containerID string) error {
	vm, err := providers.Client.VMs().Find(filter.NewVMFilter(containerID))
	if err != nil {
		return err
	}

	err = operations.DeleteVM(providers.Client, vm)
	if err != nil {
		// Failed ignite VMs may not be able to delete the underlying containers
		// due to device-mapper being busy (container may be running but VM is not)
		// In order to work around that, we delete the runtime containers first
		// this will clean up any device-mapper files and ensure DeleteVM succeeds
		filter := []*types.GenericFilter{
			{FilterType: "label", Field: "ignite.name", Operator: "=", Match: containerID},
		}
		runtimeCtrs, err := c.ctrRuntime.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list runtime containers: %s", err)
		}
		if len(runtimeCtrs) == 1 {
			runtimeCtr := runtimeCtrs[0]
			if err := c.ctrRuntime.DeleteContainer(ctx, runtimeCtr.ID); err != nil {
				return fmt.Errorf("failed to delete runtime container: %s", err)
			}
		}
		return operations.DeleteVM(providers.Client, vm)
	}

	return nil
}

// GetHostsPath returns fs path to a file which is mounted as /etc/hosts into a given container
// no-op for ignite.
func (*IgniteRuntime) GetHostsPath(context.Context, string) (string, error) {
	return "", nil
}

// GetContainerStatus retrieves the ContainerStatus of the named container.
func (*IgniteRuntime) GetContainerStatus(_ context.Context, containerID string) runtime.ContainerStatus {
	vm, err := providers.Client.VMs().Find(filter.NewVMFilter(containerID))
	if err != nil {
		return runtime.NotFound
	}
	if vm.Status.Running {
		return runtime.Running
	}
	return runtime.Stopped
}

// IsHealthy returns true is the container is reported as being healthy, false otherwise.
func (c *IgniteRuntime) IsHealthy(_ context.Context, _ string) (bool, error) {
	log.Errorf("function GetContainerHealth(...) not implemented in the Containerlab IgniteRuntime")
	return true, nil
}
