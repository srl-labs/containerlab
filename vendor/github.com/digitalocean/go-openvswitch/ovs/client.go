// Copyright 2017 DigitalOcean.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
)

// A Client is a client type which enables programmatic control of Open
// vSwitch.
type Client struct {
	// OpenFlow wraps functionality of the 'ovs-ofctl' binary.
	OpenFlow *OpenFlowService

	// App wraps functionality of the 'ovs-appctl' binary
	App *AppService

	// VSwitch wraps functionality of the 'ovs-vsctl' binary.
	VSwitch *VSwitchService

	// Additional flags applied to all OVS actions, such as timeouts
	// or retries.
	flags []string

	// Additional flags applied to 'ovs-ofctl' commands.
	ofctlFlags []string

	// Enable or disable debugging log messages for OVS commands.
	debug bool

	// Prefix all commands with "sudo".
	sudo bool

	// Implementation of ExecFunc.
	execFunc ExecFunc

	// Implementation of PipeFunc.
	pipeFunc PipeFunc
}

// An ExecFunc is a function which accepts input arguments and returns raw
// byte output and an error.  ExecFuncs are swappable to enable testing
// without OVS installed.
type ExecFunc func(cmd string, args ...string) ([]byte, error)

// shellExec is an ExecFunc which shells out to the binary cmd using the
// arguments args, and returns its combined stdout and stderr and any errors
// which may have occurred.
func shellExec(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}

// exec executes an ExecFunc using the values from cmd and args.
// The ExecFunc may shell out to an appropriate binary, or may be swapped
// for testing.
func (c *Client) exec(cmd string, args ...string) ([]byte, error) {
	// Prepend recurring flags before arguments
	flags := append(c.flags, args...)

	// If needed, prefix sudo.
	if c.sudo {
		flags = append([]string{cmd}, flags...)
		cmd = "sudo"
	}

	c.debugf("exec: %s %v", cmd, flags)

	// Execute execFunc with all flags and clean up any whitespace or
	// newlines from its output.
	out, err := c.execFunc(cmd, flags...)
	if out != nil {
		out = bytes.TrimSpace(out)
		c.debugf("exec: %q", string(out))
	}
	if err != nil {
		// Wrap errors in Error type for further introspection
		return nil, &Error{
			Out: out,
			Err: err,
		}
	}

	return out, nil
}

// A PipeFunc is a function which accepts an input stdin stream, command,
// and arguments, and returns command output and an error.  PipeFuncs are
// swappable to enable testing without OVS installed.
type PipeFunc func(stdin io.Reader, cmd string, args ...string) ([]byte, error)

// shellPipe is a PipeFunc which shells out to the binary cmd using the arguments
// args, and writing to the command's stdin using stdin.
func shellPipe(stdin io.Reader, cmd string, args ...string) ([]byte, error) {
	command := exec.Command(cmd, args...)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}

	wc, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := command.Start(); err != nil {
		return nil, err
	}

	if _, err := io.Copy(wc, stdin); err != nil {
		return nil, err
	}

	// Needed to indicate to ovs-ofctl that stdin is done being read.
	// "... if the command being run will not exit until standard input is
	// closed, the caller must close the pipe."
	// Reference: https://golang.org/pkg/os/exec/#Cmd.StdinPipe
	if err := wc.Close(); err != nil {
		return nil, err
	}

	mr := io.MultiReader(stdout, stderr)
	b, err := ioutil.ReadAll(mr)
	if err != nil {
		return nil, err
	}

	return b, command.Wait()
}

// pipe executes a PipeFunc using the values from stdin, cmd, and args.
// stdin is used to feed input data to the stdin of a forked process.
// The PipeFunc may shell out to an appropriate binary, or may be swapped
// for testing.
func (c *Client) pipe(stdin io.Reader, cmd string, args ...string) error {
	// Prepend recurring flags before arguments
	flags := append(c.flags, args...)

	// If needed, prefix sudo.
	if c.sudo {
		flags = append([]string{cmd}, flags...)
		cmd = "sudo"
	}

	c.debugf("pipe: %s %v", cmd, flags)
	c.debugf("bundle:")

	tr := io.TeeReader(stdin, writerFunc(func(p []byte) (int, error) {
		c.debugf("%s", string(p))
		return len(p), nil
	}))

	if out, err := c.pipeFunc(tr, cmd, flags...); err != nil {
		c.debugf("pipe error: %v: %q", err, string(out))
		return &pipeError{
			out: out,
			err: err,
		}
	}

	return nil

}

// A pipeError is an error returned by Client.pipe, containing combined
// stdout/stderr from a process as well as its error.
type pipeError struct {
	out []byte
	err error
}

// Error returns the string representation of a pipeError.
func (e *pipeError) Error() string {
	return fmt.Sprintf("pipe error: %v: %q", e.err, string(e.out))
}

