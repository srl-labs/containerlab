package ciscoiosxr

import (
	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

func (p *Platform) getDiffCommand() string {
	if p.replace {
		return "show configuration changes diff"
	}

	return "show commit changes diff"
}

// GetDeviceDiff is the platform implementation of GetDeviceDiff.
func (p *Platform) GetDeviceDiff(source string) (*response.PlatformResponse, error) {
	_ = source

	r, err := p.conn.SendConfig(
		p.getDiffCommand(),
		opoptions.WithPrivilegeLevel(p.configPriv),
	)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           r.Result,
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
