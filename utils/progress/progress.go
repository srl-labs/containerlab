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

// isTerminal reports whether fd is a terminal. Tests replace it.
var isTerminal = term.IsTerminal

// New returns a reporter that renders progress bars on w when it is a terminal, and logs to the
// logger in ctx otherwise.
func New(ctx context.Context, w io.Writer) (Reporter, func()) {
	if useTUI(ctx, w) {
		return newTUIReporter(w)
	}

	return plainReporter{logger: log.FromContext(ctx)}, func() {}
}

// useTUI reports whether pulls are rendered as progress bars on w. A logger in ctx means the
// caller collects the output, so the pulls are logged to it even when w is a terminal.
func useTUI(ctx context.Context, w io.Writer) bool {
	if _, ok := ctx.Value(log.ContextKey).(*log.Logger); ok {
		return false
	}

	f, ok := w.(interface{ Fd() uintptr })

	return ok && isTerminal(f.Fd())
}

// plainReporter emits one log line per image to a logger. It is used when there is no TTY or
// when the context holds a logger.
type plainReporter struct {
	logger *log.Logger
}

func (r plainReporter) Start(image string) {
	r.logger.Info("Pulling image", "image", image)
}

func (plainReporter) Update(_, _ string, _ float64) {}

func (r plainReporter) Done(image string, err error) {
	if err != nil {
		r.logger.Error("Failed to pull image", "image", image, "err", err)

		return
	}

	r.logger.Info("Done pulling image", "image", image)
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
