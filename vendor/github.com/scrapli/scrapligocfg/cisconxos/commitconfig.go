package cisconxos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// SaveConfig saves the running configuration to the startup configuration.
func (p *Platform) SaveConfig() (*cresponse.Response, error) {
	return p.conn.SendCommand(
		"copy running-config startup-config",
	)
}

// CommitConfig is the platform implementation of CommitConfig.
func (p *Platform) CommitConfig() (*response.PlatformResponse, error) {
	var rs []*cresponse.Response

	var r *cresponse.Response

	var err error

	if p.replace {
		r, err = p.conn.SendCommand(
			fmt.Sprintf(
				"rollback running-config file %s%s",
				p.filesystem,
				p.candidateF,
			),
		)
	} else {
		r, err = p.conn.SendCommand(
			fmt.Sprintf("copy %s%s running-config", p.filesystem, p.candidateF),
		)
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
