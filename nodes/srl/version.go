package srl

import (
	"context"
	"os"
	"regexp"

	_ "embed"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/mod/semver"
)

//go:embed version_configs/snmpv2.cfg
var snmpv2Config string

//go:embed version_configs/snmpv2_pre24_3.cfg
var snmpv2ConfigPre24_3 string

//go:embed version_configs/grpc_pre24_3.cfg
var grpcConfigPre24_3 string

//go:embed version_configs/acl.cfg
var aclConfig string

//go:embed version_configs/grpc.cfg
var grpcConfig string

var (
	netconfConfig = `set / system netconf-server mgmt admin-state enable ssh-server mgmt-netconf
set / system ssh-server mgmt-netconf admin-state enable
set / system ssh-server mgmt-netconf network-instance mgmt
set / system ssh-server mgmt-netconf port 830
set / system ssh-server mgmt-netconf disable-shell true
`

	ocServerConfig = `set / system management openconfig admin-state enable`

	ndkServerConfig = `set / system ndk-server admin-state enable`
)

// SrlVersion represents an sr linux version as a set of fields.
type SrlVersion struct {
	Major  string
	Minor  string
	Patch  string
	Build  string
	Commit string
}

// RunningVersion gets the software version of the running node
// by executing the "info from state /system information version | grep version" command
// and parsing the output.
func (n *srl) RunningVersion(ctx context.Context) (*SrlVersion, error) {
	cmd, _ := clabexec.NewExecCmdFromString(
		`sr_cli -d "info from state /system information version | grep version"`,
	)

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	log.Debugf("SR Linux node %s extracted raw version. stdout: %s, stderr: %s",
		n.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return n.parseVersionString(execResult.GetStdOutString()), nil
}

func (*srl) parseVersionString(s string) *SrlVersion {
	re := regexp.MustCompile(`v(\d{1,3})\.(\d{1,2})\.(\d{1,3})\-(\d{1,4})\-(\S+)`)

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
	return "v" + v.Major + "." + v.Minor + "." + v.Patch + "-" + v.Build + "-" + v.Commit
}

// MajorMinorSemverString returns a string representation of the major.minor version with a leading
// v.
func (v *SrlVersion) MajorMinorSemverString() string {
	return "v" + v.Major + "." + v.Minor
}

// setVersionSpecificParams sets version specific parameters in the template data struct
// to enable/disable version-specific configuration blocks in the config template
// or prepares data to conform to the expected format per specific version.
func (n *srl) setVersionSpecificParams(tplData *srlTemplateData) {
	// v is in the vMajor.Minor format
	v := n.swVersion.MajorMinorSemverString()

	// in srlinux >= v23.10+ linuxadmin and admin user ssh keys can only be configured via the cli
	// so we add the keys to the template data for rendering.
	if len(n.sshPubKeys) > 0 && (semver.Compare(v, "v23.10") >= 0 || n.swVersion.Major == "0") {
		tplData.SSHPubKeys = clabutils.MarshalAndCatenateSSHPubKeys(n.sshPubKeys)
	}

	// in srlinux >= v24.3+ we add ACL rules to enable http and telnet access
	// that are useful for labs and were removed as a security hardening measure.
	if semver.Compare(v, "v24.3") >= 0 || n.swVersion.Major == "0" {
		tplData.ACLConfig = aclConfig
	}

	// in srlinux >= v24.7+ we add Netconf server config to enable Netconf.
	if semver.Compare(v, "v24.7") >= 0 || n.swVersion.Major == "0" {
		tplData.NetconfConfig = netconfConfig
	}

	// in srlinux v23.10.x we need to enable GNMI unix socket services to enable
	// communications over unix socket (e.g. NDK agents)
	if semver.Compare(v, "v23.10") == 0 {
		tplData.EnableGNMIUnixSockServices = true
	}

	// in versions < v24.3 (or non 0.0 versions) we have to use the version specific
	// config for grpc and snmpv2
	if semver.Compare(v, "v24.3") == -1 && n.swVersion.Major != "0" {
		tplData.SNMPConfig = snmpv2ConfigPre24_3

		tplData.GRPCConfig = grpcConfigPre24_3
	}

	// in srlinux >= v24.10+ we add
	// - EDA configuration
	// - openconfig server enable
	if semver.Compare(v, "v24.10") >= 0 || n.swVersion.Major == "0" {
		cfg := edaDiscoveryServerConfig

		if os.Getenv("CLAB_EDA_USE_DEFAULT_GRPC_SERVER") != "" {
			cfg = cfg + "\n" + edaDefaultMgmtServerConfig
		} else {
			cfg = cfg + "\n" + edaCustomMgmtServerConfig
		}

		tplData.EDAConfig = cfg

		tplData.OCServerConfig = ocServerConfig
	}

	// in srlinux >= v25.3 we enable ndk server.
	if semver.Compare(v, "v25.3") >= 0 || n.swVersion.Major == "0" {
		tplData.NDKServerConfig = ndkServerConfig
	}
}
