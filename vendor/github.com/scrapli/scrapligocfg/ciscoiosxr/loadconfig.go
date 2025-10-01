package ciscoiosxr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

func (p *Platform) parseConfigPayload(config string) (stdConfig, eagerCfg string) {
	stdConfig = p.patterns.outputHeader.ReplaceAllString(config, "!")
	stdConfig = p.patterns.end.ReplaceAllString(stdConfig, "!")

	eagerS := make([]string, 0)
	bannerS := p.patterns.bannerDelim.FindAllString(stdConfig, -1)

	for _, bannerHeader := range bannerS {
		delim := bannerHeader[len(bannerHeader)-1:]

		currentBannerP := regexp.MustCompile(fmt.Sprintf(
			`(?ims)^%s.*?%s$`,
			regexp.QuoteMeta(bannerHeader),
			regexp.QuoteMeta(delim),
		))

		currentBanner := currentBannerP.FindString(stdConfig)
		eagerS = append(eagerS, currentBanner)

		stdConfig = strings.Replace(stdConfig, currentBanner, "1", 1)
	}

	return stdConfig, strings.Join(eagerS, "\n")
}

// LoadConfig is the platform implementation of LoadConfig. Note that 'f' argument for config
// file name and/or session is unused for iosxr.
func (p *Platform) LoadConfig(
	_, config string,
	replace bool,
	options *util.OperationOptions,
) (*response.PlatformResponse, error) {
	p.replace = replace
	p.inProgress = true
	p.configPriv = "configuration"

	// the actual value is irrelevant, if there is a key "exclusive" w/ any value we assume user is
	// wanting to use configuration_exclusive config mode
	_, ok := options.Kwargs["exclusive"]
	if ok {
		p.configPriv = "configuration-exclusive"
	}

	stdConfig, eagerConfig := p.parseConfigPayload(config)

	var rs []*cresponse.Response

	var r *cresponse.Response

	r, err := p.conn.SendConfig(
		stdConfig, opoptions.WithPrivilegeLevel(p.configPriv),
	)
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.conn.SendConfig(
		eagerConfig,
		opoptions.WithPrivilegeLevel(p.configPriv),
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
