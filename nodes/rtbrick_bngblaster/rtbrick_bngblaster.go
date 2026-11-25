// Copyright 2026 RtBrick, Inc.
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package rtbrick_bngblaster

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var kindNames = []string{"rtbrick_bngblaster", "bngblaster"}

const (
	generateable     = true
	generateIfFormat = "eth%d"

	// DefaultImage is used if the node has no image configured.
	DefaultImage = "ghcr.io/rtbrick/bngblaster:latest"

	// instanceDir is the BNG Blaster controller config folder in the container,
	// bound to the node's lab directory, so that the logs and reports of the
	// test are kept in the lab directory.
	instanceDir = "/var/bngblaster"
	// instanceName is the controller instance started from the startup-config.
	instanceName = "clab"
	configFile   = "config.json"

	envAutostart      = "BNGBLASTERCTRL_AUTOSTART"
	envWaitInterfaces = "BNGBLASTERCTRL_WAIT_INTERFACES"

	// sysctlDisableIPv6 disables IPv6 on the data interfaces, which containerlab
	// attaches after the container started. Not net.ipv6.conf.all, which would
	// also overwrite the default and disable IPv6 on the management interface.
	sysctlDisableIPv6 = "net.ipv6.conf.default.disable_ipv6"
)

// defaultHealthcheck reports the node healthy while the test runs or after it
// finished without error. It is unhealthy while the controller waits for the
// links (up to 5 minutes by default), which the start period covers.
var defaultHealthcheck = clabtypes.HealthcheckConfig{
	Test:        []string{"CMD", "/usr/bin/bngblasterctrl", "-health"},
	Interval:    5,
	Timeout:     3,
	Retries:     3,
	StartPeriod: 300,
}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(bngblaster)
	}, nrea)
}

type bngblaster struct {
	clabnodes.DefaultNode
}

func (n *bngblaster) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	if n.Cfg.Image == "" {
		n.Cfg.Image = DefaultImage
	}

	if n.Cfg.Env == nil {
		n.Cfg.Env = map[string]string{}
	}
	if n.Cfg.StartupConfig != "" {
		n.Cfg.ResStartupConfig = path.Join(n.Cfg.LabDir, instanceName, configFile)
		// A suppressed startup-config is not written, so there is no config
		// to start the instance with.
		if !n.Cfg.SuppressStartupConfig {
			setDefault(n.Cfg.Env, envAutostart, instanceName)
		}
	}

	// The node has no network namespace of its own with network-mode host or
	// container:<name>, where the runtime rejects net.* sysctls.
	if hasOwnNetns(n.Cfg.NetworkMode) {
		if n.Cfg.Sysctls == nil {
			n.Cfg.Sysctls = map[string]string{}
		}
		setDefault(n.Cfg.Sysctls, sysctlDisableIPv6, "1")
	}

	if n.Cfg.Healthcheck == nil {
		hc := defaultHealthcheck
		n.Cfg.Healthcheck = &hc
	}

	n.Cfg.Binds = append(n.Cfg.Binds, n.Cfg.LabDir+":"+instanceDir)

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *bngblaster) PreDeploy(ctx context.Context, _ *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	if n.Cfg.StartupConfig == "" {
		return nil
	}

	clabutils.CreateDirectory(path.Dir(n.Cfg.ResStartupConfig), clabconstants.PermissionsOpen)

	cfg, err := os.ReadFile(n.Cfg.StartupConfig)
	if err != nil {
		return err
	}

	return n.GenerateConfig(ctx, n.Cfg.ResStartupConfig, string(cfg))
}

// Deploy lets the controller wait for all links before it starts the test.
func (n *bngblaster) Deploy(ctx context.Context, params *clabnodes.DeployParams) error {
	if n.Cfg.Env[envAutostart] != "" {
		setDefault(n.Cfg.Env, envWaitInterfaces, strconv.Itoa(len(n.GetEndpoints())))
	}

	return n.DefaultNode.Deploy(ctx, params)
}

// PreDestroy stops the container on destroy and apply. It runs before any link
// or node of the lab is removed, also those of the device under test, so that
// BNG Blaster can tear down its sessions over the links and write the report of
// the test. On SIGTERM the controller stops the test gracefully and ends the
// teardown at once after BNGBLASTERCTRL_STOP_TIMEOUT (default 30s), still with
// the report. The container is removed afterwards anyway, also if it did not
// stop, so a failed stop is not an error.
func (n *bngblaster) PreDestroy(ctx context.Context) error {
	if n.GetContainerStatus(ctx) == clabruntime.Running {
		_ = n.Runtime.StopContainer(ctx, n.GetContainerName(), n.StopSignal)
	}

	return nil
}

// CheckInterfaceName allows any interface name, except eth0 when the
// management interface uses it.
func (n *bngblaster) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf(
				"eth0 interface name is not allowed for %s node when network mode is not set to none",
				n.Cfg.ShortName,
			)
		}
	}

	return n.CheckInterfaceOverlap()
}

// hasOwnNetns reports whether the node has a network namespace of its own.
func hasOwnNetns(networkMode string) bool {
	nm := strings.ToLower(networkMode)

	return nm != "host" && !strings.HasPrefix(nm, "container:")
}

func setDefault(m map[string]string, key, value string) {
	if _, ok := m[key]; !ok {
		m[key] = value
	}
}
