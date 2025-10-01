package options

import (
	"fmt"

	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// WithAuthPrivateKey sets the SSH key path and passphrase to use for SSH key based auth.
func WithAuthPrivateKey(ks, ps string) util.Option {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if !ok {
			return util.ErrIgnoredOption
		}

		a.PrivateKeyPath = ks
		a.PrivateKeyPassPhrase = ps

		return nil
	}
}

// WithAuthNoStrictKey disables strict SSH key checking.
func WithAuthNoStrictKey() util.Option {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if !ok {
			return util.ErrIgnoredOption
		}

		a.StrictKey = false

		return nil
	}
}

// WithSSHConfigFile sets the ssh configuration file to use with SSH connections.
func WithSSHConfigFile(s string) util.Option {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if !ok {
			return util.ErrIgnoredOption
		}

		sshF, err := util.ResolveFilePath(s)
		if err != nil {
			return util.ErrFileNotFoundError
		}

		a.ConfigFile = sshF

		return nil
	}
}

// WithSSHConfigFileSystem attempts to set the ssh configuration file to system default paths --
// this will check ~/.ssh/config first, and if it does not find a config file there, will check
// /etc/ssh/ssh_config. If neither path are resolvable an error is returned.
func WithSSHConfigFileSystem() util.Option {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if !ok {
			return util.ErrIgnoredOption
		}

		var sshF string

		var err error

		sshF, err = util.ResolveFilePath("~/.ssh/config")
		if err == nil {
			a.ConfigFile = sshF

			return nil
		}

		sshF, err = util.ResolveFilePath("/etc/ssh/ssh_config")
		if err == nil {
			a.ConfigFile = sshF

			return nil
		}

		return fmt.Errorf(
			"%w: failed resolving ssh config file",
			util.ErrBadOption,
		)
	}
}

// WithSSHKnownHostsFile sets the ssh known hosts file to use with SSH connections.
func WithSSHKnownHostsFile(s string) util.Option {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if !ok {
			return util.ErrIgnoredOption
		}

		sshF, err := util.ResolveFilePath(s)
		if err != nil {
			return util.ErrFileNotFoundError
		}

		a.KnownHostsFile = sshF

		return nil
	}
}

// WithSSHKnownHostsFileSystem attempts to set the ssh known hosts file to system default paths --
// this will check ~/.ssh/known_hosts first, and if it does not find a config file there, will check
// /etc/ssh/ssh_known_hosts. If neither path are resolvable an error is returned.
func WithSSHKnownHostsFileSystem() util.Option {
	return func(o interface{}) error {
		a, ok := o.(*transport.SSHArgs)

		if !ok {
			return util.ErrIgnoredOption
		}

		var sshF string

		var err error

		sshF, err = util.ResolveFilePath("~/.ssh/known_hosts")
		if err == nil {
			a.KnownHostsFile = sshF

			return nil
		}

		sshF, err = util.ResolveFilePath("/etc/ssh/ssh_known_hosts")
		if err == nil {
			a.KnownHostsFile = sshF

			return nil
		}

		return fmt.Errorf(
			"%w: failed resolving ssh known hosts file",
			util.ErrBadOption,
		)
	}
}
