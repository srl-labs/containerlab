package transport

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabtypes "github.com/srl-labs/containerlab/types"
	"golang.org/x/crypto/ssh"
)

type SSHSession struct {
	In      io.Reader
	Out     io.WriteCloser
	Session *ssh.Session
}

type SSHTransportOption func(*SSHTransport) error

// SSHReply is SSH reply, executed command and the prompt.
type SSHReply struct{ result, prompt, command string }

// SSHTransport setting needs to be set before calling Connect()
// SSHTransport implements the Transport interface.
type SSHTransport struct {
	// Channel used to read. Can use Expect to Write & read with timeout
	in chan SSHReply
	// SSH Session
	ses *SSHSession
	// Contains the first read after connecting
	LoginMessage *SSHReply
	// SSH parameters used in connect
	// default: 22
	Port int

	// Keep the target for logging
	Target string

	// SSH Options
	// required!
	SSHConfig *ssh.ClientConfig

	// Character to split the incoming stream (#/$/>)
	// default: #
	PromptChar string

	// Kind specific transactions & prompt checking function
	K SSHKind

	debug bool
}

func WithDebug() SSHTransportOption {
	return func(tx *SSHTransport) error {
		tx.debug = true
		return nil
	}
}

// WithUserNamePassword adds username & password authentication.
func WithUserNamePassword(username, password string) SSHTransportOption {
	return func(tx *SSHTransport) error {
		tx.SSHConfig.User = username
		if tx.SSHConfig.Auth == nil {
			tx.SSHConfig.Auth = []ssh.AuthMethod{}
		}

		tx.SSHConfig.Auth = append(tx.SSHConfig.Auth, ssh.Password(password))

		return nil
	}
}

// HostKeyCallback adds a basic username & password to a config.
// Will initialize the config if required.
func HostKeyCallback(callback ...ssh.HostKeyCallback) SSHTransportOption {
	return func(tx *SSHTransport) error {
		tx.SSHConfig.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if len(callback) == 0 {
				log.Warnf("Skipping host key verification for %s", hostname)

				return nil
			}

			for _, hkc := range callback {
				if hkc(hostname, remote, key) == nil {
					return nil
				}
			}

			return fmt.Errorf("invalid host key %s: %s", hostname, key)
		}

		return nil
	}
}

func NewSSHTransport(
	node *clabtypes.NodeConfig,
	options ...SSHTransportOption,
) (*SSHTransport, error) {
	switch node.Kind {
	case "vr-sros", "srl", "nokia_sros", "nokia_srsim", "nokia_srlinux":
		c := &SSHTransport{}
		c.SSHConfig = &ssh.ClientConfig{}

		// apply options
		for _, opt := range options {
			err := opt(c)
			if err != nil {
				return nil, err
			}
		}

		switch node.Kind {
		case "vr-sros", "nokia_sros":
			c.K = &VrSrosSSHKind{}
		case "nokia_srsim", "srsim":
			c.K = &SrosSSHKind{}
		case "srl", "nokia_srlinux":
			c.K = &SrlSSHKind{}
		}

		return c, nil
	}

	return nil, fmt.Errorf("no transport implemented for kind: %s", node.Kind)
}

// InChannel creates the channel reading the SSH connection.
//
// # The first prompt is saved in LoginMessages
//
//   - The channel read the SSH session, splits on PromptChar
//   - Uses SSHKind's PromptParse to split the received data in *result* and *prompt* parts
//     (if no valid prompt was found, prompt will simply be empty and result contain all the data)
//   - Emit data.
func (t *SSHTransport) InChannel() {
	// Ensure we have a working channel
	t.in = make(chan SSHReply)

	// setup a buffered string channel
	go func() {
		buf := make([]byte, 1024)
		tmpS := ""

		n, err := t.ses.In.Read(buf) // this reads the ssh terminal
		if err == nil {
			tmpS = string(buf[:n])
		}

		for err == nil {
			if strings.Contains(tmpS, "#") {
				parts := strings.Split(tmpS, "#")
				li := len(parts) - 1

				for i := range li {
					r := t.K.PromptParse(t, &parts[i])
					if r == nil {
						r = &SSHReply{
							result: parts[i],
						}
					}

					t.in <- *r
				}

				tmpS = parts[li]
			}

			n, err = t.ses.In.Read(buf)
			tmpS += string(buf[:n])
		}

		log.Debugf("In Channel closing: %v", err)

		t.in <- SSHReply{
			result: tmpS,
			prompt: "",
		}
	}()

	// Save first prompt
	t.LoginMessage = t.Run("", 15)

	if t.debug {
		t.LoginMessage.Info(t.Target)
	}
}

// Run a single command and wait for the reply.
func (t *SSHTransport) Run(command string, timeout int) *SSHReply {
	if command != "" {
		t.ses.Writeln(command)
		log.Debugf("--> %s\n", command)
	}

	sHistory := ""

	for {
		// Read from the channel with a timeout
		var rr string

		select {
		case <-time.After(time.Duration(timeout) * time.Second):
			log.Warnf("timeout waiting for prompt: %s", command)

			return &SSHReply{
				result:  sHistory,
				command: command,
			}
		case ret := <-t.in:
			if t.debug {
				ret.Debug(t.Target, command+"<--InChannel--")
			}

			if ret.result == "" && ret.prompt == "" {
				log.Error("received zero?")
				continue
			}

			if ret.prompt == "" && ret.result != "" {
				// we should continue reading...
				sHistory += ret.result

				if t.debug {
					log.Debugf("+")
				}

				timeout = 2 // reduce timeout, node is already sending data

				continue
			}

			if sHistory == "" {
				rr = ret.result
			} else {
				rr = sHistory + "#" + ret.result
				sHistory = "" //nolint:ineffassign,wastedassign
			}

			rr = strings.Trim(rr, " \n\r\t")

			if strings.HasPrefix(rr, command) {
				rr = strings.Trim(rr[len(command):], " \n\r\t")
			} else if !strings.Contains(rr, command) {
				log.Debugf("read more %s:%s", command, rr)
				sHistory = rr

				continue
			}

			res := &SSHReply{
				result:  rr,
				prompt:  ret.prompt,
				command: command,
			}

			res.Debug(t.Target, command+"<--RUN--")

			return res
		}
	}
}

