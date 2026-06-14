package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

const (
	// barWidth is the rendered width of each image's progress bar.
	barWidth = 40
	// nameWidth is the column width reserved for the image name.
	nameWidth = 45
	// statusWidth is the column width reserved for the status word.
	statusWidth = 12
	// gradientStart and gradientEnd are the blue→purple endpoints of the bar's
	// scaled gradient (the gradient is scaled to the filled portion for a smooth
	// fill as it grows).
	gradientStart = "#3B82F6" // blue
	gradientEnd   = "#A855F7" // purple
)

var (
	okStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	statusStyle = lipgloss.NewStyle().Faint(true)
)

// row holds the rendered state of a single image pull. Each row owns its own
// progress.Model so its bar animates independently toward the latest percent.
type row struct {
	image  string
	status string
	bar    progress.Model
	done   bool
}

// newBar builds the animated, blue→purple progress bar used for every image.
func newBar() progress.Model {
	return progress.New(
		progress.WithScaledGradient(gradientStart, gradientEnd),
		progress.WithWidth(barWidth),
	)
}

// Bubble Tea messages used to mutate the model from reporting goroutines.
type (
	startMsg  struct{ image string }
	updateMsg struct {
		image   string
		status  string
		percent float64
	}
	doneMsg struct {
		image string
		err   error
	}
)

// model is the Bubble Tea model rendering one bar per in-progress image. It is
// only mutated from within the Bubble Tea event loop (via the messages above),
// so it needs no locking even though many goroutines report concurrently.
//
// In-progress images are shown as live bars; finished images are removed from
// the live view and recorded as a persistent line printed above it (tea.Printf),
// which keeps a scrollback record without the per-layer cursor collisions that
// motivated this package.
type model struct {
	rows  []*row
	index map[string]int

	// onInterrupt is invoked when the user presses Ctrl+C. In raw mode Ctrl+C is
	// delivered as a key event rather than a signal, so containerlab's own SIGINT
	// handler never fires unless we re-raise it here.
	onInterrupt func()
}

func newModel() model {
	return model{
		index: map[string]int{},
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startMsg:
		if _, ok := m.index[msg.image]; !ok {
			m.index[msg.image] = len(m.rows)
			m.rows = append(m.rows, &row{image: msg.image, status: "Waiting", bar: newBar()})
		}
	case updateMsg:
		if i, ok := m.index[msg.image]; ok {
			m.rows[i].status = msg.status
			// SetPercent returns a command that springs the bar toward the new
			// value, giving the smooth animation from the charm example.
			return m, m.rows[i].bar.SetPercent(msg.percent)
		}
	case progress.FrameMsg:
		// Drive every active bar's animation. Each bar ignores frames that aren't
		// addressed to it, so routing the frame to all of them is safe.
		var cmds []tea.Cmd

		for _, r := range m.rows {
			if r.done {
				continue
			}

			updated, cmd := r.bar.Update(msg)
			r.bar = updated.(progress.Model)
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)
	case doneMsg:
		if i, ok := m.index[msg.image]; ok {
			m.rows[i].done = true

			if msg.err != nil {
				return m, tea.Printf("%s Failed to pull image %s: %v",
					errStyle.Render("✗"), msg.image, msg.err)
			}

			return m, tea.Printf("%s Pulled image %s", okStyle.Render("✔"), msg.image)
		}
	case tea.KeyMsg:
		// In raw mode Ctrl+C is a key event, not a signal. Hand off to onInterrupt
		// which tears down the display and re-raises SIGINT from its own goroutine;
		// we must not block the event loop or quit it out from under that teardown.
		if msg.Type == tea.KeyCtrlC && m.onInterrupt != nil {
			m.onInterrupt()
		}
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	for _, r := range m.rows {
		// Finished images are recorded as persistent printed lines, so only the
		// still-running ones get a live bar.
		if r.done {
			continue
		}

		fmt.Fprintf(&b, "  %s %s %s\n",
			padRight(r.image, nameWidth),
			statusStyle.Render(padRight(r.status, statusWidth)),
			r.bar.View(),
		)
	}

	return b.String()
}

// padRight pads s with spaces to width, truncating from the left (keeping the
// more meaningful tail of an image reference) with an ellipsis when too long.
func padRight(s string, width int) string {
	if len(s) > width {
		return "…" + s[len(s)-width+1:]
	}

	return s + strings.Repeat(" ", width-len(s))
}

// tuiReporter is the interactive Reporter backed by a Bubble Tea program. The
// program is started lazily on the first image so that nothing is rendered (and
// the terminal is not taken over) when every image is already present locally.
type tuiReporter struct {
	w         io.Writer
	startOnce sync.Once
	prog      *tea.Program
}

func newTUIReporter(w io.Writer) (Reporter, func()) {
	r := &tuiReporter{w: w}

	stop := func() {
		if r.prog != nil {
			r.prog.Quit()
			r.prog.Wait()
		}
	}

	return r, stop
}

// launch builds and runs the Bubble Tea program. It is only ever called once,
// from within startOnce.
func (r *tuiReporter) launch() {
	var (
		p             *tea.Program
		interruptOnce sync.Once
	)

	m := newModel()
	m.onInterrupt = func() {
		// Run from a dedicated goroutine (not the event loop) and only once.
		interruptOnce.Do(func() {
			go func() {
				// Restore the terminal to a sane state before re-raising SIGINT,
				// so it's cooked even if containerlab's signal handler exits the
				// process before Bubble Tea finishes tearing down.
				_ = p.ReleaseTerminal()

				if proc, err := os.FindProcess(os.Getpid()); err == nil {
					_ = proc.Signal(syscall.SIGINT)
				}
			}()
		})
	}

	p = tea.NewProgram(m, tea.WithOutput(r.w))
	r.prog = p

	go func() { _, _ = p.Run() }()
}

func (r *tuiReporter) Start(image string) {
	r.startOnce.Do(func() {
		// A single blanket INFO line, emitted before the live display takes over
		// the terminal so it renders as a normal log line above the bars. Start is
		// only called for images that actually need pulling, so this stays quiet
		// when all images are already present.
		log.Info("Pulling images")

		r.launch()
	})

	r.prog.Send(startMsg{image: image})
}

func (r *tuiReporter) Update(image, status string, percent float64) {
	r.prog.Send(updateMsg{image: image, status: status, percent: percent})
}

func (r *tuiReporter) Done(image string, err error) {
	r.prog.Send(doneMsg{image: image, err: err})
}
