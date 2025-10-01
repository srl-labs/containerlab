package transport

import (
	"errors"
	"sync"
	"time"

	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/util"
)

// InChannelAuthType is an enum-ish string that represents valid in channel auth flavors.
type InChannelAuthType string

const (
	// InChannelAuthUnsupported indicates that the transport does *not* support in channel auth.
	InChannelAuthUnsupported InChannelAuthType = "unsupported"
	// InChannelAuthSSH indicates that the transport supports in channel ssh auth.
	InChannelAuthSSH InChannelAuthType = "ssh"
	// InChannelAuthTelnet indicates that the transport supports in channel telnet auth.
	InChannelAuthTelnet InChannelAuthType = "telnet"
)

const (
	// DefaultTransport is the default transport constant for scrapligo, this defaults to the
	// "system" transport.
	DefaultTransport = "system"

	defaultPort                 = 22
	defaultTimeoutSocketSeconds = 30
	defaultReadSize             = 8_192
	defaultTermHeight           = 255
	defaultTermWidth            = 80
	defaultSSHStrictKey         = true
	tcp                         = "tcp"
)

// InChannelAuthData is a struct containing all necessary information for the Channel to handle
// "in-channel" auth if necessary.
type InChannelAuthData struct {
	Type                 InChannelAuthType
	User                 string
	Password             string
	PrivateKeyPassPhrase string
}

// NewArgs returns an instance of Args with the logging instance, host, and any provided args
// set. Users should *generally* not need to call this function as this is called during Transport
// creation (which is called by the Driver creation).
func NewArgs(l *logging.Instance, host string, options ...util.Option) (*Args, error) {
	a := &Args{
		l:             l,
		Host:          host,
		Port:          defaultPort,
		TimeoutSocket: defaultTimeoutSocketSeconds * time.Second,
		ReadSize:      defaultReadSize,
		TermHeight:    defaultTermHeight,
		TermWidth:     defaultTermWidth,
	}

	for _, option := range options {
		err := option(a)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return a, nil
}

// Args is a struct representing common transport arguments.
type Args struct {
	l                  *logging.Instance
	UserImplementation Implementation
	Host               string
	Port               int
	User               string
	Password           string
	TimeoutSocket      time.Duration
	ReadSize           int
	TermHeight         int
	TermWidth          int
}

// NewSSHArgs returns an instance of SSH arguments with provided options set. Just like NewArgs,
// this should generally not be called by users directly.
func NewSSHArgs(options ...util.Option) (*SSHArgs, error) {
	a := &SSHArgs{
		StrictKey: defaultSSHStrictKey,
	}

	for _, option := range options {
		err := option(a)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return a, nil
}

// SSHArgs is a struct representing common transport SSH specific arguments.
type SSHArgs struct {
	StrictKey            bool
	PrivateKeyPath       string
	PrivateKeyPassPhrase string
	ConfigFile           string
	KnownHostsFile       string
	NetconfConnection    bool
}

// NewTelnetArgs returns an instance of TelnetArgs with any provided options set. This should,
// just like the other NewXArgs functions, not be called directly by users.
func NewTelnetArgs(options ...util.Option) (*TelnetArgs, error) {
	a := &TelnetArgs{}

	for _, option := range options {
		err := option(a)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return a, nil
}

// TelnetArgs is a struct representing common transport Telnet specific arguments.
type TelnetArgs struct{}

// Implementation defines a valid base scrapligo transport -- for SSH-specific transports users
// should satisfy SSHImplementation -- and for transports that require authentication "in channel"
// users should satisfy InChannelAuthImplementation (this could be in addition to the SSH one!).
type Implementation interface {
	Open(a *Args) error
	Close() error
	IsAlive() bool
	Read(n int) ([]byte, error)
	Write(b []byte) error
}

// SSHImplementation is an interface that SSH transports *may* implement, this is currently only
// required if the SSH transport also requires (or just supports) "in-channel" ssh authentication.
type SSHImplementation interface {
	Implementation
	GetSSHArgs() *SSHArgs
}

// InChannelAuthImplementation is an interface that when satisfied tells us that the transport
// wants to do "in channel" authentication -- meaning actually look for user/password prompt and
// send those values in the connection rather than in the protocol/out of band.
type InChannelAuthImplementation interface {
	Implementation
	GetInChannelAuthType() InChannelAuthType
}

// Transport is a struct which wraps a transportImpl object and provides a unified interface to any
// type of transport selected by the user.
type Transport struct {
	Args        *Args
	Impl        Implementation
	implLock    *sync.Mutex
	timeoutLock *sync.Mutex
}

// Open opens the underlying transportImpl transport object.
func (t *Transport) Open() error {
	return t.Impl.Open(t.Args)
}

// Close closes the underlying transportImpl transport object. force option is required for netconf
// as there will almost certainly always be a read in progress that we cannot stop and will block,
// therefore we need a way to bypass the lock.
func (t *Transport) Close(force bool) error {
	if !force {
		t.implLock.Lock()
		defer t.implLock.Unlock()
	}

	return t.Impl.Close()
}

// IsAlive returns true if the underlying transportImpl reports liveness, otherwise false.
func (t *Transport) IsAlive() bool {
	return t.Impl.IsAlive()
}

func (t *Transport) read(n int) ([]byte, error) {
	t.implLock.Lock()
	defer t.implLock.Unlock()

	return t.Impl.Read(n)
}

// Read reads the Transport Args ReadSize bytes from the transportImpl.
func (t *Transport) Read() ([]byte, error) {
	return t.read(t.Args.ReadSize)
}

// ReadN reads n bytes from the transportImpl.
func (t *Transport) ReadN(n int) ([]byte, error) {
	return t.read(n)
}

// Write writes bytes b to the transportImpl.
func (t *Transport) Write(b []byte) error {
	return t.Impl.Write(b)
}

// GetHost is a convenience method to return the Transport Args Host value.
func (t *Transport) GetHost() string {
	return t.Args.Host
}

// GetPort is a convenience method to return the Transport Args Port value.
func (t *Transport) GetPort() int {
	return t.Args.Port
}

// InChannelAuthData returns an instance of InChannelAuthData indicating if in-channel auth is
// supported, and if so, the necessary fields to accomplish that.
func (t *Transport) InChannelAuthData() *InChannelAuthData {
	ti, ok := t.Impl.(InChannelAuthImplementation)
	if !ok {
		return &InChannelAuthData{
			Type: InChannelAuthUnsupported,
		}
	}

	d := &InChannelAuthData{
		Type:                 ti.GetInChannelAuthType(),
		User:                 t.Args.User,
		Password:             t.Args.Password,
		PrivateKeyPassPhrase: "",
	}

	if d.Type == InChannelAuthTelnet {
		return d
	}

	sshTransport, ok := ti.(SSHImplementation)
	if !ok {
		panic(
			"in channel auth requested on a non telnet transport," +
				" and transport does not implement transportImplSSH",
		)
	}

	d.PrivateKeyPassPhrase = sshTransport.GetSSHArgs().PrivateKeyPassPhrase

	return d
}
