package cisconxos

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
			version: regexp.MustCompile(`(?i)\d+\.[a-z0-9\(\).]+`),
			bytesFree: regexp.MustCompile(
				`(?i)(?P<bytes_available>\d+)(?: bytes free)`,
			),
			buildConfig:    regexp.MustCompile(`(?im)(^!command:.*$)`),
			configVersion:  regexp.MustCompile(`(?im)(^!running configuration last done.*$)`),
			configChange:   regexp.MustCompile(`(?im)(^!! last config.*$)`),
			checkpointLine: regexp.MustCompile(`(?m)^\s*!#.*$`),
		}

		patternsInst.outputHeader = regexp.MustCompile(
			fmt.Sprintf(
				`(?im)%s|%s|%s`,
				patternsInst.buildConfig.String(),
				patternsInst.configVersion.String(),
				patternsInst.configChange.String(),
			),
		)
	})

	return patternsInst
}

type patterns struct {
	version        *regexp.Regexp
	bytesFree      *regexp.Regexp
	buildConfig    *regexp.Regexp
	configVersion  *regexp.Regexp
	configChange   *regexp.Regexp
	outputHeader   *regexp.Regexp
	checkpointLine *regexp.Regexp
}

// NewCiscoNXOS returns an CiscoNXOS Platform instance.
func NewCiscoNXOS(conn *network.Driver, opts ...util.Option) (*Platform, error) {
	p := &Platform{
		conn:       conn,
		patterns:   getPatterns(),
		filesystem: "bootflash:",
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

// Platform is the CiscoNXOS platform implementation.
type Platform struct {
	conn               *network.Driver
	patterns           *patterns
	filesystem         string
	configCommands     map[string]string
	spaceAvailBuffPerc float32
	replace            bool
	candidateF         string
}

// SetFilesystem sets the target filesystem for the Platform object.
func (p *Platform) SetFilesystem(s string) {
	p.filesystem = s
}

// SetSpaceAvailBuffPerc sets the filesystem space available buffer percent for the Platform object.
func (p *Platform) SetSpaceAvailBuffPerc(f float32) {
	p.spaceAvailBuffPerc = f
}
