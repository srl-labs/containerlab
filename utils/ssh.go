package utils

import (
	"os/exec"
	"regexp"

	log "github.com/sirupsen/logrus"
)

// GetSSHVersion returns the version of the ssh client
// that is installed on the host.
func GetSSHVersion() string {
	cmd := exec.Command("ssh", "-V")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to get ssh client version: %v", err)
	}

	version := parseSSHVersion(string(out))

	return version
}

func parseSSHVersion(in string) string {
	re := regexp.MustCompile(`OpenSSH_(\d+\.\d+).+`)
	match := re.FindStringSubmatch(in)

	if len(match) < 2 {
		log.Warnf("Failed to parse ssh version from string: %s", in)
		return ""
	}

	return match[1]
}
