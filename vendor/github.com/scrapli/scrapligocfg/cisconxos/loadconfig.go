package cisconxos

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

func (p *Platform) getFilesystemBytesAvail() (int, error) {
	bytesAvail := -1

	r, err := p.conn.SendCommand(fmt.Sprintf("dir %s | i bytes", p.filesystem))
	if err != nil {
		return bytesAvail, err
	}

	match := p.patterns.bytesFree.FindStringSubmatch(r.Result)

	for i, name := range p.patterns.bytesFree.SubexpNames() {
		if i != 0 && name == "bytes_available" {
			bytesAvail, err = strconv.Atoi(match[i])
			if err != nil {
				return bytesAvail, err
			}
		}
	}

	return bytesAvail, nil
}

func (p *Platform) prepareConfigPayload(config string) string {
	tclshFilesystem := fmt.Sprintf("/%s/", strings.TrimSuffix(p.filesystem, ":"))
	tcslhStartFile := fmt.Sprintf(
		`set fl [open "%s%s" wb+]`,
		tclshFilesystem,
		p.candidateF,
	)

	splitConfig := strings.Split(config, "\n")

	tclshConfig := make([]string, 0)

	for _, configLine := range splitConfig {
		tclshConfig = append(tclshConfig, fmt.Sprintf("puts -nonewline $fl {%s\n}", configLine))
	}

	tclshEndFile := "close $fl"

	return strings.Join(
		[]string{tcslhStartFile, strings.Join(tclshConfig, "\n"), tclshEndFile},
		"\n",
	)
}

// LoadConfig is the platform implementation of LoadConfig.
func (p *Platform) LoadConfig(
	f, config string,
	replace bool,
	_ *util.OperationOptions,
) (*response.PlatformResponse, error) {
	p.candidateF = f
	p.replace = replace

	bytesAvail, err := p.getFilesystemBytesAvail()
	if err != nil {
		return nil, err
	}

	err = util.SpaceOK(bytesAvail, len(config), p.spaceAvailBuffPerc)
	if err != nil {
		return nil, err
	}

	config = p.prepareConfigPayload(config)

	var r *cresponse.Response

	err = p.conn.AcquirePriv("tclsh")
	if err != nil {
		return nil, err
	}

	r, err = p.conn.SendConfig(config, opoptions.WithPrivilegeLevel("tclsh"))
	if err != nil {
		return nil, err
	}

	err = p.conn.AcquirePriv(p.conn.DefaultDesiredPriv)
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
