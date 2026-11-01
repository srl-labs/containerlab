package vr_vaos

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames = []string{"ale_vaos"}
	// default credentials of the vAOS image
	defaultCredentials = clabnodes.NewCredentials("admin", "Switch@123")

	// front-panel ports 1/1/1 to 1/1/10 map to eth1 to eth10.
	// 1/1/29A and 1/1/30A are only addressable as eth11 and eth12.
	InterfaceRegexp = regexp.MustCompile(`^1/1/(?P<port>[1-9]|10)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "1/1/X (where 1 <= X <= 10) or ethX (where 1 <= X <= 12)"
)

const (
	// not used: containerlab save is handled by the vrnetlab launcher (see SaveConfig)
	scrapliPlatformName = ""

	// containerlab save handshake with the vrnetlab launcher, through the
	// config dir mounted at /config in the container
	saveRequestFile = ".vaos-save-request"
	saveDoneFile    = ".vaos-save-done"
	saveTimeout     = 5 * time.Minute
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindNames, func() clabnodes.Node {
		return new(vrVAOS)
	}, nrea)
}

type vrVAOS struct {
	clabnodes.VRNode
}

func (n *vrVAOS) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
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
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           n.Cfg.Credentials.Username,
		"PASSWORD":           n.Cfg.Credentials.Password,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
	// (the startup-config is copied to /config/startup-config.cfg and pushed
	// on the console by launch.py once the switch has booted)
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

// SaveConfig asks the vrnetlab launcher to save the configuration. The launcher
// runs "write memory flash-synchro" on the serial console and writes the output
// of "show configuration snapshot" to the startup-config file of the node.
func (n *vrVAOS) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	cfgDir := filepath.Join(n.Cfg.LabDir, n.ConfigDirName)
	reqFile := filepath.Join(cfgDir, saveRequestFile)
	doneFile := filepath.Join(cfgDir, saveDoneFile)

	_ = os.Remove(doneFile)
	if err := os.WriteFile(reqFile, []byte("save\n"), 0o644); err != nil {
		return nil, fmt.Errorf("%s: failed to request configuration save: %w", n.Cfg.ShortName, err)
	}

	deadline := time.Now().Add(saveTimeout)
	for {
		if b, err := os.ReadFile(doneFile); err == nil {
			_ = os.Remove(doneFile)
			if msg := strings.TrimSpace(string(b)); msg != "ok" {
				return nil, fmt.Errorf("%s: configuration save failed: %s", n.Cfg.ShortName, msg)
			}
			break
		}
		if time.Now().After(deadline) {
			_ = os.Remove(reqFile)
			return nil, fmt.Errorf("%s: timeout waiting for the configuration save", n.Cfg.ShortName)
		}
		select {
		case <-ctx.Done():
			_ = os.Remove(reqFile)
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}

	configPath := filepath.Join(cfgDir, n.StartupCfgFName)
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return &clabnodes.SaveConfigResult{
		ConfigPath: configPath,
	}, nil
}
