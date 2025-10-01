package cisconxos

import (
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// GetVersion is the platform implementation of GetVersion.
func (p *Platform) GetVersion() (*response.PlatformResponse, error) {
	r, err := p.conn.SendCommand("show version | i \"NXOS: version\"")
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           p.patterns.version.FindString(r.Result),
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
