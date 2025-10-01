package platform

import (
	"regexp"
	"time"

	"github.com/scrapli/scrapligo/driver/options"

	"github.com/scrapli/scrapligo/util"
)

const (
	port = "port"

	authBypass    = "auth-bypass"
	authStrictKey = "auth-strict-key"

	promptPattern     = "prompt-pattern"
	usernamePattern   = "username-pattern"
	passwordPattern   = "password-pattern"
	passphrasePattern = "passphrase-pattern"

	returnChar = "return-char"

	// read delay in seconds for channel read loop.
	readDelay = "read-delay"

	// timeouts in seconds.
	timeoutOps = "timeout-ops"

	transportType = "transport-type"
	// read size for transport read chunk.
	transportReadSize  = "read-size"
	transportPtyHeight = "transport-pty-height"
	transportPtyWidth  = "transport-pty-width"

	transportSystemOpenArgs = "transport-system-open-args"
)

type optionDefinition struct {
	Option string      `json:"option" yaml:"option"`
	Value  interface{} `json:"value"  yaml:"value"`
}

type optionDefinitions []*optionDefinition

func (o *optionDefinitions) asOptions() []util.Option { //nolint: gocyclo,gocognit,funlen
	opts := make([]util.Option, len(*o))

	for i, opt := range *o {
		switch opt.Option {
		case port:
			intVal, ok := opt.Value.(int)
			if !ok {
				panic("option port value must be an int")
			}

			opts[i] = options.WithPort(intVal)
		case authBypass:
			opts[i] = options.WithAuthBypass()
		case authStrictKey:
			opts[i] = options.WithAuthNoStrictKey()
		case promptPattern:
			strVal, ok := opt.Value.(string)
			if !ok {
				panic("option promptPattern value must be a string")
			}

			opts[i] = options.WithPromptPattern(regexp.MustCompile(strVal))
		case usernamePattern:
			strVal, ok := opt.Value.(string)
			if !ok {
				panic("option usernamePattern value must be a string")
			}

			opts[i] = options.WithUsernamePattern(regexp.MustCompile(strVal))
		case passwordPattern:
			strVal, ok := opt.Value.(string)
			if !ok {
				panic("option passwordPattern value must be a string")
			}

			opts[i] = options.WithPasswordPattern(regexp.MustCompile(strVal))
		case passphrasePattern:
			strVal, ok := opt.Value.(string)
			if !ok {
				panic("option passphrasePattern value must be a string")
			}

			opts[i] = options.WithPassphrasePattern(regexp.MustCompile(strVal))
		case returnChar:
			strVal, ok := opt.Value.(string)
			if !ok {
				panic("option returnChar value must be a string")
			}

			opts[i] = options.WithReturnChar(strVal)
		case readDelay:
			floatVal, ok := opt.Value.(float64)
			if !ok {
				panic("option readDelay value must be a float")
			}

			opts[i] = options.WithReadDelay(
				time.Duration(floatVal * float64(time.Second)),
			)
		case timeoutOps:
			floatVal, ok := opt.Value.(float64)
			if !ok {
				panic("option timeoutOps value must be a float")
			}

			opts[i] = options.WithTimeoutOps(
				time.Duration(floatVal * float64(time.Second)),
			)
		case transportType:
			strVal, ok := opt.Value.(string)
			if !ok {
				panic("option transportType value must be a string")
			}

			opts[i] = options.WithTransportType(strVal)
		case transportReadSize:
			intVal, ok := opt.Value.(int)
			if !ok {
				panic("option transportReadSize value must be an int")
			}

			opts[i] = options.WithTransportReadSize(intVal)
		case transportPtyHeight:
			intVal, ok := opt.Value.(int)
			if !ok {
				panic("option transportPtyHeight value must be an int")
			}

			opts[i] = options.WithTermHeight(intVal)
		case transportPtyWidth:
			intVal, ok := opt.Value.(int)
			if !ok {
				panic("option transportPtyWidth value must be an int")
			}

			opts[i] = options.WithTermWidth(intVal)
		case transportSystemOpenArgs:
			strSliceVal, ok := opt.Value.([]string)
			if !ok {
				panic("option transportSystemOpenArgs value must be an array of strings")
			}

			opts[i] = options.WithSystemTransportOpenArgs(strSliceVal)
		}
	}

	return opts
}
