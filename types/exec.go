package types

import (
	"encoding/json"
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

func ParseExecOutputFormat(s string) (ExecOutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(ExecFormatJSON):
		return ExecFormatJSON, nil
	case string(ExecFormatPLAIN), "table":
		return ExecFormatPLAIN, nil
	}
	return "", fmt.Errorf("cannot parse %q as 'ExecOutputFormat'", s)
}

type ExecExecutor interface {
	GetCmd() []string
	GetCmdString() string
	SetStdOut(stdout []byte)
	SetStdErr(stderr []byte)
	SetReturnCode(rc int)
}

type ExecReader interface {
	GetStdOutString() string
	GetStdErrString() string
	GetStdOutByteSlice() []byte
	GetStdErrByteSlice() []byte
	GetReturnCode() int
	SetCmd(s string) error
	GetCmdString() string
	GetEntryInFormat(format ExecOutputFormat) (string, error)
	String() string
}

type Exec struct {
	Cmd        []string `json:"cmd"`
	ReturnCode int      `json:"returnCode"`
	Stdout     string   `json:"stdout"`
	Stderr     string   `json:"stderr"`
}

func NewExec(cmd string) (*Exec, error) {
	result := &Exec{}
	if err := result.SetCmd(cmd); err != nil {
		return nil, err
	}
	return result, nil
}

func NewExecSlice(cmd []string) *Exec {
	return &Exec{
		Cmd: cmd,
	}
}

// SetCmd sets the command that is to be executed
func (e *Exec) SetCmd(cmd string) error {
	c, err := shlex.Split(cmd)
	if err != nil {
		return err
	}
	e.Cmd = c
	return nil
}

func (e *Exec) String() string {
	return fmt.Sprintf("Cmd: %s\nReturnCode: %d\nStdOut:\n%s\nStdErr:\n%s\n", e.GetCmdString(), e.ReturnCode, e.Stdout, e.Stderr)
}

func (e *Exec) GetEntryInFormat(format ExecOutputFormat) (string, error) {
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
func (e *Exec) GetCmdString() string {
	return strings.Join(e.Cmd, " ")
}

func (e *Exec) GetReturnCode() int {
	return e.ReturnCode
}

func (e *Exec) SetReturnCode(rc int) {
	e.ReturnCode = rc
}

func (e *Exec) GetStdOutString() string {
	return string(e.Stdout)
}

func (e *Exec) GetStdErrString() string {
	return string(e.Stderr)
}

func (e *Exec) GetStdOutByteSlice() []byte {
	return []byte(e.Stdout)
}

func (e *Exec) GetStdErrByteSlice() []byte {
	return []byte(e.Stderr)
}

func (e *Exec) GetCmd() []string {
	return e.Cmd
}

func (e *Exec) SetStdOut(data []byte) {
	e.Stdout = string(data)
}

func (e *Exec) SetStdErr(data []byte) {
	e.Stderr = string(data)
}

// internal data struct
type execCollectionData map[string][]ExecReader

type ExecCollection struct {
	execCollectionData
}

func NewExecCollection() *ExecCollection {
	return &ExecCollection{
		execCollectionData: map[string][]ExecReader{},
	}
}

func (ec *ExecCollection) Add(cId string, e ExecReader) {
	ec.execCollectionData[cId] = append(ec.execCollectionData[cId], e)
}

func (ec *ExecCollection) AddAll(cId string, e []ExecReader) {
	ec.execCollectionData[cId] = append(ec.execCollectionData[cId], e...)
}

func (ec *ExecCollection) GetInFormat(format ExecOutputFormat) (string, error) {
	result := strings.Builder{}
	switch format {
	case ExecFormatJSON:
		byteData, err := json.MarshalIndent(ec.execCollectionData, "", "  ")
		if err != nil {
			return "", err
		}
		result.Write(byteData)
	case ExecFormatPLAIN:
		printSep := false
		for k, execResults := range ec.execCollectionData {
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
	for k, execResults := range ec.execCollectionData {
		for _, er := range execResults {
			log.Infof("Executed command '%s' on %s. stdout:\n%s", er.GetCmdString(), k, er.GetStdOutString())
		}
	}
}
