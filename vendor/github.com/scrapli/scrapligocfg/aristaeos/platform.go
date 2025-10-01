package aristaeos

import (
	"errors"
	"regexp"
	"sync"

	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligocfg/util"
)

var (
	patternsInst *patterns //nolint:gochecknoglobals
	patternsOnce sync.Once //nolint:gochecknoglobals
)

func getPatterns() *patterns {
	patternsOnce.Do(func() {
		patternsInst = &patterns{
			version:           regexp.MustCompile(`(?i)\d+\.\d+\.[a-z\d\-]+(\.\d+[a-z]?)?`),
			globalCommentLine: regexp.MustCompile(`(?im)^! .*$`),
			banner: regexp.MustCompile(
				`(?ims)^banner.*EOF$`,
			),
			end: regexp.MustCompile(`end$`),
		}
	})

	return patternsInst
}

type patterns struct {
	version           *regexp.Regexp
	globalCommentLine *regexp.Regexp
	banner            *regexp.Regexp
	end               *regexp.Regexp
}

// NewAristaEOS returns an AristaEOS Platform instance.
func NewAristaEOS(conn *network.Driver, opts ...util.Option) (*Platform, error) {
	p := &Platform{
		conn:     conn,
		patterns: getPatterns(),
		configCommands: map[string]string{
			"running": "show running-config",
			"startup": "show startup-config",
		},
	}

	for _, option := range opts {
		err := option(p)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return p, nil
}

// Platform is the AristaEOS platform implementation.
type Platform struct {
	conn           *network.Driver
	patterns       *patterns
	configCommands map[string]string
	candidateS     string
}