// Write a config snippet (a set of commands)
// Session NEEDS to be configurable for other kinds
// Part of the Transport interface.
func (t *SSHTransport) Write(data, info *string) error {
	if *data == "" {
		return nil
	}

	transaction := !strings.HasPrefix(*info, "show-")

	err := t.K.ConfigStart(t, transaction)
	if err != nil {
		return err
	}

	c := 0

	for _, l := range strings.Split(*data, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}

		c += 1

		t.Run(l, 5).Info(t.Target)
	}

	if transaction {
		commit, err := t.K.ConfigCommit(t)

		msg := fmt.Sprintf("%s COMMIT - %d lines", *info, c)

		if commit.result != "" {
			msg += commit.LogString(t.Target, true, false)
		}

		if err != nil {
			log.Error(msg)

			return err
		}

		log.Info(msg)
	}

	return nil
}

// Connect to a host
// Part of the Transport interface.
func (t *SSHTransport) Connect(host string, _ ...TransportOption) error {
	// Assign Default Values
	if t.PromptChar == "" {
		t.PromptChar = "#"
	}

	if t.Port == 0 {
		t.Port = 22
	}

	if t.SSHConfig == nil {
		return fmt.Errorf("require auth credentials in SSHConfig")
	}

	// Start some client config
	host = fmt.Sprintf("%s:%d", host, t.Port)

	t.Target = host

	ses_, err := NewSSHSession(host, t.SSHConfig)
	if err != nil || ses_ == nil {
		return fmt.Errorf("cannot connect to %s: %s", host, err)
	}

	t.ses = ses_

	log.Infof("Connected to %s\n", host)
	t.InChannel()

	// Read to first prompt
	return nil
}

// Close the Session and channels
// Part of the Transport interface.
func (t *SSHTransport) Close() {
	if t.in != nil {
		close(t.in)
		t.in = nil
	}

	t.ses.Close()
}

// NewSSHSession creates a new SSH session (Dial, open in/out pipes and start the shell)
// pass the authentication details in sshConfig.
func NewSSHSession(host string, sshConfig *ssh.ClientConfig) (*SSHSession, error) {
	if !strings.Contains(host, ":") {
		return nil, fmt.Errorf("include the port in the host: %s", host)
	}

	connection, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %s", err)
	}

	session, err := connection.NewSession()
	if err != nil {
		return nil, err
	}

	sshIn, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("session stdout: %s", err)
	}

	sshOut, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("session stdin: %s", err)
	}

	// sshIn2, err := session.StderrPipe()
	// if err != nil {
	// 	return nil, fmt.Errorf("session stderr: %s", err)
	// }
	// Request PTY (required for srl)
	modes := ssh.TerminalModes{
		ssh.ECHO: 1, // disable echo
	}

	err = session.RequestPty("dumb", 24, 1000, modes)
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("pty request failed: %s", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		return nil, fmt.Errorf("session shell: %s", err)
	}

	return &SSHSession{
		Session: session,
		In:      sshIn,
		Out:     sshOut,
	}, nil
}

func (ses *SSHSession) Writeln(command string) (int, error) {
	return ses.Out.Write([]byte(command + "\r"))
}

func (ses *SSHSession) Close() {
	log.Debugf("Closing session")
	ses.Session.Close()
}

// LogString will include the entire SSHReply
//
//	Each field will be prefixed by a character.
//	# - command sent
//	| - result received
//	? - prompt part of the result
func (r *SSHReply) LogString(node string, linefeed, debug bool) string { // skipcq: RVV-A0005
	ind := 12 + len(node)

	prefix := "\n" + strings.Repeat(" ", ind)
	s := ""

	if linefeed {
		s = "\n" + strings.Repeat(" ", 11)
	}

	s += node + " # " + r.command
	s += prefix + "| "
	s += strings.Join(strings.Split(r.result, "\n"), prefix+"| ")

	if debug { // Add the prompt & more
		s = "" + strings.Repeat(" ", ind) + s
		s += prefix + "? "
		s += strings.Join(strings.Split(r.prompt, "\n"), prefix+"? ")
		s += fmt.Sprintf("%s| %v%s ? %v", prefix, []byte(r.result), prefix, []byte(r.prompt))
	}

	return s
}

func (r *SSHReply) Info(node string) *SSHReply {
	if r.result == "" {
		return r
	}

	log.Info(r.LogString(node, false, false))

	return r
}

func (r *SSHReply) Debug(node, message string, t ...any) {
	msg := message
	if len(t) > 0 {
		msg = t[0].(string)
	}

	_, fn, line, _ := runtime.Caller(1)
	msg += fmt.Sprintf("(%s line %d)", fn, line)
	msg += r.LogString(node, true, true)

	log.Debug(msg)
}
