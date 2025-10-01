package aristaeos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

// GetConfig is the platform implementation of GetConfig.
func (p *Platform) GetConfig(source string) (*response.PlatformResponse, error) {
	cmd, ok := p.configCommands[source]
	if !ok {
		return nil, fmt.Errorf("%w: config source '%s' invalid", util.ErrBadSource, source)
	}

	r, err := p.conn.SendCommand(cmd)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           r.Result,
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
