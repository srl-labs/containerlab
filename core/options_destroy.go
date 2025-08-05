package core

// DestroyOption is a type used for functional options for the Clab Destroy method.
type DestroyOption func(o *DestroyOptions)

// DestroyOptions represents the options for destroying lab(s).
type DestroyOptions struct {
	maxWorkers     uint
	keepMgmtNet    bool
	graceful       bool
	all            bool
	terminalPrompt bool
	cleanup        bool
	nodeFilter     []string
}

// NewDestroyOptions returns a new destroy options object.
func NewDestroyOptions() *DestroyOptions {
	return &DestroyOptions{}
}

// WithDestroyMaxWorkers sets max workers for destroy events.
func WithDestroyMaxWorkers(
	i uint,
) DestroyOption {
	return func(o *DestroyOptions) {
		o.maxWorkers = i
	}
}

// WithDestroyKeepMgmtNet retains the mgmt network on destroy.
func WithDestroyKeepMgmtNet() DestroyOption {
	return func(o *DestroyOptions) {
		o.keepMgmtNet = true
	}
}

// WithDestroyGraceful attempts to stop containers before destroying them.
func WithDestroyGraceful() DestroyOption {
	return func(o *DestroyOptions) {
		o.graceful = true
	}
}

// WithDestroyAll informs the destroy method to destroy all labs.
func WithDestroyAll() DestroyOption {
	return func(o *DestroyOptions) {
		o.all = true
	}
}

// WithDestroyTerminalPrompt asks the destroy method to prompt for deletion when using the all
// option.
func WithDestroyTerminalPrompt() DestroyOption {
	return func(o *DestroyOptions) {
		o.terminalPrompt = true
	}
}

// WithDestroyCleanup informs the destroy method to also try to cleanup directories.
func WithDestroyCleanup() DestroyOption {
	return func(o *DestroyOptions) {
		o.cleanup = true
	}
}

// WithDestroyNodeFilter accepts a normal node-filter to be used with the Destroy operation.
func WithDestroyNodeFilter(ss []string) DestroyOption {
	return func(o *DestroyOptions) {
		o.nodeFilter = ss
	}
}
