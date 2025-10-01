package ciscoiosxr

import (
	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// GetVersion is the platform implementation of GetVersion.
func (p *Platform) GetVersion() (*response.PlatformResponse, error) {
	var r *cresponse.Response

	var err error

	if p.inProgress {
		r, err = p.conn.SendConfig(
			"show version | i Version",
			opoptions.WithPrivilegeLevel(p.configPriv),
		)
	} else {
		r, err = p.conn.SendCommand("show version | i Version")
	}

	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           p.patterns.version.FindString(r.Result),
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
