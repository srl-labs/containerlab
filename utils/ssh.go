package utils

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"
)

// GetSSHVersion returns the version of the ssh client
// that is installed on the host.
func GetSSHVersion(ctx context.Context) string {
	cmd := exec.Command("ssh", "-V")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.FromContext(ctx).Warnf("Failed to get ssh client version: %v", err)
	}

	version := parseSSHVersion(ctx, string(out))

	return version
}

func parseSSHVersion(ctx context.Context, in string) string {
	re := regexp.MustCompile(`OpenSSH_(\d+\.\d+).+`)
	match := re.FindStringSubmatch(in)

	if len(match) < 2 {
		log.FromContext(ctx).Warnf("Failed to parse ssh version from string: %s", in)
		return ""
	}

	return match[1]
}

// MarshalSSHPubKeys marshals the ssh public keys
// and a string slice that contains string representations of the keys.
func MarshalSSHPubKeys(in []ssh.PublicKey) []string {
	r := []string{}

	for _, k := range in {
		// extract the keys in AuthorizedKeys format (e.g. "ssh-rsa <KEY>")
		ks := bytes.TrimSpace(ssh.MarshalAuthorizedKey(k))
		r = append(r, string(ks))
	}

	return r
}

// MarshalAndCatenateSSHPubKeys catenates the ssh public keys
// and produces a string that can be used in the
// cli config command to set the ssh public keys
// for users.
// Each key value in the catenated string will be double quoted.
func MarshalAndCatenateSSHPubKeys(in []ssh.PublicKey) string {
	keysSlice := MarshalSSHPubKeys(in)
	quotedKeys := make([]string, len(keysSlice))

	for i, k := range keysSlice {
		quotedKeys[i] = "\"" + k + "\""
	}

	return strings.Join(quotedKeys, " ")
}
