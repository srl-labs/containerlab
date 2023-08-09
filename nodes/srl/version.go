package srl

import (
	"context"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
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
