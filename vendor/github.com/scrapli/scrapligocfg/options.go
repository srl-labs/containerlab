package scrapligocfg

import (
	"log"

	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

// WithDedicated sets the Dedicated option -- this options means that scrapligocfg can safely assume
// that the scrapligo connection is "dedicated" to the scrapligocfg object, and it can be opened and
// closed (rather than left open for subsequent use).
func WithDedicated() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*Cfg)

		if ok {
			c.Dedicated = true

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithCandidateName sets a preferred candidate configuration name.
func WithCandidateName(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*Cfg)

		if ok {
			c.CandidateName = s

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithCandidateTimestamp enables appending a unix timestamp to the candidate configuration name.
func WithCandidateTimestamp() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*Cfg)

		if ok {
			c.CandidateTimestamp = true

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithOnPrepare sets an "OnPrepare" function that will be executed during the "prepare" phase.
func WithOnPrepare(f func(*network.Driver) error) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*Cfg)

		if ok {
			c.OnPrepare = f

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithFilesystem sets the filesystem to write candidate configurations to for those platforms that
// implement WriteToFSPlatform.
func WithFilesystem(s string) util.Option {
	return func(o interface{}) error {
		p, ok := o.(WriteToFSPlatform)

		if ok {
			p.SetFilesystem(s)

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithFilesystemSpaceAvailBuffPerc sets the filesystem space buffer percent -- or the amount of
// "wiggle room" to leave in a filesystem when determining available space. This is only applicable
// for platforms that satisfy WriteToFSPlatform.
func WithFilesystemSpaceAvailBuffPerc(f float32) util.Option {
	return func(o interface{}) error {
		p, ok := o.(WriteToFSPlatform)

		if ok {
			p.SetSpaceAvailBuffPerc(f)

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithDiffColorize sets the diff colorization to true.
func WithDiffColorize() util.Option {
	return func(o interface{}) error {
		p, ok := o.(*response.DiffResponse)

		if ok {
			p.Colorize = true

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithDiffSideBySideW sets the character width for each column in side-by-side diffs.
func WithDiffSideBySideW(i int) util.Option {
	return func(o interface{}) error {
		p, ok := o.(*response.DiffResponse)

		if ok {
			p.SideBySideW = i

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithLogger accepts a logging.Instance and applies it to the driver object.
func WithLogger(l *logging.Instance) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*Cfg)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.Logger = l

		return nil
	}
}

// WithDefaultLogger applies the default logging setup to a driver object. This means log.Print
// for the logger function, and "info" for the log level.
func WithDefaultLogger() util.Option {
	return func(o interface{}) error {
		d, ok := o.(*Cfg)

		if !ok {
			return util.ErrIgnoredOption
		}

		l, err := logging.NewInstance(
			logging.WithLevel("info"),
			logging.WithLogger(log.Print),
		)
		if err != nil {
			return err
		}

		d.Logger = l

		return nil
	}
}
