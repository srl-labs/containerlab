package channel

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/scrapli/scrapligo/util"
)

const (
	usernameSeenMax   = 2
	passwordSeenMax   = 2
	passphraseSeenMax = 2
)

type authPatterns struct {
	username   *regexp.Regexp
	password   *regexp.Regexp
	passphrase *regexp.Regexp
}

type sshErrorMessagePatterns struct {
	offeredOptions *regexp.Regexp
	badConfig      *regexp.Regexp
}

var (
	authPatternsInstance     *authPatterns //nolint:gochecknoglobals
	authPatternsInstanceOnce sync.Once     //nolint:gochecknoglobals

	sshErrorMessagePatternsInstance *sshErrorMessagePatterns //nolint:gochecknoglobals
	sshErrorMessagePatternsOnce     sync.Once                //nolint:gochecknoglobals
)

func getAuthPatterns() *authPatterns {
	authPatternsInstanceOnce.Do(func() {
		authPatternsInstance = &authPatterns{
			username:   regexp.MustCompile(`(?im)^(.*username:)|(.*login:)\s?$`),
			password:   regexp.MustCompile(`(?im)(.*@.*)?password:\s?$`),
			passphrase: regexp.MustCompile(`(?i)enter passphrase for key`),
		}
	})

	return authPatternsInstance
}

func getSSHErrorMessagePatterns() *sshErrorMessagePatterns {
	sshErrorMessagePatternsOnce.Do(func() {
		sshErrorMessagePatternsInstance = &sshErrorMessagePatterns{
			offeredOptions: regexp.MustCompile(`(?im)their offer: ([a-z0-9\-,]*)`),
			badConfig:      regexp.MustCompile(`(?im)bad configuration option: ([a-z0-9+=,]*)`),
		}
	})

	return sshErrorMessagePatternsInstance
}

func (c *Channel) authenticateSSH(ctx context.Context, p, pp []byte) *result {
	pCount := 0

	ppCount := 0

	var b []byte

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		nb, err := c.Read()
		if err != nil {
			return &result{nil, err}
		}

		b = append(b, nb...)

		err = c.sshMessageHandler(b)
		if err != nil {
			return &result{nil, err}
		}

		if c.PromptPattern.Match(b) {
			return &result{b, nil}
		}

		if c.PasswordPattern.Match(b) {
			pCount++

			if pCount > passwordSeenMax {
				c.l.Critical("password prompt seen multiple times, assuming authentication failed")

				return &result{
					nil,
					fmt.Errorf(
						"%w: password prompt seen multiple times, assuming authentication failed",
						util.ErrAuthError,
					),
				}
			}

			err = c.WriteAndReturn(p, true)
			if err != nil {
				return &result{nil, err}
			}

			// reset the buffer so we don't re-read things and so we can find the prompt (hopefully)
			b = []byte{}

			continue
		}

		if c.PassphrasePattern.Match(b) {
			ppCount++

			if ppCount > passphraseSeenMax {
				c.l.Critical(
					"private key passphrase prompt seen multiple times," +
						" assuming authentication failed",
				)

				return &result{
					nil,
					fmt.Errorf(
						"%w: private key passphrase prompt seen multiple times,"+
							" assuming authentication failed",
						util.ErrAuthError,
					),
				}
			}

			err = c.WriteAndReturn(pp, true)
			if err != nil {
				return &result{nil, err}
			}

			b = []byte{}
		}
	}
}

// AuthenticateSSH handles "in channel" SSH authentication.
func (c *Channel) AuthenticateSSH(p, pp []byte) ([]byte, error) {
	cr := make(chan *result)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go func() {
		defer close(cr)

		cr <- c.authenticateSSH(ctx, p, pp)
	}()

	t := time.NewTimer(c.TimeoutOps)

	select {
	case r := <-cr:
		return r.b, r.err
	case <-t.C:
		c.l.Critical("channel timeout during in channel ssh authentication")

		return nil, fmt.Errorf(
			"%w: channel timeout during in channel ssh authentication",
			util.ErrTimeoutError,
		)
	}
}

