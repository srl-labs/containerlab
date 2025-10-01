package netconf

import (
	"encoding/xml"
	"errors"
	"regexp"
	"sync"

	"github.com/scrapli/scrapligo/driver/generic"

	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/logging"

	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

const (
	// V1Dot0 is a constant for the NETCONF 1.0 version string.
	V1Dot0      = "1.0"
	v1Dot0Delim = `]]>]]>`
	v1Dot0Cap   = "urn:ietf:params:netconf:base:1.0"
	v1Dot0Caps  = "" +
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" +
		"<hello xmlns=\"urn:ietf:params:xml:ns:netconf:base:1.0\">\n" +
		"     <capabilities>\n" +
		"         <capability>urn:ietf:params:netconf:base:1.0</capability>\n" +
		"     </capabilities>\n" +
		"</hello>]]>]]>"

	// V1Dot1 is a constant for the NETCONF 1.1 version string.
	V1Dot1      = "1.1"
	v1Dot1Delim = `(?m)^##$`
	v1Dot1Cap   = "urn:ietf:params:netconf:base:1.1"
	v1Dot1Caps  = "" +
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" +
		"<hello xmlns=\"urn:ietf:params:xml:ns:netconf:base:1.0\">\n" +
		"     <capabilities>\n" +
		"         <capability>urn:ietf:params:netconf:base:1.1</capability>\n" +
		"     </capabilities>\n" +
		"</hello>]]>]]>"

	helloPattern      = `(?is)(<(\w+:)?hello.*</(\w+:)?hello>)`
	capabilityPattern = `(?i)(?:<(?:\w+:)?capability>)(.*?)(?:</(?:\w+:)?capability>)`

	messageIDPattern      = `(?i)(?:message-id="(\d+)")`
	subscriptionIDPattern = `(?i)<subscription-id.*>(\d+)</subscription-id>`

	subscriptionResultPattern = `(?i)<subscription-result.*>notif-bis:(.+)</subscription-result>`

	// emptyTagPattern matches netconf empty tags to allow
	// forcing of self-closing tags.
	// See https://regex101.com/r/rmsS2E/3.
	emptyTagPattern = `<([^>/]+?)(\s+[^>]+?)?>\s*</([\w-]+)>`

	defaultNamespace = "urn:ietf:params:xml:ns:yang:ietf-netconf-with-defaults"

	xmlHeader = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"

	initialMessageID = 101

	sessionID = `(?i)<session-id>(\d+)</session-id>`
)

type netconfPatterns struct {
	v1Dot0Delim        *regexp.Regexp
	v1Dot1Delim        *regexp.Regexp
	hello              *regexp.Regexp
	capability         *regexp.Regexp
	messageID          *regexp.Regexp
	subscriptionID     *regexp.Regexp
	subscriptionResult *regexp.Regexp
	emptyTags          *regexp.Regexp
	sessionID          *regexp.Regexp
}

var (
	netconfPatternsInstance     *netconfPatterns //nolint:gochecknoglobals
	netconfPatternsInstanceOnce sync.Once        //nolint:gochecknoglobals
)

func getNetconfPatterns() *netconfPatterns {
	netconfPatternsInstanceOnce.Do(func() {
		netconfPatternsInstance = &netconfPatterns{
			v1Dot0Delim:        regexp.MustCompile(v1Dot0Delim),
			v1Dot1Delim:        regexp.MustCompile(v1Dot1Delim),
			hello:              regexp.MustCompile(helloPattern),
			capability:         regexp.MustCompile(capabilityPattern),
			messageID:          regexp.MustCompile(messageIDPattern),
			subscriptionID:     regexp.MustCompile(subscriptionIDPattern),
			subscriptionResult: regexp.MustCompile(subscriptionResultPattern),
			emptyTags:          regexp.MustCompile(emptyTagPattern),
			sessionID:          regexp.MustCompile(sessionID),
		}
	})

	return netconfPatternsInstance
}

func withNetconfConnection(b bool) func(interface{}) error {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if ok {
			a.NetconfConnection = b

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// NewDriver returns an instance of Driver for the provided host with the given options set. Any
// options in the driver/options package may be passed to this function -- those options may be
// applied at the network.Driver, generic.Driver, channel.Channel, or Transport depending on the
// specific option.
func NewDriver(
	host string,
	opts ...util.Option,
) (*Driver, error) {
	opts = append(opts, withNetconfConnection(true))

	// create the generic driver just to yoink the transport and channel out of it, by doing this
	// all the "normal" options get applied, then we just take the parts we care about. we very much
	// do *not* want the "normal" driver here because then users may use things like GetPrompt and
	// the like that would break netconf-y things.
	gd, err := generic.NewDriver(host, opts...)
	if err != nil {
		return nil, err
	}

	d := &Driver{
		TransportType: gd.TransportType,
		Transport:     gd.Transport,
		Channel:       gd.Channel,

		messageID: initialMessageID,

		messages:     map[int][]byte{},
		messagesLock: &sync.Mutex{},

		subscriptions:     map[int][][]byte{},
		subscriptionsLock: &sync.Mutex{},

		errs: make(chan error),
		done: make(chan bool),
	}

	for _, option := range opts {
		err = option(d)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	if d.Logger == nil {
		// set a default logging instance w/ no assigned loggers (a noop basically)
		var l *logging.Instance

		l, err = logging.NewInstance()
		if err != nil {
			return nil, err
		}

		d.Logger = l
	}

	ncPatterns := getNetconfPatterns()

	d.Channel.PromptPattern = ncPatterns.v1Dot0Delim

	return d, nil
}

// Driver embeds generic.Driver and adds "netconf" centric functionality.
type Driver struct {
	Logger *logging.Instance

	TransportType string
	Transport     *transport.Transport

	Channel *channel.Channel

	PreferredVersion string
	SelectedVersion  string

	ForceSelfClosingTags bool
	ExcludeHeader        bool

	serverCapabilities []string
	sessionID          uint64

	messageID int

	messages     map[int][]byte
	messagesLock *sync.Mutex

	subscriptions     map[int][][]byte
	subscriptionsLock *sync.Mutex

	errs chan error
	done chan bool
}

// Open opens the underlying generic.Driver, and by extension the channel.Channel and Transport
// objects. This should be called prior to executing any RPC methods of the Driver.
func (d *Driver) Open() (reterr error) {
	d.Logger.Debugf(
		"opening connection to host '%s' on port '%d'",
		d.Transport.Args.Host,
		d.Transport.Args.Port,
	)

	err := d.Channel.Open()
	if err != nil {
		return err
	}

	defer func() {
		if reterr != nil {
			// don't leave the channel (and more importantly, the transport) open if we are going to
			// return an error
			_ = d.Channel.Close()
		}
	}()

	err = d.processServerCapabilities()
	if err != nil {
		return err
	}

	err = d.determineVersion()
	if err != nil {
		return err
	}

	err = d.sendClientCapabilities()
	if err != nil {
		return err
	}

	go d.read()

	return nil
}

// Close closes the underlying channel.Channel and Transport objects.
func (d *Driver) Close() error {
	d.Logger.Debugf(
		"closing connection to host '%s' on port '%d'",
		d.Transport.Args.Host,
		d.Transport.Args.Port,
	)

	d.done <- true

	err := d.Channel.Close()
	if err != nil {
		return err
	}

	d.Logger.Info("connection closed successfully")

	return nil
}

func (d *Driver) buildPayload(payload interface{}) *message {
	baseElem := &message{
		XMLName:   xml.Name{},
		Namespace: "urn:ietf:params:xml:ns:netconf:base:1.0",
		MessageID: d.messageID,
		Payload:   payload,
	}

	d.messageID++

	return baseElem
}

func (d *Driver) storeMessage(i int, b []byte) {
	d.messagesLock.Lock()
	defer d.messagesLock.Unlock()

	d.messages[i] = b
}

func (d *Driver) getMessage(i int) []byte {
	d.messagesLock.Lock()
	defer d.messagesLock.Unlock()

	data := d.messages[i]

	// no point keeping this in memory -- especially as some messages may be huge! we can also
	// safely delete the key in the map as we should not be getting another message for the same
	// id ever again (unlike with subscriptions).
	delete(d.messages, i)

	return data
}

func (d *Driver) storeSubscriptionMessage(i int, b []byte) {
	d.subscriptionsLock.Lock()
	defer d.subscriptionsLock.Unlock()

	d.subscriptions[i] = append(d.subscriptions[i], b)
}

// GetSubscriptionMessages fetches any messages that have been received for the given subscription
// id i.
func (d *Driver) GetSubscriptionMessages(i int) [][]byte {
	d.subscriptionsLock.Lock()
	defer d.subscriptionsLock.Unlock()

	m := d.subscriptions[i]

	d.subscriptions[i] = nil

	return m
}
