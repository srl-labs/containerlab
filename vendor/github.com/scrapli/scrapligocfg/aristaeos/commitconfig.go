package aristaeos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

// DeRegisterConfigSession de-registers a configuration session of name s.
func (p *Platform) DeRegisterConfigSession(s string) error {
	_, ok := p.conn.PrivilegeLevels[s]
	if !ok {
		return fmt.Errorf(
			"%w: cannot deregister config session, no session with name '%s' exists",
			util.ErrCandidateError,
			s,
		)
	}

	delete(p.conn.PrivilegeLevels, s)
	p.conn.UpdatePrivileges()

	p.candidateS = ""

	return nil
}

// SaveConfig saves the running configuration to the startup configuration.
func (p *Platform) SaveConfig() (*cresponse.Response, error) {
	r, err := p.conn.SendCommand("copy running-config startup-config")
	if err != nil {
		return nil, err
	}

	return r, nil
}

// CommitConfig is the platform implementation of CommitConfig.
func (p *Platform) CommitConfig() (*response.PlatformResponse, error) {
	var rs []*cresponse.Response

	r, err := p.conn.SendCommand(fmt.Sprintf("configure session %s commit", p.candidateS))
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.SaveConfig()
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	err = p.DeRegisterConfigSession(p.candidateS)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
