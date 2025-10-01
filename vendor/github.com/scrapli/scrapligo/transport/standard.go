package transport

import (
	"fmt"
	"io"
	"os"

	"github.com/scrapli/scrapligo/util"

	"golang.org/x/crypto/ssh/knownhosts"

	"golang.org/x/crypto/ssh"
)

const (
	// StandardTransport is the standard (crypto/ssh) transport for scrapligo.
	StandardTransport = "standard"

	termType = "xterm"

	defaultTTYSpeed = 115200
)

// NewStandardTransport returns an instance of Standard transport.
func NewStandardTransport(s *SSHArgs) (*Standard, error) {
	t := &Standard{
		SSHArgs:      s,
		client:       nil,
		session:      nil,
		writer:       nil,
		reader:       nil,
		ExtraCiphers: make([]string, 0),
		ExtraKexs:    make([]string, 0),
	}

	return t, nil
}

// Standard is the standard (crypto/ssh) transport object.
type Standard struct {
	SSHArgs      *SSHArgs
	client       *ssh.Client
	session      *ssh.Session
	writer       io.WriteCloser
	reader       io.Reader
	ExtraCiphers []string
	ExtraKexs    []string
}

func (t *Standard) openSession(a *Args, cfg *ssh.ClientConfig) error {
	var err error

	t.client, err = ssh.Dial(
		tcp,
		fmt.Sprintf("%s:%d", a.Host, a.Port),
		cfg,
	)
	if err != nil {
		a.l.Criticalf("error creating crypto/ssh client, error: %s", err)

		return err
	}

	t.session, err = t.client.NewSession()
	if err != nil {
		a.l.Criticalf("error spawning crypto/ssh session, error: %s", err)

		return err
	}

	t.writer, err = t.session.StdinPipe()
	if err != nil {
		a.l.Criticalf("error spawning crypto/ssh session stdin pipe, error: %s", err)

		return err
	}

	t.reader, err = t.session.StdoutPipe()
	if err != nil {
		a.l.Criticalf("error spawning crypto/ssh session stdout pipe, error: %s", err)

		return err
	}

	return nil
}

func (t *Standard) openBase(a *Args) error {
	/* #nosec G106 */
	keyCallback := ssh.InsecureIgnoreHostKey()

	if t.SSHArgs.StrictKey {
		if t.SSHArgs.KnownHostsFile == "" {
			a.l.Critical("strict host key checking requested, but no known hosts file provided")

			return fmt.Errorf(
				"%w: strict host key checking requested, but no known hosts file provided",
				util.ErrBadOption,
			)
		}

		knownHosts, err := knownhosts.New(t.SSHArgs.KnownHostsFile)
		if err != nil {
			return err
		}

		keyCallback = knownHosts
	}

	authMethods := make([]ssh.AuthMethod, 0)

	if t.SSHArgs.PrivateKeyPath != "" {
		k, err := os.ReadFile(t.SSHArgs.PrivateKeyPath)
		if err != nil {
			a.l.Criticalf("error reading ssh key: %s", err)

			return err
		}

		signer, err := ssh.ParsePrivateKey(k)
		if err != nil {
			a.l.Criticalf("error parsing ssh key: %s", err)

			return err
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if a.Password != "" {
		authMethods = append(authMethods, ssh.Password(a.Password),
			ssh.KeyboardInteractive(
				func(_, _ string, questions []string, _ []bool) ([]string, error) {
					answers := make([]string, len(questions))
					for i := range answers {
						answers[i] = a.Password
					}

					return answers, nil
				},
			))
	}

	cfg := &ssh.ClientConfig{
		User:            a.User,
		Auth:            authMethods,
		Timeout:         a.TimeoutSocket,
		HostKeyCallback: keyCallback,
	}

	if len(t.ExtraCiphers) > 0 {
		cfg.Config.Ciphers = append(cfg.Config.Ciphers, t.ExtraCiphers...)
	}

	if len(t.ExtraKexs) > 0 {
		cfg.Config.KeyExchanges = append(cfg.Config.KeyExchanges, t.ExtraKexs...)
	}

	return t.openSession(a, cfg)
}

func (t *Standard) open(a *Args) error {
	err := t.openBase(a)
	if err != nil {
		return err
	}

	term := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: defaultTTYSpeed,
		ssh.TTY_OP_OSPEED: defaultTTYSpeed,
	}

	err = t.session.RequestPty(
		termType,
		a.TermHeight,
		a.TermWidth,
		term,
	)
	if err != nil {
		return err
	}

	return t.session.Shell()
}

func (t *Standard) openNetconf(a *Args) error {
	err := t.openBase(a)
	if err != nil {
		return err
	}

	err = t.session.RequestSubsystem("netconf")

	return err
}

// Open opens the Standard transport.
func (t *Standard) Open(a *Args) error {
	if t.SSHArgs.NetconfConnection {
		return t.openNetconf(a)
	}

	return t.open(a)
}

// Close closes the Standard transport.
func (t *Standard) Close() error {
	if t.session != nil {
		err := t.session.Close()
		if err != nil {
			return err
		}

		t.session = nil
	}

	if t.client != nil {
		err := t.client.Close()
		if err != nil {
			return err
		}

		t.client = nil
	}

	return nil
}

// IsAlive returns true if the Standard transport session attribute is not nil.
func (t *Standard) IsAlive() bool {
	return t.session != nil
}

// Read reads n bytes from the transport.
func (t *Standard) Read(n int) ([]byte, error) {
	b := make([]byte, n)

	n, err := t.reader.Read(b)
	if err != nil {
		return nil, err
	}

	return b[0:n], nil
}

// Write writes bytes b to the transport.
func (t *Standard) Write(b []byte) error {
	_, err := t.writer.Write(b)

	return err
}
