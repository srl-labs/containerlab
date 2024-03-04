// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cvsr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
)

var (
	defaultCredentials = nodes.NewCredentials("admin", "admin")
)

var kindnames = []string{"cvsr", "nokia_cvsr"}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(Cvsr)
	}, defaultCredentials)
}

type Cvsr struct {
	nodes.DefaultNode
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
}

func (n *Cvsr) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.LicensePolicy = types.LicensePolicyWarn
	// SR OS requires unbound pubkey authentication mode until this is
	// gets fixed in later SR OS relase.
	n.SSHConfig.PubkeyAuthentication = types.PubkeyAuthValueUnbound

	return nil
}

func (s *Cvsr) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)

	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	// store public keys extracted from clab host
	s.sshPubKeys = params.SSHPubKeys

	err = s.createCVSRFiles()
	if err != nil {
		return err
	}
	return nil
}

func (n *Cvsr) createCVSRFiles() error {
	log.Debugf("Creating directory structure for VSR container: %s", n.Cfg.ShortName)

	// if user was not initialized to a value, use root
	if n.Cfg.User == "" {
		n.Cfg.User = "0:0"
	}

	// create and mount root/run/cfx dirs
	for _, x := range []string{"cf1", "cf2", "cf3"} {
		dir := filepath.Join(n.Cfg.LabDir, x)
		utils.CreateDirectory(dir, 0655)
		n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/root/run/%s", dir, x))
	}

	if n.Cfg.License != "" {
		if !utils.FileExists(n.Cfg.License) {
			log.Errorf("license file for node %s does not exist", n.GetShortName())
		} else {
			// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
			err := utils.CopyFile(n.Cfg.License, filepath.Join(n.Cfg.LabDir, "cf3", "license.txt"), 0644)
			if err != nil {
				return err
			}
		}
	}

	// mount /usr/lib/firmware
	s, err := os.Stat("/usr/lib/firmware")
	if err == nil && s.IsDir() {
		n.Cfg.Binds = append(n.Cfg.Binds, "/usr/lib/firmware:/usr/lib/firmware:ro")
	}
	// mount /dev/hugepages
	s, err = os.Stat("/dev/hugepages")
	if err == nil && s.IsDir() {
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev/hugepages:/dev/hugepages:rw")
	}

	vsrConf := filepath.Join(n.Cfg.LabDir, "vsr.conf")
	data := fmt.Sprintf(`mgmtIf=eth0
dpdk=1;DPDK_DEVS
dpdkHugeDir=/dev/hugepages
cfDirs=/home/sros/flash1;/home/sros/flash2;/home/sros/flash3
logDir=/root/run/log
bootString=TIMOS: name=%s slot=a chassis=VSR-I card=cpm-v/iom-v mda/1=m20-v mda/2=m20-v control-cpu-cores=2 features=2048
cpusAllowedList=
`, n.Cfg.ShortName)
	err = os.WriteFile(vsrConf, []byte(data), 0644)
	if err != nil {
		return err
	}

	// mount config file
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/regressbed/images/dut-%s/%s.0.cfg:rw", vsrConf, n.GetShortName(), n.GetShortName()))

	// mount bof
	vsrBof := filepath.Join(n.Cfg.LabDir, "vsr.bof")

	_, err = os.Stat(vsrBof)
	if err != nil {
		// create an empty file for now
		os.WriteFile(vsrBof, []byte(""), 0655)
	}
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/regressbed/images/dut-%s/bof.cfg", vsrBof, n.GetShortName()))

	custEntryFile := filepath.Join(n.Cfg.LabDir, "CustEntry.sh")
	entryData := fmt.Sprint(`#!/bin/sh
until [ -f /root/run/cf3/bof_ready ]
do
	sleep 1
	echo "waiting for bof"
done
echo "executing entrypoint"
/root/run/entrypoint.sh`)
	err = os.WriteFile(custEntryFile, []byte(entryData), 0777)
	if err != nil {
		return err
	}

	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/CustEntry.sh:ro", custEntryFile))

	n.Cfg.Entrypoint = "/CustEntry.sh"

	vsrCliConf := filepath.Join(n.Cfg.LabDir, "cf3", "config.cfg")
	cliConfData := `exit all
configure
	system
		dns
			address-pref ipv6-first
		exit
		time
			sntp
				shutdown
			exit
			zone UTC
		exit
	exit
	system
		security
			telnet-server
			ftp-server
			snmp
				community private rwa version both
				community public r version both
			exit
		exit
	exit
	log
	exit
	card 1
		card-type iom-v
	`
	err = os.WriteFile(vsrCliConf, []byte(cliConfData), 0666)
	if err != nil {
		return err
	}

	return nil
}

func (n *Cvsr) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {

	bofFile := filepath.Join(n.Cfg.LabDir, "cf3", "bof.cfg")
	bofData := fmt.Sprintf(
		`primary-image    cf3:\
primary-config   cf3:\config.cfg
license-file     cf3:\license.txt
address          %s/%d
static-route     0.0.0.0/0 next-hop %s`, n.Cfg.MgmtIPv4Address, n.Cfg.MgmtIPv4PrefixLength, n.Cfg.MgmtIPv4Gateway)
	err := os.WriteFile(bofFile, []byte(bofData), 0644)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(n.Cfg.LabDir, "cf3", "bof_ready"), []byte{}, 0644)
	if err != nil {
		return err
	}

	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (s *Cvsr) CheckInterfaceName() error {
	ifRe := regexp.MustCompile(`net\d+|eth\d+`)
	for _, e := range s.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("nokia vsr linux interface name %q doesn't match the required pattern. VSR interfaces should be named as net<x>", e.GetIfaceName())
		}
	}

	return nil
}
