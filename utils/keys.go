package utils

import (
	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"
)

// LoadSSHPubKeysFromFiles parses openssh keys from the files referenced by the paths
// and returns a slice of ssh.PublicKey pointers.
// The files may contain multiple keys each on a separate line.
func LoadSSHPubKeysFromFiles(paths []string) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey

	for _, p := range paths {
		lines, err := FileLines(p, "#")
		if err != nil {
			return nil, err
		}

		for _, l := range lines {
			pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(l))

			log.Debugf("Loaded public key %s", l)

			if err != nil {
				return nil, err
			}

			keys = append(keys, pubKey)
		}
	}

	return keys, nil
}
