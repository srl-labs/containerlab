package juniperjunos

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
			version:      regexp.MustCompile(`\d+\.[\w-]+\.\w+`),
			outputHeader: regexp.MustCompile(`(?im)^## last commit.*$\nversion.*$`),
			edit:         regexp.MustCompile(`(?m)^\[edit\]$`),
		}
	})

	return patternsInst
}

type patterns struct {
	version      *regexp.Regexp
	outputHeader *regexp.Regexp
	edit         *regexp.Regexp
}

// NewJuniperJunOS returns an JuniperJunOS Platform instance.
func NewJuniperJunOS(conn *network.Driver, opts ...util.Option) (*Platform, error) {
	p := &Platform{
		conn:       conn,
		filesystem: "/config/",
		patterns:   getPatterns(),
		configCommands: map[string]string{
			"running": "show configuration",
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

// Platform is the JuniperJunOS platform implementation.
type Platform struct {
	conn               *network.Driver
	patterns           *patterns
	filesystem         string
	configCommands     map[string]string
	spaceAvailBuffPerc float32
	replace            bool
	inProgress         bool
	configSetStyle     bool
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
