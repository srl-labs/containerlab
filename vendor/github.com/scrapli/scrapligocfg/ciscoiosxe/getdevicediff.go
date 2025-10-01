package ciscoiosxe

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

func (p *Platform) getDiffCommand(source string) string {
	if p.replace {
		return fmt.Sprintf(
			"show archive config differences system:%s-config %s%s",
			source,
			p.filesystem,
			p.candidateF,
		)
	}

	return fmt.Sprintf(
		"show archive config incremental-diffs %s%s ignorecase",
		p.filesystem,
		p.candidateF,
	)
}

// GetDeviceDiff is the platform implementation of GetDeviceDiff.
func (p *Platform) GetDeviceDiff(source string) (*response.PlatformResponse, error) {
	r, err := p.conn.SendCommand(p.getDiffCommand(source))
	if err != nil {
		return nil, err
	}

	return &response.PlatformResponse{
		Result:           r.Result,
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
