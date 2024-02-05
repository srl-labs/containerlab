package types

const (
	// env var containing the expected number of interfaces injected into every container
	// without added management interface.
	// two different env vars are injected to support the smooth phasing out
	// of the old images built with vrnetlab that still use CLAB_INTFS
	CLAB_ENV_INTFS = "CLAB_INTFS"
	// env var containing the expected number of interfaces injected into every container
	// with added management interface.
	// this env var is used with newer vrnetlab images (>=0.15.0) that support
	// network-mode:none feature and rely on this env var instead of CLAB_INTFS
	CLAB_ENV_INTFS_WITH_MGMT = "CLAB_INTFS_WITH_MGMT"
)
