package ciscoiosxe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
	"github.com/scrapli/scrapligocfg/util"
)

const (
	tclsh = "tclsh"
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
	start := fmt.Sprintf(
		`puts [open "%s%s" w+] {`,
		p.filesystem,
		p.candidateF,
	)
	end := "}"

	return strings.Join([]string{start, config, end}, "\n")
}

// LoadConfig is the platform implementation of LoadConfig.
func (p *Platform) LoadConfig(
	f, config string,
	replace bool,
	options *util.OperationOptions,
) (*response.PlatformResponse, error) {
	p.candidateF = f
	p.replace = replace

	if options.AutoClean {
		config = p.cleanConfigPayload(config)
	}

	bytesAvail, err := p.getFilesystemBytesAvail()
	if err != nil {
		return nil, err
	}

	err = util.SpaceOK(bytesAvail, len(config), p.spaceAvailBuffPerc)
	if err != nil {
		return nil, err
	}

	config = p.prepareConfigPayload(config)

	originalReturnChar := p.conn.Channel.ReturnChar

	defer func() {
		p.conn.Channel.ReturnChar = originalReturnChar
	}()

	err = p.conn.AcquirePriv(tclsh)
	if err != nil {
		return nil, err
	}

	p.conn.Channel.ReturnChar = []byte("\r")

	r, err := p.conn.SendConfig(config, opoptions.WithPrivilegeLevel(tclsh))
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
