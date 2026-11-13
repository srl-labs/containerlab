package constants

const (
	// ClabEnvIntfs an env var containing the expected number of interfaces injected into every
	// container.
	ClabEnvIntfs = "CLAB_INTFS"

	// ClabEnvWaitEth0 when set to "1" makes if-wait.sh also wait for the eth0
	// netdev before init, used when eth0 is wired as a link rather than provided
	// by the container runtime.
	ClabEnvWaitEth0 = "CLAB_WAIT_ETH0"

	ClabEnvNornirPlatformNameSchema = "CLAB_NORNIR_PLATFORM_NAME_SCHEMA"
)
