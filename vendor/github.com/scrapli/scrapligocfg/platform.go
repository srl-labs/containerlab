package scrapligocfg

import (
	"fmt"
	"os"

	"github.com/scrapli/scrapligo/logging"

	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

const (
	// GetVersion is the name of the get version operation.
	GetVersion = "GetVersion"
	// GetConfig is the name of the get config operation.
	GetConfig = "GetConfig"
	// LoadConfig is the name of the load config operation.
	LoadConfig = "LoadConfig"
	// CommitConfig is the name of the commit config operation.
	CommitConfig = "CommitConfig"
	// AbortConfig is the name of the abort config operation.
	AbortConfig = "AbortConfig"
)

// Platform defines the required methods that a scrapligocfg "platform" needs to implement.
type Platform interface {
	GetVersion() (*response.PlatformResponse, error)
	GetConfig(source string) (*response.PlatformResponse, error)
	LoadConfig(
		f, config string,
		replace bool,
		options *util.OperationOptions,
	) (*response.PlatformResponse, error)
	AbortConfig() (*response.PlatformResponse, error)
	CommitConfig() (*response.PlatformResponse, error)
	GetDeviceDiff(source string) (*response.PlatformResponse, error)
	NormalizeConfig(config string) string
	Cleanup() error
}

// WriteToFSPlatform defines additional Platform methods for those platforms that have candidate
// configurations written to the filesystem.
type WriteToFSPlatform interface {
	Platform
	SetFilesystem(s string)
	SetSpaceAvailBuffPerc(f float32)
}

// Cfg is the primary point of interaction for scrapligocfg users -- this struct wraps the target
// Platform implementation and provides a consistent look and feel for users regardless of the
// underlying device type.
type Cfg struct {
	Logger *logging.Instance

	Impl Platform
	Conn *network.Driver

	OnPrepare func(*network.Driver) error

	Dedicated bool

	prepared bool

	Candidate          string
	CandidateName      string
	CandidateTimestamp bool
}

func (c *Cfg) open() error {
	if c.Conn.Transport.IsAlive() {
		return nil
	}

	if c.Dedicated {
		return c.Conn.Open()
	}

	return fmt.Errorf(
		"%w: core connection not open and dedicated is false",
		util.ErrConnectionError,
	)
}

// Prepare opens the scrapligo Conn object and executes the optional OnPrepare function.
func (c *Cfg) Prepare() error {
	err := c.open()
	if err != nil {
		return err
	}

	if c.OnPrepare != nil {
		err = c.OnPrepare(c.Conn)
		if err != nil {
			return nil
		}
	}

	c.prepared = true

	return nil
}

func (c *Cfg) close() error {
	var err error

	if c.Dedicated && c.Conn.Transport.IsAlive() {
		err = c.Conn.Close()
	}

	return err
}

// Cleanup executes the Platform implementations Cleanup method and closes the  scrapligo Conn.
func (c *Cfg) Cleanup() error {
	err := c.Impl.Cleanup()
	if err != nil {
		return err
	}

	c.prepared = false
	c.Candidate = ""

	return c.close()
}

// GetVersion captures target device version information and stores it in the response.Response.
func (c *Cfg) GetVersion() (*response.Response, error) {
	c.Logger.Info("GetVersion requested")

	r := response.NewResponse(GetVersion, c.Conn.Transport.GetHost())

	pr, err := c.Impl.GetVersion()
	if err != nil {
		return nil, err
	}

	r.Record(pr.ScrapliResponses, pr.Result)

	return r, nil
}

// GetConfig fetches the source configuration from the target device -- the source is usually one
// of 'running', 'startup', or 'candidate', but valid options may vary from platform to platform.
func (c *Cfg) GetConfig(source string) (*response.Response, error) {
	c.Logger.Infof("GetConfig requested, source '%s'", source)

	if !c.prepared {
		return nil, fmt.Errorf(
			"%w: connection not prepared, must call `Prepare`",
			util.ErrPrepareError,
		)
	}

	r := response.NewResponse(GetConfig, c.Conn.Transport.GetHost())

	pr, err := c.Impl.GetConfig(source)
	if err != nil {
		return nil, err
	}

	r.Record(pr.ScrapliResponses, pr.Result)

	return r, nil
}

// LoadConfig loads a candidate configuration 'config' onto the target device. The replace argument
// is required -- when set to 'true' this means that scrapligocfg will load the provided candidate
// config in "replace" mode, which, when "committed" will fully replace the devices target config.
// If replace is false, the configuration will be loaded as a "merge" mode.
func (c *Cfg) LoadConfig(
	config string,
	replace bool,
	opts ...util.Option,
) (*response.Response, error) {
	c.Logger.Infof("LoadConfig requested, replace '%v'", replace)

	if !c.prepared {
		return nil, fmt.Errorf(
			"%w: connection not prepared, must call `Prepare`",
			util.ErrPrepareError,
		)
	}

	if c.Candidate != "" {
		return nil, fmt.Errorf(
			"%w: candidate config already present, abort or commit before proceeding",
			util.ErrCandidateError,
		)
	}

	c.Candidate = config

	r := response.NewResponse(LoadConfig, c.Conn.Transport.GetHost())

	op, err := util.NewOperationOptions(opts...)
	if err != nil {
		return nil, err
	}

	pr, err := c.Impl.LoadConfig(
		util.CreateCandidateConfigName(c.CandidateName, c.CandidateTimestamp),
		config,
		replace,
		op,
	)
	if err != nil {
		return nil, err
	}

	r.Record(pr.ScrapliResponses, pr.Result)

	return r, nil
}

// LoadConfigFromFile is a convenience method wrapping LoadConfig -- this method accepts a filepath
// which will be read and passed to LoadConfig.
func (c *Cfg) LoadConfigFromFile(
	f string,
	replace bool,
	opts ...util.Option,
) (*response.Response, error) {
	b, err := os.ReadFile(f) // nolint: gosec
	if err != nil {
		return nil, err
	}

	return c.LoadConfig(string(b), replace, opts...)
}

// AbortConfig aborts a loaded candidate configuration.
func (c *Cfg) AbortConfig() (*response.Response, error) { //nolint: dupl
	c.Logger.Info("AbortConfig requested")

	if !c.prepared {
		return nil, fmt.Errorf(
			"%w: connection not prepared, must call `Prepare`",
			util.ErrPrepareError,
		)
	}

	if c.Candidate == "" {
		return nil, fmt.Errorf(
			"%w: cannot abort config, candidate config is not set",
			util.ErrCandidateError,
		)
	}

	r := response.NewResponse(AbortConfig, c.Conn.Transport.GetHost())

	pr, err := c.Impl.AbortConfig()
	if err != nil {
		return nil, err
	}

	r.Record(pr.ScrapliResponses, pr.Result)

	c.Candidate = ""

	return r, nil
}

// CommitConfig commits a loaded candidate configuration.
func (c *Cfg) CommitConfig() (*response.Response, error) { //nolint: dupl
	c.Logger.Info("CommitConfig requested")

	if !c.prepared {
		return nil, fmt.Errorf(
			"%w: connection not prepared, must call `Prepare`",
			util.ErrPrepareError,
		)
	}

	if c.Candidate == "" {
		return nil, fmt.Errorf(
			"%w: cannot commit config, candidate config is not set",
			util.ErrCandidateError,
		)
	}

	r := response.NewResponse(CommitConfig, c.Conn.Transport.GetHost())

	pr, err := c.Impl.CommitConfig()
	if err != nil {
		return nil, err
	}

	r.Record(pr.ScrapliResponses, pr.Result)

	// reset the candidate config now that we've committed
	c.Candidate = ""

	return r, nil
}

// DiffConfig diffs the requested `source` config with the candidate config. Supports `WithDiff`
// options to modify the DiffResponse behavior.
func (c *Cfg) DiffConfig(source string, opts ...util.Option) (*response.DiffResponse, error) {
	c.Logger.Infof("DiffConfig requested, source '%s'", source)

	if !c.prepared {
		return nil, fmt.Errorf(
			"%w: connection not prepared, must call `Prepare`",
			util.ErrPrepareError,
		)
	}

	if c.Candidate == "" {
		return nil, fmt.Errorf(
			"%w: cannot diff config, candidate config is not set",
			util.ErrCandidateError,
		)
	}

	dr := response.NewDiffResponse(c.Conn.Transport.GetHost(), opts...)

	pr, err := c.Impl.GetDeviceDiff(source)
	if err != nil {
		return nil, err
	}

	deviceDiff := pr.Result

	dr.Record(pr.ScrapliResponses, pr.Result)

	pr, err = c.Impl.GetConfig(source)
	if err != nil {
		return nil, err
	}

	dr.Record(pr.ScrapliResponses, "")
	currentConfig := pr.Result

	dr.RecordDiff(
		deviceDiff,
		c.Impl.NormalizeConfig(c.Candidate),
		c.Impl.NormalizeConfig(currentConfig),
	)

	return dr, nil
}
