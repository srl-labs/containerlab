package juniperjunos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// CommitConfig is the platform implementation of CommitConfig.
func (p *Platform) CommitConfig() (*response.PlatformResponse, error) {
	var rs []*cresponse.Response

	r, err := p.conn.SendConfig("commit")
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.deleteFile(fmt.Sprintf("%s%s", p.filesystem, p.candidateF))
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
