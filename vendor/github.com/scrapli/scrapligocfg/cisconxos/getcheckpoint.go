package cisconxos

import (
	"fmt"
	"time"

	"github.com/scrapli/scrapligocfg/response"
)

// GetCheckpoint gets a checkpoint file of the current running configuration.
func (p *Platform) GetCheckpoint(source string) (*response.Response, error) {
	r := response.NewResponse("GetCheckpoint", p.conn.Transport.GetHost())

	timestamp := time.Now().Unix()
	checkpointCommands := []string{
		"terminal dont-ask",
		fmt.Sprintf("checkpoint file %sscrapli_cfg_tmp_%d", p.filesystem, timestamp),
		fmt.Sprintf("show file %sscrapli_cfg_tmp_%d", p.filesystem, timestamp),
		fmt.Sprintf("delete %sscrapli_cfg_tmp_%d", p.filesystem, timestamp),
	}

	mr, err := p.conn.SendCommands(checkpointCommands)
	if err != nil {
		return nil, err
	}

	r.Record(mr.Responses, mr.Responses[1].Result)

	return r, nil
}
