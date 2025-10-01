package juniperjunos

import (
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// GetDeviceDiff is the platform implementation of GetDeviceDiff.
func (p *Platform) GetDeviceDiff(_ string) (*response.PlatformResponse, error) {
	r, err := p.conn.SendConfig("show | compare")
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           r.Result,
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
