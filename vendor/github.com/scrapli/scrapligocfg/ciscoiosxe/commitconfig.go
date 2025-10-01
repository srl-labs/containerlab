package ciscoiosxe

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/generic"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// SaveConfig saves the running configuration to the startup configuration.
func (p *Platform) SaveConfig() (*cresponse.Response, error) {
	callbacks := []*generic.Callback{
		{
			ContainsRe: p.conn.Channel.PromptPattern,
			Complete:   true,
		},
		{
			Callback: func(d *generic.Driver, s string) error {
				return d.Channel.WriteReturn()
			},
			Contains:    "Source filename",
			Once:        true,
			ResetOutput: true,
		},
		{
			Callback: func(d *generic.Driver, s string) error {
				return d.Channel.WriteReturn()
			},
			Contains:    "Destination filename",
			Once:        true,
			ResetOutput: true,
		},
	}

	return p.conn.SendWithCallbacks(
		"copy running-config startup-config",
		callbacks,
		p.conn.Channel.TimeoutOps,
	)
}

func (p *Platform) commitMerge() (*cresponse.Response, error) {
	callbacks := []*generic.Callback{
		{
			ContainsRe: p.conn.Channel.PromptPattern,
			Complete:   true,
		},
		{
			Callback: func(d *generic.Driver, s string) error {
				return d.Channel.WriteReturn()
			},
			Contains:    "Source filename",
			Once:        true,
			ResetOutput: true,
		},
		{
			Callback: func(d *generic.Driver, s string) error {
				return d.Channel.WriteReturn()
			},
			Contains:    "Destination filename",
			Once:        true,
			ResetOutput: true,
		},
	}

	return p.conn.SendWithCallbacks(
		fmt.Sprintf(
			"copy %s%s running-config",
			p.filesystem,
			p.candidateF,
		),
		callbacks,
		p.conn.Channel.TimeoutOps,
	)
}

// CommitConfig is the platform implementation of CommitConfig.
func (p *Platform) CommitConfig() (*response.PlatformResponse, error) {
	var rs []*cresponse.Response

	var r *cresponse.Response

	var err error

	if p.replace {
		r, err = p.conn.SendCommand(
			fmt.Sprintf("configure replace %s%s force", p.filesystem, p.candidateF),
		)
	} else {
		r, err = p.commitMerge()
	}

	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.SaveConfig()
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.deleteFile(fmt.Sprintf("%s%s", p.filesystem, p.candidateF))
	if err != nil {
		return nil, err
	}

	p.candidateF = ""

	rs = append(rs, r)

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
