package config

import (
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type SshSession struct {
	In      io.Reader
	Out     io.WriteCloser
	Session *ssh.Session
}

// Display the SSH login message
var LoginMessages bool

// The reply the execute command and the prompt.
type SshReply struct{ result, prompt string }

// SshTransport setting needs to be set before calling Connect()
// SshTransport implement the Transport interface
type SshTransport struct {
	// Channel used to read. Can use Expect to Write & read wit timeout
	in chan SshReply
	// SSH Session
	ses *SshSession
	// Contains the first read after connecting
	LoginMessage SshReply

	// SSH parameters used in connect
	// defualt: 22
	Port int
	// SSH Options
	// required!
	SshConfig *ssh.ClientConfig
	// Character to split the incoming stream (#/$/>)
	// default: #
	PromptChar string
	// Prompt parsing function. Default return the last line of the #
	// default: DefaultPrompParse
	PromptParse func(in *string) *SshReply
}

// This is the default prompt parse function used by SSH transport
func DefaultPrompParse(in *string) *SshReply {
	n := strings.LastIndex(*in, "\n")
	if strings.Contains((*in)[n:], " ") {
		return &SshReply{
			result: *in,
			prompt: "",
		}
	}
	res := (*in)[:n]
	n = strings.LastIndex(res, "\n")
	if n < 0 {
		n = 0
	}
	return &SshReply{
		result: (*in)[:n],
		prompt: (*in)[n:] + "#",
	}
}

// The channel does
func (t *SshTransport) InChannel() {
	// Ensure we have one working channel
	t.in = make(chan SshReply)

	// setup a buffered string channel
	go func() {
		buf := make([]byte, 1024)
		tmpS := ""
		n, err := t.ses.In.Read(buf) //this reads the ssh terminal
		if err == nil {
			tmpS = string(buf[:n])
		}
		for err == nil {

			if strings.Contains(tmpS, "#") {
				parts := strings.Split(tmpS, "#")
				li := len(parts) - 1
				for i := 0; i < li; i++ {
					t.in <- *t.PromptParse(&parts[i])
				}
				tmpS = parts[li]
			}
			n, err = t.ses.In.Read(buf)
			tmpS += string(buf[:n])
		}
		log.Debugf("In Channel closing: %v", err)
		t.in <- SshReply{
			result: tmpS,
			prompt: "",
		}
	}()

	t.LoginMessage = t.Run("", 15)
	if LoginMessages {
		log.Infof("%s\n", t.LoginMessage.result)
	}
	//log.Debugf("%s\n", t.BootMsg.prompt)
}

// Run a single command and wait for the reply
func (t *SshTransport) Run(command string, timeout int) SshReply {
	if command != "" {
		t.ses.Writeln(command)
	}

	sHistory := ""

	for {
		// Read from the channel with a timeout
		select {
		case <-time.After(time.Duration(timeout) * time.Second):
			log.Warnf("timeout waiting for prompt: %s", command)
			return SshReply{}
		case ret := <-t.in:
			if ret.prompt == "" && ret.result != "" {
				// we should continue reading...
				sHistory += ret.result
				timeout = 1 // reduce timeout, node is already sending data
				continue
			}
			if ret.result == "" && ret.prompt == "" {
				log.Errorf("received zero?")
				continue
			}
			rr := strings.Trim(ret.result, " \n")
			if sHistory != "" {
				rr = sHistory + rr
				sHistory = ""
			}

			if strings.HasPrefix(rr, command) {
				rr = strings.Trim(rr[len(command):], " \n\r")
				// fmt.Print(rr)
			} else if !strings.Contains(rr, command) {
				sHistory = rr
				continue
			}
			return SshReply{
				result: rr,
				prompt: ret.prompt,
			}
		}
	}
}

// Write a config snippet (a set of commands)
// Session NEEDS to be configurable for other kinds
// Part of the Transport interface
func (t *SshTransport) Write(snip *ConfigSnippet) error {
	t.Run("/configure global", 2)
	t.Run("discard", 2)

	c, b := 0, 0
	for _, l := range snip.Lines() {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		c += 1
		b += len(l)
		t.Run(l, 3)
	}

	// Commit
	commit := t.Run("commit", 10)
	//commit += t.Run("", 10)
	log.Infof("COMMIT %s - %d lines %d bytes\n%s", snip, c, b, commit.result)
	return nil
}

// Connect to a host
// Part of the Transport interface
func (t *SshTransport) Connect(host string) error {
	// Assign Default Values
	if t.PromptParse == nil {
		t.PromptParse = DefaultPrompParse
	}
	if t.PromptChar == "" {
		t.PromptChar = "#"
	}
	if t.Port == 0 {
		t.Port = 22
	}
	if t.SshConfig == nil {
		return fmt.Errorf("require auth credentials in SshConfig")
	}

	// Start some client config
	host = fmt.Sprintf("%s:%d", host, t.Port)
	//sshConfig := &ssh.ClientConfig{}
	//SshConfigWithUserNamePassword(sshConfig, "admin", "admin")

	ses_, err := NewSshSession(host, t.SshConfig)
	if err != nil || ses_ == nil {
		return fmt.Errorf("cannot connect to %s: %s", host, err)
	}
	t.ses = ses_

	log.Infof("Connected to %s\n", host)
	t.InChannel()
	//Read to first prompt
	return nil
}

// Close the Session and channels
// Part of the Transport interface
func (t *SshTransport) Close() {
	if t.in != nil {
		close(t.in)
		t.in = nil
	}
	t.ses.Close()
}

// Add a basic username & password to a config.
// Will initilize the config if required
func SshConfigWithUserNamePassword(config *ssh.ClientConfig, username, password string) {
	if config == nil {
		config = &ssh.ClientConfig{}
	}
	config.User = username
	if config.Auth == nil {
		config.Auth = []ssh.AuthMethod{}
	}
	config.Auth = append(config.Auth, ssh.Password(password))
	config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
}

// Create a new SSH session (Dial, open in/out pipes and start the shell)
// pass the authntication details in sshConfig
func NewSshSession(host string, sshConfig *ssh.ClientConfig) (*SshSession, error) {
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
	if err := session.Shell(); err != nil {
		session.Close()
		return nil, fmt.Errorf("session shell: %s", err)
	}

	return &SshSession{
		Session: session,
		In:      sshIn,
		Out:     sshOut,
	}, nil
}

func (ses *SshSession) Writeln(command string) (int, error) {
	return ses.Out.Write([]byte(command + "\r"))
}

func (ses *SshSession) Close() {
	log.Debugf("Closing session")
	ses.Session.Close()
}
