package srl

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

const banner = `................................................................
:                  Welcome to Nokia SR Linux!                  :
:              Open Network OS for the NetOps era.             :
:                                                              :
:    This is a freely distributed official container image.    :
:                    Use it - Share it                         :
:                                                              :
: Get started: https://learn.srlinux.dev                       :
: Container:   https://go.srlinux.dev/container-image          :
: Docs:        https://doc.srlinux.dev/%s-%s                   :
: Rel. notes:  https://doc.srlinux.dev/rn%s-%s-%s               :
: Discord:     https://discord.gg/tZvgjQ6PZf                   :
: Contact:     https://go.srlinux.dev/contact-sales            :
................................................................
`

// banner returns a banner string with a docs version filled in based on the version information queried from the node
func (s *srl) banner(ctx context.Context) (string, error) {
	stdout, stderr, err := s.runtime.Exec(ctx, s.cfg.LongName, []string{
		"sr_cli", "-d", "info from state /system information version | grep version",
	})

	if err != nil {
		return "", err
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", s.cfg.ShortName, stdout, stderr)

	v := s.parseVersionString(string(stdout))

	// massage minor version to have extra whitespace if it is a single digit
	// this is needed to have banner table aligned nicely
	if len(v.minor) == 1 {
		v.minor = v.minor + " "
	}

	b := fmt.Sprintf(banner, v.major, v.minor, v.major, v.minor, v.patch)

	return b, nil
}
