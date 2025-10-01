package cisconxos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// AbortConfig is the platform implementation of AbortConfig.
func (p *Platform) AbortConfig() (*response.PlatformResponse, error) {
	r, err := p.deleteFile(fmt.Sprintf("%s%s", p.filesystem, p.candidateF))
	if err != nil {
		return nil, err
	}

	p.candidateF = ""

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: []*cresponse.Response{r},
	}, nil
}
