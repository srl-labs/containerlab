package containerd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/docker/go-units"
	"github.com/dustin/go-humanize"
	"github.com/google/shlex"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	containerdNamespace = "clab"
	cniCache            = "/opt/cni/cache"
	runtimeName         = "containerd"
	defaultTimeout      = 30 * time.Second
)

func init() {
	runtime.Register(runtimeName, func() runtime.ContainerRuntime {
		return &ContainerdRuntime{
			Mgmt: new(types.MgmtNet)}
	})
}

func (c *ContainerdRuntime) Init(opts ...runtime.RuntimeOption) error {
	var err error
	log.Debug("Runtime: containerd")
	c.client, err = containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return err
	}
	cniPath := utils.GetCNIBinaryPath()
	binaries := []string{"tuning", "bridge", "host-local"}
	for _, binary := range binaries {
		binary = filepath.Join(cniPath, binary)
		if _, err := os.Stat(binary); err != nil {
			return errors.WithMessagef(err, "CNI binaries not found. [ %s ] are required.", strings.Join(binaries, ","))
		}
	}
	for _, o := range opts {
		o(c)
	}
	return nil
}

type ContainerdRuntime struct {
	config runtime.RuntimeConfig
	client *containerd.Client
	Mgmt   *types.MgmtNet
}

func (c *ContainerdRuntime) WithConfig(cfg *runtime.RuntimeConfig) {
	c.config.Timeout = cfg.Timeout
	c.config.Debug = cfg.Debug
	c.config.GracefulShutdown = cfg.GracefulShutdown
	if c.config.Timeout <= 0 {
		c.config.Timeout = defaultTimeout
	}
}

func (c *ContainerdRuntime) WithMgmtNet(n *types.MgmtNet) {
	if n.Bridge == "" {
		netname := "clab"
		if n.Network != "" {
			netname = n.Network
		}
		n.Bridge = "br-" + netname
	}
	c.Mgmt = n
}

func (c *ContainerdRuntime) WithKeepMgmtNet() {
	c.config.KeepMgmtNet = true
}
func (*ContainerdRuntime) GetName() string                 { return runtimeName }
func (c *ContainerdRuntime) Config() runtime.RuntimeConfig { return c.config }

func (*ContainerdRuntime) CreateNet(_ context.Context) error {
	log.Debug("CreateNet() - Not needed with containerd")
	return nil
}
func (c *ContainerdRuntime) DeleteNet(context.Context) error {
	var err error
	bridgename := c.Mgmt.Bridge
	brInUse := true
	for i := 0; i < 10; i++ {
		brInUse, err = utils.CheckBrInUse(bridgename)
		if err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 100)
		if !brInUse {
			// Stop early if bridge no longer in use
			// Need to wait some time, since the earlier veth deletion
			// triggert from the cotnainer deletion is async and needs
			// to finish. W'll have a race condition otherwise.
			break
		}
	}
	if c.config.KeepMgmtNet || brInUse {
		log.Infof("Skipping deletion of bridge '%s'", bridgename)
		return nil
	}
	return utils.DeleteLinkByName(bridgename)
}

