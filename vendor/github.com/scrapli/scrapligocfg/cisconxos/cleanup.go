package cisconxos

import (
	"fmt"

	cresponse "github.com/scrapli/scrapligo/response"
)

func (p *Platform) deleteFile(f string) (*cresponse.Response, error) {
	_, _ = p.conn.SendCommand("terminal dont-ask")

	return p.conn.SendCommand(fmt.Sprintf("delete %s", f))
}

// Cleanup is the platform implementation of Cleanup.
func (p *Platform) Cleanup() error {
	var err error

	if p.candidateF != "" {
		// if candidateF is still set, clean it up and the file itself
		_, err = p.AbortConfig()
	}

	return err
}
