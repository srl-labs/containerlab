// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_sdwan

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"cisco_sdwan"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`eth(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "ethX (where X >= 1)"
)

const (
	scrapliPlatformName = "cisco_iosxe"

	generateable     = true
	generateIfFormat = "eth%d"

	// Component types.
	componentTypeManager    = "manager"
	componentTypeController = "controller"
	componentTypeValidator  = "validator"

	// Default component type if not specified.
	defaultComponentType = componentTypeManager
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformAttrs,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(ciscoSdwan)
	}, nrea)
}

type ciscoSdwan struct {
	clabnodes.VRNode
	componentType string
}

func (n *ciscoSdwan) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// Determine component type from NodeType field
	// NodeType should be one of: manager, controller, validator
	n.componentType = n.Cfg.NodeType
	if n.componentType == "" {
		n.componentType = defaultComponentType
	}

	// Validate component type
	if !isValidComponentType(n.componentType) {
		return fmt.Errorf(
			"invalid component type %q for vr_cisco_sdwan node %q. Must be one of: manager, controller, validator",
			n.componentType,
			n.Cfg.ShortName,
		)
	}

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support cloud-init configuration
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	// Build launch.py command with component-specific parameters
	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --component-type %s --trace",
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
		n.componentType,
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *ciscoSdwan) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	return createCiscoSdwanFiles(n)
}

func (n *ciscoSdwan) GetImages(_ context.Context) map[string]string {
	images := make(map[string]string)

	// Determine image name based on component type
	// vrnetlab builds images as: vrnetlab/cisco_sdwan-{manager,controller,validator}:version
	imageName := n.Cfg.Image
	if imageName == "" {
		// If no image specified, construct default image name
		// User will need to specify the version tag
		imageName = fmt.Sprintf("vrnetlab/cisco_sdwan-%s", n.componentType)
	}

	images[clabnodes.ImageKey] = imageName

	return images
}

// isValidComponentType checks if the component type is valid.
func isValidComponentType(componentType string) bool {
	switch componentType {
	case componentTypeManager, componentTypeController, componentTypeValidator:
		return true
	default:
		return false
	}
}

// createCiscoSdwanFiles handles startup configuration files for Cisco SD-WAN nodes.
// It supports both cloud-init.yaml (full) and zcloud.xml (partial) configuration files.
func createCiscoSdwanFiles(node *ciscoSdwan) error {
	nodeCfg := node.Config()

	// Skip if no startup config is specified
	if nodeCfg.StartupConfig == "" {
		return nil
	}

	// Determine config file type and destination based on file extension
	configDir := filepath.Join(nodeCfg.LabDir, node.ConfigDirName)

	// Ensure config directory exists
	clabutils.CreateDirectory(configDir, clabconstants.PermissionsOpen)

	var dstFilename string
	ext := strings.ToLower(filepath.Ext(nodeCfg.StartupConfig))

	switch {
	case ext == ".yaml" || ext == ".yml":
		// Full cloud-init file
		dstFilename = "cloud-init.yaml"
		log.Debugf("Using cloud-init configuration file for node %s", nodeCfg.ShortName)
	case ext == ".xml":
		// zCloud XML configuration
		dstFilename = "zcloud.xml"
		log.Debugf("Using zCloud XML configuration file for node %s", nodeCfg.ShortName)
	default:
		return fmt.Errorf(
			"unsupported startup config file format for node %s: %s. Supported formats: .yaml/.yml (cloud-init) or .xml (zcloud)",
			nodeCfg.ShortName,
			nodeCfg.StartupConfig,
		)
	}

	dst := filepath.Join(configDir, dstFilename)

	// Check if config already exists
	if _, err := os.Stat(dst); err == nil {
		log.Infof("Config file already exists for node %s at %s", nodeCfg.ShortName, dst)
		return nil
	}

	// Copy the startup config to the destination
	if err := clabutils.CopyFile(context.Background(), nodeCfg.StartupConfig, dst,
		clabconstants.PermissionsFileDefault); err != nil {
		return fmt.Errorf("failed to copy startup config [src: %s -> dst: %s]: %v",
			nodeCfg.StartupConfig, dst, err)
	}

	log.Debugf("Copied startup config: %s -> %s", nodeCfg.StartupConfig, dst)

	return nil
}
