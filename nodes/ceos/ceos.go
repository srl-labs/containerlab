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
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	ifWaitScriptContainerPath = "/mnt/flash/if-wait.sh"
)

var (
	kindnames = []string{"ceos", "arista_ceos"}
	// defined env vars for the ceos
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

	saveCmd = []string{"Cli", "-p", "15", "-c", "wr"}
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(ceos)
	})

}

type ceos struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

// intfMap represents interface mapping config file
type intfMap struct {
	ManagementIntf struct {
		Eth0 string `json:"eth0"`
	} `json:"ManagementIntf"`
	EthernetIntf struct {
		eth map[string]string
	} `json:"EthernetIntf"`
}

func (n *ceos) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	n.cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.cfg.Env = utils.MergeStringMaps(ceosEnv, n.cfg.Env)

	// the node.Cmd should be aligned with the environment.
	// prepending original Cmd with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	var envSb strings.Builder
	envSb.WriteString("bash -c '" + ifWaitScriptContainerPath + " ; exec /sbin/init ")
	for k, v := range n.cfg.Env {
		envSb.WriteString("systemd.setenv=" + k + "=" + v + " ")
	}
	envSb.WriteString("'")
	n.cfg.Cmd = envSb.String()
	n.cfg.MacAddress = utils.GenMac("00:1c:73")

	// mount config dir
	cfgPath := filepath.Join(n.cfg.LabDir, "flash")
	n.cfg.Binds = append(n.cfg.Binds, fmt.Sprintf("%s:/mnt/flash/", cfgPath))
	return nil
}

func (n *ceos) Config() *types.NodeConfig { return n.cfg }

func (n *ceos) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(n.cfg.LabDir, 0777)
	return n.createCEOSFiles()
}

func (n *ceos) Deploy(ctx context.Context) error {
	cID, err := n.runtime.CreateContainer(ctx, n.cfg)
	if err != nil {
		return err
	}
	_, err = n.runtime.StartContainer(ctx, cID, n.cfg)
	return err
}

func (n *ceos) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for Arista cEOS '%s' node", n.cfg.ShortName)
	return n.ceosPostDeploy()
}

func (*ceos) WithMgmtNet(*types.MgmtNet)               {}
func (n *ceos) WithRuntime(r runtime.ContainerRuntime) { n.runtime = r }
func (n *ceos) GetRuntime() runtime.ContainerRuntime   { return n.runtime }

func (n *ceos) SaveConfig(ctx context.Context) error {
	_, stderr, err := n.runtime.Exec(ctx, n.cfg.LongName, saveCmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", n.cfg.ShortName, err)
	}

	if len(stderr) > 0 {
		return fmt.Errorf("%s errors: %s", n.cfg.ShortName, string(stderr))
	}

	confPath := n.cfg.LabDir + "/flash/startup-config"
	log.Infof("saved cEOS configuration from %s node to %s\n", n.cfg.ShortName, confPath)

	return nil
}

func (n *ceos) createCEOSFiles() error {
	nodeCfg := n.Config()
	// generate config directory
	utils.CreateDirectory(path.Join(n.cfg.LabDir, "flash"), 0777)
	cfg := filepath.Join(n.cfg.LabDir, "flash", "startup-config")
	nodeCfg.ResStartupConfig = cfg

	// set mgmt ipv4 gateway as it is already known by now
	// since the container network has been created before we launch nodes
	// and mgmt gateway can be used in ceos.cfg template to configure default route for mgmt
	nodeCfg.MgmtIPv4Gateway = n.runtime.Mgmt().IPv4Gw
	nodeCfg.MgmtIPv6Gateway = n.runtime.Mgmt().IPv6Gw

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

	err = nodeCfg.GenerateConfig(nodeCfg.ResStartupConfig, cfgTemplate)
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

// ceosPostDeploy runs postdeploy actions which are required for ceos nodes
func (n *ceos) ceosPostDeploy() error {
	nodeCfg := n.Config()
	d, err := utils.SpawnCLIviaExec("arista_eos", nodeCfg.LongName, n.runtime.GetName())
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

func (n *ceos) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: n.cfg.Image,
	}
}

func (n *ceos) Delete(ctx context.Context) error {
	return n.runtime.DeleteContainer(ctx, n.cfg.LongName)
}
