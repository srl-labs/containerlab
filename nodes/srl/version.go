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

// setVersionSpecificParams sets version specific parameters in the template data struct
// to enable/disable version-specific configuration blocks in the config template
// or prepares data to conform to the expected format per specific version.
func (n *srl) setVersionSpecificParams(tplData *srlTemplateData) {
	v := n.swVersion.String()

	// in srlinux >= v23.10+ linuxadmin and admin user ssh keys can only be configured via the cli
	// so we add the keys to the template data for rendering.
	if len(n.sshPubKeys) > 0 && (semver.Compare(v, "v23.10") >= 0 || n.swVersion.major == "0") {
		tplData.SSHPubKeys = catenateKeys(n.sshPubKeys)
	}

	// in srlinux v23.10+ till 24.3 we need to enable GNMI unix socket services to enable
	// communications over unix socket (e.g. NDK agents)
	if semver.Compare(v, "v23.10") >= 0 && semver.Compare(v, "v24.3") < 0 {
		tplData.EnableGNMIUnixSockServices = true
	}

	// in versions <= v24.3 SNMPv2 is done differently
	if semver.Compare(v, "v24.3") < 0 && n.swVersion.major != "0" {
		tplData.SNMPConfig = snmpv2ConfigPre24_3
	}
}
