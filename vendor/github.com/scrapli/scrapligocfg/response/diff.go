package response

import (
	"fmt"
	"strings"
	"time"

	"github.com/scrapli/scrapligocfg/util"
	"golang.org/x/term"
)

// NewDiffResponse returns a new DiffResponse object.
func NewDiffResponse(host string, opts ...util.Option) *DiffResponse {
	r := &Response{
		Op:        "DiffConfig",
		Host:      host,
		StartTime: time.Now(),
		EndTime:   time.Time{},
	}

	dr := &DiffResponse{
		Response: r,
	}

	for _, option := range opts {
		_ = option(dr)
	}

	return dr
}

// DiffResponse is a special response object specifically for `GetDiff` operations.
type DiffResponse struct {
	*Response

	Colorize bool

	DeviceDiff string
	Candidate  string
	Current    string

	Difflines    []string
	Additions    []string
	Subtractions []string

	SideBySideW int
	sideBySide  string

	unified string
}

func (r *DiffResponse) generateColors() (unknown, subtraction, addition, end string) {
	if !r.Colorize {
		return "? ", "- ", "+ ", ""
	}

	return "\033[93m", "\033[91m", "\033[92m", "\033[0m"
}

func (r *DiffResponse) terminalWidth() int {
	if r.SideBySideW != 0 {
		return r.SideBySideW
	}

	width, _, _ := term.GetSize(0)

	return width
}

// RecordDiff records the outcome of a GetDiff result.
func (r *DiffResponse) RecordDiff(device, candidate, current string) {
	r.DeviceDiff = device
	r.Candidate = candidate
	r.Current = current

	r.Difflines = util.GetDiffLines(r.Current, r.Candidate)

	for _, v := range r.Difflines {
		if v[:2] == util.DiffAddition {
			r.Additions = append(r.Additions, v[2:])
		} else if v[:2] == util.DiffSubtraction {
			r.Subtractions = append(r.Subtractions, v[2:])
		}
	}
}

// SideBySideDiff returns the diff in a side-by-side output.
func (r *DiffResponse) SideBySideDiff() string {
	if len(r.sideBySide) > 0 {
		return r.sideBySide
	}

	yellow, red, green, end := r.generateColors()

	termWidth := r.terminalWidth()

	halfTermWidth := termWidth / 2
	diffSideWidth := halfTermWidth - 5

	sideBySideDiffLines := make([]string, 0)

	for _, line := range r.Difflines {
		var current, candidate string

		trimLen := diffSideWidth
		difflineLen := len(line)

		if difflineLen-1 <= trimLen {
			trimLen = difflineLen - 2
		}

		switch line[:2] {
		case util.DiffUnknown:
			current = yellow + fmt.Sprintf(
				"%-*s",
				halfTermWidth,
				strings.TrimRight(line[2:][:trimLen], " "),
			) + end
			candidate = yellow + strings.TrimRight(line[2:][:trimLen], " ") + end
		case util.DiffSubtraction:
			current = red + fmt.Sprintf(
				"%-*s",
				halfTermWidth,
				strings.TrimRight(line[2:][:trimLen], " "),
			) + end
			candidate = ""
		case util.DiffAddition:
			current = strings.Repeat(" ", halfTermWidth)
			candidate = green + strings.TrimRight(line[2:][:trimLen], " ") + end
		default:
			current = fmt.Sprintf(
				"%-*s",
				halfTermWidth,
				strings.TrimRight(line[2:][:trimLen], " "),
			)
			candidate = strings.TrimRight(line[2:][:trimLen], " ")
		}

		sideBySideDiffLines = append(sideBySideDiffLines, current+candidate)
	}

	r.sideBySide = strings.Join(sideBySideDiffLines, "\n")

	return r.sideBySide
}

// UnifiedDiff returns the diff in a unified output.
func (r *DiffResponse) UnifiedDiff() string {
	if len(r.unified) > 0 {
		return r.unified
	}

	yellow, red, green, end := r.generateColors()

	unifiedDiffLines := make([]string, 0)

	for _, line := range r.Difflines {
		var diffLine string

		switch line[:2] {
		case util.DiffUnknown:
			diffLine = yellow + line[2:] + end
		case util.DiffSubtraction:
			diffLine = red + line[2:] + end
		case util.DiffAddition:
			diffLine = green + line[2:] + end
		default:
			diffLine = line[2:]
		}

		unifiedDiffLines = append(unifiedDiffLines, diffLine)
	}

	r.unified = strings.Join(unifiedDiffLines, "\n")

	return r.unified
}
