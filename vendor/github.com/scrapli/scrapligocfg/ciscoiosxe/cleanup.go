package ciscoiosxe

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/generic"
	cresponse "github.com/scrapli/scrapligo/response"
)

func (p *Platform) deleteFile(f string) (*cresponse.Response, error) {
	callbacks := []*generic.Callback{
		{
			ContainsRe: p.conn.Channel.PromptPattern,
			Complete:   true,
		},
		{
			Callback: func(d *generic.Driver, s string) error {
				return d.Channel.WriteReturn()
			},
			Contains:    "Delete filename",
			Once:        true,
			ResetOutput: true,
		},
		{
			Callback: func(d *generic.Driver, s string) error {
				return d.Channel.WriteReturn()
			},
			Contains:    "[confirm]",
			Once:        true,
			ResetOutput: true,
		},
	}

	return p.conn.SendWithCallbacks(
		fmt.Sprintf(
			"delete %s",
			f,
		),
		callbacks,
		p.conn.Channel.TimeoutOps,
	)
}

// Cleanup is the platform implementation of Cleanup.
func (p *Platform) Cleanup() error {
	var err error

	if p.candidateF != "" {
		_, err = p.AbortConfig()
	}

	return err
}
