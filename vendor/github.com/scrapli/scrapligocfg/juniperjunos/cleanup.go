package juniperjunos

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/opoptions"
	cresponse "github.com/scrapli/scrapligo/response"
)

func (p *Platform) deleteFile(f string) (*cresponse.Response, error) {
	return p.conn.SendConfig(
		fmt.Sprintf("rm %s", f),
		opoptions.WithPrivilegeLevel("root-shell"),
	)
}

// Cleanup is the platform implementation of Cleanup.
func (p *Platform) Cleanup() error {
	var err error

	if p.inProgress {
		_, err = p.AbortConfig()
	}

	return err
}
