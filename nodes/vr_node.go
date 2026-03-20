package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnetconf "github.com/srl-labs/containerlab/netconf"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var VMInterfaceRegexp = regexp.MustCompile(`eth[1-9]\d*$`) // skipcq: GO-C4007
var imageTagRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type VRNode struct {
	DefaultNode
	ScrapliPlatformName string
	ConfigDirName       string
	StartupCfgFName     string
	Credentials         *Credentials
}

func NewVRNode(n NodeOverwrites, creds *Credentials, scrapliPlatformName string) *VRNode {
	vr := &VRNode{}

	vr.DefaultNode = *NewDefaultNode(n)

	vr.Credentials = creds
	vr.ScrapliPlatformName = scrapliPlatformName

	vr.InterfaceMappedPrefix = "eth"
	vr.InterfaceOffset = 0
	vr.FirstDataIfIndex = 1
	vr.ConfigDirName = "config"
	vr.StartupCfgFName = "startup-config.cfg"

	return vr
}

// Init stub function.
func (n *VRNode) Init(cfg *clabtypes.NodeConfig, opts ...NodeOption) error {
	return nil
}

// PreDeploy default function: create lab directory, generate certificates, generate startup config
// file.
func (n *VRNode) PreDeploy(_ context.Context, params *PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return LoadStartupConfigFileVr(n, n.ConfigDirName, n.StartupCfgFName)
}

// AddEndpoint override version maps the endpoint name to an ethX-based name before adding it to the
// node endpoints. Returns an error if the mapping goes wrong.
func (vr *VRNode) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	// Slightly modified check: if it doesn't match the VMInterfaceRegexp, pass it to
	// GetMappedInterfaceName. If it fails, then the interface name is wrong.
	if vr.InterfaceRegexp != nil && !(VMInterfaceRegexp.MatchString(endpointName)) {
		mappedName, err := vr.OverwriteNode.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf(
				"%q interface name %q could not be mapped to an ethX-based interface name: %w",
				vr.Cfg.ShortName,
				e.GetIfaceName(),
				err,
			)
		}
		log.Debugf(
			"Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)",
			endpointName,
			mappedName,
		)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}
	vr.Endpoints = append(vr.Endpoints, e)

	return nil
}

// CheckInterfaceName checks interface names for generic VM-based nodes.
// Displays InterfaceHelp if the check fails for the expected VM interface regexp.
func (vr *VRNode) CheckInterfaceName() error {
	err := vr.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, ep := range vr.Endpoints {
		ifName := ep.GetIfaceName()
		if !VMInterfaceRegexp.MatchString(ifName) {
			return fmt.Errorf(
				"%q interface name %q does not match the required interface patterns: %q",
				vr.Cfg.ShortName,
				ifName,
				vr.InterfaceHelp,
			)
		}
	}

	return nil
}

func (n *VRNode) SaveConfig(_ context.Context) (*SaveConfigResult, error) {
	config, err := clabnetconf.GetConfig(n.Cfg.LongName,
		n.Credentials.GetUsername(),
		n.Credentials.GetPassword(),
		n.ScrapliPlatformName,
	)
	if err != nil {
		return nil, err
	}

	// Save config to mounted labdir startup config path
	configPath := filepath.Join(n.Cfg.LabDir, n.ConfigDirName, n.StartupCfgFName)
	err = os.WriteFile(
		configPath,
		[]byte(config),
		clabconstants.PermissionsOpen,
	) // skipcq: GO-S2306
	if err != nil {
		return nil, fmt.Errorf(
			"failed to write config by %s path from %s container: %v",
			configPath,
			n.Cfg.ShortName,
			err,
		)
	}
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return &SaveConfigResult{
		ConfigPath: configPath,
	}, nil
}

// Stop prepares vrnetlab-specific state (qcow alias) and then delegates to
// DefaultNode.Stop which parks interfaces and stops the container.
func (vr *VRNode) Stop(ctx context.Context) error {
	preStopPrepareVrnetlabQcowAlias(ctx, &vr.DefaultNode)
	return vr.DefaultNode.Stop(ctx)
}

func preStopPrepareVrnetlabQcowAlias(ctx context.Context, d *DefaultNode) {
	aliasName, ok := vrnetlabQcowAliasName(d.Config().Image)
	if !ok {
		log.Debugf(
			"node %q pre-stop vrnetlab qcow alias skipped: unable to infer tag from image %q",
			d.Config().ShortName,
			d.Config().Image,
		)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Some vrnetlab nodes rename the original versioned qcow image after first boot and fail on
	// subsequent starts when they try to rediscover a versioned qcow filename. If there is exactly
	// one non-overlay qcow file in / and our alias is absent, create a hardlink alias based on the
	// image tag.
	cmd := fmt.Sprintf(
		`alias="/%s"; `+
			`[ -e "$alias" ] && exit 0; `+
			`src=""; `+
			`if [ -f /sros.qcow2 ] && [ "/sros.qcow2" != "$alias" ]; then `+
			`src="/sros.qcow2"; `+
			`else `+
			`set -- /*.qcow2; `+
			`if [ "$1" != "/*.qcow2" ]; then `+
			`for f in "$@"; do `+
			`[ "$f" = "$alias" ] && continue; `+
			`base="${f##*/}"; `+
			`case "$base" in *overlay*.qcow2) continue ;; esac; `+
			`if [ -n "$src" ]; then src=""; break; fi; `+
			`src="$f"; `+
			`done; `+
			`fi; `+
			`fi; `+
			`[ -n "$src" ] || exit 0; `+
			`ln "$src" "$alias"`,
		aliasName,
	)

	execCmd := clabexec.NewExecCmdFromSlice([]string{"sh", "-lc", cmd})
	res, err := d.RunExec(ctx, execCmd)
	if err != nil {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias preparation failed: %v",
			d.Config().ShortName,
			err,
		)
		return
	}

	if res != nil && res.ReturnCode != 0 {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias prep returned code %d (stderr: %s)",
			d.Config().ShortName,
			res.ReturnCode,
			res.Stderr,
		)
	}
}

func vrnetlabQcowAliasName(image string) (string, bool) {
	tag, ok := imageTag(image)
	if !ok {
		return "", false
	}

	return "clab-" + tag + ".qcow2", true
}

func imageTag(image string) (string, bool) {
	if at := strings.LastIndex(image, "@"); at != -1 {
		image = image[:at]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 || lastColon < lastSlash {
		return "", false
	}

	tag := image[lastColon+1:]
	if tag == "" {
		return "", false
	}

	if !imageTagRE.MatchString(tag) {
		return "", false
	}

	return tag, true
}