func (c *ContainerdRuntime) PullImageIfRequired(ctx context.Context, imagename string) error {
	log.Debugf("Looking up %s container image", imagename)
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	if !strings.Contains(imagename, ":") {
		imagename = imagename + ":latest"
	}
	_, err := c.client.GetImage(ctx, imagename)
	if err == nil {
		log.Debugf("Image %s present, skip pulling", imagename)
		return nil
	}
	n := utils.GetCanonicalImageName(imagename)
	_, err = c.client.Pull(ctx, n, containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerdRuntime) CreateContainer(ctx context.Context, node *types.NodeConfig) (interface{}, error) {
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)

	var img containerd.Image
	if !strings.Contains(node.Image, ":") {
		node.Image = node.Image + ":latest"
	}
	img, err := c.client.GetImage(ctx, node.Image)
	if err != nil {
		// try fetching the image with canonical name
		// as it might be that we pulled this image with canonical name
		img, err = c.client.GetImage(ctx, utils.GetCanonicalImageName(node.Image))
		if err != nil {
			return nil, err
		}
	}

	cmd, err := shlex.Split(node.Cmd)
	if err != nil {
		return nil, err
	}

	mounts := make([]specs.Mount, len(node.Binds))

	for idx, mount := range node.Binds {
		s := strings.Split(mount, ":")

		m := specs.Mount{
			Source:      s[0],
			Destination: s[1],
			Options:     []string{"rbind", "rprivate"},
		}
		if len(mount) == 3 {
			m.Options = append(m.Options, strings.Split(s[2], ",")...)
		}
		mounts[idx] = m
	}

	opts := []oci.SpecOpts{
		oci.WithImageConfig(img),
		oci.WithEnv(utils.ConvertEnvs(node.Env)),
		oci.WithHostname(node.ShortName),
		WithSysctls(node.Sysctls),
		oci.WithoutRunMount,
		oci.WithPrivileged,
		oci.WithHostLocaltime,
		oci.WithNamespacedCgroup(),
		oci.WithAllDevicesAllowed,
		oci.WithDefaultUnixDevices,
		oci.WithNewPrivileges,
	}
	if len(cmd) > 0 {
		opts = append(opts, oci.WithProcessArgs(cmd...))
	}
	if node.User != "" {
		opts = append(opts, oci.WithUser(node.User))
	}
	if node.Memory != "" {
		mem, err := humanize.ParseBytes(node.Memory)
		if err != nil {
			return nil, err
		}
		opts = append(opts, oci.WithMemoryLimit(mem))
	}
	if node.CPU != 0 {
		opts = append(opts, oci.WithCPUCFS(int64(node.CPU*100000), 100000))
	}
	if node.CPUSet != "" {
		opts = append(opts, oci.WithCPUs(node.CPUSet))
	}
	if len(mounts) > 0 {
		opts = append(opts, oci.WithMounts(mounts))
	}

	var cnic *libcni.CNIConfig
	var cncl *libcni.NetworkConfigList
	var cnirc *libcni.RuntimeConf

	switch node.NetworkMode {
	case "host":
		opts = append(opts,
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithHostHostsFile,
			oci.WithHostResolvconf)
	case "none":
		// Done!
	default:
		cnic, cncl, cnirc, err = cniInit(node.LongName, "eth0", c.Mgmt)
		if err != nil {
			return nil, err
		}

		// set mac if defined in node
		if node.MacAddress != "" {
			cnirc.CapabilityArgs["mac"] = node.MacAddress
		}

		portmappings := []portMapping{}

		for contdatasl, hostdata := range node.PortBindings {
			// fmt.Printf("%+v", hostdata)
			// fmt.Printf("%+v", contdatasl)
			for _, x := range hostdata {
				hostport, err := strconv.Atoi(x.HostPort)
				if err != nil {
					return nil, err
				}
				portmappings = append(portmappings, portMapping{HostPort: hostport, ContainerPort: contdatasl.Int(), Protocol: contdatasl.Proto()})
			}
		}
		if len(portmappings) > 0 {
			cnirc.CapabilityArgs["portMappings"] = portmappings
		}
	}

	var cOpts []containerd.NewContainerOpts
	cOpts = append(cOpts,
		containerd.WithImage(img),
		containerd.WithNewSnapshot(node.LongName+"-snapshot", img),
		containerd.WithAdditionalContainerLabels(node.Labels),
		containerd.WithNewSpec(opts...),
	)

	newContainer, err := c.client.NewContainer(
		ctx,
		node.LongName,
		cOpts...,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf("Container '%s' created", node.LongName)
	log.Debugf("Start container: %s", node.LongName)

	err = c.StartContainer(ctx, node.LongName)
	if err != nil {
		return nil, err
	}

	log.Debugf("Container started: %s", node.LongName)

	node.NSPath, err = c.GetNSPath(ctx, node.LongName)
	if err != nil {
		return nil, err
	}

	err = utils.LinkContainerNS(node.NSPath, node.LongName)
	if err != nil {
		return nil, err
	}

	// if this is not a host network namespace container
	// we have prepared a lot of stuff further up, which
	// is now to be applied
	if cnic != nil {
		cnirc.NetNS = node.NSPath
		res, err := cnic.AddNetworkList(ctx, cncl, cnirc)
		if err != nil {
			return nil, err
		}
		result, _ := current.NewResultFromResult(res)

		ipv4, ipv6 := "", ""
		ipv4nm, ipv6nm := 0, 0
		for _, ip := range result.IPs {
			switch ip.Version {
			case "4":
				ipv4 = ip.Address.IP.String()
				ipv4nm, _ = ip.Address.Mask.Size()
			case "6":
				ipv6 = ip.Address.IP.String()
				ipv6nm, _ = ip.Address.Mask.Size()
			}
		}

		additionalLabels := map[string]string{
			"clab.ipv4.addr":    ipv4,
			"clab.ipv4.netmask": strconv.Itoa(ipv4nm),
			"clab.ipv6.addr":    ipv6,
			"clab.ipv6.netmask": strconv.Itoa(ipv6nm),
		}
		_, err = newContainer.SetLabels(ctx, additionalLabels)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func cniInit(cId, ifName string, mgmtNet *types.MgmtNet) (*libcni.CNIConfig, *libcni.NetworkConfigList, *libcni.RuntimeConf, error) {
	// allow overwriting cni plugin binary path via ENV var

	cnic := libcni.NewCNIConfigWithCacheDir([]string{utils.GetCNIBinaryPath()}, cniCache, nil)

	cniConfig := fmt.Sprintf(`
	{
		"cniVersion": "0.4.0",
		"name": "clabmgmt",
		"plugins": [
		  {
			"type": "bridge",
			"bridge": "%s",
			"isDefaultGateway": true,
			"forceAddress": false,
			"ipMasq": true,
			"hairpinMode": true,
			"ipam": {
			  "type": "host-local",
			  "ranges": [
				[
				  {
					"subnet": "%s"
				  }
				],
				[
				  {
					"subnet": "%s"
				  }
				]
			  ]
			}
		  },
		  {
			"type": "tuning",
			"mtu": %s,
			"capabilities": {
			  "mac": true
			}
		  },
		  {
			"type": "portmap",
			"capabilities": {
			  "portMappings": true
			}
		  }
		]
	  }
	`, mgmtNet.Bridge, mgmtNet.IPv4Subnet, mgmtNet.IPv6Subnet, mgmtNet.MTU)

	cncl, err := libcni.ConfListFromBytes([]byte(cniConfig))
	if err != nil {
		return nil, nil, nil, err
	}

	cnirc := &libcni.RuntimeConf{
		ContainerID: cId,
		IfName:      ifName,
		// // NetNS must be set later, can just be determined after container start
		// NetNS:          node.NSPath,
		CapabilityArgs: make(map[string]interface{}),
	}
	return cnic, cncl, cnirc, nil
}

type portMapping struct {
	HostPort      int    `json:"hostPort"`
	HostIP        string `json:"hostIP,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
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
	task, err := container.NewTask(ctx, cio.LogFile("/tmp/clab/"+containername+".log"))
	if err != nil {
		return err
	}
	err = task.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}
func (c *ContainerdRuntime) StopContainer(ctx context.Context, containername string) error {
	ctask, err := c.getContainerTask(ctx, containername)
	if err != nil {
		log.Debugf("container %s: %v", containername, err)
		return nil
	}
	taskstatus, err := ctask.Status(ctx)
	if err != nil {
		return err
	}

	paused := false
	needsStop := true
	switch taskstatus.Status {
	case containerd.Created, containerd.Stopped:
		needsStop = false
	case containerd.Paused, containerd.Pausing:
		paused = true
	default:
	}

	if needsStop {
		// NOTE: ctx is main context so that it's ok to use for task.Wait().
		exitCh, err := ctask.Wait(ctx)
		if err != nil {
			return err
		}

		// signal will be sent once resume is finished
		if paused {
			if err := ctask.Resume(ctx); err != nil {
				log.Warnf("Cannot unpause container %s: %s", containername, err)
			}
		}

		err = ctask.Kill(ctx, syscall.SIGKILL)
		if err != nil {
			return err
		}

		err = waitContainerStop(ctx, exitCh)
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

func waitContainerStop(ctx context.Context, exitCh <-chan containerd.ExitStatus) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case status := <-exitCh:
		return status.Error()
	}
}

func (c *ContainerdRuntime) getContainerTask(ctx context.Context, containername string) (containerd.Task, error) {
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	cont, err := c.client.LoadContainer(ctx, containername)
	if err != nil {
		return nil, err
	}
	return cont.Task(ctx, nil)
}

func (c *ContainerdRuntime) ListContainers(ctx context.Context, filter []*types.GenericFilter) ([]types.GenericContainer, error) {
	log.Debug("listing containers")
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)

	filterstring := c.buildFilterString(filter)
	containerlist, err := c.client.Containers(ctx, filterstring)
	if err != nil {
		return nil, err
	}

	return c.produceGenericContainerList(ctx, containerlist)
}

// TODO this will probably not work. need to work out the exact filter format
func (c *ContainerdRuntime) GetContainer(ctx context.Context, containerID string) (*types.GenericContainer, error) {
	var ctr *types.GenericContainer
	gFilter := types.GenericFilter{
		FilterType: "name",
		Field:      "",
		Operator:   "",
		Match:      containerID,
	}
	ctrs, err := c.ListContainers(ctx, []*types.GenericFilter{&gFilter})
	if err != nil {
		return ctr, err
	}
	if len(ctrs) != 1 {
		return ctr, fmt.Errorf("found unexpected number of containers: %d", len(ctrs))
	}
	return &ctrs[0], nil
}

func (*ContainerdRuntime) buildFilterString(filter []*types.GenericFilter) string {
	filterstring := ""
	delim := ","
	for counter, filterEntry := range filter {
		isExistsOperator := false

		operator := filterEntry.Operator
		switch filterEntry.Operator {
		case "=":
			operator = "=="
		case "exists":
			operator = ""
			isExistsOperator = true
		}

		if counter+1 == len(filter) {
			delim = ""
		}

		if filterEntry.FilterType == "label" {
			filterstring = filterstring + "labels.\"" + filterEntry.Field + "\""
			if !isExistsOperator {
				filterstring = filterstring + operator + "\"" + filterEntry.Match + "\"" + delim
			}

		} // more might be implemented later
	}
	log.Debug("Filterstring: " + filterstring)
	return filterstring
}

// Transform docker-specific to generic container format
func (*ContainerdRuntime) produceGenericContainerList(ctx context.Context, input []containerd.Container) ([]types.GenericContainer, error) {
	var result []types.GenericContainer

	for _, i := range input {

		ctr := types.GenericContainer{}

		info, err := i.Info(ctx)
		if err != nil {
			return nil, err
		}

		ctr.Names = []string{i.ID()}
		ctr.ID = i.ID()
		ctr.ShortID = ctr.ID
		ctr.Image = info.Image
		ctr.Labels = info.Labels

		ctr.NetworkSettings, err = extractIPInfoFromLabels(ctr.Labels)
		if err != nil {
			return nil, err
		}

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
				return nil, fmt.Errorf("failed to retrieve task status")
			}
			ctr.State = string(status.Status)

			switch status.Status {
			case containerd.Stopped:
				ctr.Status = fmt.Sprintf("Exited (%v) %s", status.ExitStatus, timeSinceInHuman(status.ExitTime))
			case containerd.Running:
				ctr.Status = "Up"
			default:
				ctr.Status = strings.Title(string(status.Status))
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

func extractIPInfoFromLabels(labels map[string]string) (types.GenericMgmtIPs, error) {
	var ipv4mask int
	var ipv6mask int
	var err error
	if val, exists := labels["clab.ipv4.netmask"]; exists {
		ipv4mask, err = strconv.Atoi(val)
		if err != nil {
			return types.GenericMgmtIPs{}, err
		}
	}
	if val, exists := labels["clab.ipv6.netmask"]; exists {
		ipv6mask, err = strconv.Atoi(val)
		if err != nil {
			return types.GenericMgmtIPs{}, err
		}
	}
	return types.GenericMgmtIPs{IPv4addr: labels["clab.ipv4.addr"], IPv4pLen: ipv4mask, IPv6addr: labels["clab.ipv6.addr"], IPv6pLen: ipv6mask}, nil
}

func timeSinceInHuman(since time.Time) string {
	return units.HumanDuration(time.Since(since)) + " ago"
}

func (c *ContainerdRuntime) GetNSPath(ctx context.Context, containername string) (string, error) {
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	task, err := c.getContainerTask(ctx, containername)
	if err != nil {
		return "", err
	}
	return "/proc/" + strconv.Itoa(int(task.Pid())) + "/ns/net", nil
}
func (c *ContainerdRuntime) Exec(ctx context.Context, containername string, cmd []string) ([]byte, []byte, error) {
	return c.internalExec(ctx, containername, cmd, false)
}

func (c *ContainerdRuntime) ExecNotWait(ctx context.Context, containername string, cmd []string) error {
	_, _, err := c.internalExec(ctx, containername, cmd, true)
	return err
}

func (c *ContainerdRuntime) internalExec(ctx context.Context, containername string, cmd []string, detach bool) ([]byte, []byte, error) { //skipcq: RVV-A0005

	clabExecId := "clabexec"
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)
	container, err := c.client.LoadContainer(ctx, containername)
	if err != nil {
		return nil, nil, err
	}

	var stdinbuf, stdoutbuf, stderrbuf bytes.Buffer

	cio_opt := cio.WithStreams(&stdinbuf, &stdoutbuf, &stderrbuf)
	ioCreator := cio.NewCreator(cio_opt)

	spec, err := container.Spec(ctx)
	if err != nil {
		return nil, nil, err
	}
	pspec := spec.Process
	pspec.Terminal = false
	pspec.Args = cmd
	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	needToDelete := true
	p, err := task.LoadProcess(ctx, clabExecId, nil)
	if err != nil {
		needToDelete = false
	}

	if needToDelete {
		log.Debugf("Deleting old process with exec-id %s", clabExecId)
		_, err := p.Delete(ctx, containerd.WithProcessKill)
		if err != nil {
			return nil, nil, err
		}
	}

	process, err := task.Exec(ctx, clabExecId, pspec, ioCreator)
	// task, err := container.NewTask(ctx, cio.NewCreator(cio_opt))
	if err != nil {
		return nil, nil, err
	}

	var statusC <-chan containerd.ExitStatus
	if !detach {

		defer func() {
			exitStatus, err := process.Delete(ctx)
			if err != nil {
				log.Errorf("failed to delete process: %v", err)
				return
			}
			if exitStatus.Error() != nil {
				log.Errorf("failed to delete process: %v", exitStatus.Error())
			}
		}()

		statusC, err = process.Wait(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := process.Start(ctx); err != nil {
		return nil, nil, err
	}
	if !detach {
		status := <-statusC
		code, _, err := status.Result()
		if err != nil {
			return nil, nil, err
		}

		log.Infof("Exit code: %d", code)
	}
	return stdoutbuf.Bytes(), stderrbuf.Bytes(), nil
}

func (c *ContainerdRuntime) DeleteContainer(ctx context.Context, containerID string) error {
	log.Debugf("deleting container %s", containerID)
	ctx = namespaces.WithNamespace(ctx, containerdNamespace)

	err := c.StopContainer(ctx, containerID)
	if err != nil {
		return err
	}

	cnic, cncl, cnirc, err := cniInit(containerID, "eth0", c.Mgmt)
	if err != nil {
		return err
	}

	err = cnic.DelNetworkList(ctx, cncl, cnirc)
	if err != nil {
		return err
	}

	cont, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}
	var delOpts []containerd.DeleteOpts
	delOpts = append(delOpts, containerd.WithSnapshotCleanup)

	if err := cont.Delete(ctx, delOpts...); err != nil {
		return err
	}

	log.Debugf("successfully deleted container %s", containerID)

	return nil
}
