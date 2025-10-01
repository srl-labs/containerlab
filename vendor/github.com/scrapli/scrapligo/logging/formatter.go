package logging

import (
	"fmt"
	"strings"
)

const (
	colorStop = "\033[0m"
)

var colorMap = map[string]string{ //nolint:gochecknoglobals
	Debug:    "\033[34m",
	Info:     "\033[32m",
	Critical: "\033[31m",
}

// DefaultFormatter is the default logging instance formatter -- this formatter simply adds colors
// to the log message based on log level.
func DefaultFormatter(l, m string) string {
	switch l {
	case Debug:
		l = colorMap[Debug] + strings.ToUpper(l) + colorStop
	case Info:
		l = colorMap[Info] + strings.ToUpper(l) + colorStop
	case Critical:
		l = colorMap[Critical] + strings.ToUpper(l) + colorStop
	}

	// pad the level extra for the ansi code magic
	return fmt.Sprintf("%17s %s", l, m)
}
