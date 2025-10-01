package cisconxos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// GetDeviceDiff is the platform implementation of GetDeviceDiff.
func (p *Platform) GetDeviceDiff(source string) (*response.PlatformResponse, error) {
	_ = source

	if !p.replace {
		// only can diff if we are replacing
		return &response.PlatformResponse{}, nil
	}

	r, err := p.conn.SendCommand(
		fmt.Sprintf(
			"show diff rollback-patch %s-config file %s%s",
			source,
			p.filesystem,
			p.candidateF,
		),
	)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           r.Result,
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
