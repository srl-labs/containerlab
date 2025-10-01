package juniperjunos

import (
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// GetVersion is the platform implementation of GetVersion.
func (p *Platform) GetVersion() (*response.PlatformResponse, error) {
	cmd := "show version"

	var r *cresponse.Response

	var err error

	if p.inProgress {
		cmd = "run " + cmd

		r, err = p.conn.SendConfig(cmd)
	} else {
		r, err = p.conn.SendCommand("show version")
	}

	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           p.patterns.version.FindString(r.Result),
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
