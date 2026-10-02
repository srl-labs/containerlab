// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package fortinet_fortigate

import (
	"context"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/dustin/go-humanize"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"fortinet_fortigate", "fortinet_fortiproxy"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`port(?P<port>\d+)$`)
	InterfaceOffset = 2
	InterfaceHelp   = "portX (where X >= 2) or ethX (where X >= 1)"
)

const (
	scrapliPlatformName = "fortinet_fortios"
	tftpDirName         = "tftpboot"
	licenseFileName     = "appliance.lic"
	generateable        = true
	generateIfFormat    = "eth%d"
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

	r.Register(kindnames, func() clabnodes.Node {
		return new(fortigate)
	}, nrea)
}

type fortigate struct {
	clabnodes.VRNode
}

func (n *fortigate) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           n.Cfg.Credentials.Username,
		"PASSWORD":           n.Cfg.Credentials.Password,
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"QEMU_SMP":           "2",
		"QEMU_MEMORY":        "2048",
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}

	if n.Cfg.CPU != 0 {
		defEnv["QEMU_SMP"] = strconv.FormatFloat(n.Cfg.CPU, 'f', -1, 64)
	}
	if n.Cfg.Memory != "" {
		memBytes, err := humanize.ParseBytes(n.Cfg.Memory)
		if err != nil {
			return fmt.Errorf("failed to parse memory %q: %w", n.Cfg.Memory, err)
		}
		if memBytes < 1024*1024 {
			return fmt.Errorf("memory %q is below the 1MB QEMU minimum", n.Cfg.Memory)
		}
		defEnv["QEMU_MEMORY"] = strconv.FormatUint(
			uint64(math.Ceil(float64(memBytes)/(1024*1024))), 10)
	}

	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Credentials.Username,
		n.Cfg.Credentials.Password,
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *fortigate) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {

	err := n.VRNode.PreDeploy(ctx, params)
	if err != nil {
		return err
	}

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	cfg := n.Config()

	if cfg.License != "" {
		// copy license file to node specific lab directory
		src := cfg.License
		dst := filepath.Join(cfg.LabDir, tftpDirName, licenseFileName)
		if err := clabutils.CopyFile(context.Background(), src, dst,
			clabconstants.PermissionsFileDefault); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
		n.Cfg.Binds = append(
			n.Cfg.Binds,
			fmt.Sprint(path.Join(n.Cfg.LabDir, tftpDirName), ":/"+tftpDirName),
		)
	} else {
		log.Debugf("No license configured")
	}

	return nil
}
