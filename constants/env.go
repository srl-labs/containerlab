package constants

const (
	// ClabEnvIntfs an env var containing the expected number of interfaces injected into every
	// container.
	ClabEnvIntfs = "CLAB_INTFS"

	ClabEnvNornirPlatformNameSchema = "CLAB_NORNIR_PLATFORM_NAME_SCHEMA"

	// ClabExecInteractiveShellEnv is the env var that can be set on a node (via topology
	// env: block at defaults/kind/group/node level) to override the interactive shell
	// used by `clab exec --interactive`. Example: CLAB_EXEC_INTERACTIVE_SHELL="/bin/bash -l".
	ClabExecInteractiveShellEnv = "CLAB_EXEC_INTERACTIVE_SHELL"
)
