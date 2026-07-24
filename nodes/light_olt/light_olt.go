// Copyright 2026 Abel Perez, Eluzmar Alviarez
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package light_olt implements the Containerlab light_olt node kind.
package light_olt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	defaultShmSize          = "4GiB"
	generateIfFormat        = "eth%d"
	configDirName           = "config"
	configDirDst            = "/clab/config"
	startupConfigFileName   = "light-olt-startup.tgz"
	startupOverlayFileName  = "light-olt-startup.txt"
	startupConfigEnv        = "CLAB_STARTUP_CONFIG"
	enforceStartupConfigEnv = "CLAB_ENFORCE_STARTUP_CONFIG"
	configBundleCommand     = "/usr/local/bin/olt-config-bundle"
	healthcheckCommand      = "/usr/local/bin/olt-healthcheck"

	// overlayExt is the extension of an eCLI overlay; bundleExt is the
	// extension of a TGZ backup produced by `clab save`.
	overlayExt = ".txt"
	bundleExt  = ".tgz"
)

var (
	kindNames          = []string{"light_olt"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	// interfaceAliases maps Lightspan port names to the Linux interfaces
	// exposed by the emulator. The emulator supports a single NT uplink
	// (1/2/1) plus up to four network ports on the first shelf; extend this
	// map together with the container's interface allocation if that ever
	// changes.
	interfaceAliases = map[string]string{
		"1/2/1": "eth1",
		"1/1/1": "eth2",
		"1/1/2": "eth3",
		"1/1/3": "eth4",
		"1/1/4": "eth5",
	}
)

// Register registers the light_olt kind in the node registry.
func Register(r *clabnodes.NodeRegistry) {
	// Topologies may be generated with `clab generate`, which needs the
	// interface naming format below.
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(true, generateIfFormat)
	attributes := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		nil,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(lightOLT)
	}, attributes)
}

type lightOLT struct {
	clabnodes.DefaultNode
	// configDir is the host-side node artifact directory mounted at
	// /clab/config. configPath is always the generated multi-plane backup,
	// while startupPath may point to either that backup or an eCLI overlay.
	// startupPath is only resolved in prepareStartupConfig, because the
	// choice depends on whether a saved bundle exists on disk.
	configDir   string
	configPath  string
	startupPath string
	// aliasedBy records which Lightspan port name was translated into each
	// Linux interface, so collisions can be reported using topology names.
	aliasedBy map[string]string
}

// Init applies the defaults required by the Light OLT runtime and declares
// the Containerlab artifact used by the save command.
func (n *lightOLT) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg
	n.aliasedBy = make(map[string]string)

	if err := validateStartupConfigExt(n.Cfg.StartupConfig); err != nil {
		return err
	}

	if n.Cfg.Env == nil {
		n.Cfg.Env = make(map[string]string)
	}
	n.configDir = filepath.Join(n.Cfg.LabDir, configDirName)
	n.configPath = filepath.Join(n.configDir, startupConfigFileName)
	n.Cfg.ResStartupConfig = n.configPath

	// Init may run more than once against the same NodeConfig, and the
	// topology may already declare this bind; appending unconditionally
	// would mount the same directory several times.
	bind := fmt.Sprintf("%s:%s", n.configDir, configDirDst)
	if !containsString(n.Cfg.Binds, bind) {
		n.Cfg.Binds = append(n.Cfg.Binds, bind)
	}

	n.Cfg.Env[enforceStartupConfigEnv] = fmt.Sprintf("%t", n.Cfg.EnforceStartupConfig)
	// startupConfigEnv is deliberately not set here: PreDeploy decides
	// between the saved bundle and the topology overlay, and setting a
	// provisional value would leave a stale path if that decision differs.

	if n.Cfg.ShmSize == "" {
		n.Cfg.ShmSize = defaultShmSize
	}

	// Data links are attached after container creation. An automatic runtime
	// restart can lose those interfaces, so lifecycle changes remain under
	// Containerlab control.
	if n.Cfg.RestartPolicy == "" {
		n.Cfg.RestartPolicy = "no"
	}
	if n.Cfg.Healthcheck == nil {
		n.Cfg.Healthcheck = &clabtypes.HealthcheckConfig{
			Test:        []string{"CMD", healthcheckCommand},
			Interval:    5,
			Timeout:     3,
			Retries:     24,
			StartPeriod: 30,
		}
	}

	for _, o := range opts {
		o(n)
	}

	return nil
}

// validateStartupConfigExt rejects startup-config files the emulator cannot
// consume, instead of silently treating them as a TGZ bundle.
func validateStartupConfigExt(startupConfig string) error {
	if startupConfig == "" {
		return nil
	}

	switch ext := strings.ToLower(filepath.Ext(startupConfig)); ext {
	case overlayExt, bundleExt:
		return nil
	default:
		return fmt.Errorf(
			"unsupported startup-config extension %q for %s: want %s (eCLI overlay) or %s (saved bundle)",
			ext, startupConfig, overlayExt, bundleExt,
		)
	}
}

