package ciscoiosxe

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
			version: regexp.MustCompile(`(?i)\d+\.[a-z\d().]+`),
			bytesFree: regexp.MustCompile(
				`(?i)(?P<bytes_available>\d+)(?: bytes free)`,
			),
			outputHeader: regexp.MustCompile(`(?im)(^version \d+\.\d+$)`),
		}
	})

	return patternsInst
}

type patterns struct {
	version      *regexp.Regexp
	bytesFree    *regexp.Regexp
	outputHeader *regexp.Regexp
}

// NewCiscoIOSXE returns an CiscoIOSXE Platform instance.
func NewCiscoIOSXE(conn *network.Driver, opts ...util.Option) (*Platform, error) {
	p := &Platform{
		conn:       conn,
		patterns:   getPatterns(),
		filesystem: "flash:",
		configCommands: map[string]string{
			"running": "show running-config",
			"startup": "show startup-config",
		},
		spaceAvailBuffPerc: 10,
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

// Platform is the CiscoIOSXE platform implementation.
type Platform struct {
	conn               *network.Driver
	patterns           *patterns
	filesystem         string
	configCommands     map[string]string
	spaceAvailBuffPerc float32
	candidateF         string
	replace            bool
}

// SetFilesystem sets the target filesystem for the Platform object.
func (p *Platform) SetFilesystem(s string) {
	p.filesystem = s
}

// SetSpaceAvailBuffPerc sets the filesystem space available buffer percent for the Platform object.
func (p *Platform) SetSpaceAvailBuffPerc(f float32) {
	p.spaceAvailBuffPerc = f
}
