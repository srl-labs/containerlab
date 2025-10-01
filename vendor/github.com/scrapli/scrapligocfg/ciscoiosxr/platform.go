package ciscoiosxr

import (
	"errors"
	"fmt"
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
			version: regexp.MustCompile(`(?i)\d+\.\d+\.\d+`),
			bannerDelim: regexp.MustCompile(
				`(?im)(^banner\s(?:exec|incoming|login|motd|prompt-timeout|slip-ppp)\s(.))`,
			),
			timestamp: regexp.MustCompile(
				`(?im)^(mon|tue|wed|thur|fri|sat|sun)\s+` +
					`(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)` +
					`\s+\d+\s+\d+:\d+:\d+((\.\d+\s\w+)|\s\d+)$`,
			),
			buildConfig:   regexp.MustCompile(`(?im)(^building configuration\.{3}$)`),
			configVersion: regexp.MustCompile(`(?im)(^!! ios xr.*$)`),
			configChange:  regexp.MustCompile(`(?im)(^!! last config.*$)`),
			end:           regexp.MustCompile(`end$`),
		}

		patternsInst.outputHeader = regexp.MustCompile(
			fmt.Sprintf(
				`(?im)%s|%s|%s|%s`,
				patternsInst.timestamp.String(),
				patternsInst.buildConfig.String(),
				patternsInst.configVersion.String(),
				patternsInst.configChange.String(),
			),
		)
	})

	return patternsInst
}

type patterns struct {
	version       *regexp.Regexp
	bannerDelim   *regexp.Regexp
	timestamp     *regexp.Regexp
	buildConfig   *regexp.Regexp
	configVersion *regexp.Regexp
	configChange  *regexp.Regexp
	outputHeader  *regexp.Regexp
	end           *regexp.Regexp
}

// NewCiscoIOSXR returns an CiscoIOSXR Platform instance.
func NewCiscoIOSXR(conn *network.Driver, opts ...util.Option) (*Platform, error) {
	p := &Platform{
		conn:     conn,
		patterns: getPatterns(),
		configCommands: map[string]string{
			"running": "show running-config",
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

// Platform is the CiscoIOSXR platform implementation.
type Platform struct {
	conn           *network.Driver
	patterns       *patterns
	configCommands map[string]string
	replace        bool
	inProgress     bool
	configPriv     string
}
