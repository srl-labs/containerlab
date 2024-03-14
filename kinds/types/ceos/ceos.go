// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	ifWaitScriptContainerPath = "/mnt/flash/if-wait.sh"
)

var (
	kindnames = []string{"ceos", "arista_ceos"}
	// defined env vars for the ceos.
	ceosEnv = map[string]string{
		"CEOS":                                "1",
		"EOS_PLATFORM":                        "ceoslab",
		"container":                           "docker",
		"ETBA":                                "1",
		"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT": "1",
		"INTFTYPE":                            "eth",
		"MAPETH0":                             "1",
		"MGMT_INTF":                           "eth0",
	}

	//go:embed ceos.cfg
	cfgTemplate string

	saveCmd = "Cli -p 15 -c wr"

	defaultCredentials = kind_registry.NewCredentials("admin", "admin")
)

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(ceos)
	}, defaultCredentials)
}

type ceos struct {
	nodes.DefaultNode
}

// intfMap represents interface mapping config file.
type intfMap struct {
	ManagementIntf struct {
		Eth0 string `json:"eth0"`
	} `json:"ManagementIntf"`
	EthernetIntf struct {
		eth map[string]string
	} `json:"EthernetIntf"`
}

func (n *ceos) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.Cfg.Env = utils.MergeStringMaps(ceosEnv, n.Cfg.Env)

	// the node.Cmd should be aligned with the environment.
	// prepending original Cmd with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	var envSb strings.Builder
	envSb.WriteString("bash -c '" + ifWaitScriptContainerPath + " ; exec /sbin/init ")
	for k, v := range n.Cfg.Env {
		envSb.WriteString("systemd.setenv=" + k + "=" + v + " ")
	}
	envSb.WriteString("'")

	n.Cfg.Cmd = envSb.String()
	hwa, err := utils.GenMac("00:1c:73")
	if err != nil {
		return err
	}
	n.Cfg.MacAddress = hwa.String()

	// mount config dir
	cfgPath := filepath.Join(n.Cfg.LabDir, "flash")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/mnt/flash/", cfgPath))
	return nil
}

func (n *ceos) PreDeploy(ctx context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return n.createCEOSFiles(ctx)
}

func (n *ceos) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Arista cEOS '%s' node", n.Cfg.ShortName)
	return n.ceosPostDeploy(ctx)
}

func (n *ceos) SaveConfig(ctx context.Context) error {
	cmd, _ := exec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", n.Cfg.ShortName, err)
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("%s errors: %s", n.Cfg.ShortName, execResult.GetStdErrString())
	}

	confPath := n.Cfg.LabDir + "/flash/startup-config"
	log.Infof("saved cEOS configuration from %s node to %s\n", n.Cfg.ShortName, confPath)

	return nil
}

func (n *ceos) createCEOSFiles(_ context.Context) error {
	nodeCfg := n.Config()
	// generate config directory
	utils.CreateDirectory(path.Join(n.Cfg.LabDir, "flash"), 0777)
	cfg := filepath.Join(n.Cfg.LabDir, "flash", "startup-config")
	nodeCfg.ResStartupConfig = cfg

	// set mgmt ipv4 gateway as it is already known by now
	// since the container network has been created before we launch nodes
	// and mgmt gateway can be used in ceos.Cfg template to configure default route for mgmt
	nodeCfg.MgmtIPv4Gateway = n.Runtime.Mgmt().IPv4Gw
	nodeCfg.MgmtIPv6Gateway = n.Runtime.Mgmt().IPv6Gw

	// set the mgmt interface name for the node
	err := setMgmtInterface(nodeCfg)
	if err != nil {
		return err
	}

	// use startup config file provided by a user
	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	err = n.GenerateConfig(nodeCfg.ResStartupConfig, cfgTemplate)
	if err != nil {
		return err
	}

	// if extras have been provided copy these into the flash directory
	if nodeCfg.Extras != nil && len(nodeCfg.Extras.CeosCopyToFlash) != 0 {
		extras := nodeCfg.Extras.CeosCopyToFlash
		flash := filepath.Join(nodeCfg.LabDir, "flash")

		for _, extrapath := range extras {
			basename := filepath.Base(extrapath)
			dest := filepath.Join(flash, basename)

			topoDir := filepath.Dir(filepath.Dir(nodeCfg.LabDir)) // topo dir is needed to resolve extrapaths
			if err := utils.CopyFile(utils.ResolvePath(extrapath, topoDir), dest, 0644); err != nil {
				return fmt.Errorf("extras: copy-to-flash %s -> %s failed %v", extrapath, dest, err)
			}
		}
	}

	// sysmac is a system mac that is +1 to Ma0 mac
	m, err := net.ParseMAC(nodeCfg.MacAddress)
	if err != nil {
		return err
	}
	m[5] = m[5] + 1

	sysMacPath := path.Join(nodeCfg.LabDir, "flash", "system_mac_address")

	if !utils.FileExists(sysMacPath) {
		err = utils.CreateFile(sysMacPath, m.String())
	}

	// adding if-wait.sh script to flash dir
	ifScriptP := path.Join(nodeCfg.LabDir, "flash", "if-wait.sh")
	utils.CreateFile(ifScriptP, utils.IfWaitScript)
	os.Chmod(ifScriptP, 0777) // skipcq: GSC-G302

	return err
}

