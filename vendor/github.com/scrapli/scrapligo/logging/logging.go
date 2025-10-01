package logging

import (
	"fmt"
	"sync"

	"github.com/scrapli/scrapligo/util"
)

const (
	// Debug is the debug log level.
	Debug = "debug"
	// Info is the info(rmational) log level.
	Info = "info"
	// Critical is the critical log level.
	Critical = "critical"
)

// NewInstance returns a new logging Instance.
func NewInstance(opts ...util.Option) (*Instance, error) {
	i := &Instance{
		Level:     Info,
		Formatter: DefaultFormatter,
		Loggers:   nil,
	}

	for _, o := range opts {
		err := o(i)
		if err != nil {
			return nil, err
		}
	}

	return i, nil
}

// Instance is a simple logging object.
type Instance struct {
	Level     string
	Formatter func(string, string) string
	Loggers   []func(...interface{})
}

// Emit "emits" a logging message m to all the loggers in the Instance.
func (i *Instance) Emit(m interface{}) {
	wg := sync.WaitGroup{}

	for _, f := range i.Loggers {
		wg.Add(1)

		lf := f

		go func() {
			lf(m)

			wg.Done()
		}()
	}

	wg.Wait()
}

func (i *Instance) shouldLog(l string) bool {
	if len(i.Loggers) == 0 {
		return false
	}

	switch i.Level {
	case Debug:
		return true
	case Info:
		switch l {
		case Info, Critical:
			return true
		default:
			return false
		}
	case Critical:
		if l == Critical {
			return true
		}
	}

	return false
}

// Debug accepts a Debug level log message with no formatting.
func (i *Instance) Debug(f string) {
	if !i.shouldLog(Debug) {
		return
	}

	i.Emit(i.Formatter(Debug, f))
}

// Debugf accepts a Debug level log message normal fmt.Sprintf type formatting.
func (i *Instance) Debugf(f string, a ...interface{}) {
	if !i.shouldLog(Debug) {
		return
	}

	i.Emit(i.Formatter(Debug, fmt.Sprintf(f, a...)))
}

// Info accepts an Info level log message with no formatting.
func (i *Instance) Info(f string) {
	if !i.shouldLog(Info) {
		return
	}

	i.Emit(i.Formatter(Info, f))
}

// Infof accepts an Info level log message normal fmt.Sprintf type formatting.
func (i *Instance) Infof(f string, a ...interface{}) {
	if !i.shouldLog(Info) {
		return
	}

	i.Emit(i.Formatter(Info, fmt.Sprintf(f, a...)))
}

// Critical accepts a Critical level log message with no formatting.
func (i *Instance) Critical(f string) {
	if !i.shouldLog(Critical) {
		return
	}

	i.Emit(i.Formatter(Critical, f))
}

// Criticalf accepts a Critical level log message normal fmt.Sprintf type formatting.
func (i *Instance) Criticalf(f string, a ...interface{}) {
	if !i.shouldLog(Critical) {
		return
	}

	i.Emit(i.Formatter(Critical, fmt.Sprintf(f, a...)))
}
