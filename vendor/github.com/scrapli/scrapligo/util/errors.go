package util

import "errors"

var (
	// ErrIgnoredOption is the error returned when attempting to apply an option to a struct that
	// is not of the expected type. This error should not be exposed to end users.
	ErrIgnoredOption = errors.New("errIgnoredOption")
	// ErrConnectionError is the error returned for non auth related connection failures typically
	// encountered during *in channel ssh authentication* -- things like host key verification
	// failures and other openssh errors.
	ErrConnectionError = errors.New("errConnectionError")
	// ErrBadOption is returned when a bad value is passed to an option function.
	ErrBadOption = errors.New("errBadOption")
	// ErrTimeoutError is returned for any scrapligo timeout issues, meaning socket, transport or
	// channel (ops) timeouts.
	ErrTimeoutError = errors.New("errTimeoutError")
	// ErrAuthError is returned if any authentication errors are returned.
	ErrAuthError = errors.New("errAuthError")
	// ErrPrivilegeError is returned if there are any issues acquiring a privilege level or a user
	// requests an invalid privilege level etc..
	ErrPrivilegeError = errors.New("errPrivilegeError")
	// ErrFileNotFoundError is returned if a file cannot be found.
	ErrFileNotFoundError = errors.New("errFileNotFoundError")
	// ErrParseError is returned when parsing contents/files fails.
	ErrParseError = errors.New("errParseError")
	// ErrPlatformError is returned for any "platform" issues.
	ErrPlatformError = errors.New("errPlatformError")
	// ErrNetconfError is a blanket error for NETCONF issues that don't fall into another more
	// explicit error.
	ErrNetconfError = errors.New("errNetconfError")
	// ErrOperationError is returned for any "operation" issues -- mostly meaning ops timeouts.
	ErrOperationError = errors.New("errOperationError")
	// ErrNoOp is an error returned when a "no op" event happens -- that is a SendCommands or
	// SendConfigs method is called with an empty command/config slice provided.
	ErrNoOp = errors.New("errNoOp")
)
