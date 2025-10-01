package opoptions

import (
	"regexp"
	"time"

	"github.com/scrapli/scrapligo/driver/netconf"

	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/util"
)

// WithNoStripPrompt disables stripping the prompt out from the read bytes.
func WithNoStripPrompt() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*channel.OperationOptions)

		if ok {
			c.StripPrompt = false

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithEager forces the channel read operation into "eager" mode -- that is, it will no longer read
// inputs off of the channel prior to sending a return, hence "eager".
func WithEager() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*channel.OperationOptions)

		if ok {
			c.Eager = true

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithExactMatchInput makes the channel use the historical default `ReadUntilExplicit` when
// reading until a given input (during a SendX operation). This forces scrapli to look for *exactly*
// the input you have given. This is more explicit (obviously), and probably also *slightly* faster
// because we just simply do a direct `Bytes.Contains` rather than the "fuzzy" matching option when
// this option is false.
//
// With fuzzy matching when reading until the input we don't care if the input comes with extra
// "stuff" in the middle of it -- or if the device splits the input across multiple lines. For
// example, when entering a cert key in nokia srlinux things look something like this:
// ```
//
//	--{ * candidate private private-admin }--[  ]--
//	A:srl# set / system tls server-profile carl key "-----BEGIN RSA PRIVATE KEY-----
//	...SOMEVALUES
//	...SOMEMOREVALUES
//	...-----END RSA PRIVATE KEY-----"
//	--{ * candidate private private-admin }--[  ]--
//	A:srl
//
// ```
//
// The above output would cause the ExactMatchInput style (and historically scrapli by default) to
// fail as we never read the *exact* input. But, remember that the "exact" match is maybe a tick
// more efficient and certainly is more exacting. This option enables the historical/legacy default.
func WithExactMatchInput() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*channel.OperationOptions)

		if ok {
			c.ExactMatchInput = true

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithTimeoutOps modifies the timeout "ops" value, or the timeout for a given operation. This only
// modifies the timeout for the current operation and does not update the actual Channel TimeoutOps
// value permanently.
func WithTimeoutOps(t time.Duration) util.Option {
	return func(o interface{}) error {
		switch oo := o.(type) {
		case *channel.OperationOptions:
			oo.Timeout = t
		case *netconf.OperationOptions:
			oo.Timeout = t
		default:
			return util.ErrIgnoredOption
		}

		return nil
	}
}

// WithCompletePatterns is a slice of regex patterns that, if seen, indicate that the operation is
// complete -- this is used with SendInteractive.
func WithCompletePatterns(p []*regexp.Regexp) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*channel.OperationOptions)

		if ok {
			c.CompletePatterns = p

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithInterimPromptPattern is a slice of regex patterns that are valid prompts during a send X
// operation (either command or config as this is a channel level option). This can be used when
// devices change their prompt to indicate a multiline input.
// For example, when editing `trace-options` on nokia srl devices with scrapli(go) the prompt
// changes to ellipses to indicate you are editing the list still, this looks like this:
//
//	```
//	A:srl# enter candidate private
//	Candidate 'private-admin' is not empty
//	--{ * candidate private private-admin }--[  ]--
//	A:srl#
//	--{ * candidate private private-admin }--[  ]--
//	A:srl# system {
//	--{ * candidate private private-admin }--[ system ]--
//	A:srl# gnmi-server {
//	--{ * candidate private private-admin }--[ system gnmi-server ]--
//	A:srl# admin-state enable
//	--{ * candidate private private-admin }--[ system gnmi-server ]--
//	A:srl# trace-options [
//	...
//	````
//
// Without this option (or modifying the base comms prompt pattern/driver prompt patterns),
// scrapligo does not accept "..." as a prompt and will time out as it cant "find the prompt". This
// option allows you to cope with output like the above without modifying the driver/patterns
// themselves.
func WithInterimPromptPattern(p []*regexp.Regexp) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*channel.OperationOptions)

		if ok {
			c.InterimPromptPatterns = p

			return nil
		}

		return util.ErrIgnoredOption
	}
}
