package aristaeos

import (
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// AbortConfig is the platform implementation of AbortConfig.
func (p *Platform) AbortConfig() (*response.PlatformResponse, error) {
	err := p.conn.AcquirePriv(p.candidateS)
	if err != nil {
		return nil, err
	}

	err = p.conn.Channel.WriteAndReturn([]byte("abort"), false)
	if err != nil {
		return nil, err
	}

	p.conn.CurrentPriv = "privilege-exec"

	err = p.DeRegisterConfigSession(p.candidateS)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: []*cresponse.Response{},
	}, nil
}
