package juniperjunos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligocfg/response"
)

// AbortConfig is the platform implementation of AbortConfig.
func (p *Platform) AbortConfig() (*response.PlatformResponse, error) {
	var rs []*cresponse.Response

	var r *cresponse.Response

	var err error

	r, err = p.conn.SendConfig("rollback 0")
	if err != nil {
		return nil, err
	}

	rs = append(rs, r)

	r, err = p.deleteFile(fmt.Sprintf("%s%s", p.filesystem, p.candidateF))
	if err != nil {
		return nil, err
	}

	p.inProgress = false
	p.candidateF = ""

	rs = append(rs, r)

	return &response.PlatformResponse{
		Result:           "",
		ScrapliResponses: rs,
	}, nil
}
