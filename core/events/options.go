package events

import (
	"io"

	clabcore "github.com/srl-labs/containerlab/core"
)

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
