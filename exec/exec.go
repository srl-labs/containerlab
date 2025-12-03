package exec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/log"

	"github.com/google/shlex"
	clabconstants "github.com/srl-labs/containerlab/constants"
)

var ErrRunExecNotSupported = errors.New("exec not supported for this kind")

// ParseExecOutputFormat parses the exec output format user input.
func ParseExecOutputFormat(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case clabconstants.FormatJSON:
		return clabconstants.FormatJSON, nil
	case clabconstants.FormatPlain, clabconstants.FormatTable:
		return clabconstants.FormatPlain, nil
	}
	return "", fmt.Errorf("cannot parse %q as execution output format, supported output formats %q",
		s, []string{clabconstants.FormatJSON, clabconstants.FormatPlain})
}

// ExecCmd represents an exec command.
type ExecCmd struct {
	Cmd []string `json:"cmd"` // Cmd is a slice-based representation of a string command.
}

// NewExecCmdFromString creates ExecCmd for a string-based command.
func NewExecCmdFromString(cmd string) (*ExecCmd, error) {
	result := &ExecCmd{}
	if err := result.SetCmd(cmd); err != nil {
		return nil, err
	}
	return result, nil
}

// NewExecCmdFromSlice creates ExecCmd for a command represented as a slice of strings.
func NewExecCmdFromSlice(cmd []string) *ExecCmd {
	return &ExecCmd{
		Cmd: cmd,
	}
}

// Stdout type alias for a string is an artificial type
// to allow for custom marshaling of stdout output which can be either
// a valid or non valid JSON.
// For that reason a custom MarshalJSON method is implemented to take care of both.
type Stdout string

// MarshalJSON implements a custom marshaller for a custom Stdout type.
func (s Stdout) MarshalJSON() ([]byte, error) {
	switch {
	case json.Valid([]byte(s)):
		return []byte(s), nil
	default:
		return json.Marshal(string(s))
	}
}

// ExecResult represents a result of a command execution.
type ExecResult struct {
	Cmd        []string `json:"cmd"`
	ReturnCode int      `json:"return-code"`
	Stdout     Stdout   `json:"stdout"`
	Stderr     string   `json:"stderr"`
}

func NewExecResult(op *ExecCmd) *ExecResult {
	er := &ExecResult{Cmd: op.GetCmd()}
	return er
}

// SetCmd sets the command that is to be executed.
func (e *ExecCmd) SetCmd(cmd string) error {
	c, err := shlex.Split(cmd)
	if err != nil {
		return err
	}
	e.Cmd = c
	return nil
}

// GetCmd sets the command that is to be executed.
func (e *ExecCmd) GetCmd() []string {
	return e.Cmd
}

// GetCmdString sets the command that is to be executed.
func (e *ExecCmd) GetCmdString() string {
	return strings.Join(e.Cmd, " ")
}

func (e *ExecResult) String() string {
	var s strings.Builder

	s.WriteString(fmt.Sprintf("Cmd: %s\nReturnCode: %d", e.GetCmdString(), e.ReturnCode))

	if e.Stdout != "" {
		s.WriteString(fmt.Sprintf("\nStdout: %q", e.Stdout))
	}
	if e.Stderr != "" {
		s.WriteString(fmt.Sprintf("\nStderr: %q", e.Stderr))
	}

	return s.String()
}

// Dump dumps execution result as a string in one of the provided formats.
func (e *ExecResult) Dump(format string) (string, error) {
	var result string
	switch format {
	case clabconstants.FormatJSON:
		byteData, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			return "", err
		}
		result = string(byteData)
	case clabconstants.FormatPlain:
		result = e.String()
	}
	return result, nil
}

// GetCmdString returns the initially parsed cmd as a string for e.g. log output purpose.
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
	return e.Stderr
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
	e.Stdout = Stdout(data)
}

func (e *ExecResult) SetStdErr(data []byte) {
	e.Stderr = string(data)
}

// execEntries is a map indexed by container IDs storing lists of ExecResult.
type execEntries map[string][]*ExecResult

// ExecCollection represents a datastore for exec commands execution results.
type ExecCollection struct {
	execEntries
	m sync.RWMutex
}

// NewExecCollection initializes the collection of exec command results.
func NewExecCollection() *ExecCollection {
	return &ExecCollection{
		execEntries: execEntries{},
		m:           sync.RWMutex{},
	}
}

func (ec *ExecCollection) Add(cId string, e *ExecResult) {
	ec.m.Lock()
	defer ec.m.Unlock()
	ec.execEntries[cId] = append(ec.execEntries[cId], e)
}

func (ec *ExecCollection) AddAll(cId string, e []*ExecResult) {
	ec.m.Lock()
	defer ec.m.Unlock()
	ec.execEntries[cId] = append(ec.execEntries[cId], e...)
}

// Dump dumps the contents of ExecCollection as a string in one of the provided formats.
func (ec *ExecCollection) Dump(format string) (string, error) {
	ec.m.RLock()
	defer ec.m.RUnlock()
	result := strings.Builder{}
	switch format {
	case clabconstants.FormatJSON:
		byteData, err := json.MarshalIndent(ec.execEntries, "", "  ")
		if err != nil {
			return "", err
		}

		result.Write(byteData)
	case clabconstants.FormatPlain:
		printSep := false
		for k, execResults := range ec.execEntries {
			if len(execResults) == 0 {
				// skip if there is no result
				continue
			}

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

// Log writes to the log execution results stored in ExecCollection.
// If execution result contains error, the error log facility is used,
// otherwise it is logged as INFO.
func (ec *ExecCollection) Log() {
	ec.m.RLock()
	defer ec.m.RUnlock()
	for k, execResults := range ec.execEntries {
		for _, er := range execResults {
			switch {
			case er.GetReturnCode() != 0:
				log.Error(
					"Failed to execute command",
					"command", er.GetCmdString(),
					"node", k,
					"rc", er.GetReturnCode(),
					"stdout", er.GetStdOutString(),
					"stderr", er.GetStdErrString(),
				)
			default:
				log.Info(
					"Executed command",
					"node", k,
					"command", er.GetCmdString(),
					"stdout", er.GetStdOutString(),
				)
			}
		}
	}
}
