package events

import (
	"io"

	clabcore "github.com/srl-labs/containerlab/core"
)

// Options configure how runtime and interface events are sourced and rendered.
type Options struct {
	Format      string
	Runtime     string
	ClabOptions []clabcore.ClabOption
	Writer      io.Writer
}

func (o Options) writer() io.Writer {
	if o.Writer != nil {
		return o.Writer
	}

	return io.Discard
}