// debugf prints a logging debug message when debugging is enabled.
func (c *Client) debugf(format string, a ...interface{}) {
	if !c.debug {
		return
	}

	log.Printf("ovs: "+format, a...)
}

// New creates a new Client with zero or more OptionFunc configurations
// applied.
func New(options ...OptionFunc) *Client {
	// Always execute and pipe using shell when created with New.
	c := &Client{
		flags:      make([]string, 0),
		ofctlFlags: make([]string, 0),
		execFunc:   shellExec,
		pipeFunc:   shellPipe,
	}
	for _, o := range options {
		o(c)
	}

	vss := &VSwitchService{
		c: c,
	}
	vss.Get = &VSwitchGetService{
		v: vss,
	}
	vss.Set = &VSwitchSetService{
		v: vss,
	}
	c.VSwitch = vss

	ofs := &OpenFlowService{
		c: c,
	}
	c.OpenFlow = ofs

	app := &AppService{
		c: c,
	}
	c.App = app

	return c
}

// An OptionFunc is a function which can apply configuration to a Client.
type OptionFunc func(c *Client)

// Timeout returns an OptionFunc which sets a timeout in seconds for all
// Open vSwitch interactions.
func Timeout(seconds int) OptionFunc {
	return func(c *Client) {
		c.flags = append(c.flags, fmt.Sprintf("--timeout=%d", seconds))
	}
}

// Debug returns an OptionFunc which enables debugging output for the Client
// type.
func Debug(enable bool) OptionFunc {
	return func(c *Client) {
		c.debug = enable
	}
}

// Exec returns an OptionFunc which sets an ExecFunc for use with a Client.
// This function should typically only be used in tests.
func Exec(fn ExecFunc) OptionFunc {
	return func(c *Client) {
		c.execFunc = fn
	}
}

// Pipe returns an OptionFunc which sets a PipeFunc for use with a Client.
// This function should typically only be used in tests.
func Pipe(fn PipeFunc) OptionFunc {
	return func(c *Client) {
		c.pipeFunc = fn
	}
}

const (
	// FlowFormatNXMTableID is a flow format which allows Nicira Extended match
	// with the ability to place a flow in a specific table.
	FlowFormatNXMTableID = "NXM+table_id"

	// FlowFormatOXMOpenFlow14 is a flow format which allows Open vSwitch
	// extensible match.
	FlowFormatOXMOpenFlow14 = "OXM-OpenFlow14"
)

// FlowFormat specifies the flow format to be used when shelling to
// 'ovs-ofctl'.
func FlowFormat(format string) OptionFunc {
	return func(c *Client) {
		c.ofctlFlags = append(c.ofctlFlags, fmt.Sprintf("--flow-format=%s", format))
	}
}

// Protocol constants for use with Protocols and BridgeOptions.
const (
	ProtocolOpenFlow10 = "OpenFlow10"
	ProtocolOpenFlow11 = "OpenFlow11"
	ProtocolOpenFlow12 = "OpenFlow12"
	ProtocolOpenFlow13 = "OpenFlow13"
	ProtocolOpenFlow14 = "OpenFlow14"
	ProtocolOpenFlow15 = "OpenFlow15"
)

// Protocols specifies one or more OpenFlow protocol versions to be used when shelling
// to 'ovs-ofctl'.
func Protocols(versions []string) OptionFunc {
	return func(c *Client) {
		c.ofctlFlags = append(c.ofctlFlags,
			fmt.Sprintf("--protocols=%s", strings.Join(versions, ",")),
		)
	}
}

// SetSSLParam configures SSL authentication using a private key, certificate,
// and CA certificate for use with ovs-ofctl.
func SetSSLParam(pkey string, cert string, cacert string) OptionFunc {
	return func(c *Client) {
		c.ofctlFlags = append(c.ofctlFlags, fmt.Sprintf("--private-key=%s", pkey),
			fmt.Sprintf("--certificate=%s", cert), fmt.Sprintf("--ca-cert=%s", cacert))
	}
}

// SetTCPParam configures the OVSDB connection using a TCP format ip:port
// for use with all ovs-vsctl commands.
func SetTCPParam(addr string) OptionFunc {
	return func(c *Client) {
		c.flags = append(c.flags, fmt.Sprintf("--db=tcp:%s", addr))
	}
}

// Sudo specifies that "sudo" should be prefixed to all OVS commands.
func Sudo() OptionFunc {
	return func(c *Client) {
		c.sudo = true
	}
}

// A writerFunc is an adapter for a function to be used as an io.Writer.
type writerFunc func(p []byte) (n int, err error)

func (fn writerFunc) Write(p []byte) (int, error) {
	return fn(p)
}
