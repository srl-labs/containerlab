package vr_vaos

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames = []string{"ale_vaos"}
	// defaultCredentials are the default credentials of the vAOS image.
	defaultCredentials = clabnodes.NewCredentials("admin", "Switch@123")

	// InterfaceRegexp maps front-panel ports 1/1/1 to 1/1/10 to eth1 to eth10.
	// 1/1/29A and 1/1/30A are only addressable as eth11 and eth12.
	InterfaceRegexp = regexp.MustCompile(`^1/1/(?P<port>[1-9]|10)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "1/1/X (where 1 <= X <= 10) or ethX (where X >= 1)"
)

const (
	// scrapliPlatformName is empty: the vrnetlab launcher handles containerlab save.
	scrapliPlatformName = ""

	// containerlab save handshake with the vrnetlab launcher, through the
	// config dir mounted at /config in the container.
	saveRequestFile = ".vaos-save-request"
	saveDoneFile    = ".vaos-save-done"
	saveTimeout     = 5 * time.Minute
	savePollPeriod  = time.Second
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

// SaveConfig asks the vrnetlab launcher to save the configuration. The request
// file holds a token. The launcher runs "write memory flash-synchro" on the
// serial console, writes the output of "show configuration snapshot" to the
// startup-config file of the node, then writes "<token> ok" or "<token> <error>"
// in the done file.
func (n *vrVAOS) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	cfgDir := filepath.Join(n.Cfg.LabDir, n.ConfigDirName)
	reqFile := filepath.Join(cfgDir, saveRequestFile)
	doneFile := filepath.Join(cfgDir, saveDoneFile)

	token := strconv.FormatInt(time.Now().UnixNano(), 10)

	// write then rename, so the launcher never reads a partial request
	err := os.WriteFile(reqFile+".tmp", []byte(token+"\n"), 0o644)
	if err == nil {
		err = os.Rename(reqFile+".tmp", reqFile)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to request configuration save: %w",
			n.Cfg.ShortName, err)
	}

	msg, err := waitSaveDone(ctx, doneFile, token, saveTimeout)
	if err != nil {
		_ = os.Remove(reqFile)

		return nil, fmt.Errorf("%s: %w", n.Cfg.ShortName, err)
	}

	if msg != "ok" {
		return nil, fmt.Errorf("%s: configuration save failed: %s", n.Cfg.ShortName, msg)
	}

	configPath := filepath.Join(cfgDir, n.StartupCfgFName)
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return &clabnodes.SaveConfigResult{
		ConfigPath: configPath,
	}, nil
}

// waitSaveDone waits for the launcher to write the result of the request
// identified by token in doneFile and returns it. A result without the token
// (an earlier request, or a file still being written) is ignored.
func waitSaveDone(
	ctx context.Context,
	doneFile, token string,
	timeout time.Duration,
) (string, error) {
	deadline := time.Now().Add(timeout)

	for {
		b, _ := os.ReadFile(doneFile)
		if msg, ok := strings.CutPrefix(strings.TrimSpace(string(b)), token+" "); ok {
			_ = os.Remove(doneFile)

			return msg, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for the configuration save")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(savePollPeriod):
		}
	}
}
