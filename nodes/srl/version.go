package srl

import (
	"context"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"golang.org/x/mod/semver"
)

const (
	snmpv2Config = `set / system snmp access-group SNMPv2-RO-Community security-level no-auth-no-priv community-entry RO-Community community public
set / system snmp network-instance mgmt admin-state enable`

	snmpv2ConfigPre24_3 = `set / system snmp community public
set / system snmp network-instance mgmt admin-state enable`

	// grpcConfigPre24_3 contains the gnmi server configuration for srlinux versions < 24.3.
	grpcConfigPre24_3 = `set / system gnmi-server admin-state enable network-instance mgmt admin-state enable tls-profile clab-profile
set / system gnmi-server rate-limit 65000
set / system gnmi-server trace-options [ request response common ]
set / system gnmi-server unix-socket admin-state enable`

	// grpc contains the grpc server(s) configuration for srlinux versions >= 24.3.
	grpcConfig = `set / system grpc-server clab services [ gnmi gnoi gribi p4rt ]
set / system grpc-server clab tls-profile clab-profile
set / system grpc-server clab rate-limit 65000
set / system grpc-server clab network-instance mgmt
set / system grpc-server clab trace-options [ request response common ]
set / system grpc-server clab unix-socket admin-state enable
set / system grpc-server clab admin-state enable`
)

// SrlVersion represents an sr linux version as a set of fields.
type SrlVersion struct {
	major  string
	minor  string
	patch  string
	build  string
	commit string
}

// RunningVersion gets the software version of the running node
// by executing the "info from state /system information version | grep version" command
// and parsing the output.
func (n *srl) RunningVersion(ctx context.Context) (*SrlVersion, error) {
	cmd, _ := exec.NewExecCmdFromString(`sr_cli -d "info from state /system information version | grep version"`)

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", n.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return n.parseVersionString(execResult.GetStdOutString()), nil
}

func (n *srl) parseVersionString(s string) *SrlVersion {
	re, _ := regexp.Compile(`v(\d{1,3})\.(\d{1,2})\.(\d{1,3})\-(\d{1,4})\-(\S+)`)

	v := re.FindStringSubmatch(s)
	// 6 matches must be returned if all goes well
	if len(v) != 6 {
		// return all zeroes if failed to parse
		return &SrlVersion{"0", "0", "0", "0", "0"}
	}

	return &SrlVersion{v[1], v[2], v[3], v[4], v[5]}
}

// String returns a string representation of the version in a semver fashion (with leading v).
func (v *SrlVersion) String() string {
	return "v" + v.major + "." + v.minor + "." + v.patch + "-" + v.build + "-" + v.commit
}

// MajorMinorSemverString returns a string representation of the major.minor version with a leading v.
func (v *SrlVersion) MajorMinorSemverString() string {
	return "v" + v.major + "." + v.minor
}

// setVersionSpecificParams sets version specific parameters in the template data struct
// to enable/disable version-specific configuration blocks in the config template
// or prepares data to conform to the expected format per specific version.
func (n *srl) setVersionSpecificParams(tplData *srlTemplateData) {
	// v is in the vMajor.Minor format
	v := n.swVersion.MajorMinorSemverString()

	// in srlinux >= v23.10+ linuxadmin and admin user ssh keys can only be configured via the cli
	// so we add the keys to the template data for rendering.
	if len(n.sshPubKeys) > 0 && (semver.Compare(v, "v23.10") >= 0 || n.swVersion.major == "0") {
		tplData.SSHPubKeys = catenateKeys(n.sshPubKeys)
	}

	// in srlinux v23.10.x we need to enable GNMI unix socket services to enable
	// communications over unix socket (e.g. NDK agents)
	if semver.Compare(v, "v23.10") == 0 {
		tplData.EnableGNMIUnixSockServices = true
	}

	// in versions < v24.3 (or non 0.0 versions) we have to use the version specific
	// config for grpc and snmpv2
	if semver.Compare(v, "v24.3") == -1 && n.swVersion.major != "0" {
		tplData.SNMPConfig = snmpv2ConfigPre24_3

		tplData.GRPCConfig = grpcConfigPre24_3
	}
}