func (c *Channel) authenticateTelnet(ctx context.Context, u, p []byte) *result {
	uCount := 0

	pCount := 0

	var b []byte

	for {
		nb, err := c.ReadUntilAnyPrompt(
			ctx, []*regexp.Regexp{c.PromptPattern, c.UsernamePattern, c.PasswordPattern},
		)
		if err != nil {
			return &result{nil, err}
		}

		b = append(b, nb...)

		if c.PromptPattern.Match(b) {
			return &result{b, nil}
		}

		if c.UsernamePattern.Match(b) {
			b = []byte{}

			uCount++

			if uCount > usernameSeenMax {
				c.l.Critical(
					"username prompt seen multiple times, assuming authentication failed",
				)

				return &result{
					nil,
					fmt.Errorf(
						"%w: username prompt seen multiple times, assuming authentication failed",
						util.ErrAuthError,
					),
				}
			}

			err = c.WriteAndReturn(u, true)
			if err != nil {
				return &result{nil, err}
			}

			continue
		}

		if c.PasswordPattern.Match(b) {
			b = []byte{}

			pCount++

			if pCount > passwordSeenMax {
				c.l.Critical(
					"password prompt seen multiple times, assuming authentication failed",
				)

				return &result{
					nil,
					fmt.Errorf(
						"%w: password prompt seen multiple times, assuming authentication failed",
						util.ErrAuthError,
					),
				}
			}

			err = c.WriteAndReturn(p, true)
			if err != nil {
				return &result{nil, err}
			}
		}
	}
}

// AuthenticateTelnet handles "in channel" telnet authentication.
func (c *Channel) AuthenticateTelnet(u, p []byte) ([]byte, error) {
	cr := make(chan *result)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go func() {
		cr <- c.authenticateTelnet(ctx, u, p)
	}()

	t := time.NewTimer(c.TimeoutOps)

	select {
	case r := <-cr:
		return r.b, r.err
	case <-t.C:
		c.l.Critical("channel timeout during in channel telnet authentication")

		return nil, fmt.Errorf(
			"%w: channel timeout during in channel telnet authentication",
			util.ErrTimeoutError,
		)
	}
}

func (c *Channel) sshMessageHandler(b []byte) error { //nolint:gocyclo
	var errorMessage string

	normalizedB := bytes.ToLower(b)

	switch {
	case bytes.Contains(normalizedB, []byte("host key verification failed")):
		errorMessage = "host key verification failed"
	case bytes.Contains(normalizedB, []byte("operation timed out")) ||
		bytes.Contains(normalizedB, []byte("connection timed out")):
		errorMessage = "timed out connecting to host"
	case bytes.Contains(normalizedB, []byte("no route to host")):
		errorMessage = "no route to host"
	case bytes.Contains(normalizedB, []byte("no matching")):
		switch {
		case bytes.Contains(normalizedB, []byte("no matching host key")):
			errorMessage = "no matching host key found for host"
		case bytes.Contains(normalizedB, []byte("no matching key exchange")):
			errorMessage = "no matching key exchange found for host"
		case bytes.Contains(normalizedB, []byte("no matching cipher")):
			errorMessage = "no matching cipher found for host"
		}

		patterns := getSSHErrorMessagePatterns()

		theirOffer := patterns.offeredOptions.FindSubmatch(b)
		if len(theirOffer) > 0 {
			errorMessage += fmt.Sprintf(", their offer: %s", theirOffer[0])
		}
	case bytes.Contains(normalizedB, []byte("bad configuration")):
		errorMessage = "bad ssh configuration option(s) for host"

		patterns := getSSHErrorMessagePatterns()

		badOption := patterns.offeredOptions.FindSubmatch(b)
		if len(badOption) > 0 {
			errorMessage += fmt.Sprintf(", bad configuration option: %s", badOption[0])
		}
	case bytes.Contains(normalizedB, []byte("warning: unprotected private key file")):
		errorMessage = "permissions for private key are too open"
	case bytes.Contains(normalizedB, []byte("could not resolve hostname")):
		errorMessage = "could not resolve hostname"
	case bytes.Contains(normalizedB, []byte("permission denied")):
		errorMessage = "permission denied"
	}

	if errorMessage != "" {
		return fmt.Errorf(
			"%w: encountered error output during in channel ssh authentication, error: '%s'",
			util.ErrConnectionError,
			errorMessage,
		)
	}

	return nil
}
