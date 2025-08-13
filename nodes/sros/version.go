package sros

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
)

var (
	//go:embed configs/snmpv2.cfg
	snmpv2Config string

	//go:embed configs/grpc.cfg
	grpcConfig string

	//go:embed configs/grpc_ixr.cfg
	grpcConfigIXR string

	//go:embed configs/netconf.cfg
	netconfConfig string

	//go:embed configs/logging.cfg
	loggingConfig string

	//go:embed configs/system.cfg
	systemCfg string

	//go:embed configs/system_ixr.cfg
	systemCfgIXR string

	//go:embed configs/ssh.cfg
	sshConfig string
)

// SrosVersion represents an SR-OS version as a set of fields.
type SrosVersion struct {
	Major string
	Minor string
	Build string
}

// RunningVersion gets the software version of the running node
// by executing the "cat /etc/sros-version" command
// and parsing the output.
func (n *sros) RunningVersion(ctx context.Context) (*SrosVersion, error) {
	cmd, _ := clabexec.NewExecCmdFromString(`cat /etc/sros-version`)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	log.Debug("Extracted raw SR OS version",
		"node", n.Cfg.ShortName, "stdout", execResult.GetStdOutString(), "stderr", execResult.GetStdErrString())

	return n.parseVersionString(execResult.GetStdOutString()), nil
}

func (*sros) parseVersionString(s string) *SrosVersion {
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.([A-Za-z0-9]+)`)

	v := re.FindStringSubmatch(s)
	// 4 matches must be returned if all goes well
	if len(v) != 4 {
		// return all zeroes if failed to parse
		return &SrosVersion{"0", "0", "0"}
	}
	return &SrosVersion{v[1], v[2], v[3]}
}

// String returns a string representation of the version in a semver fashion (with leading v).
func (v *SrosVersion) String() string {
	return fmt.Sprintf("v%s.%s.%s", v.Major, v.Minor, v.Build)
}

// MajorMinorSemverString returns a string representation of the major.minor version with a leading v.
func (v *SrosVersion) MajorMinorSemverString() string {
	return fmt.Sprintf("v%s.%s", v.Major, v.Minor)
}
