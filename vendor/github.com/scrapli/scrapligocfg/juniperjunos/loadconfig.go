package juniperjunos

import (
	"fmt"
	"strings"

	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

func (p *Platform) prepareConfigPayload(config string) string {
	finalConfigs := make([]string, 0)

	for _, l := range strings.Split(config, "\n") {
		finalConfigs = append(
			finalConfigs,
			fmt.Sprintf("echo >> %s%s '%s'", p.filesystem, p.candidateF, l),
		)
	}

	return strings.Join(
		finalConfigs,
		"\n",
	)
}

// LoadConfig is the platform implementation of LoadConfig.
func (p *Platform) LoadConfig(
	f, config string,
	replace bool,
	options *util.OperationOptions,
) (*response.PlatformResponse, error) {
	p.candidateF = f
	p.replace = replace

	// the actual value is irrelevant, if there is a key "set" w/ any value we assume user is
	// loading a "set" style config
	_, ok := options.Kwargs["set"]
	if ok {
		p.configSetStyle = true
	}

	config = p.prepareConfigPayload(config)

	var rs []*cresponse.Response

	r, err := p.conn.SendConfig(config, opoptions.WithPrivilegeLevel("root-shell"))
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	p.inProgress = true

	loadCmd := fmt.Sprintf("load override %s%s", p.filesystem, p.candidateF)

	if !p.replace {
		loadCmd = fmt.Sprintf("load merge %s%s", p.filesystem, p.candidateF)

		if p.configSetStyle {
			loadCmd = fmt.Sprintf("load set %s%s", p.filesystem, p.candidateF)
		}
	}

	r, err = p.conn.SendConfig(loadCmd)
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
