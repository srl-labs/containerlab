package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnetconf "github.com/srl-labs/containerlab/netconf"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var VMInterfaceRegexp = regexp.MustCompile(`eth[1-9]\d*$`) // skipcq: GO-C4007

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

// PreDeploy default function: create lab directory, generate certificates, generate startup config file.
func (n *VRNode) PreDeploy(_ context.Context, params *PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return LoadStartupConfigFileVr(n, n.ConfigDirName, n.StartupCfgFName)
}

// AddEndpoint override version maps the endpoint name to an ethX-based name before adding it to the node endpoints. Returns an error if the mapping goes wrong.
func (vr *VRNode) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	// Slightly modified check: if it doesn't match the VMInterfaceRegexp, pass it to GetMappedInterfaceName. If it fails, then the interface name is wrong.
	if vr.InterfaceRegexp != nil && !(VMInterfaceRegexp.MatchString(endpointName)) {
		mappedName, err := vr.OverwriteNode.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf("%q interface name %q could not be mapped to an ethX-based interface name: %w",
				vr.Cfg.ShortName, e.GetIfaceName(), err)
		}
		log.Debugf("Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)", endpointName, mappedName)
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
			return fmt.Errorf("%q interface name %q does not match the required interface patterns: %q",
				vr.Cfg.ShortName, ifName, vr.InterfaceHelp)
		}
	}

	return nil
}

func (n *VRNode) SaveConfig(_ context.Context) error {
	config, err := clabnetconf.GetConfig(n.Cfg.LongName,
		n.Credentials.GetUsername(),
		n.Credentials.GetPassword(),
		n.ScrapliPlatformName,
	)
	if err != nil {
		return err
	}

	// Save config to mounted labdir startup config path
	configPath := filepath.Join(n.Cfg.LabDir, n.ConfigDirName, n.StartupCfgFName)
	err = os.WriteFile(configPath, []byte(config), clabconstants.PermissionsOpen) // skipcq: GO-S2306
	if err != nil {
		return fmt.Errorf("failed to write config by %s path from %s container: %v", configPath, n.Cfg.ShortName, err)
	}
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return nil
}