func setMgmtInterface(node *types.NodeConfig) error {
	// use interface mapping file to set the Management interface if it is provided in the binds section
	// default is Management0
	mgmtInterface := "Management0"
	for _, bindelement := range node.Binds {
		if !strings.Contains(bindelement, "EosIntfMapping.json") {
			continue
		}

		bindsplit := strings.Split(bindelement, ":")
		if len(bindsplit) < 2 {
			return fmt.Errorf("malformed bind instruction: %s", bindelement)
		}

		var m []byte // byte representation of a map file
		m, err := os.ReadFile(bindsplit[0])
		if err != nil {
			return err
		}

		// Reset management interface if defined in the intfMapping file
		var intfMappingJson intfMap
		err = json.Unmarshal(m, &intfMappingJson)
		if err != nil {
			log.Debugf("Management interface could not be read from intfMapping file for '%s' node.", node.ShortName)
			return err
		}
		mgmtInterface = intfMappingJson.ManagementIntf.Eth0

	}
	log.Debugf("Management interface for '%s' node is set to %s.", node.ShortName, mgmtInterface)
	node.MgmtIntf = mgmtInterface

	return nil
}

// ceosPostDeploy runs postdeploy actions which are required for ceos nodes.
func (n *ceos) ceosPostDeploy(_ context.Context) error {
	nodeCfg := n.Config()
	d, err := utils.SpawnCLIviaExec("arista_eos", nodeCfg.LongName, n.Runtime.GetName())
	if err != nil {
		return err
	}

	defer d.Close()

	cfgs := []string{
		"interface " + nodeCfg.MgmtIntf,
		"no ip address",
		"no ipv6 address",
	}

	// adding ipv4 address to configs
	if nodeCfg.MgmtIPv4Address != "" {
		cfgs = append(cfgs,
			fmt.Sprintf("ip address %s/%d", nodeCfg.MgmtIPv4Address, nodeCfg.MgmtIPv4PrefixLength),
		)
	}

	// adding ipv6 address to configs
	if nodeCfg.MgmtIPv6Address != "" {
		cfgs = append(cfgs,
			fmt.Sprintf("ipv6 address %s/%d", nodeCfg.MgmtIPv6Address, nodeCfg.MgmtIPv6PrefixLength),
		)
	}

	// add save to startup cmd
	cfgs = append(cfgs, "wr")
	resp, err := d.SendConfigs(cfgs)
	if err != nil {
		return err
	} else if resp.Failed != nil {
		return errors.New("failed CLI configuration")
	}

	return err
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *ceos) CheckInterfaceName() error {
	// allow eth and et interfaces
	// https://regex101.com/r/umQW5Z/2
	ifRe := regexp.MustCompile(`eth[1-9][\w\.]*$|et[1-9][\w\.]*$`)
	for _, e := range n.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("arista cEOS node %q has an interface named %q which doesn't match the required pattern. Interfaces should be named as ethX or etX, where X consists of alpanumerical characters", n.Cfg.ShortName, e.GetIfaceName())
		}
	}

	return nil
}
