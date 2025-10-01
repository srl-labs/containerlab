package aristaeos

import (
	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// GetDeviceDiff is the platform implementation of GetDeviceDiff.
func (p *Platform) GetDeviceDiff(source string) (*response.PlatformResponse, error) {
	_ = source

	r, err := p.conn.SendConfig(
		"show session-config diffs",
		opoptions.WithPrivilegeLevel(p.candidateS),
	)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           r.Result,
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
