package progress

import (
	"context"
	"io"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
)

type Reporter interface {
	// Start registers an image whose pull is beginning.
	Start(image string)
	// Update the progress bar, status is some text
	// like "Downloading", "Extracting". percent is the completion percentage (0-1)
	Update(image, status string, percent float64)
	// Done marks an image pull as done.
	Done(image string, err error)
}

func New(w io.Writer) (Reporter, func()) {
	if f, ok := w.(interface{ Fd() uintptr }); ok && term.IsTerminal(f.Fd()) {
		return newTUIReporter(w)
	}

	return plainReporter{}, func() {}
}

// plainReporter emits one log line per image and is used when there is no TTY.
type plainReporter struct{}

func (plainReporter) Start(image string) {
	log.Info("Pulling image", "image", image)
}

func (plainReporter) Update(_, _ string, _ float64) {}

func (plainReporter) Done(image string, err error) {
	if err != nil {
		log.Error("Failed to pull image", "image", image, "err", err)

		return
	}

	log.Info("Done pulling image", "image", image)
}

type reporterKey struct{}

// NewContext returns a copy of ctx carrying r so that the runtime layer can pick
// it up during PullImage.
func NewContext(ctx context.Context, r Reporter) context.Context {
	return context.WithValue(ctx, reporterKey{}, r)
}

// FromContext returns the Reporter stored in ctx, or nil if none was set. A nil
// return signals callers to use their legacy single-image progress rendering.
func FromContext(ctx context.Context) Reporter {
	if r, ok := ctx.Value(reporterKey{}).(Reporter); ok {
		return r
	}

	return nil
}
