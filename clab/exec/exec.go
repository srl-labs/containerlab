package exec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/google/shlex"
)

type ExecOutputFormat string

const (
	ExecFormatJSON  ExecOutputFormat = "json"
	ExecFormatPLAIN ExecOutputFormat = "plain"
)

var (
	ErrRunExecNotSupported = errors.New("exec not supported for this kind")
)

func ParseExecOutputFormat(s string) (ExecOutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(ExecFormatJSON):
		return ExecFormatJSON, nil
	case string(ExecFormatPLAIN), "table":
		return ExecFormatPLAIN, nil
	}
	return "", fmt.Errorf("cannot parse %q as 'ExecOutputFormat'", s)
}

type ExecCmd interface {
	GetCmd() []string
	GetCmdString() string
}

type ExecOp struct {
	Cmd []string `json:"cmd"`
}

func NewExecCmdFromString(cmd string) (ExecCmd, error) {
	result := &ExecOp{}
	if err := result.SetCmd(cmd); err != nil {
		return nil, err
	}
	return result, nil
}

func NewExecCmdFromSlice(cmd []string) ExecCmd {
	return &ExecOp{
		Cmd: cmd,
	}
}

type ExecResultHolder interface {
	GetStdOutString() string
	GetStdErrString() string
	GetStdOutByteSlice() []byte
	GetStdErrByteSlice() []byte
	GetReturnCode() int
	GetCmdString() string
	GetEntryInFormat(format ExecOutputFormat) (string, error)
	String() string
}

type ExecResult struct {
	Cmd        []string `json:"cmd"`
	ReturnCode int      `json:"returnCode"`
	Stdout     string   `json:"stdout"`
	Stderr     string   `json:"stderr"`
}

func NewExecResult(op ExecCmd) *ExecResult {
	er := &ExecResult{Cmd: op.GetCmd()}
	return er
}

// SetCmd sets the command that is to be executed
func (e *ExecOp) SetCmd(cmd string) error {
	c, err := shlex.Split(cmd)
	if err != nil {
		return err
	}
	e.Cmd = c
	return nil
}

// GetCmd sets the command that is to be executed
func (e *ExecOp) GetCmd() []string {
	return e.Cmd
}

// GetCmdString sets the command that is to be executed
func (e *ExecOp) GetCmdString() string {
	return strings.Join(e.Cmd, " ")
}

func (e *ExecResult) String() string {
	return fmt.Sprintf("Cmd: %s\nReturnCode: %d\nStdOut:\n%s\nStdErr:\n%s\n", e.GetCmdString(), e.ReturnCode, e.Stdout, e.Stderr)
}

func (e *ExecResult) GetEntryInFormat(format ExecOutputFormat) (string, error) {
	var result string
	switch format {
	case ExecFormatJSON:
		byteData, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			return "", err
		}
		result = string(byteData)
	case ExecFormatPLAIN:
		result = e.String()
	}
	return result, nil
}

// GetCmdString returns the initially parsed cmd as a string for e.g. log output purpose
func (e *ExecResult) GetCmdString() string {
	return strings.Join(e.Cmd, " ")
}

func (e *ExecResult) GetReturnCode() int {
	return e.ReturnCode
}

func (e *ExecResult) SetReturnCode(rc int) {
	e.ReturnCode = rc
}

func (e *ExecResult) GetStdOutString() string {
	return string(e.Stdout)
}

func (e *ExecResult) GetStdErrString() string {
	return string(e.Stderr)
}

func (e *ExecResult) GetStdOutByteSlice() []byte {
	return []byte(e.Stdout)
}

func (e *ExecResult) GetStdErrByteSlice() []byte {
	return []byte(e.Stderr)
}

func (e *ExecResult) GetCmd() []string {
	return e.Cmd
}

func (e *ExecResult) SetStdOut(data []byte) {
	e.Stdout = string(data)
}

func (e *ExecResult) SetStdErr(data []byte) {
	e.Stderr = string(data)
}

// execEntries is a map indexed by container IDs storing lists of ExecResultHolder.
// ExecResultHolder is an interface that is backed by the type storing data for the executed command.
type execEntries map[string][]ExecResultHolder

// ExecCollection represents a datastore for exec commands execution results.
type ExecCollection struct {
	execEntries
}

// NewExecCollection initializes the collection of exec command results.
func NewExecCollection() *ExecCollection {
	return &ExecCollection{
		execEntries{},
	}
}

func (ec *ExecCollection) Add(cId string, e ExecResultHolder) {
	ec.execEntries[cId] = append(ec.execEntries[cId], e)
}

func (ec *ExecCollection) AddAll(cId string, e []ExecResultHolder) {
	ec.execEntries[cId] = append(ec.execEntries[cId], e...)
}

func (ec *ExecCollection) GetInFormat(format ExecOutputFormat) (string, error) {
	result := strings.Builder{}
	switch format {
	case ExecFormatJSON:
		byteData, err := json.MarshalIndent(ec.execEntries, "", "  ")
		if err != nil {
			return "", err
		}
		result.Write(byteData)
	case ExecFormatPLAIN:
		printSep := false
		for k, execResults := range ec.execEntries {
			if len(execResults) == 0 {
				// skip if there is no result
				continue
			}
			// write seperator
			if printSep {
				result.WriteString("\n+++++++++++++++++++++++++++++\n\n")
			}
			// write header for entry
			result.WriteString("Node: ")
			result.WriteString(k)
			result.WriteString("\n")
			for _, er := range execResults {
				// write entry
				result.WriteString(er.String())
			}
			// starting second run, print sep
			printSep = true
		}
	}
	return result.String(), nil
}

func (ec *ExecCollection) WriteLogInfo() {
	for k, execResults := range ec.execEntries {
		for _, er := range execResults {
			log.Infof("Executed command '%s' on %s. stdout:\n%s", er.GetCmdString(), k, er.GetStdOutString())
		}
	}
}
