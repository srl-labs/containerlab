package ciscoiosxr

import (
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// AbortConfig is the platform implementation of AbortConfig.
func (p *Platform) AbortConfig() (*response.PlatformResponse, error) {
	err := p.conn.AcquirePriv(p.configPriv)
	if err != nil {
		return nil, err
	}

	err = p.conn.Channel.WriteAndReturn([]byte("abort"), false)
	if err != nil {
		return nil, err
	}

	p.conn.CurrentPriv = "privilege-exec"

	p.inProgress = false

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: []*cresponse.Response{},
	}, nil
}
