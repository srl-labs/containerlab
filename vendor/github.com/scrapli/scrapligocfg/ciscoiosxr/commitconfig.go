package ciscoiosxr

import (
	"github.com/scrapli/scrapligo/driver/generic"
	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// CommitConfig is the platform implementation of CommitConfig.
func (p *Platform) CommitConfig() (*response.PlatformResponse, error) {
	var rs []*cresponse.Response

	var r *cresponse.Response

	err := p.conn.AcquirePriv(p.configPriv)
	if err != nil {
		return nil, err
	}

	if p.replace {
		callbacks := []*generic.Callback{
			{
				ContainsRe: p.conn.Channel.PromptPattern,
				Complete:   true,
			},
			{
				Callback: func(d *generic.Driver, s string) error {
					return d.Channel.WriteAndReturn([]byte("yes"), false)
				},
				Contains:    "proceed?",
				Once:        true,
				ResetOutput: true,
			},
		}

		r, err = p.conn.SendWithCallbacks(
			"commit replace",
			callbacks,
			p.conn.Channel.TimeoutOps,
		)
	} else {
		r, err = p.conn.SendConfig("commit", opoptions.WithPrivilegeLevel(p.configPriv))
	}

	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	p.inProgress = false

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
