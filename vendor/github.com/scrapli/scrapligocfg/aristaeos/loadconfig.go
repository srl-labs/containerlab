package aristaeos

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/scrapli/scrapligo/driver/network"

	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

// RegisterConfigSession registers a configuration session of name s.
func (p *Platform) RegisterConfigSession(s string) error {
	_, ok := p.conn.PrivilegeLevels[s]
	if ok {
		return fmt.Errorf(
			"%w: cannot register config session, session with name '%s' already exists",
			util.ErrCandidateError,
			s,
		)
	}

	sessionPrompt := regexp.QuoteMeta(s[:6])
	sessionPromptPattern := fmt.Sprintf(
		`(?im)^[\w.\-@()/:\s]{1,63}\(config\-s\-%s[\w.\-@_/:]{0,32}\)#\s?$`,
		sessionPrompt,
	)
	sessionPrivilegeLevel := &network.PrivilegeLevel{
		Pattern:        sessionPromptPattern,
		Name:           s,
		PreviousPriv:   "privilege-exec",
		Deescalate:     "end",
		Escalate:       fmt.Sprintf("configure session %s", s),
		EscalateAuth:   false,
		EscalatePrompt: "",
	}

	p.conn.PrivilegeLevels[s] = sessionPrivilegeLevel
	p.conn.UpdatePrivileges()

	return nil
}

func (p *Platform) parseConfigPayload(config string) (stdConfig, eagerCfg string) {
	stdConfig = p.patterns.globalCommentLine.ReplaceAllString(config, "!")
	stdConfig = p.patterns.end.ReplaceAllString(stdConfig, "!")

	eagerS := p.patterns.banner.FindStringSubmatch(stdConfig)
	eagerCfg = strings.Join(eagerS, "\n")

	for _, s := range eagerS {
		stdConfig = strings.Replace(stdConfig, s, "!", -1)
	}

	return stdConfig, eagerCfg
}

// LoadConfig is the platform implementation of LoadConfig.
func (p *Platform) LoadConfig(
	f, config string,
	replace bool,
	options *util.OperationOptions,
) (*response.PlatformResponse, error) {
	// options are not used for eos at this time
	_ = options

	err := p.RegisterConfigSession(f)
	if err != nil {
		return nil, err
	}

	p.candidateS = f

	stdConfig, eagerConfig := p.parseConfigPayload(config)

	var rs []*cresponse.Response

	var r *cresponse.Response

	if replace {
		r, err = p.conn.SendConfig(
			"rollback clean-config",
			opoptions.WithPrivilegeLevel(p.candidateS),
		)
		if err != nil {
			return nil, err
		}

		rs = append(rs, r)
	}

	r, err = p.conn.SendConfig(stdConfig, opoptions.WithPrivilegeLevel(p.candidateS))
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.conn.SendConfig(
		eagerConfig,
		opoptions.WithPrivilegeLevel(p.candidateS),
		opoptions.WithEager(),
	)
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