// isOverlay reports whether the configured startup-config is an eCLI overlay
// rather than a TGZ bundle.
func (n *lightOLT) isOverlay() bool {
	return strings.ToLower(filepath.Ext(n.Cfg.StartupConfig)) == overlayExt
}

// PreDeploy creates the node artifact directory and stages the configured
// startup file in the path mounted by the container.
func (n *lightOLT) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.MkdirAll(n.configDir, clabconstants.PermissionsDirDefault); err != nil {
		return fmt.Errorf("creating Light OLT config directory: %w", err)
	}

	if _, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName); err != nil {
		return err
	}

	return n.prepareStartupConfig(ctx)
}

// prepareStartupConfig resolves startupPath and copies a topology-provided
// TXT overlay or TGZ backup into the stable Containerlab node artifact path.
// It is the single place where startupConfigEnv is set, so the container
// always receives the path that was actually staged.
func (n *lightOLT) prepareStartupConfig(ctx context.Context) error {
	// A bundle produced by `clab save` is the node's saved configuration and
	// takes precedence over the topology's original TXT overlay on subsequent
	// deployments. Init cannot make this choice because PreDeploy is where the
	// node artifact directory and any previously saved bundle are available.
	if !n.Cfg.EnforceStartupConfig && clabutils.FileExists(n.configPath) {
		n.useStartupPath(n.configPath, startupConfigFileName)
		return nil
	}

	if n.isOverlay() {
		n.useStartupPath(filepath.Join(n.configDir, startupOverlayFileName), startupOverlayFileName)
	} else {
		n.useStartupPath(n.configPath, startupConfigFileName)
	}

	if n.Cfg.StartupConfig == "" {
		return nil
	}
	if !n.Cfg.EnforceStartupConfig && clabutils.FileExists(n.startupPath) {
		return nil
	}

	source, err := filepath.Abs(n.Cfg.StartupConfig)
	if err != nil {
		return fmt.Errorf("resolving Light OLT startup-config: %w", err)
	}
	destination, err := filepath.Abs(n.startupPath)
	if err != nil {
		return fmt.Errorf("resolving Light OLT generated startup-config: %w", err)
	}
	if source == destination {
		return nil
	}

	if err := clabutils.CopyFile(ctx, source, destination,
		clabconstants.PermissionsFileDefault); err != nil {
		return fmt.Errorf("copying Light OLT startup-config: %w", err)
	}

	return nil
}

// useStartupPath points the node and the container at the same staged file.
func (n *lightOLT) useStartupPath(hostPath, fileName string) {
	n.startupPath = hostPath
	n.Cfg.Env[startupConfigEnv] = filepath.Join(configDirDst, fileName)
}

// SaveConfig exports all active Sysrepo planes as one TGZ artifact and
// reports its host path to the Containerlab save command.
func (n *lightOLT) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	execCmd := clabexec.NewExecCmdFromSlice([]string{
		configBundleCommand,
		"export",
		filepath.Join(configDirDst, startupConfigFileName),
	})
	execResult, err := n.RunExec(ctx, execCmd)
	if err != nil {
		return nil, fmt.Errorf("exporting Light OLT configuration: %w", err)
	}
	if execResult.GetReturnCode() != 0 {
		return nil, fmt.Errorf(
			"exporting Light OLT configuration failed (rc=%d): %s",
			execResult.GetReturnCode(), execResult.GetStdErrString(),
		)
	}
	if !clabutils.FileExists(n.configPath) {
		return nil, fmt.Errorf("configuration export did not create %s", n.configPath)
	}

	return &clabnodes.SaveConfigResult{ConfigPath: n.configPath}, nil
}

// LinkApplyMode declares that Linux data links can be added and removed while
// the Light OLT container remains running.
func (*lightOLT) LinkApplyMode(context.Context) clabnodes.LinkApplyMode {
	return clabnodes.LinkApplyModeLive
}

// AddEndpoint maps Lightspan port names to the Linux interfaces used by the
// emulator. Native ethN names remain valid for backwards compatibility.
func (n *lightOLT) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	mappedName, isAlias := interfaceAliases[endpointName]
	if !isAlias {
		mappedName = endpointName
	}

	// Report the collision here, where both the alias and the native name are
	// still known. CheckInterfaceName would only see the translated name and
	// could not identify which Lightspan port produced it.
	if previous, taken := n.aliasedBy[mappedName]; taken {
		return fmt.Errorf(
			"interface %s of node %s is already used by %q; %q maps to the same interface",
			mappedName, n.Cfg.ShortName, previous, endpointName,
		)
	}
	n.aliasedBy[mappedName] = endpointName

	if isAlias {
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}

	n.Endpoints = append(n.Endpoints, e)
	return nil
}

// CheckInterfaceName reserves eth0 for management. Lightspan aliases have
// already been translated to eth1, eth2, ... by AddEndpoint.
func (n *lightOLT) CheckInterfaceName() error {
	if err := clabnodes.GenericVMInterfaceCheck(n.Cfg.ShortName, n.Endpoints); err != nil {
		return err
	}

	return n.DefaultNode.CheckInterfaceName()
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
